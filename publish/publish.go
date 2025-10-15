package publish

import (
	"fmt"
	"io"
	"net/http"

	"github.com/calypr/forge/client/sower"
	"github.com/calypr/forge/utils/gitutil"
)

// This job name must match the sower config otherwise job won't start
const FHIR_JOB_NAME = "fhir_import_export"
const SOURCE_GH_USER_ENDPOINT = "https://source.ohsu.edu/api/v3/user"
const POD_PUT_METHOD = "put"

func RunPublish(token string) error {
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
	hash, err := gitutil.GetLastLocalCommit(repo)
	if err != nil {
		return err
	}
	sc, err := sower.NewSowerClient()
	if err != nil {
		return err
	}

	dispatchArgs := &sower.DispatchArgs{
		BucketName:     sc.BucketName,
		ProjectId:      sc.ProjectId,
		APIEndpoint:    sc.Cred.APIEndpoint,
		Profile:        sc.Cred.Profile,
		Method:         POD_PUT_METHOD,
		GHPAccessToken: token,
		GHUserName:     username,
		GHRepoURL:      url,
		GHCommitHash:   hash.String(),
	}

	resp, err := sc.DispatchJob(
		FHIR_JOB_NAME,
		dispatchArgs,
	)
	if err != nil {
		return fmt.Errorf("failed to dispatch job: %w", err)
	}
	fmt.Println("Sower Dispatch Response: ", resp)

	return nil
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
