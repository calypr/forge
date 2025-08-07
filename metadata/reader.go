package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
)

func findMetaFiles(root string, rebuild bool) ([]string, error) {
	if rebuild {
		// Case 1: Rebuild is true. We need to find ALL .meta files, regardless of Git status.
		var metaFiles []string
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), FILE_META_EXT) {
				metaFiles = append(metaFiles, path)
			}
			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("failed to walk directory %s: %w", root, err)
		}
		return metaFiles, nil
	}

	// Case 2: Rebuild is false. We only care about files with an active Git status.
	repo, err := git.PlainOpen(root)
	if err != nil {
		return nil, fmt.Errorf("failed to open Git repository at %s: %w", root, err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	s, err := worktree.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree status: %w", err)
	}

	var metaFiles []string

	// The `for` loop logic from your original function is actually correct for this case.
	// We'll iterate over the status map, which only contains changed/untracked files.
	for file, stat := range s {
		if !strings.HasSuffix(file, FILE_META_EXT) {
			continue
		}

		// Your original condition covers modified and untracked files.
		// Let's use the simplified and more robust condition I suggested previously
		// to also catch staged-but-not-committed changes.
		if stat.Worktree != git.Unmodified || stat.Staging != git.Unmodified {
			fullPath := filepath.Join(root, file)
			metaFiles = append(metaFiles, fullPath)
		}
	}

	return metaFiles, nil
}
