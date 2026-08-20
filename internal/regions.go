package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var regionSourcePaths = []string{
	"cldr/codeMappings.json",
	"cldr/en-territories.json",
	"cldr/zh-territories.json",
	"libphonenumber/PhoneNumberMetadata.xml",
}

type codeMapping struct {
	Alpha3  string `json:"_alpha3"`
	Numeric string `json:"_numeric"`
}

type codeMappingsFile struct {
	Supplemental struct {
		CodeMappings map[string]codeMapping `json:"codeMappings"`
	} `json:"supplemental"`
}

type territoryNamesFile struct {
	Main map[string]struct {
		LocaleDisplayNames struct {
			Territories map[string]string `json:"territories"`
		} `json:"localeDisplayNames"`
	} `json:"main"`
}

type phoneMetadata struct {
	Territories []phoneTerritory `xml:"territories>territory"`
}

type phoneTerritory struct {
	ID                 string `xml:"id,attr"`
	CountryCode        string `xml:"countryCode,attr"`
	LeadingDigits      string `xml:"leadingDigits,attr"`
	MainCountryForCode bool   `xml:"mainCountryForCode,attr"`
}

// GenerateRegions 在任一依赖更新时重新生成 regions.json，否则解析既有生成文件。
func GenerateRegions(root string, states *States) (RegionGeneration, error) {
	if cached := states.Regions(); cached != nil {
		return *cached, nil
	}

	result, err := generateRegions(root, states)
	if err != nil {
		return RegionGeneration{}, err
	}
	states.SetRegions(result)
	return result, nil
}

func generateRegions(root string, states *States) (RegionGeneration, error) {
	m49, err := GenerateM49(root, states)
	if err != nil {
		return RegionGeneration{}, err
	}
	phones, err := GeneratePhones(root, states)
	if err != nil {
		return RegionGeneration{}, err
	}
	if !m49.Updated && !phones.Updated && !states.SourcesUpdated(regionSourcePaths) {
		return readGeneratedRegions(root)
	}

	files, err := states.SourceFiles(regionSourcePaths)
	if err != nil {
		return RegionGeneration{}, err
	}
	regions, err := parseRegions(files, m49.Indexes, phones.CallingCodes, phones.PhonePrefixes)
	if err != nil {
		return RegionGeneration{}, err
	}
	if err := validateRegions(regions); err != nil {
		return RegionGeneration{}, err
	}
	body, err := json.MarshalIndent(regions, "", "  ")
	if err != nil {
		return RegionGeneration{}, err
	}
	if err := writeFile(filepath.Join(root, "generated"), "regions.json", append(body, '\n')); err != nil {
		return RegionGeneration{}, err
	}
	if err := os.Remove(filepath.Join(root, "generated", "countries.json")); err != nil && !os.IsNotExist(err) {
		return RegionGeneration{}, fmt.Errorf("remove legacy countries: %w", err)
	}
	return RegionGeneration{Regions: regions, Updated: true}, nil
}

func parseRegions(files map[string][]byte, indexes map[string]M49Index, callingCodes, phonePrefixes map[string][]string) ([]Region, error) {
	var mappings codeMappingsFile
	var en, zh territoryNamesFile
	if err := json.Unmarshal(files["cldr/codeMappings.json"], &mappings); err != nil {
		return nil, fmt.Errorf("parse codeMappings: %w", err)
	}
	if err := json.Unmarshal(files["cldr/en-territories.json"], &en); err != nil {
		return nil, fmt.Errorf("parse English territories: %w", err)
	}
	if err := json.Unmarshal(files["cldr/zh-territories.json"], &zh); err != nil {
		return nil, fmt.Errorf("parse Chinese territories: %w", err)
	}

	enNames, zhNames := namesFor(en, "en"), namesFor(zh, "zh")
	regions := make([]Region, 0, len(indexes))
	for iso, index := range indexes {
		mapping, exists := mappings.Supplemental.CodeMappings[iso]
		name := enNames[iso]
		if !exists || !isISORegion(iso, mapping) || name == "" {
			continue
		}
		regions = append(regions, Region{
			ISO:           iso,
			ISO3:          mapping.Alpha3,
			Flag:          flagFor(iso),
			CallingCodes:  callingCodes[iso],
			PhonePrefixes: phonePrefixesFor(iso, phonePrefixes, callingCodes),
			Names:         Names{English: name, Chinese: zhNames[iso]},
			Numeric:       mapping.Numeric,
			M49:           index,
		})
	}
	sort.Slice(regions, func(i, j int) bool { return regions[i].ISO < regions[j].ISO })
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

func namesFor(file territoryNamesFile, locale string) map[string]string {
	return file.Main[locale].LocaleDisplayNames.Territories
}

func sortedUnique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
