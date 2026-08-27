package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWithoutExpectedTitle(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "src.txt")
	output := filepath.Join(dir, "hagezi-normal.txt")

	src := strings.Join([]string{
		"# Title: HaGeZi's Pro DNS Blocklist",
		"example.com",
		"ads.example.net",
		"",
	}, "\n")

	if err := os.WriteFile(input, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config{
		input:      input,
		output:     output,
		sourceURL:  "https://raw.githubusercontent.com/hagezi/dns-blocklists/main/wildcard/multi-onlydomains.txt",
		minEntries: 2,
		maxEntries: 10,
	}

	if err := run(cfg); err != nil {
		t.Fatalf("run without expected-title: %v", err)
	}

	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)

	for _, want := range []string{
		"# Source title: HaGeZi's Pro DNS Blocklist",
		"DOMAIN-SUFFIX,example.com",
		"DOMAIN-SUFFIX,ads.example.net",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestRunConvertsDomains(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "src.txt")
	output := filepath.Join(dir, "hagezi-pro.txt")
	yamlOutput := filepath.Join(dir, "hagezi-pro.yaml")

	src := strings.Join([]string{
		"# Title: HaGeZi's Pro DNS Blocklist",
		"example.com",
		"ads.example.net",
		"google.com",
		"",
	}, "\n")

	if err := os.WriteFile(input, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config{
		input:      input,
		output:     output,
		yamlOutput: yamlOutput,
		minEntries: 3,
		maxEntries: 10,
	}

	if err := run(cfg); err != nil {
		t.Fatalf("run: %v", err)
	}

	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(raw), "DOMAIN-SUFFIX,"); count != 3 {
		t.Fatalf("rule count = %d, want 3", count)
	}

	yaml, err := os.ReadFile(yamlOutput)
	if err != nil {
		t.Fatal(err)
	}
	wantYAML := "payload:\n" +
		"  - DOMAIN-SUFFIX,example.com\n" +
		"  - DOMAIN-SUFFIX,ads.example.net\n" +
		"  - DOMAIN-SUFFIX,google.com\n"
	if string(yaml) != wantYAML {
		t.Fatalf("YAML output =\n%s\nwant =\n%s", yaml, wantYAML)
	}
}

func TestRunAddsSha256(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "src.txt")
	output := filepath.Join(dir, "hagezi-light.txt")

	if err := os.WriteFile(input, []byte("# Title: t\nlight.example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config{
		input:      input,
		output:     output,
		minEntries: 1,
		maxEntries: 10,
	}

	if err := run(cfg); err != nil {
		t.Fatalf("run: %v", err)
	}

	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "# Source SHA256: ") {
		t.Fatalf("output missing sha256 line:\n%s", raw)
	}
}

func TestRunRejectsNonDomainLine(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "src.txt")
	output := filepath.Join(dir, "hagezi-pro.txt")

	// v2fly 风格语法行（@属性 / keyword: / regexp: / include:）
	// 在上游为纯域名黑名单时不支持，必须 fail loud 而非静默丢弃。
	src := "example.com\nregexp:^ads\\.example\\.com$"

	if err := os.WriteFile(input, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config{
		input:      input,
		output:     output,
		minEntries: 1,
		maxEntries: 10,
	}

	if err := run(cfg); err == nil {
		t.Fatal("expected error for non-domain line")
	}
}
