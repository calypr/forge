package remoteutil

import (
	"fmt"

	"github.com/calypr/git-drs/config"
)

// LoadRemoteOrDefault loads the git-drs config and returns the specified remote
// or the default remote if remoteName is empty.
func LoadRemoteOrDefault(remoteName string) (*config.Remote, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to load config: %w", err)
	}
	remote, err := cfg.GetRemoteOrDefault(remoteName)
	if err != nil {
		return nil, fmt.Errorf("could not locate remote: %w", err)
	}

	return &remote, nil
}
