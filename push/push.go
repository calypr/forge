package push

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/calypr/forge/client"
	"github.com/calypr/forge/client/sower"
	"github.com/calypr/forge/utils/gitutil"
)

func RunPush() error {
	repo, err := gitutil.OpenRepository(".")
	if err != nil {
		return err
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

	cli, err := client.NewGen3Client()
	if err != nil {
		return err
	}

	for _, hash := range hashes {
		hashDir := filepath.Join(".forge", "snapshots", hash.String())
		fmt.Printf("Searching for zip files in: %s\n", hashDir)

		if _, err := os.Stat(hashDir); os.IsNotExist(err) {
			fmt.Printf("Warning: Commit directory %s does not exist. Skipping zip file upload for this commit.\n", hashDir)
			// Even if no zip file, we still want to include the commit hash in the dispatch
			commitDetails = append(commitDetails, sower.CommitDetail{
				ObjectId: hash.String(),
			})
			continue
		}

		entries, err := os.ReadDir(hashDir)
		if err != nil {
			return fmt.Errorf("failed to read commit directory %s: %w", hashDir, err)
		}

		foundZipForCommit := false
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".zip") {
				zipFilePath := filepath.Join(hashDir, entry.Name())
				fmt.Printf("Found zip file: %s\n", zipFilePath)

				zipHash, err := hashFileSHA256(zipFilePath)
				if err != nil {
					return err
				}

				if err != nil {
					return fmt.Errorf("failed to upload zip file %s: %w", zipFilePath, err)
				}
				fmt.Printf("Uploaded %s with hash %s to %s\n", zipFilePath, zipHash, cli.BucketName)
				commitDetails = append(commitDetails, sower.CommitDetail{
					ObjectId: zipHash,
					FileName: entry.Name(),
				})
				foundZipForCommit = true
			}
		}

		if !foundZipForCommit {
			// If no zip file was found for this commit, but the commit itself exists,
			// add just the ObjectId. This handles cases where a commit has no associated metadata changes.
			commitDetails = append(commitDetails, sower.CommitDetail{
				ObjectId: hash.String(),
			})
		}
	}

	dispatchArgs := &sower.DispatchArgs{
		Push: sower.PushDetails{ // Use the named type here
			Commits: commitDetails,
		},
		ProjectID: sc.ProjectId,
		Method:    "put",
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
