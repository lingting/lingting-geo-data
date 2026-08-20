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
	sources, err := downloadAll(ctx, root, baseSources(), previous)
	if err != nil {
		return false, err
	}
	states := newStates(sources)
	if err := writeUpdatedSources(root, states); err != nil {
		return false, err
	}
	m49, err := GenerateM49(root, states)
	if err != nil {
		return false, err
	}
	phones, err := GeneratePhones(root, states)
	if err != nil {
		return false, err
	}
	regions, err := GenerateRegions(root, states)
	if err != nil {
		return false, err
	}
	flagsUpdated, err := GenerateFlags(root, states)
	if err != nil {
		return false, err
	}
	if err := validateGeneratedData(root); err != nil {
		return false, err
	}
	return states.SourcesAreUpdated() || m49.Updated || phones.Updated || regions.Updated || flagsUpdated, nil
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

func writeUpdatedSources(root string, states *States) error {
	sources := states.Sources()
	index := SourceIndex{Sources: make(map[string]SourceRecord, len(sources))}
	for sourcePath, state := range sources {
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
