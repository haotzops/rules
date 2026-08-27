package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunConvertsSurgeRulesToYAML(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "small.txt")
	output := filepath.Join(dir, "small.yaml")

	src := strings.Join([]string{
		"# Version: test",
		"# Title: oisd small",
		"# Syntax: Shadow Rocket",
		"",
		"DOMAIN,fhnfile.oss-cn-shenzhen.aliyuncs.com",
		"DOMAIN-SUFFIX,115.com",
		"DOMAIN-SUFFIX,115cdn.com",
		"DOMAIN-SUFFIX,google.com",
		"",
	}, "\n")

	if err := os.WriteFile(input, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config{
		input:         input,
		output:        output,
		expectedTitle: "oisd small",
		minEntries:    4,
		maxEntries:    10,
	}

	if err := run(cfg); err != nil {
		t.Fatalf("run: %v", err)
	}

	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	want := "payload:\n" +
		"  - DOMAIN,fhnfile.oss-cn-shenzhen.aliyuncs.com\n" +
		"  - DOMAIN-SUFFIX,115.com\n" +
		"  - DOMAIN-SUFFIX,115cdn.com\n" +
		"  - DOMAIN-SUFFIX,google.com\n"
	if string(raw) != want {
		t.Fatalf("output =\n%s\nwant =\n%s", raw, want)
	}
}

func TestRunRejectsDuplicateRule(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "small.txt")
	output := filepath.Join(dir, "small.yaml")

	if err := os.WriteFile(input, []byte("DOMAIN-SUFFIX,example.com\nDOMAIN-SUFFIX,example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config{input: input, output: output, minEntries: 1, maxEntries: 10}
	if err := run(cfg); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestRunRejectsNonSurgeRule(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "small.txt")
	output := filepath.Join(dir, "small.yaml")

	if err := os.WriteFile(input, []byte("example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config{input: input, output: output, minEntries: 1, maxEntries: 10}
	if err := run(cfg); err == nil || !strings.Contains(err.Error(), "invalid Surge rule") {
		t.Fatalf("expected Surge rule error, got %v", err)
	}
}

func TestRunChangeGuard(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "small.txt")
	output := filepath.Join(dir, "small.yaml")

	if err := os.WriteFile(input, []byte("DOMAIN-SUFFIX,example.com\nDOMAIN-SUFFIX,ads.example.net\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("payload:\n"+strings.Repeat("  - DOMAIN-SUFFIX,x.example.com\n", 9)), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config{
		input:      input,
		output:     output,
		minEntries: 1,
		maxEntries: 10,
		maxChange:  0.5,
	}

	if err := run(cfg); err == nil {
		t.Fatal("expected change error")
	}
}
