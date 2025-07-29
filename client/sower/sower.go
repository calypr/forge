package sower

import (
	"encoding/json"
	"net/http"

	"github.com/calypr/forge/client"
)

const sowerDispatch = "/dispatch"

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

type Commits struct {
	CommitId string `json:"commit_id"`
	ObjectId string `json:"object_id"`
	MetaPath string `json:"meta_path"`
}

type Push struct {
	Commits []Commits `json:"commits"`
}

type DispatchArgs struct {
	Push      Push   `json:"push"`
	ProjectID string `json:"project_id"`
	Method    string `json:"method"`
}

type JobArgs struct {
	Input  DispatchArgs `json:"input"`
	Action string       `json:"action"`
}

func (sc *SowerClient) DispatchJob(name string, args *DispatchArgs) (*DispatchResp, error) {
	body := JobArgs{
		Action: "fhir_import_export",
		Input: DispatchArgs{
			Push:      Push{},
			ProjectID: sc.ProjectId,
			Method:    "put",
		},
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	resp := sc.MakeReq(http.MethodPost, sowerDispatch, bodyBytes)
	if resp.Err != nil {
		return nil, resp.Err
	}

	return nil, nil
}
