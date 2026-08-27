package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var domainRE = regexp.MustCompile(`^[a-z0-9_-]+(?:\.[a-z0-9_-]+)*$`)

type config struct {
	input         string
	output        string
	yamlOutput    string
	sourceURL     string
	expectedTitle string
	minEntries    int
	maxEntries    int
	maxChange     float64
}

func main() {
	cfg := parseFlags()

	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() config {
	var cfg config

	flag.StringVar(&cfg.input, "input", "", "path to an upstream domain list text file")
	flag.StringVar(&cfg.output, "output", "", "path to generated .txt ruleset")
	flag.StringVar(&cfg.yamlOutput, "yaml-output", "", "path to generated .yaml ruleset; empty disables YAML output")
	flag.StringVar(&cfg.expectedTitle, "expected-title", "", "expected '# Title:' value; empty disables title validation")
	flag.StringVar(&cfg.sourceURL, "source-url", "", "upstream source URL recorded in generated files")
	flag.IntVar(&cfg.minEntries, "min", 1, "minimum accepted number of domains")
	flag.IntVar(&cfg.maxEntries, "max", 2_000_000, "maximum accepted number of domains")
	flag.Float64Var(&cfg.maxChange, "max-change", 0.35, "maximum relative entry-count change vs existing output; 0 disables")

	flag.Parse()

	return cfg
}

// sourceData 是一次读取上游文件的结果。
type sourceData struct {
	title   string
	domains []string
	sha256  string
}

func run(cfg config) error {
	if cfg.input == "" {
		return fmt.Errorf("-input is required")
	}
	if cfg.output == "" {
		return fmt.Errorf("-output is required")
	}
	if cfg.minEntries < 0 || cfg.maxEntries < cfg.minEntries {
		return fmt.Errorf("invalid min/max entry bounds")
	}
	if cfg.maxChange < 0 {
		return fmt.Errorf("-max-change must be >= 0")
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

	if err := checkChange(cfg.output, len(src.domains), cfg.maxChange); err != nil {
		return err
	}

	if err := writeList(cfg.output, cfg.sourceURL, src); err != nil {
		return err
	}
	if cfg.yamlOutput != "" {
		if err := writeYAML(cfg.yamlOutput, src.domains); err != nil {
			return err
		}
	}

	fmt.Fprintf(
		os.Stderr,
		"converted %d domains: %s -> %s (sha256=%s)\n",
		len(src.domains),
		cfg.input,
		cfg.output,
		src.sha256,
	)
	if cfg.yamlOutput != "" {
		fmt.Fprintf(os.Stderr, "generated YAML: %s\n", cfg.yamlOutput)
	}

	return nil
}

// readSource 读取上游纯域名列表：
//   - 空行与 # 开头的行忽略，# Title: 行提取为标题；
//   - 其余行必须是合法小写域名；
//   - 重复域名报错（fail loud，防止上游变化时静默产出错误规则）。
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
		return fmt.Errorf("empty")
	}
	if len(domain) > 253 {
		return fmt.Errorf("longer than 253 bytes")
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return fmt.Errorf("leading or trailing dot")
	}
	if strings.Contains(domain, "..") {
		return fmt.Errorf("contains consecutive dots")
	}
	if !domainRE.MatchString(domain) {
		return fmt.Errorf("contains characters outside [a-z0-9_.-]")
	}

	for _, label := range strings.Split(domain, ".") {
		if len(label) == 0 || len(label) > 63 {
			return fmt.Errorf("invalid label length %d", len(label))
		}
	}

	return nil
}

// countListLines 统计 .txt 输出中的有效规则行（非注释）。
func countListLines(path string) (count int, ok bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		return 0, false, err
	}
	if count == 0 {
		return 0, false, nil
	}
	return count, true, nil
}

func checkChange(oldPath string, newCount int, limit float64) error {
	if limit <= 0 {
		return nil
	}
	oldCount, ok, err := countListLines(oldPath)
	if err != nil {
		return err
	}
	if !ok {
		return nil // 首次生成，无既有输出可比
	}

	change := float64(newCount-oldCount) / float64(oldCount)
	if change < 0 {
		change = -change
	}
	if change > limit {
		return fmt.Errorf(
			"entry-count change too large: old=%d new=%d change=%.2f%% limit=%.2f%%",
			oldCount, newCount, change*100, limit*100,
		)
	}
	return nil
}

// writeList 原子写 .txt：注释头 + 每行一条 DOMAIN-SUFFIX 规则。
// HaGeZi 为全屏蔽黑名单，域与其所有子域一律屏蔽，等价于 DOMAIN-SUFFIX 语义。
func writeList(path, sourceURL string, src sourceData) error {
	var b strings.Builder

	b.WriteString("# Generated by the domain list converter. Do not edit by hand.\n")
	if sourceURL != "" {
		b.WriteString("# Source: " + sourceURL + "\n")
	}
	if src.title != "" {
		b.WriteString("# Source title: " + src.title + "\n")
	}
	b.WriteString("# Source SHA256: " + src.sha256 + "\n")
	b.WriteString(fmt.Sprintf("# Entries: %d\n", len(src.domains)))

	for _, domain := range src.domains {
		b.WriteString("DOMAIN-SUFFIX," + domain + "\n")
	}

	if err := atomicWrite(path, []byte(b.String())); err != nil {
		return err
	}
	return nil
}

// writeYAML 原子写 Mihomo YAML ruleset。
func writeYAML(path string, domains []string) error {
	var b strings.Builder
	b.WriteString("payload:\n")
	for _, domain := range domains {
		b.WriteString("  - DOMAIN-SUFFIX,")
		b.WriteString(domain)
		b.WriteByte('\n')
	}

	return atomicWrite(path, []byte(b.String()))
}

// atomicWrite 以 tmp + rename 原子写文件。
func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create tmp output: %w", err)
	}

	w := bufio.NewWriter(f)
	if _, err := w.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("write tmp output: %w", err)
	}
	if err := w.Flush(); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("flush tmp output: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("sync tmp output: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("close tmp output: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("replace output: %w", err)
	}

	return nil
}
