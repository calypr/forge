package fence

import (
	"encoding/json"
	"net/http"

	"github.com/calypr/data-client/client/commonUtils"
	"github.com/calypr/forge/client"
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

func UserPing(f Fence) (*PingResp, error) {
	return f.UserPing()
}

func NewFenceClient() (*FenceClient, error) {
	gen3Client, err := client.NewGen3Client()
	if err != nil {
		return nil, err
	}
	return &FenceClient{Gen3Client: gen3Client}, nil
}

type FenceClient struct {
	*client.Gen3Client
}

func (fc *FenceClient) UserPing() (*PingResp, error) {
	reqResp := fc.MakeReq(http.MethodGet, commonUtils.FenceUserEndpoint, nil)
	if reqResp.Body == nil || reqResp.Err != nil {
		return nil, reqResp.Err
	}
	var uResp FenceUserResp
	err := json.Unmarshal(reqResp.Body, &uResp)
	if err != nil {
		return nil, err
	}

	bucketResp := fc.MakeReq(http.MethodGet, FenceBucketEndpoint, nil)
	if reqResp.Body == nil || reqResp.Err != nil {
		return nil, bucketResp.Err
	}
	var bResp BucketResp
	err = json.Unmarshal(bucketResp.Body, &bResp)
	if err != nil {
		return nil, err
	}

	return &PingResp{
		Profile:        fc.Cred.Profile,
		Username:       uResp.Username,
		Endpoint:       fc.Cred.APIEndpoint,
		BucketPrograms: ParseBucketResp(bResp),
		YourAccess:     ParseUserResp(uResp),
	}, nil

}
