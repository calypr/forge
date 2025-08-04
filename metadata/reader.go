package metadata

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
)

func findMetaFiles(root string) ([]string, error) {
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

	for file, stat := range s {
		if !strings.HasSuffix(file, ".meta") {
			continue
		}

		if stat.Worktree == git.Modified || stat.Worktree == git.Untracked || stat.Staging == git.Added {
			fullPath := filepath.Join(root, file)
			metaFiles = append(metaFiles, fullPath)
		}
	}

	return metaFiles, nil
}
