package domainlist

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunConvertsDomains(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "small.txt")
	output := filepath.Join(dir, "small.yaml")

	src := strings.Join([]string{
		"# Version: test",
		"# Title: oisd small",
		"# Syntax: Domains (wildcards) without *.",
		"",
		"example.com",
		"ads.example.net",
		"",
	}, "\n")

	if err := os.WriteFile(input, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Run(Config{
		input:         input,
		output:        output,
		expectedTitle: "oisd small",
		sourceURL:     "https://small.oisd.nl/domainswild2",
		canaries:      "apple.com,google.com",
		minEntries:    2,
		maxEntries:    10,
		maxChange:     0,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}

	got := string(raw)
	for _, want := range []string{
		"# Source: https://small.oisd.nl/domainswild2",
		"# Source title: oisd small",
		"# Entries: 2",
		"  - '+.example.com'",
		"  - '+.ads.example.net'",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}

	count, err := ParsePayloadCount(got)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("payload count = %d, want 2", count)
	}
}

func TestRunAllowsSourceWithoutTitle(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "list.txt")
	output := filepath.Join(dir, "list.yaml")

	if err := os.WriteFile(input, []byte("example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Run(Config{
		input:      input,
		output:     output,
		canaries:   "google.com",
		minEntries: 1,
		maxEntries: 10,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "Source title:") {
		t.Fatalf("output unexpectedly contains source title:\n%s", raw)
	}
}

func TestRunRejectsCanary(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "small.txt")
	output := filepath.Join(dir, "small.yaml")

	src := "# Title: oisd small\nexample.com\ngoogle.com\n"
	if err := os.WriteFile(input, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Run(Config{
		input:         input,
		output:        output,
		expectedTitle: "oisd small",
		canaries:      "google.com",
		minEntries:    1,
		maxEntries:    10,
	})
	if err == nil || !strings.Contains(err.Error(), "canary") {
		t.Fatalf("expected canary error, got %v", err)
	}
}

func TestRunRejectsDuplicate(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "small.txt")
	output := filepath.Join(dir, "small.yaml")

	src := "# Title: oisd small\nexample.com\nexample.com\n"
	if err := os.WriteFile(input, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Run(Config{
		input:         input,
		output:        output,
		expectedTitle: "oisd small",
		minEntries:    1,
		maxEntries:    10,
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestRelativeChange(t *testing.T) {
	cases := []struct {
		old, new int
		want     float64
	}{
		{100, 100, 0},
		{100, 125, 0.25},
		{100, 75, 0.25},
	}

	for _, tc := range cases {
		got := relativeChange(tc.old, tc.new)
		if got != tc.want {
			t.Fatalf("relativeChange(%d, %d) = %v, want %v", tc.old, tc.new, got, tc.want)
		}
	}
}
