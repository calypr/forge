package sower

import (
	"encoding/json"
	"net/http"

	"github.com/calypr/forge/client"
)

const sowerDispatch = "/job/dispatch"
const sowerStatus = "/job/status"
const sowerList = "/job/list"
const sowerJobOutput = "/job/output"

type Sower interface {
	DispatchJob(name string, args *DispatchArgs) (*StatusResp, error)
	Status(uid string) (*StatusResp, error)
	List() (*[]StatusResp, error)
	Output(uid string) (*OutputResp, error)
}

func DispatchJob(s Sower, name string, args *DispatchArgs) (*StatusResp, error) {
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

func (sc *SowerClient) DispatchJob(name string, args *DispatchArgs) (*StatusResp, error) {
	body := JobArgs{
		Action: name,
		Input:  *args,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	resp := sc.MakeReq(http.MethodPost, sowerDispatch, bodyBytes, nil)
	if resp.Err != nil {
		return nil, resp.Err
	}

	StatusResp := &StatusResp{}
	err = json.Unmarshal(resp.Body, StatusResp)
	if err != nil {
		return nil, err
	}
	return StatusResp, nil
}

func (sc *SowerClient) Status(uid string) (*StatusResp, error) {
	resp := sc.MakeReq(http.MethodGet, sowerStatus, nil, map[string]string{"UID": uid})
	if resp.Err != nil {
		return nil, resp.Err
	}

	StatusResp := &StatusResp{}
	err := json.Unmarshal(resp.Body, StatusResp)
	if err != nil {
		return nil, err
	}
	return StatusResp, nil
}

func (sc *SowerClient) Output(uid string) (*OutputResp, error) {
	params := map[string]string{"UID": uid}
	resp := sc.MakeReq(http.MethodGet, sowerJobOutput, nil, params)
	if resp.Err != nil {
		return nil, resp.Err
	}

	var outputResp OutputResp
	err := json.Unmarshal(resp.Body, &outputResp)
	if err != nil {
		return nil, err
	}
	return &outputResp, nil
}

func (sc *SowerClient) List() ([]StatusResp, error) {
	resp := sc.MakeReq(http.MethodGet, sowerList, nil, nil)
	if resp.Err != nil {
		return nil, resp.Err
	}

	ListResp := []StatusResp{{}}
	err := json.Unmarshal(resp.Body, &ListResp)
	if err != nil {
		return nil, err
	}
	return ListResp, nil
}
