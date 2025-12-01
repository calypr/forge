package client

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	token "github.com/bmeg/grip-graphql/middleware"
	"github.com/calypr/git-drs/config"
	drsConfig "github.com/calypr/git-drs/config"

	"github.com/calypr/data-client/client/commonUtils"
	"github.com/calypr/data-client/client/jwt"
)

type Gen3Client struct {
	Base       *url.URL
	Cred       jwt.Credential
	ProjectId  string
	BucketName string
}

// load repo-level config and return a new IndexDClient
func NewGen3Client(remote config.Remote) (*Gen3Client, error) {
	var conf jwt.Configure

	cfg, err := drsConfig.LoadConfig()
	if err != nil {
		return nil, err
	}
	gfc, ok := cfg.Remotes[remote]
	if !ok {
		return nil, fmt.Errorf("remote %s not found in config: %v", remote, cfg.Remotes)
	}

	cred, err := conf.ParseConfig(string(remote))
	if err != nil {
		return nil, err
	}

	baseUrl, err := url.Parse(cred.APIEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error parsing base URL from profile %s: %v", remote, err)
	}

	// get the gen3Project and gen3Bucket from the config
	projectId := gfc.Gen3.ProjectID
	if projectId == "" {
		return nil, fmt.Errorf("No gen3 project specified. Please provide a gen3Project key in your .drsconfig")
	}
	bucketName := gfc.Gen3.Bucket
	if bucketName == "" {
		return nil, fmt.Errorf("No gen3 bucket specified. Please provide a gen3Bucket key in your .drsconfig")
	}
	return &Gen3Client{Base: baseUrl, Cred: cred, ProjectId: projectId, BucketName: bucketName}, err
}

type Resp struct {
	Body []byte
	Err  error
}

func (cl *Gen3Client) MakeReq(method string, path string, body []byte, params map[string]string) *Resp {
	a := *cl.Base
	a.Path = filepath.Join(a.Path, path)

	var reqBodyReader io.Reader
	if body != nil {
		reqBodyReader = bytes.NewBuffer(body)
	}

	req, err := http.NewRequest(method, a.String(), reqBodyReader)
	if err != nil {
		return &Resp{nil, err}
	}

	q := req.URL.Query()
	for key, val := range params {
		q.Add(key, val)
	}
	req.URL.RawQuery = q.Encode()

	expiration, err := token.GetExpiration(cl.Cred.AccessToken)
	if err != nil {
		return &Resp{nil, err}
	}
	// Update AccessToken if token is old
	if expiration.Before(time.Now()) {
		r := jwt.Request{}
		err := r.RequestNewAccessToken(cl.Base.String()+commonUtils.FenceAccessTokenEndpoint, &cl.Cred)
		if err != nil {
			return &Resp{nil, err}
		}
	}
	if cl.Cred.AccessToken == "" {
		return &Resp{nil, fmt.Errorf("access token not found in profile config")}
	}
	req.Header.Set("Authorization", "bearer "+cl.Cred.AccessToken)

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return &Resp{nil, err}
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return &Resp{nil, fmt.Errorf("failed to check authz, response body: %v", response)}
	}

	RespBody, err := io.ReadAll(response.Body)
	if err != nil {
		return &Resp{nil, fmt.Errorf("failed to read response Body")}
	}

	return &Resp{RespBody, nil}
}
