package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var domainRE = regexp.MustCompile(`^[a-z0-9_-]+(?:\.[a-z0-9_-]+)*$`)

type config struct {
	input         string
	output        string
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

	flag.StringVar(&cfg.input, "input", "", "path to an upstream Surge ruleset")
	flag.StringVar(&cfg.output, "output", "", "path to generated .yaml ruleset")
	flag.StringVar(&cfg.expectedTitle, "expected-title", "", "expected '# Title:' value; empty disables title validation")
	flag.IntVar(&cfg.minEntries, "min", 1, "minimum accepted number of rules")
	flag.IntVar(&cfg.maxEntries, "max", 2_000_000, "maximum accepted number of rules")
	flag.Float64Var(&cfg.maxChange, "max-change", 0.35, "maximum relative rule-count change vs existing output; 0 disables")

	flag.Parse()

	return cfg
}

type sourceData struct {
	title string
	rules []string
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

	if len(src.rules) < cfg.minEntries {
		return fmt.Errorf(
			"suspiciously small list: %d entries; minimum is %d",
			len(src.rules),
			cfg.minEntries,
		)
	}
	if len(src.rules) > cfg.maxEntries {
		return fmt.Errorf(
			"suspiciously large list: %d entries; maximum is %d",
			len(src.rules),
			cfg.maxEntries,
		)
	}

	if err := checkChange(cfg.output, len(src.rules), cfg.maxChange); err != nil {
		return err
	}

	if err := writeYAML(cfg.output, src.rules); err != nil {
		return err
	}

	fmt.Fprintf(
		os.Stderr,
		"converted %d rules: %s -> %s\n",
		len(src.rules),
		cfg.input,
		cfg.output,
	)

	return nil
}

// readSource 读取 Surge 域名规则：
//   - 空行与 # 开头的行忽略，# Title: 行提取为标题；
//   - 其余行必须是 DOMAIN 或 DOMAIN-SUFFIX 规则；
//   - 重复规则报错，防止上游变化时静默产出错误 YAML。
func readSource(path string) (sourceData, error) {
	f, err := os.Open(path)
	if err != nil {
		return sourceData{}, fmt.Errorf("open input: %w", err)
	}
	defer f.Close()

	var out sourceData
	seen := make(map[string]struct{})

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

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

		parts := strings.Split(line, ",")
		if len(parts) != 2 || (parts[0] != "DOMAIN" && parts[0] != "DOMAIN-SUFFIX") {
			return sourceData{}, fmt.Errorf("invalid Surge rule %q", line)
		}

		domain := parts[1]
		if domain != strings.ToLower(domain) {
			return sourceData{}, fmt.Errorf("domain is not lowercase: %q", domain)
		}
		if err := validateDomain(domain); err != nil {
			return sourceData{}, fmt.Errorf("invalid domain %q: %w", domain, err)
		}
		if _, exists := seen[line]; exists {
			return sourceData{}, fmt.Errorf("duplicate rule: %q", line)
		}

		seen[line] = struct{}{}
		out.rules = append(out.rules, line)
	}

	if err := scanner.Err(); err != nil {
		return sourceData{}, fmt.Errorf("read input: %w", err)
	}

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

// countYAMLRules 统计生成的 YAML 中的有效规则行。
func countYAMLRules(path string) (count int, ok bool, err error) {
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
		if strings.HasPrefix(line, "- ") {
			count++
		}
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
	oldCount, ok, err := countYAMLRules(oldPath)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	change := float64(newCount-oldCount) / float64(oldCount)
	if change < 0 {
		change = -change
	}
	if change > limit {
		return fmt.Errorf(
			"entry-count change too large: old=%d new=%d change=%.2f%% limit=%.2f%%",
			oldCount,
			newCount,
			change*100,
			limit*100,
		)
	}
	return nil
}

// writeYAML 原子写 Mihomo YAML ruleset。
func writeYAML(path string, rules []string) error {
	var b strings.Builder
	b.WriteString("payload:\n")
	for _, rule := range rules {
		b.WriteString("  - ")
		b.WriteString(rule)
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
