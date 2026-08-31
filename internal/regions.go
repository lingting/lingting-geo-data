package internal

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/lingting/lingting-geo-data/internal/cldr"
)

// GenerateRegions 在任一依赖更新或生成目标缺失时重新生成 regions.json。
func GenerateRegions(root string, states *States) (RegionGeneration, error) {
	if cached, err := states.ReadRegions(); err == nil {
		return cached, nil
	}
	m49, err := GenerateM49(root, states)
	if err != nil {
		return RegionGeneration{}, err
	}
	phones, err := GeneratePhones(root, states)
	if err != nil {
		return RegionGeneration{}, err
	}
	data, err := states.ReadCLDR(root)
	if err != nil {
		return RegionGeneration{}, err
	}
	if !data.Updated && !m49.Updated && !phones.Updated && targetExists(filepath.Join(root, "generated", "regions.json")) {
		result, err := readGeneratedRegions(root)
		if err != nil {
			return RegionGeneration{}, err
		}
		states.SetRegions(result)
		return result, nil
	}
	regions, err := parseRegions(data, m49.Indexes, phones.CallingCodes, phones.PhonePrefixes)
	if err != nil {
		return RegionGeneration{}, err
	}
	if err := validateRegions(regions); err != nil {
		return RegionGeneration{}, err
	}
	result := RegionGeneration{Regions: regions}
	states.SetRegions(result)
	return result, nil
}

func parseRegions(data cldr.Data, indexes map[string]M49Index, callingCodes, phonePrefixes map[string][]string) ([]Region, error) {
	regions := make([]Region, 0, len(indexes))
	for iso, index := range indexes {
		mapping, exists := data.CodeMappings.Mappings[iso]
		name := data.EnglishTerritory.Names[iso]
		if !exists || !isISORegion(iso, mapping) || name == "" {
			continue
		}
		regions = append(regions, Region{
			ISO:           iso,
			ISO3:          mapping.Alpha3,
			Flag:          flagFor(iso),
			CallingCodes:  callingCodes[iso],
			PhonePrefixes: phonePrefixesFor(iso, phonePrefixes, callingCodes),
			Names: Names{
				English: name,
				Chinese: data.ChineseTerritory.Names[iso],
			},
			Numeric: mapping.Numeric,
			M49:     index,
		})
	}
	sort.Slice(regions, func(i, j int) bool {
		return regions[i].ISO < regions[j].ISO
	})
	return regions, nil
}

func phonePrefixesFor(iso string, prefixes, callingCodes map[string][]string) []string {
	if values := prefixes[iso]; values != nil {
		return values
	}
	if values := callingCodes[iso]; values != nil {
		return values
	}
	return []string{}
}

func flagFor(iso string) string {
	if iso == "HK" || iso == "MO" || iso == "TW" {
		return "cn"
	}
	return strings.ToLower(iso)
}
