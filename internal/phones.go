package internal

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp/syntax"
	"sort"
	"strconv"
	"strings"
)

const phoneMetadataSourcePath = "libphonenumber/PhoneNumberMetadata.xml"

// PhonePrefix 表示国际号码前缀与 ISO 区域代码的映射。
type PhonePrefix struct {
	Prefix int    `json:"prefix"`
	Region string `json:"region"`
}

// GeneratePhones 在 libphonenumber 元数据更新时重新生成 phones.json，并返回区域电话前缀索引。
func GeneratePhones(
	root string,
	states map[string]SourceState,
) (map[string][]string, map[string][]string, bool, error) {
	phones, err := parsePhones(states[phoneMetadataSourcePath].Files[phoneMetadataSourcePath])
	if err != nil {
		return nil, nil, false, err
	}
	phonePrefixes := phonePrefixes(phones)
	callingCodes, err := phoneCallingCodes(states[phoneMetadataSourcePath].Files[phoneMetadataSourcePath])
	if err != nil {
		return nil, nil, false, err
	}
	body, err := json.MarshalIndent(phones, "", "  ")
	if err != nil {
		return nil, nil, false, err
	}
	updated, err := writeGeneratedPhones(root, append(body, '\n'))
	if err != nil {
		return nil, nil, false, err
	}
	return callingCodes, phonePrefixes, updated, nil
}
func phonePrefixes(phones []PhonePrefix) map[string][]string {
	prefixes := make(map[string][]string)
	for _, phone := range phones {
		prefixes[phone.Region] = append(prefixes[phone.Region], strconv.Itoa(phone.Prefix))
	}
	for region, values := range prefixes {
		prefixes[region] = sortedUnique(values)
	}
	return prefixes
}

func writeGeneratedPhones(root string, body []byte) (bool, error) {
	phonesPath := filepath.Join(root, "generated", "phones.json")
	current, err := os.ReadFile(phonesPath)
	if err == nil && string(current) == string(body) {
		return false, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read generated phones: %w", err)
	}
	if err := writeFile(filepath.Join(root, "generated"), "phones.json", body); err != nil {
		return false, err
	}
	return true, nil
}

func parsePhones(body []byte) ([]PhonePrefix, error) {
	var metadata phoneMetadata
	if err := xml.Unmarshal(body, &metadata); err != nil {
		return nil, fmt.Errorf("parse libphonenumber metadata: %w", err)
	}
	byCountryCode := make(map[string][]phoneTerritory)
	for _, territory := range metadata.Territories {
		byCountryCode[territory.CountryCode] = append(byCountryCode[territory.CountryCode], territory)
	}

	prefixes := make(map[int]string)
	for countryCode, territories := range byCountryCode {
		if len(territories) == 1 {
			if err := addPhonePrefix(prefixes, countryCode, territories[0].ID); err != nil {
				return nil, err
			}
			continue
		}
		for _, territory := range territories {
			leadingDigits, err := expandLeadingDigits(territory.LeadingDigits)
			if err != nil {
				return nil, fmt.Errorf("expand leading digits for %s: %w", territory.ID, err)
			}
			for _, leadingDigit := range leadingDigits {
				if err := addPhonePrefix(prefixes, countryCode+leadingDigit, territory.ID); err != nil {
					return nil, err
				}
			}
			if territory.MainCountryForCode {
				if err := addPhonePrefix(prefixes, countryCode, territory.ID); err != nil {
					return nil, err
				}
			}
		}
	}

	phones := make([]PhonePrefix, 0, len(prefixes))
	for prefix, region := range prefixes {
		phones = append(phones, PhonePrefix{Prefix: prefix, Region: region})
	}
	sort.Slice(phones, func(i, j int) bool {
		return phones[i].Prefix > phones[j].Prefix
	})
	return phones, nil
}
func phoneCallingCodes(body []byte) (map[string][]string, error) {
	var metadata phoneMetadata
	if err := xml.Unmarshal(body, &metadata); err != nil {
		return nil, fmt.Errorf("parse libphonenumber metadata: %w", err)
	}
	callingCodes := make(map[string][]string)
	for _, territory := range metadata.Territories {
		if territory.ID != "" && territory.CountryCode != "" {
			callingCodes[territory.ID] = append(callingCodes[territory.ID], territory.CountryCode)
		}
	}
	for region, prefixes := range callingCodes {
		callingCodes[region] = sortedUnique(prefixes)
	}
	return callingCodes, nil
}

func expandLeadingDigits(value string) ([]string, error) {
	if value == "" {
		return nil, nil
	}
	regexp, err := syntax.Parse(strings.ReplaceAll(value, "(?:", "("), syntax.Perl)
	if err != nil {
		return nil, err
	}
	prefixes, err := expandRegexp(regexp.Simplify())
	if err != nil {
		return nil, err
	}
	return uniqueStrings(prefixes), nil
}

func expandRegexp(regexp *syntax.Regexp) ([]string, error) {
	switch regexp.Op {
	case syntax.OpEmptyMatch:
		return []string{""}, nil
	case syntax.OpLiteral:
		return []string{string(regexp.Rune)}, nil
	case syntax.OpCharClass:
		return expandCharClass(regexp.Rune)
	case syntax.OpCapture:
		return expandRegexp(regexp.Sub[0])
	case syntax.OpAlternate:
		var values []string
		for _, sub := range regexp.Sub {
			expanded, err := expandRegexp(sub)
			if err != nil {
				return nil, err
			}
			values = append(values, expanded...)
		}
		return values, nil
	case syntax.OpConcat:
		values := []string{""}
		for _, sub := range regexp.Sub {
			expanded, err := expandRegexp(sub)
			if err != nil {
				return nil, err
			}
			values = combinePrefixes(values, expanded)
		}
		return values, nil
	case syntax.OpQuest:
		expanded, err := expandRegexp(regexp.Sub[0])
		return append([]string{""}, expanded...), err
	default:
		return nil, fmt.Errorf("unsupported regular expression operation %s", regexp.Op)
	}
}

func expandCharClass(ranges []rune) ([]string, error) {
	var values []string
	for index := 0; index < len(ranges); index += 2 {
		for value := ranges[index]; value <= ranges[index+1]; value++ {
			if value < '0' || value > '9' {
				return nil, fmt.Errorf("non-numeric character %q", value)
			}
			values = append(values, string(value))
		}
	}
	return values, nil
}

func combinePrefixes(left, right []string) []string {
	values := make([]string, 0, len(left)*len(right))
	for _, prefix := range left {
		for _, suffix := range right {
			values = append(values, prefix+suffix)
		}
	}
	return values
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; !exists {
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}

func addPhonePrefix(prefixes map[int]string, value, region string) error {
	prefix, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("parse phone prefix %q: %w", value, err)
	}
	if previous, exists := prefixes[prefix]; exists && previous != region {
		return fmt.Errorf("phone prefix %d maps to both %s and %s", prefix, previous, region)
	}
	prefixes[prefix] = region
	return nil
}
