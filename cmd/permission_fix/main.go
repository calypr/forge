package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/calypr/calypr-cli/g3client"
	"github.com/calypr/calypr-cli/logs"
	"github.com/calypr/calypr-cli/request"
)

// UpdateInputInfo mirrors the indexd update payload
type UpdateInputInfo struct {
	FileName     string         `json:"file_name,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	URLsMetadata map[string]any `json:"urls_metadata,omitempty"`
	Version      string         `json:"version,omitempty"`
	URLs         []string       `json:"urls,omitempty"`
	ACL          []string       `json:"acl,omitempty"`
	Authz        []string       `json:"authz,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <profile_name>")
	}
	profile := os.Args[1]

	// 1. Initialize Gen3 Client
	logger, closer := logs.New(profile, logs.WithConsole())
	defer closer()
	sc, err := g3client.NewGen3Interface(profile, logger, g3client.WithClients(g3client.IndexdClient))
	if err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}

	did := "fa4ee697-f689-5291-a7cc-8ebe2f3ea8b9"
	// Change from /programs/Ellrott_Lab/projects/hla2vec to /programs/Ellrott_Lab/projects/embedding_rotation
	newAuthz := []string{"/programs/Ellrott_Lab/projects/embedding_rotation"}

	ctx := context.Background()
	cred := sc.GetCredential()

	// 2. Fetch current record to get the revision (rev)
	fmt.Printf("Fetching current record for %s...\n", did)
	getURL := fmt.Sprintf("%s/index/index/%s", cred.APIEndpoint, did)

	reqBuilder := &request.RequestBuilder{
		Method: http.MethodGet,
		Url:    getURL,
		Headers: map[string]string{
			"Accept": "application/json",
		},
		Token: cred.AccessToken,
	}

	// sc (Gen3Client) embeds RequestInterface
	resp, err := sc.(request.RequestInterface).Do(ctx, reqBuilder)
	if err != nil {
		log.Fatalf("Failed to fetch record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Fatalf("Failed to fetch record: %s (status: %d)", string(body), resp.StatusCode)
	}

	var currentRecord struct {
		Rev          string         `json:"rev"`
		FileName     string         `json:"file_name"`
		URLs         []string       `json:"urls"`
		ACL          []string       `json:"acl"`
		Authz        []string       `json:"authz"`
		Metadata     map[string]any `json:"metadata"`
		URLsMetadata map[string]any `json:"urls_metadata"`
		Version      string         `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&currentRecord); err != nil {
		log.Fatalf("Failed to decode current record: %v", err)
	}

	fmt.Printf("Current Rev: %s\n", currentRecord.Rev)
	fmt.Printf("Current Authz: %v\n", currentRecord.Authz)

	// 3. Prepare Update Payload (REPLACING Authz)
	updatePayload := UpdateInputInfo{
		FileName:     currentRecord.FileName,
		URLs:         currentRecord.URLs,
		ACL:          currentRecord.ACL,
		Authz:        newAuthz, // REPLACED
		Metadata:     currentRecord.Metadata,
		URLsMetadata: currentRecord.URLsMetadata,
		Version:      currentRecord.Version,
	}

	jsonBytes, err := json.Marshal(updatePayload)
	if err != nil {
		log.Fatalf("Failed to marshal update payload: %v", err)
	}

	// 4. Perform PUT Update
	fmt.Printf("Updating authz to %v...\n", newAuthz)
	putURL := fmt.Sprintf("%s/index/index/%s?rev=%s", cred.APIEndpoint, did, currentRecord.Rev)

	updateReq := &request.RequestBuilder{
		Method: http.MethodPut,
		Url:    putURL,
		Body:   bytes.NewBuffer(jsonBytes),
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
		Token: cred.AccessToken,
	}

	putResp, err := sc.(request.RequestInterface).Do(ctx, updateReq)
	if err != nil {
		log.Fatalf("Failed to update record: %v", err)
	}
	defer putResp.Body.Close()

	if putResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(putResp.Body)
		log.Fatalf("Update failed: %s (status: %d)", string(body), putResp.StatusCode)
	}

	fmt.Println("Successfully updated project permissions!")
}
