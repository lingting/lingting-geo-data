package internal

import (
	"fmt"
	"os"
	"path/filepath"
)

func validateGeneratedData(root string) error {
	for _, path := range []string{
		filepath.Join(root, "generated", "sources.json"),
		filepath.Join(root, "generated", "countries.json"),
		filepath.Join(root, "generated", "flags"),
	} {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("validate generated data %s: %w", path, err)
		}
		if info.IsDir() && path == filepath.Join(root, "generated", "flags") {
			entries, err := os.ReadDir(path)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				return fmt.Errorf("validate generated data: flags directory is empty")
			}
		}
	}
	return nil
}
