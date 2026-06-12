package remoteutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
)

func TestGetRemoteOrDefault(t *testing.T) {
	cfg := &RepoConfig{
		DefaultRemote: "origin",
		Remotes: map[string]RemoteConfig{
			"origin": {Name: "origin", ProjectID: "proj-a"},
			"prod":   {Name: "prod", ProjectID: "proj-b"},
		},
	}

	remote, err := cfg.GetRemoteOrDefault("")
	if err != nil || remote.Name != "origin" {
		t.Fatalf("default remote resolution failed: %+v %v", remote, err)
	}

	remote, err = cfg.GetRemoteOrDefault("prod")
	if err != nil || remote.Name != "prod" {
		t.Fatalf("explicit remote resolution failed: %+v %v", remote, err)
	}
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	_, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(filepath.Clean(dir)); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	for _, kv := range [][2]string{
		{defaultRemoteKey, "origin"},
		{"drs.remote.origin.project", "proj-a"},
		{"drs.remote.origin.endpoint", "https://example.org"},
		{"drs.remote.origin.bucket", "bucket-a"},
	} {
		cmd := exec.Command("git", "config", "--local", kv[0], kv[1])
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to set git config %s: %v (%s)", kv[0], err, string(out))
		}
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if loaded.DefaultRemote != "origin" {
		t.Fatalf("unexpected default remote: %q", loaded.DefaultRemote)
	}
	if loaded.Remotes["origin"].ProjectID != "proj-a" {
		t.Fatalf("unexpected project: %+v", loaded.Remotes["origin"])
	}
}

func TestMissingDefaultRemote(t *testing.T) {
	cfg := &RepoConfig{Remotes: map[string]RemoteConfig{}}
	if _, err := cfg.GetRemoteOrDefault(""); err == nil {
		t.Fatal("expected missing default remote error")
	}
}
