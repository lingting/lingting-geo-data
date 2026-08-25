package internal

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp/syntax"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// PhonePrefix 表示国际号码前缀与 ISO 区域代码的映射。
type PhonePrefix struct {
	Prefix  int    `json:"prefix"`
	Calling int    `json:"calling"`
	Region  string `json:"region"`
}

// GeneratePhones 在元数据更新时重新生成 phones.json，否则解析既有生成文件。
func GeneratePhones(root string, states *States) (PhoneGeneration, error) {
	if cached, err := states.ReadPhones(); err == nil {
		return cached, nil
	}
	resource := states.ReadPhoneMetadata()
	result, err := generatePhones(root, resource)
	if err != nil {
		return PhoneGeneration{}, err
	}
	states.SetPhones(result)
	return result, nil
}

func generatePhones(root string, resource Resource) (PhoneGeneration, error) {
	var regions map[string][]PhoneNumber
	if !resource.Updated {
		regions, _ = readCache[map[string][]PhoneNumber](root, "phone-number-metadata.json")
	}
	if regions == nil {
		regions, err := parsePhones(resource.Files[resource.Spec.Path])
		if err != nil {
			return PhoneGeneration{}, err
		}
		if err := writeCache(root, "phone-number-metadata.json", regions); err != nil {
			return PhoneGeneration{}, err
		}
	}
	result := newPhoneGeneration(regions, false)
	body, err := marshalGeneratedData(flattenPhones(regions))
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
		if phones[i].Region != phones[j].Region {
			return phones[i].Region < phones[j].Region
		}
		return phones[i].Calling < phones[j].Calling
	})
	return phones
}

func removeFallbackCoveredPrefixes(regions map[string][]PhoneNumber) map[string][]PhoneNumber {
	fallbacks := make(map[string][]string, len(regions))
	for region, numbers := range regions {
		for _, number := range numbers {
			for _, prefix := range number.Prefixes {
				if prefix == number.Calling {
					fallbacks[region] = append(fallbacks[region], strconv.Itoa(prefix))
				}
			}
		}
	}
	for region, numbers := range regions {
		for index, number := range numbers {
			prefixes := make([]int, 0, len(number.Prefixes))
			for _, prefix := range number.Prefixes {
				if !isCoveredByFallback(strconv.Itoa(prefix), fallbacks[region]) {
					prefixes = append(prefixes, prefix)
				}
			}
			numbers[index].Prefixes = uniqueInts(prefixes)
		}
		regions[region] = numbers
	}
	return regions
}

func isCoveredByFallback(prefix string, fallbacks []string) bool {
	for _, fallback := range fallbacks {
		if prefix != fallback && strings.HasPrefix(prefix, fallback) {
			return true
		}
	}
	return false
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
	callingByRegion := make(map[string]int, len(metadata.Territories))
	regionsByCalling := make(map[int][]string)
	mainByCalling := make(map[int]string)
	for _, territory := range metadata.Territories {
		if territory.ID == "" {
			return nil, fmt.Errorf("missing territory ID")
		}
		calling, err := strconv.Atoi(territory.CountryCode)
		if err != nil {
			return nil, fmt.Errorf("parse calling code %q: %w", territory.CountryCode, err)
		}
		callingByRegion[territory.ID] = calling
		regionsByCalling[calling] = append(regionsByCalling[calling], territory.ID)
		if territory.MainCountryForCode {
			mainByCalling[calling] = territory.ID
		}
		values, err := geographicPrefixes(territory, calling)
		if err != nil {
			return nil, fmt.Errorf("parse geographic prefixes for %s: %w", territory.ID, err)
		}
		regions[territory.ID] = []PhoneNumber{{Calling: calling, Prefixes: values}}
	}
	for region, numbers := range regions {
		calling := callingByRegion[region]
		prefixes := numbers[0].Prefixes
		if fallbackRegion(calling, regionsByCalling, mainByCalling) == region {
			prefixes = append(prefixes, calling)
		}
		regions[region] = []PhoneNumber{{Calling: calling, Prefixes: uniqueInts(prefixes)}}
	}
	return removeFallbackCoveredPrefixes(regions), nil
}
func fallbackRegion(calling int, regionsByCalling map[int][]string, mainByCalling map[int]string) string {
	if region := mainByCalling[calling]; region != "" {
		return region
	}
	regions := regionsByCalling[calling]
	if len(regions) == 1 {
		return regions[0]
	}
	return ""
}

func geographicPrefixes(territory phoneTerritory, calling int) ([]int, error) {
	patterns := []string{territory.LeadingDigits, territory.FixedLine.NationalNumberPattern, territory.Mobile.NationalNumberPattern}
	var prefixes []int
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern) == "" {
			continue
		}
		values, err := expandLeadingDigits(pattern)
		if err != nil {
			return nil, err
		}
		for _, value := range values {
			value = numericPrefix(value)
			if value == "" {
				continue
			}
			prefix, err := strconv.Atoi(strconv.Itoa(calling) + value)
			if err != nil {
				return nil, err
			}
			prefixes = append(prefixes, prefix)
		}
	}
	return compactPhonePrefixes(uniqueInts(prefixes)), nil
}

type phonePrefixNode struct {
	children [10]*phonePrefixNode
	terminal bool
}

func compactPhonePrefixes(prefixes []int) []int {
	root := &phonePrefixNode{}
	for _, prefix := range prefixes {
		node := root
		for _, digit := range strconv.Itoa(prefix) {
			index := digit - '0'
			if node.children[index] == nil {
				node.children[index] = &phonePrefixNode{}
			}
			node = node.children[index]
		}
		node.terminal = true
	}
	values := make([]string, 0, len(root.children))
	for digit, child := range root.children {
		if child != nil {
			values = append(values, child.compact(strconv.Itoa(digit))...)
		}
	}
	result := make([]int, 0, len(values))
	for _, value := range values {
		prefix, _ := strconv.Atoi(value)
		result = append(result, prefix)
	}
	return uniqueInts(result)
}

func (node *phonePrefixNode) compact(prefix string) []string {
	if node.terminal {
		return []string{prefix}
	}
	values := make([]string, 0, len(node.children))
	allCovered := true
	for digit, child := range node.children {
		if child == nil {
			allCovered = false
			continue
		}
		childValues := child.compact(prefix + strconv.Itoa(digit))
		if len(childValues) != 1 || childValues[0] != prefix+strconv.Itoa(digit) {
			allCovered = false
		}
		values = append(values, childValues...)
	}
	if allCovered {
		return []string{prefix}
	}
	return values
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
	value = strings.Join(strings.Fields(value), "")
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

func numericPrefix(value string) string {
	for index, runeValue := range value {
		if !unicode.IsDigit(runeValue) {
			return value[:index]
		}
	}
	return value
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
			if len(values) > maxPrefixLength {
				values = trimPrefixes(values, maxPrefixLength)
				break
			}
		}
		return values, nil
	case syntax.OpRepeat, syntax.OpStar, syntax.OpPlus:
		return []string{""}, nil
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

const (
	maxExpandedPrefixes = 1_024
	maxPrefixLength     = 7
)

func combinePrefixes(left, right []string) []string {
	values := make([]string, 0, min(len(left)*len(right), maxExpandedPrefixes))
	for _, prefix := range left {
		for _, suffix := range right {
			values = append(values, prefix+suffix)
			if len(values) == maxExpandedPrefixes {
				return values
			}
		}
	}
	return values
}
func trimPrefixes(values []string, length int) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if len(value) > length {
			value = value[:length]
		}
		result = append(result, value)
	}
	return uniqueStrings(result)
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
