package domainlist

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var domainRE = regexp.MustCompile(`^[a-z0-9_-]+(?:\.[a-z0-9_-]+)*$`)

type Config struct {
	input         string
	output        string
	expectedTitle string
	sourceURL     string
	canaries      string
	minEntries    int
	maxEntries    int
	maxChange     float64
}

type sourceData struct {
	title   string
	domains []string
	sha256  string
}

func Main() {
	cfg := parseFlags()

	if err := Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() Config {
	var cfg Config

	flag.StringVar(&cfg.input, "input", "", "path to an upstream domain list text file")
	flag.StringVar(&cfg.output, "output", "", "path to generated Mihomo YAML")
	flag.StringVar(&cfg.expectedTitle, "expected-title", "", "expected '# Title:' value; empty disables title validation")
	flag.StringVar(&cfg.sourceURL, "source-url", "", "upstream source URL recorded in generated YAML")
	flag.StringVar(
		&cfg.canaries,
		"canaries",
		"apple.com,microsoft.com,google.com,facebook.com,amazon.com,youtube.com,reddit.com,github.com",
		"comma-separated root domains that must never appear",
	)
	flag.IntVar(&cfg.minEntries, "min", 1, "minimum accepted number of domains")
	flag.IntVar(&cfg.maxEntries, "max", 2_000_000, "maximum accepted number of domains")
	flag.Float64Var(
		&cfg.maxChange,
		"max-change",
		0.35,
		"maximum relative entry-count change versus existing output; 0 disables",
	)

	flag.Parse()

	return cfg
}

func Run(cfg Config) error {
	if cfg.input == "" {
		return errors.New("-input is required")
	}
	if cfg.output == "" {
		return errors.New("-output is required")
	}
	if cfg.minEntries < 0 || cfg.maxEntries < cfg.minEntries {
		return errors.New("invalid min/max entry bounds")
	}
	if cfg.maxChange < 0 {
		return errors.New("-max-change must be >= 0")
	}

	src, err := readSource(cfg.input)
	if err != nil {
		return err
	}

	if cfg.expectedTitle != "" && src.title != cfg.expectedTitle {
		return fmt.Errorf(
			"unexpected source title: got %q, want %q",
			src.title,
			cfg.expectedTitle,
		)
	}

	if len(src.domains) < cfg.minEntries {
		return fmt.Errorf(
			"suspiciously small list: %d entries; minimum is %d",
			len(src.domains),
			cfg.minEntries,
		)
	}
	if len(src.domains) > cfg.maxEntries {
		return fmt.Errorf(
			"suspiciously large list: %d entries; maximum is %d",
			len(src.domains),
			cfg.maxEntries,
		)
	}

	if err := checkCanaries(src.domains, cfg.canaries); err != nil {
		return err
	}

	if cfg.maxChange > 0 {
		if oldCount, ok, err := existingOutputCount(cfg.output); err != nil {
			return err
		} else if ok {
			change := relativeChange(oldCount, len(src.domains))
			if change > cfg.maxChange {
				return fmt.Errorf(
					"entry-count change too large: old=%d new=%d change=%.2f%% limit=%.2f%%",
					oldCount,
					len(src.domains),
					change*100,
					cfg.maxChange*100,
				)
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(cfg.output), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	tmp := cfg.output + ".tmp"
	if err := writeOutput(tmp, cfg.sourceURL, src); err != nil {
		return err
	}

	if err := os.Rename(tmp, cfg.output); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace output: %w", err)
	}

	fmt.Fprintf(
		os.Stderr,
		"converted %d domains: %s -> %s (sha256=%s)\n",
		len(src.domains),
		cfg.input,
		cfg.output,
		src.sha256,
	)

	return nil
}

func readSource(path string) (sourceData, error) {
	f, err := os.Open(path)
	if err != nil {
		return sourceData{}, fmt.Errorf("open input: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	tee := io.TeeReader(f, h)

	scanner := bufio.NewScanner(tee)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var out sourceData
	seen := make(map[string]struct{})

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#") {
			const prefix = "# Title:"
			if strings.HasPrefix(line, prefix) {
				out.title = strings.TrimSpace(strings.TrimPrefix(line, prefix))
			}
			continue
		}

		domain := strings.ToLower(line)

		if domain != line {
			return sourceData{}, fmt.Errorf("domain is not lowercase: %q", line)
		}
		if err := validateDomain(domain); err != nil {
			return sourceData{}, fmt.Errorf("invalid domain %q: %w", domain, err)
		}
		if _, exists := seen[domain]; exists {
			return sourceData{}, fmt.Errorf("duplicate domain: %q", domain)
		}

		seen[domain] = struct{}{}
		out.domains = append(out.domains, domain)
	}

	if err := scanner.Err(); err != nil {
		return sourceData{}, fmt.Errorf("read input: %w", err)
	}

	out.sha256 = hex.EncodeToString(h.Sum(nil))

	return out, nil
}

func validateDomain(domain string) error {
	if len(domain) == 0 {
		return errors.New("empty")
	}
	if len(domain) > 253 {
		return errors.New("longer than 253 bytes")
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return errors.New("leading or trailing dot")
	}
	if strings.Contains(domain, "..") {
		return errors.New("contains consecutive dots")
	}
	if !domainRE.MatchString(domain) {
		return errors.New("contains characters outside [a-z0-9_.-]")
	}

	for _, label := range strings.Split(domain, ".") {
		if len(label) == 0 || len(label) > 63 {
			return fmt.Errorf("invalid label length %d", len(label))
		}
	}

	return nil
}

func checkCanaries(domains []string, csv string) error {
	if strings.TrimSpace(csv) == "" {
		return nil
	}

	blocked := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		blocked[domain] = struct{}{}
	}

	for _, raw := range strings.Split(csv, ",") {
		canary := strings.TrimSpace(strings.ToLower(raw))
		if canary == "" {
			continue
		}
		if _, exists := blocked[canary]; exists {
			return fmt.Errorf("canary root domain unexpectedly present: %s", canary)
		}
	}

	return nil
}

func existingOutputCount(path string) (int, bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("open previous output: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	inPayload := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "payload:" {
			inPayload = true
			continue
		}
		if !inPayload {
			continue
		}
		if strings.HasPrefix(line, "- ") {
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, false, fmt.Errorf("read previous output: %w", err)
	}
	if count == 0 {
		return 0, false, nil
	}

	return count, true, nil
}

func relativeChange(oldCount, newCount int) float64 {
	if oldCount <= 0 {
		return 0
	}

	delta := newCount - oldCount
	if delta < 0 {
		delta = -delta
	}

	return float64(delta) / float64(oldCount)
}

func writeOutput(path, sourceURL string, src sourceData) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}

	w := bufio.NewWriter(f)

	writeErr := func() error {
		defer f.Close()

		if _, err := fmt.Fprintln(w, "# Generated by the domain list converter. Do not edit by hand."); err != nil {
			return err
		}
		if sourceURL != "" {
			if _, err := fmt.Fprintf(w, "# Source: %s\n", sourceURL); err != nil {
				return err
			}
		}
		if src.title != "" {
			if _, err := fmt.Fprintf(w, "# Source title: %s\n", src.title); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "# Source SHA256: %s\n", src.sha256); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "# Entries: %d\n", len(src.domains)); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "payload:"); err != nil {
			return err
		}

		for _, domain := range src.domains {
			if _, err := fmt.Fprintf(w, "  - '+.%s'\n", domain); err != nil {
				return err
			}
		}

		if err := w.Flush(); err != nil {
			return err
		}
		if err := f.Sync(); err != nil {
			return err
		}

		return nil
	}()

	if writeErr != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return fmt.Errorf("write output: %w", writeErr)
	}

	return nil
}

// parsePayloadCount is used by tests and intentionally kept tiny.
func ParsePayloadCount(raw string) (int, error) {
	count := 0
	inPayload := false

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "payload:" {
			inPayload = true
			continue
		}
		if !inPayload {
			continue
		}
		if strings.HasPrefix(line, "- ") {
			count++
		}
	}

	if !inPayload {
		return 0, errors.New("payload section not found")
	}

	return count, nil
}
