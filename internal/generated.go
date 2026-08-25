package internal

import (
	"fmt"
	"os"
	"path/filepath"
)

func validateGeneratedData(root string) error {
	for _, path := range []string{"sources.json", "m49.json", "regions.json", "phones.json", "flags"} {
		if _, err := os.Stat(filepath.Join(root, "generated", path)); err != nil {
			return fmt.Errorf("validate generated data %s: %w", path, err)
		}
	}
	regions, err := readGeneratedRegions(root)
	if err != nil {
		return err
	}
	if len(regions.Regions) == 0 {
		return fmt.Errorf("validate generated data: regions are empty")
	}
	for _, region := range regions.Regions {
		if _, err := os.Stat(filepath.Join(root, "generated", "flags", region.Flag+".svg")); err != nil {
			return fmt.Errorf("validate generated data: flag for %s: %w", region.ISO, err)
		}
	}
	return nil
}

func readGeneratedRegions(root string) (RegionGeneration, error) {
	regions, err := readJSON[[]Region](filepath.Join(root, "generated", "regions.json"))
	if err != nil {
		return RegionGeneration{}, fmt.Errorf("read generated regions: %w", err)
	}
	return RegionGeneration{Regions: regions}, nil
}
