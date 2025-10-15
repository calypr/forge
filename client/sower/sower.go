package sower

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/calypr/forge/client"
)

const sowerDispatch = "/job/dispatch"

type Sower interface {
	DispatchJob(name string, args *DispatchArgs) (*DispatchResp, error)
}

func DispatchJob(s Sower, name string, args *DispatchArgs) (*DispatchResp, error) {
	return s.DispatchJob(name, args)
}

func NewSowerClient() (*SowerClient, error) {
	gen3Client, err := client.NewGen3Client()
	if err != nil {
		return nil, err
	}
	return &SowerClient{Gen3Client: gen3Client}, nil
}

type SowerClient struct {
	*client.Gen3Client
}

type File struct {
	FileTitle string `json:"fileTitle,omitempty"` // Filename of the uploaded artifact (e.g., zip file)
	FilePath  string `json:"filePath"`            // The path that will be used for fetching the file in the job
}

type DispatchArgs struct {
	Method         string `json:"method"`
	ProjectId      string `json:"projectId"`
	Profile        string `json:"profile"`
	BucketName     string `json:"bucketName"`
	APIEndpoint    string `json:"APIEndpoint`
	GHCommitHash   string `json:"ghCommitHash"`
	GHPAccessToken string `json:"ghToken"`
	GHUserName     string `json:"ghUserName"`
	GHRepoURL      string `json:"ghRepoUrl"`
}

type JobArgs struct {
	Input  DispatchArgs `json:"input"`
	Action string       `json:"action"`
}

func (sc *SowerClient) DispatchJob(name string, args *DispatchArgs) (*DispatchResp, error) {
	body := JobArgs{
		Action: name,
		Input:  *args,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	resp := sc.MakeReq(http.MethodPost, sowerDispatch, bodyBytes)
	if resp.Err != nil {
		return nil, resp.Err
	}
	fmt.Println("RESP: ", string(resp.Body))

	return nil, nil
}
