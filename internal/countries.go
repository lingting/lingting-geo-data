package internal

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"sort"
)

var countrySourcePaths = []string{
	"cldr/codeMappings.json",
	"cldr/en-territories.json",
	"cldr/zh-territories.json",
	"libphonenumber/PhoneNumberMetadata.xml",
}

type codeMappingsFile struct {
	Supplemental struct {
		CodeMappings map[string]struct {
			Alpha3 string `json:"_alpha3"`
		} `json:"codeMappings"`
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
	Territories []struct {
		ID          string `xml:"id,attr"`
		CountryCode string `xml:"countryCode,attr"`
	} `xml:"territories>territory"`
}

// GenerateCountries 在任一依赖源更新时重新生成 countries.json。
func GenerateCountries(root string, states map[string]SourceState) (bool, error) {
	if !anySourceUpdated(states, countrySourcePaths) {
		return false, nil
	}
	files := make(map[string][]byte, len(countrySourcePaths))
	for _, sourcePath := range countrySourcePaths {
		state, exists := states[sourcePath]
		if !exists {
			return false, fmt.Errorf("missing country source %s", sourcePath)
		}
		for filePath, body := range state.Files {
			files[filePath] = body
		}
	}
	countries, _, err := parseCountries(files)
	if err != nil {
		return false, err
	}
	body, err := json.MarshalIndent(countries, "", "  ")
	if err != nil {
		return false, err
	}
	if err := writeFile(filepath.Join(root, "generated"), "countries.json", append(body, '\n')); err != nil {
		return false, err
	}
	return true, nil
}

func anySourceUpdated(states map[string]SourceState, paths []string) bool {
	for _, sourcePath := range paths {
		if states[sourcePath].Updated {
			return true
		}
	}
	return false
}

func parseCountries(files map[string][]byte) ([]Country, []SourceSpec, error) {
	var mappings codeMappingsFile
	var en, zh territoryNamesFile
	var phone phoneMetadata
	if err := json.Unmarshal(files["cldr/codeMappings.json"], &mappings); err != nil {
		return nil, nil, fmt.Errorf("parse codeMappings: %w", err)
	}
	if err := json.Unmarshal(files["cldr/en-territories.json"], &en); err != nil {
		return nil, nil, fmt.Errorf("parse English territories: %w", err)
	}
	if err := json.Unmarshal(files["cldr/zh-territories.json"], &zh); err != nil {
		return nil, nil, fmt.Errorf("parse Chinese territories: %w", err)
	}
	if err := xml.Unmarshal(files["libphonenumber/PhoneNumberMetadata.xml"], &phone); err != nil {
		return nil, nil, fmt.Errorf("parse libphonenumber metadata: %w", err)
	}

	enNames, zhNames := namesFor(en, "en"), namesFor(zh, "zh")
	callingCodes := make(map[string][]string)
	for _, territory := range phone.Territories {
		if territory.ID != "" && territory.CountryCode != "" {
			callingCodes[territory.ID] = append(callingCodes[territory.ID], territory.CountryCode)
		}
	}
	countries := make([]Country, 0, len(mappings.Supplemental.CodeMappings))
	for iso := range mappings.Supplemental.CodeMappings {
		mapping, exists := mappings.Supplemental.CodeMappings[iso]
		name := enNames[iso]
		if !exists || !validISO(iso) || !validISO3(mapping.Alpha3) || name == "" || nonISOTerritories[iso] {
			continue
		}
		countries = append(countries, Country{
			ISO: iso, ISO3: mapping.Alpha3, CallingCodes: sortedUnique(callingCodes[iso]),
			Names: Names{English: name, Chinese: zhNames[iso]},
		})
	}
	sort.Slice(countries, func(i, j int) bool { return countries[i].ISO < countries[j].ISO })
	return countries, nil, validateCountries(countries)
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
