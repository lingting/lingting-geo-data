package internal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// Sync 下载、校验并确定性生成数据；任何下载或解析失败均不会写入项目目录。
func Sync(ctx context.Context, root string) (bool, error) {
	before, err := dataHash(root)
	if err != nil {
		return false, err
	}
	previous, err := readSourceIndex(root)
	if err != nil {
		return false, err
	}
	specs := baseSources()
	downloads, err := downloadAll(ctx, root, specs, previous)
	if err != nil {
		return false, err
	}
	files, records := flattenDownloads(downloads)
	countries, _, err := parseCountries(files)
	if err != nil {
		return false, err
	}
	if err := writeStagedData(root, specs, files, records, countries); err != nil {
		return false, err
	}
	after, err := dataHash(root)
	if err != nil {
		return false, err
	}
	return before != after, nil
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

func flattenDownloads(downloads map[string]downloadedSource) (map[string][]byte, map[string]SourceRecord) {
	files := make(map[string][]byte)
	records := make(map[string]SourceRecord, len(downloads))
	for path, download := range downloads {
		for filePath, body := range download.Files {
			files[filePath] = body
		}
		records[path] = download.Record
	}
	return files, records
}

func writeStagedData(root string, specs []SourceSpec, files map[string][]byte, records map[string]SourceRecord, countries []Country) error {
	stage, err := os.MkdirTemp(root, ".sync-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	sourcesDir, generatedDir := filepath.Join(stage, "sources"), filepath.Join(stage, "generated")
	index := SourceIndex{Sources: make(map[string]SourceRecord, len(specs))}
	for _, spec := range specs {
		matching := sourceFiles(spec.Path, files)
		if len(matching) == 0 {
			return fmt.Errorf("source %s is empty", spec.Path)
		}
		for path, body := range matching {
			if err := writeFile(sourcesDir, path, body); err != nil {
				return err
			}
		}
		record := records[spec.Path]
		record.SHA256 = filesHash(matching)
		index.Sources[spec.Path] = record
	}
	countriesJSON, err := json.MarshalIndent(countries, "", "  ")
	if err != nil {
		return err
	}
	indexJSON, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFile(generatedDir, "countries.json", append(countriesJSON, '\n')); err != nil {
		return err
	}
	if err := writeFile(generatedDir, "sources.json", append(indexJSON, '\n')); err != nil {
		return err
	}
	for path, body := range sourceFiles("flag-icons", files) {
		generatedPath := filepath.ToSlash(filepath.Join("flags", filepath.Base(path)))
		if err := writeFile(generatedDir, generatedPath, body); err != nil {
			return err
		}
	}
	return replaceDataDirectories(root, sourcesDir, generatedDir)
}

func sourceFiles(sourcePath string, files map[string][]byte) map[string][]byte {
	result := make(map[string][]byte)
	prefix := sourcePath + "/"
	for path, body := range files {
		if path == sourcePath || len(path) > len(prefix) && path[:len(prefix)] == prefix {
			result[path] = body
		}
	}
	return result
}

func filesHash(files map[string][]byte) string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		digest := sha256.Sum256(files[path])
		_, _ = hash.Write([]byte(path + "\x00" + hex.EncodeToString(digest[:]) + "\n"))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func writeFile(root, relative string, body []byte) error {
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

type directoryReplacement struct{ current, staged, backup string }

func replaceDataDirectories(root, stagedSources, stagedGenerated string) error {
	replacements := []directoryReplacement{{filepath.Join(root, "sources"), stagedSources, filepath.Join(root, ".sources-backup")}, {filepath.Join(root, "generated"), stagedGenerated, filepath.Join(root, ".generated-backup")}}
	for _, item := range replacements {
		if err := os.RemoveAll(item.backup); err != nil {
			return err
		}
		if _, err := os.Stat(item.current); err == nil {
			if err := os.Rename(item.current, item.backup); err != nil {
				return rollbackDirectories(replacements, err)
			}
		} else if !os.IsNotExist(err) {
			return rollbackDirectories(replacements, err)
		}
	}
	for _, item := range replacements {
		if err := os.Rename(item.staged, item.current); err != nil {
			return rollbackDirectories(replacements, err)
		}
	}
	for _, item := range replacements {
		if err := os.RemoveAll(item.backup); err != nil {
			return err
		}
	}
	return nil
}
func rollbackDirectories(items []directoryReplacement, cause error) error {
	for _, item := range items {
		if _, err := os.Stat(item.backup); err == nil {
			_ = os.RemoveAll(item.current)
			_ = os.Rename(item.backup, item.current)
		}
	}
	return cause
}

func dataHash(root string) (string, error) {
	entries := make([]string, 0)
	for _, name := range []string{"sources", "generated"} {
		dir := filepath.Join(root, name)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return "", err
		}
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.Type().IsRegular() {
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(body)
			entries = append(entries, filepath.ToSlash(relative)+"\x00"+hex.EncodeToString(digest[:]))
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	sort.Strings(entries)
	hash := sha256.New()
	for _, entry := range entries {
		_, _ = hash.Write([]byte(entry + "\n"))
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
