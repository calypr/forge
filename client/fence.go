package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/calypr/data-client/data-client/commonUtils"
	"github.com/calypr/data-client/data-client/jwt"
	drsClient "github.com/calypr/git-drs/client"
)

// FenceBucketEndpoint is is the endpoint postfix for FENCE bucket list
const FenceBucketEndpoint = "/user/data/buckets"

type Authz struct {
	Resource string
	Method   string
	Service  string
}

type AuthzResponse struct {
	Ok    bool
	Error error
}

type Fence interface {
	UserPing() (*PingResp, error)
	AuthzCheck(authz *Authz) *AuthzResponse
}

func UserPing(a Fence) (*PingResp, error) {
	return a.UserPing()
}

type FenceOBJ struct {
	Base       *url.URL
	Cred       jwt.Credential
	ProjectId  string
	BucketName string
}

// load repo-level config and return a new IndexDClient
func NewFenceClient() (*FenceOBJ, error) {
	var conf jwt.Configure

	cfg, err := drsClient.LoadConfig()
	if err != nil {
		return nil, err
	}

	profile := cfg.Gen3Profile
	if profile == "" {
		return nil, fmt.Errorf("No gen3 profile specified. Please provide a gen3Profile key in your .drsconfig")
	}

	cred, err := conf.ParseConfig(profile)
	if err != nil {
		return nil, err
	}

	baseUrl, err := url.Parse(cred.APIEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error parsing base URL from profile %s: %v", profile, err)
	}

	// get the gen3Project and gen3Bucket from the config
	projectId := cfg.Gen3Project
	if projectId == "" {
		return nil, fmt.Errorf("No gen3 project specified. Please provide a gen3Project key in your .drsconfig")
	}

	bucketName := cfg.Gen3Bucket
	if bucketName == "" {
		return nil, fmt.Errorf("No gen3 bucket specified. Please provide a gen3Bucket key in your .drsconfig")
	}

	return &FenceOBJ{Base: baseUrl, Cred: cred, ProjectId: projectId, BucketName: bucketName}, err
}

func (cl *FenceOBJ) UserPing() (*PingResp, error) {

	reqResp := cl.makeGen3Req(http.MethodGet, commonUtils.FenceUserEndpoint)
	if reqResp.Body == nil || reqResp.Err != nil {
		return nil, reqResp.Err
	}
	var uResp FenceUserResp
	err := json.Unmarshal(reqResp.Body, &uResp)
	if err != nil {
		return nil, err
	}

	bucketResp := cl.makeGen3Req(http.MethodGet, FenceBucketEndpoint)
	if reqResp.Body == nil || reqResp.Err != nil {
		return nil, bucketResp.Err
	}
	var bResp BucketResp
	err = json.Unmarshal(bucketResp.Body, &bResp)
	if err != nil {
		return nil, err
	}

	return &PingResp{
		Profile:        cl.Cred.Profile,
		Username:       uResp.Username,
		Endpoint:       cl.Cred.APIEndpoint,
		BucketPrograms: ParseBucketResp(bResp),
		YourAccess:     ParseUserResp(uResp),
	}, nil

}
