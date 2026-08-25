package internal

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// GenerateFlags 仅生成被 regions.json 引用的旗帜，并删除其余旗帜文件。
func GenerateFlags(root string, states *States) (bool, error) {
	regions, err := GenerateRegions(root, states)
	if err != nil {
		return false, err
	}
	if !regions.Updated && !states.ReadFlagIcons().Updated && targetExists(filepath.Join(root, "generated", "flags")) {
		return false, nil
	}

	flagsDir := filepath.Join(root, "generated", "flags")
	used := make(map[string]struct{}, len(regions.Regions))
	updated := false
	for _, region := range regions.Regions {
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
