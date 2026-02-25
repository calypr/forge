package publish

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/calypr/data-client/g3client"
	"github.com/calypr/data-client/sower"
	"github.com/calypr/forge/client"
	"github.com/calypr/forge/utils/gitutil"
	"github.com/calypr/git-drs/config"
)

// This job name must match the sower config otherwise job won't start
const FHIR_JOB_NAME = "fhir_import_export"
const SOURCE_GH_USER_ENDPOINT = "https://source.ohsu.edu/api/v3/user"
const POD_PUT_METHOD = "put"
const POD_DELETE_METHOD = "delete"

func RunEmpty(projectId string, remote config.Remote) (*sower.StatusResp, error) {
	sc, closer, err := client.NewGen3Client(
		remote, g3client.WithClients(g3client.SowerClient, g3client.FenceClient))
	if err != nil {
		return nil, err
	}
	defer closer()

	dispatchArgs := &sower.DispatchArgs{
		ProjectId:   sc.GetProjectId(),
		APIEndpoint: sc.GetGen3Interface().GetCredential().APIEndpoint,
		Profile:     sc.GetGen3Interface().GetCredential().Profile,
		Method:      POD_DELETE_METHOD,
	}
	resp, err := sc.GetGen3Interface().Sower().DispatchJob(
		context.Background(),
		FHIR_JOB_NAME,
		dispatchArgs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dispatch empty job. resp: %v err: %s", resp, err)
	}

	return resp, nil
}

func RunPublish(token string, profile config.Remote) (*sower.StatusResp, error) {
	err := checkGHPAccessToken(token)
	if err != nil {
		return nil, err
	}
	repo, err := gitutil.OpenRepository(".")
	if err != nil {
		return nil, err
	}
	// NOTE: hardcode to retrieve from git remote "origin"
	remote, err := repo.Remote(string(profile))
	if err != nil {
		return nil, fmt.Errorf("failed to get 'origin' remote: %w", err)
	}
	urls := remote.Config().URLs
	if len(urls) == 0 {
		return nil, fmt.Errorf("no URLs found for 'origin' remote")
	}
	if len(urls) > 1 {
		return nil, fmt.Errorf("not expecting more than 1 remote url. Got %d: %s", len(urls), urls)
	}
	remoteURL := urls[0]
	url, err := gitutil.TrimGitURLPrefix(remoteURL)
	if err != nil {
		return nil, err
	}

	// Validate that the repository is actually reachable before starting the job
	fmt.Printf("Validating access to repository: https://%s\n", url)
	if err := gitutil.ValidateGitURL(url, token); err != nil {
		return nil, fmt.Errorf("pre-publish validation failed: %w", err)
	}

	username, err := gitutil.GetGlobalUserIdentity()
	if err != nil {
		return nil, fmt.Errorf("Unable to read global git config to get username: %s", err)
	}
	hash, err := gitutil.GetLastLocalCommit(repo)
	if err != nil {
		return nil, err
	}
	sc, closer, err := client.NewGen3Client(profile, g3client.WithClients(g3client.SowerClient, g3client.FenceClient, g3client.IndexdClient))
	if err != nil {
		return nil, err
	}
	defer closer()

	// Check if any objects are indexed for this project on the remote
	// We use a short-lived context for the check but avoid immediate manual cancellation
	// to reduce noise in data-client logs while ensuring cleanup via defer.
	checkCtx, checkCancel := context.WithCancel(context.Background())
	defer checkCancel()
	recs, err := sc.ListObjectsByProject(checkCtx, sc.GetProjectId())
	if err == nil {
		hasRemoteRecords := false
		for res := range recs {
			if res.Error != nil {
				// If it's not a cancellation error, it's a real issue (e.g. 401 Unauthorized)
				if checkCtx.Err() == nil {
					fmt.Printf("\nDEBUG: Error checking for remote records: %v\n", res.Error)
				}
				continue
			}
			if res.Object != nil {
				hasRemoteRecords = true
				break
			}
		}

		if !hasRemoteRecords {
			fmt.Printf("\nWARNING: No files are indexed for project '%s' on remote '%s'.\n", sc.GetProjectId(), sc.GetGen3Interface().GetCredential().APIEndpoint)
			fmt.Println("The publish job likely won't produce any file related results. Metadata only projects will still be published. Use git-drs to upload and index files first if you intend on viewing metadata for existing files")
		}
	}

	dispatchArgs := &sower.DispatchArgs{
		BucketName:     sc.GetBucketName(),
		ProjectId:      sc.GetProjectId(),
		APIEndpoint:    sc.GetGen3Interface().GetCredential().APIEndpoint,
		Profile:        sc.GetGen3Interface().GetCredential().Profile,
		Method:         POD_PUT_METHOD,
		GHPAccessToken: token,
		GHUserName:     username,
		GHRepoURL:      url,
		GHCommitHash:   hash.String(),
	}

	resp, err := sc.GetGen3Interface().Sower().DispatchJob(
		context.Background(),
		FHIR_JOB_NAME,
		dispatchArgs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dispatch job. resp: %v err: %s", resp, err)
	}
	return resp, nil
}

func checkGHPAccessToken(token string) error {
	req, err := http.NewRequest(http.MethodGet, SOURCE_GH_USER_ENDPOINT, nil)
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
