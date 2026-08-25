package internal

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const requestTimeout = 30 * time.Second

type githubRelease struct {
	ZipballURL string `json:"zipball_url"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func downloadResources(ctx context.Context, root string, specs Resources, previous SourceIndex) (Resources, error) {
	client := &http.Client{Timeout: requestTimeout}
	result := specs
	var err error
	result.CldrCodeMappings, err = downloadResource(ctx, root, client, specs.CldrCodeMappings, previous.CldrCodeMappings)
	if err != nil {
		return Resources{}, err
	}
	result.CldrEnTerritories, err = downloadResource(ctx, root, client, specs.CldrEnTerritories, previous.CldrEnTerritories)
	if err != nil {
		return Resources{}, err
	}
	result.CldrZhTerritories, err = downloadResource(ctx, root, client, specs.CldrZhTerritories, previous.CldrZhTerritories)
	if err != nil {
		return Resources{}, err
	}
	result.CldrSupplementalData, err = downloadResource(ctx, root, client, specs.CldrSupplementalData, previous.CldrSupplementalData)
	if err != nil {
		return Resources{}, err
	}
	result.PhoneNumberMetadata, err = downloadResource(ctx, root, client, specs.PhoneNumberMetadata, previous.PhoneNumberMetadata)
	if err != nil {
		return Resources{}, err
	}
	result.FlagIcons, err = downloadResource(ctx, root, client, specs.FlagIcons, previous.FlagIcons)
	if err != nil {
		return Resources{}, err
	}
	return result, nil
}

func downloadResource(ctx context.Context, root string, client *http.Client, resource Resource, previous SourceRecord) (Resource, error) {
	result, err := download(ctx, root, client, resource.Spec, previous)
	if err != nil {
		return Resource{}, err
	}
	return result, nil
}

func download(ctx context.Context, root string, client *http.Client, spec SourceSpec, previous SourceRecord) (Resource, error) {
	if spec.Kind == directSource {
		return downloadDirect(ctx, root, client, spec, previous)
	}
	return downloadLatestRelease(ctx, root, client, spec, previous)
}

func downloadDirect(ctx context.Context, root string, client *http.Client, spec SourceSpec, previous SourceRecord) (Resource, error) {
	body, etag, unchanged, err := get(ctx, client, spec.URL, previous.ETag)
	if err != nil {
		return Resource{}, fmt.Errorf("download %s: %w", spec.URL, err)
	}
	files := map[string][]byte{spec.Path: body}
	if unchanged {
		files, err = readCachedSource(root, spec.Path)
		if err != nil {
			return Resource{}, err
		}
	}
	if !validContent(spec.Path, files[spec.Path]) {
		return Resource{}, fmt.Errorf("invalid content for %s", spec.Path)
	}
	return Resource{
		Spec:    spec,
		Files:   files,
		Record:  SourceRecord{URL: spec.URL, ETag: etag, Provenance: spec.Provenance},
		Updated: !unchanged,
	}, nil
}

func downloadLatestRelease(ctx context.Context, root string, client *http.Client, spec SourceSpec, previous SourceRecord) (Resource, error) {
	releaseURL := "https://api.github.com/repos/" + spec.Repository + "/releases/latest"
	body, etag, unchanged, err := get(ctx, client, releaseURL, previous.ETag)
	if err != nil {
		return Resource{}, fmt.Errorf("get latest release for %s: %w", spec.Repository, err)
	}
	if unchanged {
		files, err := readCachedSource(root, spec.Path)
		if err != nil {
			return Resource{}, err
		}
		return Resource{Spec: spec, Files: files, Record: previous}, nil
	}
	var release githubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return Resource{}, fmt.Errorf("parse latest release for %s: %w", spec.Repository, err)
	}
	downloadURL, err := releaseDownloadURL(spec, release)
	if err != nil {
		return Resource{}, err
	}
	archive, _, _, err := get(ctx, client, downloadURL, "")
	if err != nil {
		return Resource{}, fmt.Errorf("download release file %s: %w", downloadURL, err)
	}
	files, err := archiveFiles(spec.Path, archive)
	if err != nil {
		return Resource{}, err
	}
	return Resource{
		Spec:    spec,
		Files:   files,
		Record:  SourceRecord{URL: releaseURL, ETag: etag, Provenance: spec.Provenance},
		Updated: true,
	}, nil
}

func releaseDownloadURL(spec SourceSpec, release githubRelease) (string, error) {
	if spec.Kind == githubLatestReleaseArchive {
		if release.ZipballURL == "" {
			return "", fmt.Errorf("latest release of %s has no source ZIP", spec.Repository)
		}
		return release.ZipballURL, nil
	}
	for _, asset := range release.Assets {
		if asset.Name == spec.Asset && asset.BrowserDownloadURL != "" {
			return asset.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("latest release of %s has no asset %q", spec.Repository, spec.Asset)
}

func archiveFiles(targetPath string, archive []byte) (map[string][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("open source ZIP: %w", err)
	}
	files := make(map[string][]byte)
	for _, entry := range reader.File {
		name := strings.TrimPrefix(path.Clean(entry.Name), "./")
		parts := strings.Split(name, "/")
		if len(parts) < 3 || parts[1] != "flags" || parts[2] != "4x3" || !strings.HasSuffix(name, ".svg") {
			continue
		}
		body, err := readArchiveFile(entry)
		if err != nil {
			return nil, err
		}
		files[filepath.ToSlash(filepath.Join(targetPath, "4x3", path.Base(name)))] = body
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("source ZIP does not contain flags/4x3 SVG files")
	}
	return files, nil
}

func readArchiveFile(entry *zip.File) ([]byte, error) {
	if entry.UncompressedSize64 > 4<<20 {
		return nil, fmt.Errorf("archive entry %s is too large", entry.Name)
	}
	reader, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func readCachedSource(root, sourcePath string) (map[string][]byte, error) {
	base := filepath.Join(root, "sources", filepath.FromSlash(sourcePath))
	info, err := os.Stat(base)
	if err != nil {
		return nil, fmt.Errorf("read cached %s: %w", sourcePath, err)
	}
	if !info.IsDir() {
		body, err := os.ReadFile(base)
		if err != nil {
			return nil, err
		}
		return map[string][]byte{sourcePath: body}, nil
	}
	files := make(map[string][]byte)
	err = filepath.WalkDir(base, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		body, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(filepath.Join(root, "sources"), filePath)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = body
		return nil
	})
	return files, err
}

func get(ctx context.Context, client *http.Client, url, etag string) ([]byte, string, bool, error) {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err == nil {
			req.Header.Set("Accept", "application/vnd.github+json, application/json, application/xml, text/xml;q=0.9")
			if etag != "" {
				req.Header.Set("If-None-Match", etag)
			}
			resp, doErr := client.Do(req)
			if doErr == nil {
				body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
				resp.Body.Close()
				if readErr == nil && resp.StatusCode == http.StatusNotModified {
					return nil, etag, true, nil
				}
				if readErr == nil && resp.StatusCode == http.StatusOK {
					return body, resp.Header.Get("ETag"), false, nil
				}
				if readErr != nil {
					err = readErr
				} else {
					err = fmt.Errorf("unexpected HTTP status %s", resp.Status)
				}
			} else {
				err = doErr
			}
		}
		lastErr = err
		if attempt < 3 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}
	return nil, "", false, lastErr
}

func validContent(path string, body []byte) bool {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" || strings.HasPrefix(strings.ToLower(trimmed), "<!doctype html") || strings.HasPrefix(strings.ToLower(trimmed), "<html") {
		return false
	}
	if strings.HasSuffix(path, ".json") {
		return strings.HasPrefix(trimmed, "{")
	}
	return strings.Contains(trimmed, "<territory")
}
