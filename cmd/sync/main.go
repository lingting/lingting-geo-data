package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lingting/lingting-geo-data/internal"
)

func main() {
	root, err := filepath.Abs(".")
	if err != nil {
		fail(err)
	}
	changed, err := internal.Sync(context.Background(), root)
	if err != nil {
		fail(err)
	}
	if changed {
		fmt.Println("changes detected")
		return
	}
	fmt.Println("no changes")
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "sync failed: %v\n", err)
	os.Exit(1)
}
