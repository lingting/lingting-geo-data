package internal

import (
	"fmt"
	"os"
	"path/filepath"
)

func validateGeneratedData(root string) error {
	for _, path := range []string{
		filepath.Join(root, "generated", "sources.json"),
		filepath.Join(root, "generated", "m49.json"),
		filepath.Join(root, "generated", "regions.json"),
		filepath.Join(root, "generated", "phones.json"),
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
	phones, err := readGeneratedPhones(root)
	if err != nil {
		return err
	}
	if len(regions) == 0 {
		return fmt.Errorf("validate generated data: regions are empty")
	}
	phoneRegions := make(map[string]string, len(phones))
	for _, phone := range phones {
		phoneRegions[fmt.Sprintf("%d", phone.Prefix)] = phone.Region
	}
	for _, region := range regions {
		flagPath := filepath.Join(root, "generated", "flags", region.Flag+".svg")
		if _, err := os.Stat(flagPath); err != nil {
			return fmt.Errorf("validate generated data: flag for %s: %w", region.ISO, err)
		}
		for _, callingCode := range region.CallingCodes {
			if _, exists := phoneRegions[callingCode]; !exists {
				return fmt.Errorf("validate generated data: calling code %s for %s is not in phones", callingCode, region.ISO)
			}
		}
		for _, prefix := range region.PhonePrefixes {
			if _, exists := phoneRegions[prefix]; !exists {
				return fmt.Errorf("validate generated data: phone prefix %s for %s is not in phones", prefix, region.ISO)
			}
		}
	}
	return nil
}
