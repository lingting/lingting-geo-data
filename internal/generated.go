package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
)

func marshalGeneratedData(value any) ([]byte, error) {
	return json.MarshalIndent(normalizeNilSlices(value), "", "  ")
}

func normalizeNilSlices(value any) any {
	if value == nil {
		return nil
	}
	return normalizeNilSlicesValue(reflect.ValueOf(value)).Interface()
}

func normalizeNilSlicesValue(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return value
	}

	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return value
		}
		return normalizeNilSlicesValue(value.Elem())
	case reflect.Ptr:
		if value.IsNil() {
			return value
		}
		result := reflect.New(value.Type().Elem())
		result.Elem().Set(normalizeNilSlicesValue(value.Elem()))
		return result
	case reflect.Struct:
		result := reflect.New(value.Type()).Elem()
		result.Set(value)
		for index := range value.NumField() {
			field := result.Field(index)
			if field.CanSet() {
				field.Set(normalizeNilSlicesValue(value.Field(index)))
			}
		}
		return result
	case reflect.Slice:
		result := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for index := range value.Len() {
			result.Index(index).Set(normalizeNilSlicesValue(value.Index(index)))
		}
		return result
	case reflect.Map:
		if value.IsNil() {
			return value
		}
		result := reflect.MakeMapWithSize(value.Type(), value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			result.SetMapIndex(iterator.Key(), normalizeNilSlicesValue(iterator.Value()))
		}
		return result
	default:
		return value
	}
}

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
	if len(regions.Regions) == 0 {
		return fmt.Errorf("validate generated data: regions are empty")
	}
	for _, region := range regions.Regions {
		flagPath := filepath.Join(root, "generated", "flags", region.Flag+".svg")
		if _, err := os.Stat(flagPath); err != nil {
			return fmt.Errorf("validate generated data: flag for %s: %w", region.ISO, err)
		}
	}
	return nil
}

func readGeneratedRegions(root string) (RegionGeneration, error) {
	body, err := os.ReadFile(filepath.Join(root, "generated", "regions.json"))
	if err != nil {
		return RegionGeneration{}, fmt.Errorf("read generated regions: %w", err)
	}
	var regions []Region
	if err := json.Unmarshal(body, &regions); err != nil {
		return RegionGeneration{}, fmt.Errorf("parse generated regions: %w", err)
	}
	return RegionGeneration{Regions: regions}, nil
}
