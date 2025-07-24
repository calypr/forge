package template

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func findMetaFiles(root string) ([]string, error) {
	var metaFiles []string

	err := fs.WalkDir(os.DirFS(root), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error accessing path %q: %v\n", path, err)
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".meta") {
			fullPath := filepath.Join(root, path)
			metaFiles = append(metaFiles, fullPath)
		}
		return nil
	})

	return metaFiles, err
}
