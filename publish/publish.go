package publish

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/calypr/calypr-cli/g3client"
	"github.com/calypr/calypr-cli/sower"
	"github.com/calypr/forge/client"
	"github.com/calypr/forge/utils/gitutil"
	"github.com/calypr/forge/utils/remoteutil"
	syservices "github.com/calypr/syfon/client/services"
)

// This job name must match the sower config otherwise job won't start
const FHIR_JOB_NAME = "fhir_import_export"
const POD_PUT_METHOD = "put"
const POD_DELETE_METHOD = "delete"

func RunEmpty(projectId string, profileName string) (*sower.StatusResp, error) {
	sc, closer, err := client.NewGen3Client(
		profileName, g3client.WithClients(g3client.SowerClient, g3client.FenceClient))
	if err != nil {
		return nil, err
	}
	defer closer()

	dispatchArgs := &sower.DispatchArgs{
		ProjectId:   projectId,
		APIEndpoint: sc.Credential().APIEndpoint,
		Profile:     sc.Credential().Profile,
		Method:      POD_DELETE_METHOD,
	}
	resp, err := sc.Gen3.SowerClient().DispatchJob(
		context.Background(),
		FHIR_JOB_NAME,
		dispatchArgs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dispatch empty job. resp: %v err: %s", resp, err)
	}

	return resp, nil
}

func RunPublish(token string, profileName string, gitRemoteName string) (*sower.StatusResp, error) {
	repoRemoteConfig, err := remoteutil.LoadRemoteOrDefault(gitRemoteName)
	if err != nil {
		return nil, err
	}

	repo, err := gitutil.OpenRepository(".")
	if err != nil {
		return nil, err
	}
	resolvedGitRemoteName, err := gitutil.ResolveGitRemoteName(repo, gitRemoteName)
	if err != nil {
		return nil, err
	}

	remote, err := repo.Remote(resolvedGitRemoteName)
	if err != nil {
		return nil, fmt.Errorf("failed to get git remote '%s': %w", resolvedGitRemoteName, err)
	}
	urls := remote.Config().URLs
	if len(urls) == 0 {
		return nil, fmt.Errorf("no URLs found for git remote '%s'", resolvedGitRemoteName)
	}
	if len(urls) > 1 {
		return nil, fmt.Errorf("not expecting more than 1 remote url. Got %d: %v", len(urls), urls)
	}
	remoteURL := urls[0]
	url, err := gitutil.TrimGitURLPrefix(remoteURL)
	if err != nil {
		return nil, err
	}

	// Determine the correct GitHub API endpoint and validate the token.
	// We use this call to also retrieve the actual username (login) associated with the token.
	apiEndpoint := getGitHubAPIEndpoint(url)
	username, err := checkGHPAccessToken(token, apiEndpoint)
	if err != nil {
		return nil, err
	}

	// Validate that the repository is actually reachable before starting the job
	fmt.Printf("Validating access to repository: https://%s\n", url)
	if err := gitutil.ValidateGitURL(url, token); err != nil {
		return nil, fmt.Errorf("pre-publish validation failed: %w", err)
	}

	hash, err := gitutil.GetLastLocalCommit(repo)
	if err != nil {
		return nil, err
	}
	sc, closer, err := client.NewGen3Client(profileName, g3client.WithClients(g3client.SowerClient, g3client.FenceClient, g3client.SyfonClient))
	if err != nil {
		return nil, err
	}
	defer closer()

	checkCtx, checkCancel := context.WithCancel(context.Background())
	defer checkCancel()
	page, err := sc.Gen3.SyfonClient().Index().List(checkCtx, syservices.ListRecordsOptions{
		Organization: repoRemoteConfig.Organization,
		ProjectID:    repoRemoteConfig.ProjectID,
		Limit:        1,
		Page:         1,
	})
	if err == nil && (page.Records == nil || len(*page.Records) == 0) {
		fmt.Printf("\nWARNING: No files are indexed for project '%s' on profile '%s'.\n", repoRemoteConfig.DispatchProjectID(), sc.Credential().Profile)
		fmt.Println("The publish job likely won't produce any file related results. Metadata only projects will still be published. Use git-drs to upload and index files first if you intend on viewing metadata for existing files")
	}

	dispatchArgs := &sower.DispatchArgs{
		BucketName:     repoRemoteConfig.BucketName,
		ProjectId:      repoRemoteConfig.DispatchProjectID(),
		APIEndpoint:    sc.Credential().APIEndpoint,
		Profile:        sc.Credential().Profile,
		Method:         POD_PUT_METHOD,
		GHPAccessToken: token,
		GHUserName:     username,
		GHRepoURL:      url,
		GHCommitHash:   hash.String(),
	}

	resp, err := sc.Gen3.SowerClient().DispatchJob(
		context.Background(),
		FHIR_JOB_NAME,
		dispatchArgs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dispatch job. resp: %v err: %s", resp, err)
	}
	return resp, nil
}

func checkGHPAccessToken(token string, apiEndpoint string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, apiEndpoint, nil)
	if err != nil {
		return "", fmt.Errorf("Error creating request: %s\n", err)
	}
	req.Header.Set("Authorization", "token "+token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Error sending request: %s\n", err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Error reading response body: %s\n", err)
	}

	if resp.StatusCode == http.StatusOK {
		var user struct {
			Login string `json:"login"`
		}
		if err := json.Unmarshal(body, &user); err != nil {
			return "", fmt.Errorf("Error parsing user info: %s\n", err)
		}
		if user.Login == "" {
			return "", fmt.Errorf("Could not find 'login' field in user info response")
		}
		return user.Login, nil
	} else if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("Error: The personal access token is invalid or expired.")
	} else if resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("\nError: The personal access token is valid, but lacks the necessary permissions (scopes) to access this resource.")
	} else {
		return "", fmt.Errorf("\nUnexpected response status: %d\n", resp.StatusCode)
	}
}

func getGitHubAPIEndpoint(normalizedURL string) string {
	if strings.HasPrefix(normalizedURL, "source.ohsu.edu") {
		return "https://source.ohsu.edu/api/v3/user"
	}
	return "https://api.github.com/user"
}
