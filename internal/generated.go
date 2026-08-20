package internal

import (
	"fmt"
	"os"
	"path/filepath"
)

func validateGeneratedData(root string) error {
	for _, path := range []string{
		filepath.Join(root, "generated", "sources.json"),
		filepath.Join(root, "generated", "regions.json"),
		filepath.Join(root, "generated", "flags"),
	} {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("validate generated data %s: %w", path, err)
		}
	}
	regions, err := readGeneratedRegions(root)
	if err != nil {
		return err
	}
	if len(regions) == 0 {
		return fmt.Errorf("validate generated data: regions are empty")
	}
	for _, region := range regions {
		flagPath := filepath.Join(root, "generated", "flags", region.Flag+".svg")
		if _, err := os.Stat(flagPath); err != nil {
			return fmt.Errorf("validate generated data: flag for %s: %w", region.ISO, err)
		}
	}
	return nil
}
