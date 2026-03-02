package gitutil

import (
	"testing"
)

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
