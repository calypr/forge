package gitutil

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

// OpenRepository opens the Git repository at the given path.
func OpenRepository(path string) (*git.Repository, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open git repository at %s: %w", path, err)
	}
	return repo, nil
}

func GetLastLocalCommit(repo *git.Repository) (plumbing.Hash, error) {
	headRef, err := repo.Head()
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	// The hash of the HEAD reference is the hash of the latest commit.
	// You don't need to check if it's a branch or not, as HEAD always
	// points to the last commit, even in a detached state.
	return headRef.Hash(), nil
}

func GetGlobalUserIdentity() (string, error) {
	cfg, err := config.LoadConfig(config.GlobalScope)
	if err != nil {
		return "", fmt.Errorf("failed to load global git config: %w", err)
	}
	userName := cfg.User.Name
	if userName == "" {
		return "", fmt.Errorf("user.name not found in global git configuration %#v", cfg)
	}
	return userName, nil
}

// TrimGitURLPrefix removes the protocol prefix (https:// or git@) from a Git URL
// and then replaces the first colon with a slash to make it a valid HTTP URL.
// This is useful for converting SSH-style URLs (git@host:repo.git) into
// a format that can be used with HTTPS authentication.
func TrimGitURLPrefix(rawURL string) (string, error) {
	trimmedURL := rawURL

	// 1. Strip protocol if present (http, https, git, ssh)
	prefixes := []string{"https://", "http://", "git://", "ssh://"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(trimmedURL, prefix) {
			trimmedURL = strings.TrimPrefix(trimmedURL, prefix)
			break
		}
	}

	// 2. Strip user info if present (anything before the LAST @)
	// This handles standard "user:pass@host" and custom "user/token@host"
	if idx := strings.LastIndex(trimmedURL, "@"); idx != -1 {
		trimmedURL = trimmedURL[idx+1:]
	}

	// 3. Handle SSH-style "host:path/to/repo" by converting to "host/path/to/repo"
	// Only replace the FIRST colon (which separates host from path).
	// We don't want to replace all colons if there's a port or something else,
	// but for git remotes, the first colon is typically the host/path separator.
	trimmedURL = strings.Replace(trimmedURL, ":", "/", 1)

	// 4. Clean up trailing .git suffix
	trimmedURL = strings.TrimSuffix(trimmedURL, ".git")

	// 5. Trim trailing slash if present
	trimmedURL = strings.TrimSuffix(trimmedURL, "/")

	if trimmedURL == "" {
		return "", fmt.Errorf("resultant URL after trimming is empty for input: %s", rawURL)
	}

	return trimmedURL, nil
}
