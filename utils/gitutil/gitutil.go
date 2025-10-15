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
	// Trim the https:// prefix
	const httpsPrefix = "https://"
	if strings.HasPrefix(rawURL, httpsPrefix) {
		trimmedURL := strings.TrimPrefix(rawURL, httpsPrefix)
		return strings.Replace(trimmedURL, ":", "/", 1), nil
	}

	// Trim the git@ prefix
	const sshPrefix = "git@"
	if strings.HasPrefix(rawURL, sshPrefix) {
		trimmedURL := strings.TrimPrefix(rawURL, sshPrefix)
		return strings.Replace(trimmedURL, ":", "/", 1), nil
	}

	// If no recognized prefix is found, return an error.
	return rawURL, fmt.Errorf("Expecting either https:// prefix or ssh prefix (git@) but got %s instead", rawURL)
}
