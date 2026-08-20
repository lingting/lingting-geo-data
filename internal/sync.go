package internal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Sync 更新上游源，并仅在相关源更新时重新生成产物。
func Sync(ctx context.Context, root string) (bool, error) {
	previous, err := readSourceIndex(root)
	if err != nil {
		return false, err
	}
	states, err := downloadAll(ctx, root, baseSources(), previous)
	if err != nil {
		return false, err
	}
	if err := writeUpdatedSources(root, states); err != nil {
		return false, err
	}
	countriesUpdated, err := GenerateCountries(root, states)
	if err != nil {
		return false, err
	}
	flagsUpdated, err := GenerateFlags(root, states["flag-icons"])
	if err != nil {
		return false, err
	}
	if err := validateGeneratedData(root); err != nil {
		return false, err
	}
	return sourcesUpdated(states) || countriesUpdated || flagsUpdated, nil
}

func readSourceIndex(root string) (SourceIndex, error) {
	body, err := os.ReadFile(filepath.Join(root, "generated", "sources.json"))
	if os.IsNotExist(err) {
		return SourceIndex{Sources: map[string]SourceRecord{}}, nil
	}
	if err != nil {
		return SourceIndex{}, err
	}
	var index SourceIndex
	if err := json.Unmarshal(body, &index); err != nil {
		return SourceIndex{}, fmt.Errorf("parse sources.json: %w", err)
	}
	if index.Sources == nil {
		index.Sources = map[string]SourceRecord{}
	}
	return index, nil
}

func writeUpdatedSources(root string, states map[string]SourceState) error {
	index := SourceIndex{Sources: make(map[string]SourceRecord, len(states))}
	for sourcePath, state := range states {
		state.Record.SHA256 = filesHash(state.Files)
		index.Sources[sourcePath] = state.Record
		if !state.Updated {
			continue
		}
		for filePath, body := range state.Files {
			if err := writeFile(filepath.Join(root, "sources"), filePath, body); err != nil {
				return err
			}
		}
	}
	body, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(root, "generated"), "sources.json", append(body, '\n'))
}

func sourcesUpdated(states map[string]SourceState) bool {
	for _, state := range states {
		if state.Updated {
			return true
		}
	}
	return false
}

func filesHash(files map[string][]byte) string {
	paths := make([]string, 0, len(files))
	for filePath := range files {
		paths = append(paths, filePath)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, filePath := range paths {
		digest := sha256.Sum256(files[filePath])
		_, _ = hash.Write([]byte(filePath + "\x00" + hex.EncodeToString(digest[:]) + "\n"))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func writeFile(root, relative string, body []byte) error {
	filePath := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filePath, body, 0o644)
}
