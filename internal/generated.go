package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

func validateGeneratedData(root string, m49 M49Generation, regions RegionGeneration, phones PhoneGeneration) error {
	if m49.Root == nil {
		return fmt.Errorf("validate generated data: M.49 root is nil")
	}
	if len(regions.Regions) == 0 {
		return fmt.Errorf("validate generated data: regions are empty")
	}
	if len(phones.Regions) == 0 {
		return fmt.Errorf("validate generated data: phones are empty")
	}
	if err := validateRegions(regions.Regions); err != nil {
		return err
	}
	if err := validateFlags(root, regions.Regions); err != nil {
		return err
	}
	return validatePhonePrefixes(regions.Regions, flattenPhones(phones.Regions))
}

func validateFlags(root string, regions []Region) error {
	for _, region := range regions {
		path := filepath.Join(root, "sources", "flag-icons", "4x3", region.Flag+".svg")
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("validate flag for %s: %w", region.ISO, err)
		}
	}
	return nil
}
func validatePhonePrefixes(regions []Region, phones []PhonePrefix) error {
	prefixesByRegion := make(map[string]map[string]struct{}, len(regions))
	for _, region := range regions {
		prefixes := make(map[string]struct{}, len(region.PhonePrefixes))
		for _, prefix := range region.PhonePrefixes {
			prefixes[prefix] = struct{}{}
		}
		prefixesByRegion[region.ISO] = prefixes
	}
	for _, phone := range phones {
		prefix := strconv.Itoa(phone.Prefix)
		prefixes, exists := prefixesByRegion[phone.Region]
		if !exists {
			continue
		}
		if _, exists := prefixes[prefix]; !exists {
			return fmt.Errorf("validate generated data: phone prefix %d missing from %s", phone.Prefix, phone.Region)
		}
	}
	for region, prefixes := range prefixesByRegion {
		for prefix := range prefixes {
			if hasCompletePhonePrefixChildren(prefix, prefixes) {
				return fmt.Errorf("validate generated data: phone prefix %s for %s is not shortest", prefix, region)
			}
		}
	}
	return nil
}

func hasCompletePhonePrefixChildren(prefix string, prefixes map[string]struct{}) bool {
	for digit := '0'; digit <= '9'; digit++ {
		if _, exists := prefixes[prefix+string(digit)]; !exists {
			return false
		}
	}
	return true
}

func readGeneratedRegions(root string) (RegionGeneration, error) {
	regions, err := readJSON[[]Region](filepath.Join(root, "generated", "regions.json"))
	if err != nil {
		return RegionGeneration{}, fmt.Errorf("read generated regions: %w", err)
	}
	return RegionGeneration{Regions: regions}, nil
}
