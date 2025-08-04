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

type CommitDetail struct {
	ObjectId  string `json:"objectId"`            // Corresponds to git.Commit.Hash().String()
	FileName  string `json:"fileName,omitempty"`  // Filename of the uploaded artifact (e.g., zip file)
	RemoteURL string `json:"remoteURL,omitempty"` // URL where the uploaded file is accessible
}

type PushDetails struct {
	Commits []CommitDetail `json:"commits"`
}

type DispatchArgs struct {
	Push      PushDetails `json:"push"`
	ProjectID string      `json:"projectId"`
	Method    string      `json:"method"`
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
