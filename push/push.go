package push

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/calypr/forge/client/sower"
	"github.com/calypr/forge/utils/gitutil"
)

func RunPush(token string) error {
	err := checkGHPAccessToken(token)
	if err != nil {
		return err
	}
	repo, err := gitutil.OpenRepository(".")
	if err != nil {
		return err
	}
	remote, err := repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("failed to get 'origin' remote: %w", err)
	}
	urls := remote.Config().URLs
	if len(urls) == 0 {
		return fmt.Errorf("no URLs found for 'origin' remote")
	}
	if len(urls) > 1 {
		return fmt.Errorf("not expecting more than 1 remote url. Got %d: %s", len(urls), urls)
	}
	remoteURL := urls[0]
	url, err := gitutil.TrimGitURLPrefix(remoteURL)
	if err != nil {
		return err
	}

	username, err := gitutil.GetGlobalUserIdentity()
	if err != nil {
		return fmt.Errorf("Unable to read global git config to get username: %s", err)
	}

	hashes, err := gitutil.GetUnpushedCommits(repo)
	if err != nil {
		return err
	}
	if len(hashes) == 0 {
		fmt.Println("No unpushed commits found. Nothing to push.")
		return nil
	}

	sc, err := sower.NewSowerClient()
	if err != nil {
		return err
	}

	var commitDetails []sower.CommitDetail
	// All unpushed local commits are sent to the job with snapshot location
	for _, hash := range hashes {
		hashDir := filepath.Join(".forge", "snapshots", hash.String())
		fmt.Printf("Searching for files in: %s\n", hashDir)

		commitDetail := sower.CommitDetail{
			CommitId: hash.String(),
		}
		foundSnapshot := false
		if _, err := os.Stat(hashDir); !os.IsNotExist(err) {
			entries, err := os.ReadDir(hashDir)
			if err != nil {
				return fmt.Errorf("failed to read commit directory %s: %w", hashDir, err)
			}

			for _, entry := range entries {
				if !entry.IsDir() {
					commitDetail.FileTitle = entry.Name()
					commitDetail.RepoUrl = url
					commitDetail.FilePath = filepath.Join(hashDir, entry.Name())
					foundSnapshot = true
					break // We only expect one snapshot file per commit
				}
			}
		}
		commitDetails = append(commitDetails, commitDetail)
		if foundSnapshot {
			fmt.Printf("Found snapshot for commit %s: %s\n", hash.String(), commitDetail.FileTitle)
		} else {
			fmt.Printf("No snapshot found for commit %s. Adding commit hash only.\n", hash.String())
		}
	}

	dispatchArgs := &sower.DispatchArgs{
		Push: sower.PushDetails{
			Commits: commitDetails,
		},
		ProjectID:      sc.ProjectId,
		Method:         "put",
		GHPAccessToken: token,
		GHUserName:     username,
	}

	resp, err := sc.DispatchJob(
		"fhir_import_export",
		dispatchArgs,
	)
	if err != nil {
		return fmt.Errorf("failed to dispatch job: %w", err)
	}
	fmt.Println("Sower Dispatch Response: ", resp)

	return nil
}

func hashFileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close() // Ensure the file is closed

	hasher := sha256.New()

	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to copy file content to hasher: %w", err)
	}

	hashInBytes := hasher.Sum(nil)
	hashString := hex.EncodeToString(hashInBytes)

	return hashString, nil
}

func checkGHPAccessToken(token string) error {

	req, err := http.NewRequest("GET", "https://source.ohsu.edu/api/v3/user", nil)
	if err != nil {
		return fmt.Errorf("Error creating request: %s\n", err)
	}
	req.Header.Set("Authorization", "token "+token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Error sending request: %s\n", err)
	}

	defer resp.Body.Close()
	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Error reading response body: %s\n", err)
	}

	if resp.StatusCode == http.StatusOK {
		return nil
	} else if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("Error: The personal access token is invalid or expired.")
	} else if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("\nError: The personal access token is valid, but lacks the necessary permissions (scopes) to access this resource.")
	} else {
		return fmt.Errorf("\nUnexpected response status: %d\n", resp.StatusCode)
	}
}
