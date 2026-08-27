package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/metacubex/mihomo/component/geodata/router"
	"google.golang.org/protobuf/proto"
)

func TestExtractGeositeKeepsSelectedEntriesAndAttributes(t *testing.T) {
	directory := t.TempDir()
	inputPath := filepath.Join(directory, "geosite.dat")
	outputPath := filepath.Join(directory, "nested", "geosite-lite.dat")
	categoriesPath := filepath.Join(directory, "categories.txt")

	source := &router.GeoSiteList{Entry: []*router.GeoSite{
		{
			CountryCode: "keep",
			Domain: []*router.Domain{{
				Type:      router.Domain_Domain,
				Value:     "example.com",
				Attribute: []*router.Domain_Attribute{{Key: "cn"}},
			}},
		},
		{CountryCode: "drop"},
	}}
	writeProto(t, inputPath, source)
	writeText(t, categoriesPath, "# comment\nkeep\nmissing\n")

	if err := extractGeosite(inputPath, outputPath, categoriesPath); err != nil {
		t.Fatalf("extractGeosite() error = %v", err)
	}

	var result router.GeoSiteList
	readProto(t, outputPath, &result)
	if len(result.Entry) != 1 || result.Entry[0].GetCountryCode() != "keep" {
		t.Fatalf("selected entries = %v, want only keep", result.Entry)
	}
	domain := result.Entry[0].Domain[0]
	if domain.GetType() != router.Domain_Domain || domain.GetValue() != "example.com" {
		t.Fatalf("domain = %v, want original type and value", domain)
	}
	if len(domain.Attribute) != 1 || domain.Attribute[0].GetKey() != "cn" {
		t.Fatalf("attributes = %v, want cn", domain.Attribute)
	}
}

func TestExtractGeoIPSelectsCountries(t *testing.T) {
	directory := t.TempDir()
	inputPath := filepath.Join(directory, "geoip.dat")
	outputPath := filepath.Join(directory, "nested", "geoip-lite.dat")

	source := &router.GeoIPList{Entry: []*router.GeoIP{
		{CountryCode: "cn", Cidr: []*router.CIDR{{Ip: []byte{192, 0, 2, 0}, Prefix: 24}}},
		{CountryCode: "jp"},
		{CountryCode: "us"},
		{CountryCode: "de"},
	}}
	writeProto(t, inputPath, source)

	if err := extractGeoIP(inputPath, outputPath, "CN,JP,US"); err != nil {
		t.Fatalf("extractGeoIP() error = %v", err)
	}

	var result router.GeoIPList
	readProto(t, outputPath, &result)
	if len(result.Entry) != 3 {
		t.Fatalf("selected countries = %d, want 3", len(result.Entry))
	}
	for index, want := range []string{"cn", "jp", "us"} {
		if got := result.Entry[index].GetCountryCode(); got != want {
			t.Errorf("country[%d] = %q, want %q", index, got, want)
		}
	}
}

func writeProto(t *testing.T, path string, message proto.Message) {
	t.Helper()
	data, err := proto.Marshal(message)
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readProto(t *testing.T, path string, message proto.Message) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := proto.Unmarshal(data, message); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
}

func writeText(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
