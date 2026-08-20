package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// GenerateFlags 仅生成被 regions.json 引用的旗帜，并删除其余旗帜文件。
func GenerateFlags(root string) (bool, error) {
	regions, err := readGeneratedRegions(root)
	if err != nil {
		return false, err
	}
	flagsDir := filepath.Join(root, "generated", "flags")
	used := make(map[string]struct{}, len(regions))
	updated := false
	for _, region := range regions {
		used[region.Flag] = struct{}{}
		changed, err := syncFlag(root, flagsDir, region.Flag)
		if err != nil {
			return false, err
		}
		updated = updated || changed
	}
	entries, err := os.ReadDir(flagsDir)
	if err != nil {
		return false, fmt.Errorf("read generated flags: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".svg" {
			continue
		}
		name := entry.Name()[:len(entry.Name())-len(".svg")]
		if _, exists := used[name]; exists {
			continue
		}
		if err := os.Remove(filepath.Join(flagsDir, entry.Name())); err != nil {
			return false, fmt.Errorf("remove unused flag %s: %w", entry.Name(), err)
		}
		updated = true
	}
	return updated, nil
}

func syncFlag(root, flagsDir, flag string) (bool, error) {
	sourcePath := filepath.Join(root, "sources", "flag-icons", "4x3", flag+".svg")
	body, err := os.ReadFile(sourcePath)
	if err != nil {
		return false, fmt.Errorf("read flag source %s: %w", flag, err)
	}
	targetPath := filepath.Join(flagsDir, flag+".svg")
	current, err := os.ReadFile(targetPath)
	if err == nil && bytes.Equal(current, body) {
		return false, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read generated flag %s: %w", flag, err)
	}
	if err := os.WriteFile(targetPath, body, 0o644); err != nil {
		return false, fmt.Errorf("write generated flag %s: %w", flag, err)
	}
	return true, nil
}

func readGeneratedPhones(root string) ([]PhonePrefix, error) {
	body, err := os.ReadFile(filepath.Join(root, "generated", "phones.json"))
	if err != nil {
		return nil, fmt.Errorf("read generated phones: %w", err)
	}
	var phones []PhonePrefix
	if err := json.Unmarshal(body, &phones); err != nil {
		return nil, fmt.Errorf("parse generated phones: %w", err)
	}
	return phones, nil
}

func readGeneratedRegions(root string) ([]Region, error) {
	body, err := os.ReadFile(filepath.Join(root, "generated", "regions.json"))
	if err != nil {
		return nil, fmt.Errorf("read generated regions: %w", err)
	}
	var regions []Region
	if err := json.Unmarshal(body, &regions); err != nil {
		return nil, fmt.Errorf("parse generated regions: %w", err)
	}
	return regions, nil
}
