// Package cldr 解析并缓存同步的 Unicode CLDR 数据。
package cldr

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
)

const cacheVersion = 1

type CodeMapping struct {
	Alpha3  string `json:"_alpha3"`
	Numeric string `json:"_numeric"`
}

type CodeMappings struct {
	Mappings map[string]CodeMapping `json:"mappings"`
}

type TerritoryNames struct {
	Names map[string]string `json:"names"`
}

type TerritoryGroup struct {
	Type     string `json:"type"`
	Contains string `json:"contains"`
	Status   string `json:"status,omitempty"`
	Grouping bool   `json:"grouping,omitempty"`
}

type SupplementalData struct {
	Groups []TerritoryGroup `json:"groups"`
}

type Data struct {
	CodeMappings     CodeMappings
	EnglishTerritory TerritoryNames
	ChineseTerritory TerritoryNames
	SupplementalData SupplementalData
	Updated          bool
}

type cacheEnvelope[T any] struct {
	Version int `json:"version"`
	Data    T   `json:"data"`
}

func Read(root string, codeMappingsBody, englishBody, chineseBody, supplementalBody []byte, updated bool) (Data, error) {
	codeMappings, err := ReadCodeMappings(root, codeMappingsBody, updated)
	if err != nil {
		return Data{}, err
	}
	englishTerritory, err := ReadEnglishTerritories(root, englishBody, updated)
	if err != nil {
		return Data{}, err
	}
	chineseTerritory, err := ReadChineseTerritories(root, chineseBody, updated)
	if err != nil {
		return Data{}, err
	}
	supplementalData, err := ReadSupplementalData(root, supplementalBody, updated)
	if err != nil {
		return Data{}, err
	}
	return Data{
		CodeMappings:     codeMappings,
		EnglishTerritory: englishTerritory,
		ChineseTerritory: chineseTerritory,
		SupplementalData: supplementalData,
		Updated:          updated,
	}, nil
}

func ReadCodeMappings(root string, body []byte, updated bool) (CodeMappings, error) {
	return load(root, "cldr-code-mappings.json", body, updated, parseCodeMappings)
}

func ReadEnglishTerritories(root string, body []byte, updated bool) (TerritoryNames, error) {
	return load(root, "cldr-en-territories.json", body, updated, parseTerritoryNames("en"))
}

func ReadChineseTerritories(root string, body []byte, updated bool) (TerritoryNames, error) {
	return load(root, "cldr-zh-territories.json", body, updated, parseTerritoryNames("zh"))
}

func ReadSupplementalData(root string, body []byte, updated bool) (SupplementalData, error) {
	return load(root, "cldr-supplemental-data.json", body, updated, parseSupplementalData)
}

func load[T any](root, name string, body []byte, updated bool, parse func([]byte) (T, error)) (T, error) {
	if !updated {
		if value, ok := readCache[T](root, name); ok {
			return value, nil
		}
	}
	value, err := parse(body)
	if err != nil {
		return value, err
	}
	return value, writeCache(root, name, value)
}

func readCache[T any](root, name string) (T, bool) {
	var empty T
	body, err := os.ReadFile(filepath.Join(root, "cached", name))
	if err != nil {
		return empty, false
	}
	var cache cacheEnvelope[T]
	if err := json.Unmarshal(body, &cache); err != nil || cache.Version != cacheVersion {
		return empty, false
	}
	return cache.Data, true
}

func writeCache[T any](root, name string, data T) error {
	body, err := json.MarshalIndent(cacheEnvelope[T]{Version: cacheVersion, Data: data}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, "cached"), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "cached", name), append(body, '\n'), 0o644)
}

func parseCodeMappings(body []byte) (CodeMappings, error) {
	var file struct {
		Supplemental struct {
			CodeMappings map[string]CodeMapping `json:"codeMappings"`
		} `json:"supplemental"`
	}
	if err := json.Unmarshal(body, &file); err != nil {
		return CodeMappings{}, fmt.Errorf("parse code mappings: %w", err)
	}
	return CodeMappings{Mappings: file.Supplemental.CodeMappings}, nil
}

func parseTerritoryNames(locale string) func([]byte) (TerritoryNames, error) {
	return func(body []byte) (TerritoryNames, error) {
		var file struct {
			Main map[string]struct {
				LocaleDisplayNames struct {
					Territories map[string]string `json:"territories"`
				} `json:"localeDisplayNames"`
			} `json:"main"`
		}
		if err := json.Unmarshal(body, &file); err != nil {
			return TerritoryNames{}, fmt.Errorf("parse %s territories: %w", locale, err)
		}
		return TerritoryNames{Names: file.Main[locale].LocaleDisplayNames.Territories}, nil
	}
}

func parseSupplementalData(body []byte) (SupplementalData, error) {
	var file struct {
		TerritoryContainment struct {
			Groups []TerritoryGroup `xml:"group"`
		} `xml:"territoryContainment"`
	}
	if err := xml.Unmarshal(body, &file); err != nil {
		return SupplementalData{}, fmt.Errorf("parse supplemental data: %w", err)
	}
	return SupplementalData{Groups: file.TerritoryContainment.Groups}, nil
}
