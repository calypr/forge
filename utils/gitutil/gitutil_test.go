package gitutil

import (
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
)

func TestResolveGitRemoteName(t *testing.T) {
	newRepo := func(t *testing.T, remoteNames ...string) *git.Repository {
		t.Helper()
		repo, err := git.PlainInit(t.TempDir(), false)
		if err != nil {
			t.Fatalf("init repository: %v", err)
		}
		for _, name := range remoteNames {
			if _, err := repo.CreateRemote(&config.RemoteConfig{
				Name: name,
				URLs: []string{"https://github.com/example/" + name + ".git"},
			}); err != nil {
				t.Fatalf("create remote %q: %v", name, err)
			}
		}
		return repo
	}

	t.Run("prefers origin", func(t *testing.T) {
		got, err := ResolveGitRemoteName(newRepo(t, "mirror", "origin"), "")
		if err != nil || got != "origin" {
			t.Fatalf("ResolveGitRemoteName() = %q, %v; want origin, nil", got, err)
		}
	})

	t.Run("uses the only remote", func(t *testing.T) {
		got, err := ResolveGitRemoteName(newRepo(t, "upstream"), "")
		if err != nil || got != "upstream" {
			t.Fatalf("ResolveGitRemoteName() = %q, %v; want upstream, nil", got, err)
		}
	})

	t.Run("requires an override for multiple non-origin remotes", func(t *testing.T) {
		_, err := ResolveGitRemoteName(newRepo(t, "mirror", "upstream"), "")
		if err == nil || !strings.Contains(err.Error(), "specify one with --git-remote") {
			t.Fatalf("ResolveGitRemoteName() error = %v; want a remote-selection error", err)
		}
	})
}

func TestTrimGitURLPrefix(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		expected string
		wantErr  bool
	}{
		{
			name:     "HTTPS URL without token",
			rawURL:   "https://github.com/user/repo.git",
			expected: "github.com/user/repo",
			wantErr:  false,
		},
		{
			name:     "HTTPS URL with token",
			rawURL:   "https://ghp_token123@github.com/user/repo.git",
			expected: "github.com/user/repo",
			wantErr:  false,
		},
		{
			name:     "SSH URL",
			rawURL:   "git@github.com:user/repo.git",
			expected: "github.com/user/repo",
			wantErr:  false,
		},
		{
			name:     "Custom user with token and slash (reported leak)",
			rawURL:   "matthewpeterkort/ghp_vk18ll75@source.ohsu.edu/CBDS/git-drs-e2e-test/",
			expected: "source.ohsu.edu/CBDS/git-drs-e2e-test",
			wantErr:  false,
		},
		{
			name:     "SSH URL over 443",
			rawURL:   "git@ssh.github.com:443/EllrottLab/hla2vec.git",
			expected: "github.com/EllrottLab/hla2vec",
			wantErr:  false,
		},
		{
			name:     "Invalid empty result",
			rawURL:   "https://@",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TrimGitURLPrefix(tt.rawURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("TrimGitURLPrefix() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("TrimGitURLPrefix() got = %v, want %v", got, tt.expected)
			}
		})
	}
}
