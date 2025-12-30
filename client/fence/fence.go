package fence

import (
	"encoding/json"
	"net/http"

	"github.com/calypr/data-client/client/common"
	"github.com/calypr/forge/client"
	"github.com/calypr/git-drs/config"
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

func NewFenceClient(remote config.Remote) (*FenceClient, func(), error) {
	gen3Client, closer, err := client.NewGen3Client(remote)
	if err != nil {
		return nil, closer, err
	}
	return &FenceClient{Gen3Client: gen3Client}, closer, nil
}

type FenceClient struct {
	*client.Gen3Client
}

func (fc *FenceClient) UserPing() (*PingResp, error) {
	reqResp := fc.MakeReq(http.MethodGet, common.FenceUserEndpoint, nil, nil)
	if reqResp.Body == nil || reqResp.Err != nil {
		return nil, reqResp.Err
	}
	var uResp FenceUserResp
	err := json.Unmarshal(reqResp.Body, &uResp)
	if err != nil {
		return nil, err
	}

	bucketResp := fc.MakeReq(http.MethodGet, FenceBucketEndpoint, nil, nil)
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
