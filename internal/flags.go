package internal

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// GenerateFlags 在旗帜源码更新后，以 shell 文件操作同步 4x3 SVG。
func GenerateFlags(root string, state SourceState) (bool, error) {
	if !state.Updated {
		return false, nil
	}
	sourceDir := filepath.ToSlash(filepath.Join(root, "sources", "flag-icons", "4x3"))
	generatedDir := filepath.ToSlash(filepath.Join(root, "generated", "flags"))
	command := `mkdir -p "$2" && rm -f "$2"/*.svg && cp "$1"/*.svg "$2"/`
	if output, err := exec.Command("sh", "-c", command, "flags", sourceDir, generatedDir).CombinedOutput(); err != nil {
		return false, fmt.Errorf("generate flags: %w: %s", err, output)
	}
	return true, nil
}
