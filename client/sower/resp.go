package sower

type StatusResp struct {
	Uid    string `json:"uid"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type OutputResp struct {
	Output string `json:"output"`
}
