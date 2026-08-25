package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
)

const cacheVersion = 3

type cacheEnvelope[T any] struct {
	Version int `json:"version"`
	Data    T   `json:"data"`
}

func readCache[T any](root, name string) (T, bool) {
	var empty T
	body, err := os.ReadFile(filepath.Join(root, "cached", name))
	if err != nil {
		return empty, false
	}
	var envelope cacheEnvelope[T]
	if json.Unmarshal(body, &envelope) != nil || envelope.Version != cacheVersion {
		return empty, false
	}
	return envelope.Data, true
}

func writeCache[T any](root, name string, data T) error {
	body, err := marshalJSON(cacheEnvelope[T]{Version: cacheVersion, Data: data})
	if err != nil {
		return err
	}
	return writePath(filepath.Join(root, "cached", name), append(body, '\n'))
}

func readJSON[T any](path string) (T, error) {
	var value T
	body, err := os.ReadFile(path)
	if err != nil {
		return value, err
	}
	if err := json.Unmarshal(body, &value); err != nil {
		return value, err
	}
	return value, nil
}

func writeGeneratedJSON(root, name string, value any) (bool, error) {
	body, err := marshalJSON(value)
	if err != nil {
		return false, err
	}
	body = append(body, '\n')
	path := filepath.Join(root, "generated", name)
	current, err := os.ReadFile(path)
	if err == nil && bytes.Equal(current, body) {
		return false, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read generated %s: %w", name, err)
	}
	return true, writePath(path, body)
}

func writePath(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

func targetExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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

func marshalJSON(value any) ([]byte, error) {
	return json.MarshalIndent(normalizeNilSlices(value), "", "  ")
}

// 兼容逐步迁移中的生成器调用。
func marshalGeneratedData(value any) ([]byte, error) {
	return marshalJSON(value)
}

func writeFile(root, relative string, body []byte) error {
	return writePath(filepath.Join(root, filepath.FromSlash(relative)), body)
}

func normalizeNilSlices(value any) any {
	if value == nil {
		return nil
	}
	return normalizeValue(reflect.ValueOf(value)).Interface()
}

func normalizeValue(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return value
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return value
		}
		return normalizeValue(value.Elem())
	case reflect.Ptr:
		if value.IsNil() {
			return value
		}
		result := reflect.New(value.Type().Elem())
		result.Elem().Set(normalizeValue(value.Elem()))
		return result
	case reflect.Struct:
		result := reflect.New(value.Type()).Elem()
		result.Set(value)
		for i := range value.NumField() {
			if result.Field(i).CanSet() {
				result.Field(i).Set(normalizeValue(value.Field(i)))
			}
		}
		return result
	case reflect.Slice:
		result := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := range value.Len() {
			result.Index(i).Set(normalizeValue(value.Index(i)))
		}
		return result
	case reflect.Map:
		if value.IsNil() {
			return value
		}
		result := reflect.MakeMapWithSize(value.Type(), value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			result.SetMapIndex(iterator.Key(), normalizeValue(iterator.Value()))
		}
		return result
	default:
		return value
	}
}
