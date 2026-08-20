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
	Prefix  int    `json:"prefix"`
	Calling int    `json:"calling"`
	Region  string `json:"region"`
}

// GeneratePhones 在元数据更新时重新生成 phones.json，否则解析既有生成文件。
func GeneratePhones(root string, states *States) (PhoneGeneration, error) {
	if cached := states.Phones(); cached != nil {
		return *cached, nil
	}

	result, err := generatePhones(root, states)
	if err != nil {
		return PhoneGeneration{}, err
	}
	states.SetPhones(result)
	return result, nil
}

func generatePhones(root string, states *States) (PhoneGeneration, error) {
	state, err := states.Source(phoneMetadataSourcePath)
	if err != nil {
		return PhoneGeneration{}, err
	}

	regions, err := parsePhones(state.Files[phoneMetadataSourcePath])
	if err != nil {
		return PhoneGeneration{}, err
	}
	result := newPhoneGeneration(regions, false)
	body, err := json.MarshalIndent(flattenPhones(regions), "", "  ")
	if err != nil {
		return PhoneGeneration{}, err
	}
	updated, err := writeGeneratedPhones(root, append(body, '\n'))
	if err != nil {
		return PhoneGeneration{}, err
	}
	result.Updated = updated
	return result, nil
}

func newPhoneGeneration(regions map[string][]PhoneNumber, updated bool) PhoneGeneration {
	return PhoneGeneration{
		Regions:       regions,
		CallingCodes:  phoneCallingCodes(regions),
		PhonePrefixes: phonePrefixes(regions),
		Updated:       updated,
	}
}

func phonePrefixes(regions map[string][]PhoneNumber) map[string][]string {
	prefixes := make(map[string][]string, len(regions))
	for region, numbers := range regions {
		for _, number := range numbers {
			for _, prefix := range number.Prefixes {
				prefixes[region] = append(prefixes[region], strconv.Itoa(prefix))
			}
		}
		prefixes[region] = sortedUnique(prefixes[region])
	}
	return prefixes
}

func phoneCallingCodes(regions map[string][]PhoneNumber) map[string][]string {
	callingCodes := make(map[string][]string, len(regions))
	for region, numbers := range regions {
		for _, number := range numbers {
			callingCodes[region] = append(callingCodes[region], strconv.Itoa(number.Calling))
		}
		callingCodes[region] = sortedUnique(callingCodes[region])
	}
	return callingCodes
}

func flattenPhones(regions map[string][]PhoneNumber) []PhonePrefix {
	phones := make([]PhonePrefix, 0)
	for region, numbers := range regions {
		for _, number := range numbers {
			for _, prefix := range number.Prefixes {
				phones = append(phones, PhonePrefix{
					Prefix:  prefix,
					Calling: number.Calling,
					Region:  region,
				})
			}
		}
	}
	sort.Slice(phones, func(i, j int) bool {
		if phones[i].Prefix != phones[j].Prefix {
			return phones[i].Prefix > phones[j].Prefix
		}
		if phones[i].Calling != phones[j].Calling {
			return phones[i].Calling > phones[j].Calling
		}
		return phones[i].Region < phones[j].Region
	})
	return phones
}

func generatedPhonesNeedCalling(err error) bool {
	return strings.Contains(err.Error(), "missing calling")
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

func parsePhones(body []byte) (map[string][]PhoneNumber, error) {
	var metadata phoneMetadata
	if err := xml.Unmarshal(body, &metadata); err != nil {
		return nil, fmt.Errorf("parse libphonenumber metadata: %w", err)
	}

	regions := make(map[string][]PhoneNumber, len(metadata.Territories))
	for _, territory := range metadata.Territories {
		if territory.ID == "" {
			return nil, fmt.Errorf("missing territory ID")
		}
		calling, err := strconv.Atoi(territory.CountryCode)
		if err != nil {
			return nil, fmt.Errorf("parse calling code %q: %w", territory.CountryCode, err)
		}
		prefixes, err := phonePrefixesForTerritory(territory, calling)
		if err != nil {
			return nil, err
		}
		regions[territory.ID] = append(regions[territory.ID], PhoneNumber{
			Calling:  calling,
			Prefixes: prefixes,
		})
	}
	return regions, nil
}

func phonePrefixesForTerritory(territory phoneTerritory, calling int) ([]int, error) {
	prefixes := []int{calling}
	leadingDigits, err := expandLeadingDigits(territory.LeadingDigits)
	if err != nil {
		return nil, fmt.Errorf("expand leading digits for %s: %w", territory.ID, err)
	}
	for _, leadingDigit := range leadingDigits {
		prefix, err := strconv.Atoi(strconv.Itoa(calling) + leadingDigit)
		if err != nil {
			return nil, fmt.Errorf("parse phone prefix for %s: %w", territory.ID, err)
		}
		prefixes = append(prefixes, prefix)
	}
	return uniqueInts(prefixes), nil
}

func uniqueInts(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; !exists {
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	sort.Ints(result)
	return result
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
