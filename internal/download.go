package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const requestTimeout = 30 * time.Second

type downloadedSource struct {
	Files  map[string][]byte
	Record SourceRecord
}

func downloadAll(ctx context.Context, root string, specs []SourceSpec, previous SourceIndex) (map[string]downloadedSource, error) {
	client := &http.Client{Timeout: requestTimeout}
	results := make(map[string]downloadedSource, len(specs))
	for _, spec := range specs {
		result, err := download(ctx, root, client, spec, previous.Sources[spec.Path])
		if err != nil {
			return nil, err
		}
		results[spec.Path] = result
	}
	return results, nil
}

func download(ctx context.Context, root string, client *http.Client, spec SourceSpec, previous SourceRecord) (downloadedSource, error) {
	body, etag, unchanged, err := get(ctx, client, spec.URL, previous.ETag)
	if err != nil {
		return downloadedSource{}, fmt.Errorf("download %s: %w", spec.URL, err)
	}
	if unchanged {
		body, err = os.ReadFile(filepath.Join(root, "sources", filepath.FromSlash(spec.Path)))
		if err != nil {
			return downloadedSource{}, fmt.Errorf("read cached %s: %w", spec.Path, err)
		}
	}
	if !validContent(spec.Path, body) {
		return downloadedSource{}, fmt.Errorf("invalid content for %s", spec.Path)
	}
	return downloadedSource{Files: map[string][]byte{spec.Path: body}, Record: SourceRecord{URL: spec.URL, ETag: etag, Provenance: spec.Provenance}}, nil
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
