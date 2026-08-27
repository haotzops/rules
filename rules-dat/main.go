package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/metacubex/mihomo/component/geodata/router"
	"google.golang.org/protobuf/proto"
)

type config struct {
	geositeInput      string
	geositeOutput     string
	geositeCategories string
	geoipInput        string
	geoipOutput       string
	geoipCountries    string
}

func main() {
	cfg := config{}
	flag.StringVar(&cfg.geositeInput, "geosite-input", "", "input V2Ray geosite.dat")
	flag.StringVar(&cfg.geositeOutput, "geosite-output", "", "output filtered geosite.dat")
	flag.StringVar(&cfg.geositeCategories, "geosite-categories", "", "file containing geosite category names")
	flag.StringVar(&cfg.geoipInput, "geoip-input", "", "input V2Ray geoip.dat")
	flag.StringVar(&cfg.geoipOutput, "geoip-output", "", "output filtered geoip.dat")
	flag.StringVar(&cfg.geoipCountries, "geoip-countries", "", "comma-separated geoip country codes")
	flag.Parse()

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "extract geo data:", err)
		os.Exit(1)
	}
}

func run(cfg config) error {
	if cfg.geositeInput != "" || cfg.geositeOutput != "" || cfg.geositeCategories != "" {
		if cfg.geositeInput == "" || cfg.geositeOutput == "" || cfg.geositeCategories == "" {
			return fmt.Errorf("geosite-input, geosite-output and geosite-categories must be provided together")
		}
		if err := extractGeosite(cfg.geositeInput, cfg.geositeOutput, cfg.geositeCategories); err != nil {
			return err
		}
	}

	if cfg.geoipInput != "" || cfg.geoipOutput != "" || cfg.geoipCountries != "" {
		if cfg.geoipInput == "" || cfg.geoipOutput == "" || cfg.geoipCountries == "" {
			return fmt.Errorf("geoip-input, geoip-output and geoip-countries must be provided together")
		}
		if err := extractGeoIP(cfg.geoipInput, cfg.geoipOutput, cfg.geoipCountries); err != nil {
			return err
		}
	}

	if cfg.geositeInput == "" && cfg.geoipInput == "" {
		return fmt.Errorf("at least one extraction must be configured")
	}
	return nil
}

func extractGeosite(inputPath, outputPath, categoriesPath string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read geosite input: %w", err)
	}

	var source router.GeoSiteList
	if err := proto.Unmarshal(data, &source); err != nil {
		return fmt.Errorf("decode geosite input: %w", err)
	}

	categories, err := readNames(categoriesPath)
	if err != nil {
		return fmt.Errorf("read geosite categories: %w", err)
	}
	wanted := make(map[string]bool, len(categories))
	for _, category := range categories {
		wanted[strings.ToLower(category)] = true
	}

	selected := make([]*router.GeoSite, 0, len(categories))
	found := make(map[string]bool, len(categories))
	for _, entry := range source.Entry {
		name := strings.ToLower(entry.GetCountryCode())
		if wanted[name] {
			selected = append(selected, entry)
			found[name] = true
		}
	}
	if len(selected) == 0 {
		return fmt.Errorf("none of the requested geosite categories exist in input")
	}
	for _, category := range categories {
		if !found[strings.ToLower(category)] {
			fmt.Fprintf(os.Stderr, "warning: geosite category %q is not present upstream; skipped\n", category)
		}
	}

	encoded, err := proto.Marshal(&router.GeoSiteList{Entry: selected})
	if err != nil {
		return fmt.Errorf("encode geosite output: %w", err)
	}
	if err := writeAtomically(outputPath, encoded); err != nil {
		return fmt.Errorf("write geosite output: %w", err)
	}
	fmt.Printf("extracted %d geosite categories into %s\n", len(selected), outputPath)
	return nil
}

func extractGeoIP(inputPath, outputPath, countries string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read geoip input: %w", err)
	}

	var source router.GeoIPList
	if err := proto.Unmarshal(data, &source); err != nil {
		return fmt.Errorf("decode geoip input: %w", err)
	}

	wanted := make(map[string]bool)
	for _, country := range strings.Split(countries, ",") {
		country = strings.TrimSpace(country)
		if country != "" {
			wanted[strings.ToLower(country)] = true
		}
	}
	if len(wanted) == 0 {
		return fmt.Errorf("geoip-countries is empty")
	}

	selected := make([]*router.GeoIP, 0, len(wanted))
	found := make(map[string]bool, len(wanted))
	for _, entry := range source.Entry {
		name := strings.ToLower(entry.GetCountryCode())
		if wanted[name] {
			selected = append(selected, entry)
			found[name] = true
		}
	}
	if len(selected) != len(wanted) {
		for country := range wanted {
			if !found[country] {
				return fmt.Errorf("geoip country %q is not present upstream", country)
			}
		}
	}

	encoded, err := proto.Marshal(&router.GeoIPList{Entry: selected})
	if err != nil {
		return fmt.Errorf("encode geoip output: %w", err)
	}
	if err := writeAtomically(outputPath, encoded); err != nil {
		return fmt.Errorf("write geoip output: %w", err)
	}
	fmt.Printf("extracted %d geoip countries into %s\n", len(selected), outputPath)
	return nil
}

func readNames(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		names = append(names, line)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no category names found")
	}
	return names, nil
}

func writeAtomically(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".geo-extract-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
