package gitutil

import (
	"fmt"
	"net/http"
	"strings"
	"time"

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

	// 1. Strip protocol if present (http, https, git, ssh, git+ssh)
	prefixes := []string{"https://", "http://", "git://", "ssh://", "git+ssh://"}
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

	// 4. Handle GitHub SSH-over-443 specifically (ssh.github.com and altssh.github.com)
	// Normalize them back to canonical github.com
	ghSSHHosts := []string{"ssh.github.com/", "altssh.github.com/"}
	for _, host := range ghSSHHosts {
		if strings.HasPrefix(trimmedURL, host+"443/") {
			trimmedURL = "github.com/" + strings.TrimPrefix(trimmedURL, host+"443/")
			break
		}
		if strings.HasPrefix(trimmedURL, host) {
			trimmedURL = "github.com/" + strings.TrimPrefix(trimmedURL, host)
			break
		}
	}

	// 5. Clean up trailing .git suffix
	trimmedURL = strings.TrimSuffix(trimmedURL, ".git")

	// 6. Trim trailing slash if present
	trimmedURL = strings.TrimSuffix(trimmedURL, "/")

	if trimmedURL == "" {
		return "", fmt.Errorf("resultant URL after trimming is empty for input: %s", rawURL)
	}

	return trimmedURL, nil
}

// ValidateGitURL checks if the provided normalized Git URL is reachable via HTTPS.
// If a token is provided, it uses it for authentication.
func ValidateGitURL(normalizedURL string, token string) error {
	// Reconstruct a full HTTPS URL for validation
	validationURL := "https://" + normalizedURL
	req, err := http.NewRequest("HEAD", validationURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create validation request for %s: %w", validationURL, err)
	}

	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach Git repository at %s: %w", validationURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return nil
	}

	return fmt.Errorf("Git repository at %s returned status %s (check if the repository exists or if your token has access)", validationURL, resp.Status)
}
