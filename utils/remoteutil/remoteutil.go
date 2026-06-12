package remoteutil

import (
	"fmt"
	"sort"
	"strings"

	"github.com/calypr/forge/utils/gitutil"
)

const (
	defaultRemoteKey = "drs.default-remote"
	configSection    = "drs"
	remotePrefixSub  = "remote."
)

type RemoteConfig struct {
	Name          string
	Type          string
	Endpoint      string
	ProjectID     string
	BucketName    string
	Organization  string
	StoragePrefix string
}

func (r RemoteConfig) DispatchProjectID() string {
	project := strings.TrimSpace(r.ProjectID)
	organization := strings.TrimSpace(r.Organization)
	if project == "" {
		return ""
	}
	if organization == "" {
		return project
	}
	return organization + "-" + project
}

type RepoConfig struct {
	DefaultRemote string
	Remotes       map[string]RemoteConfig
}

func LoadConfig() (*RepoConfig, error) {
	repo, err := gitutil.OpenRepository(".")
	if err != nil {
		return nil, fmt.Errorf("unable to open repository: %w", err)
	}
	cfg, err := repo.Config()
	if err != nil {
		return nil, fmt.Errorf("unable to read repository config: %w", err)
	}

	out := &RepoConfig{
		Remotes: make(map[string]RemoteConfig),
	}

	for _, section := range cfg.Raw.Sections {
		if section.Name != configSection {
			continue
		}

		if v := strings.TrimSpace(section.Option("default-remote")); v != "" {
			out.DefaultRemote = v
		}

		for _, subsection := range section.Subsections {
			name := strings.TrimSpace(subsection.Name)
			if !strings.HasPrefix(name, remotePrefixSub) {
				continue
			}
			remoteName := strings.TrimSpace(strings.TrimPrefix(name, remotePrefixSub))
			if remoteName == "" {
				continue
			}
			remoteCfg := out.Remotes[remoteName]
			remoteCfg.Name = remoteName
			remoteCfg.Type = strings.TrimSpace(subsection.Option("type"))
			remoteCfg.Endpoint = strings.TrimSpace(subsection.Option("endpoint"))
			remoteCfg.ProjectID = strings.TrimSpace(subsection.Option("project"))
			remoteCfg.BucketName = strings.TrimSpace(subsection.Option("bucket"))
			remoteCfg.Organization = strings.TrimSpace(subsection.Option("organization"))
			remoteCfg.StoragePrefix = strings.TrimSpace(subsection.Option("storage_prefix"))
			out.Remotes[remoteName] = remoteCfg
		}
	}

	return out, nil
}

func (c *RepoConfig) GetRemoteOrDefault(remoteName string) (*RemoteConfig, error) {
	name := strings.TrimSpace(remoteName)
	if name == "" {
		name = strings.TrimSpace(c.DefaultRemote)
		if name == "" {
			if len(c.Remotes) == 1 {
				for onlyName := range c.Remotes {
					name = onlyName
				}
			}
		}
		if name == "" {
			if len(c.Remotes) == 0 {
				return nil, fmt.Errorf("no git-drs remotes configured under [%s %q]", configSection, remotePrefixSub+"<name>")
			}
			names := make([]string, 0, len(c.Remotes))
			for remoteName := range c.Remotes {
				names = append(names, remoteName)
			}
			sort.Strings(names)
			return nil, fmt.Errorf("no git-drs default remote configured in %q; available git-drs remotes: %s", defaultRemoteKey, strings.Join(names, ", "))
		}
	}

	remote, ok := c.Remotes[name]
	if !ok {
		return nil, fmt.Errorf("git-drs remote %q not found", name)
	}
	if strings.TrimSpace(remote.ProjectID) == "" {
		return nil, fmt.Errorf("git-drs remote %q is missing project configuration", name)
	}
	return &remote, nil
}

func LoadRemoteOrDefault(remoteName string) (*RemoteConfig, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	remote, err := cfg.GetRemoteOrDefault(remoteName)
	if err != nil {
		return nil, fmt.Errorf("could not locate git remote config: %w", err)
	}
	return remote, nil
}
