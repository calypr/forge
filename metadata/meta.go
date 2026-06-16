package metadata

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/calypr/calypr-cli/g3client"
	"github.com/calypr/forge/client"
	"github.com/calypr/forge/utils/remoteutil"
	drsapi "github.com/calypr/syfon/apigen/client/drs"
	"github.com/calypr/syfon/apigen/client/internalapi"
	syservices "github.com/calypr/syfon/client/services"
	fver "github.com/google/fhir/go/fhirversion"
	"github.com/google/fhir/go/jsonformat"
	code "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/codes_go_proto"
	dtpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/datatypes_go_proto"
	cprb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/resources/bundle_and_contained_resource_go_proto"
	drpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/resources/document_reference_go_proto"
	rspb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/resources/research_study_go_proto"
)

const (
	META_DIR          = "./META"
	NDJSON_EXT        = ".ndjson"
	bulkHashChunkSize = 500
)

type LFSRecord struct {
	Name    string
	Size    int64
	OIDType string
	OID     string
	Version string
}

type MetaObject struct {
	ID               string
	Name             string
	Size             int64
	Checksums        map[string]string
	CreatedTime      time.Time
	AccessURL        string
	ControlledAccess []string
}

func CreateMeta(outPath string, profileName string, gitRemoteName string) error {
	sc, closer, err := client.NewGen3Client(profileName, g3client.WithClients(g3client.SyfonClient))
	if err != nil {
		return err
	}
	defer closer()

	repoRemote, err := remoteutil.LoadRemoteOrDefault(gitRemoteName)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outPath, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	marshaller, err := jsonformat.NewMarshaller(false, "", "", fver.R5)
	if err != nil {
		return fmt.Errorf("failed to create FHIR marshaller: %w", err)
	}
	unmarshaller, err := jsonformat.NewUnmarshallerWithoutValidation("America/Los_Angeles", fver.R5)
	if err != nil {
		return fmt.Errorf("failed to create FHIR unmarshaller: %w", err)
	}

	rsID, err := getResearchStudy(outPath, repoRemote.ProjectID, sc.Credential().APIEndpoint, marshaller, unmarshaller)
	if err != nil {
		return err
	}

	repairObjects := make([]MetaObject, 0)

	repairDIDs, err := documentReferenceDIDsNeedingIdentifierRepair(outPath)
	if err != nil {
		return err
	}
	if len(repairDIDs) > 0 {
		log.Printf("Repairing existing DocumentReference identifiers from %d candidate DIDs", len(repairDIDs))
		didObjects, err := listProjectObjectsByDIDs(context.Background(), sc, repoRemote.Organization, repoRemote.ProjectID, repairDIDs)
		if err != nil {
			return err
		}
		log.Printf("Resolved %d Syfon records by DID for existing metadata repair", len(didObjects))
		if len(didObjects) > 0 {
			repaired, err := repairDocumentReferenceIdentifiersByDID(didObjects, outPath, sc.Credential().APIEndpoint, repoRemote.ProjectID)
			if err != nil {
				return fmt.Errorf("failed to repair existing DRS records by DID: %w", err)
			}
			log.Printf("Repaired %d existing DocumentReference identifiers by DID", repaired)
			repairObjects = mergeMetaObjects(repairObjects, didObjects)
		}
	}

	repairPaths, err := documentReferencePathsNeedingIdentifierRepair(outPath)
	if err != nil {
		return err
	}
	if len(repairPaths) > 0 {
		log.Printf("Repairing existing DocumentReference identifiers from %d candidate paths", len(repairPaths))
		pathObjects, err := listProjectObjectsByPaths(context.Background(), sc, repoRemote.Organization, repoRemote.ProjectID, repairPaths)
		if err != nil {
			return err
		}
		log.Printf("Resolved %d Syfon records by path for existing metadata repair", len(pathObjects))
		if len(pathObjects) > 0 {
			repaired, err := repairDocumentReferenceIdentifiersByPath(pathObjects, outPath, sc.Credential().APIEndpoint, repoRemote.ProjectID)
			if err != nil {
				return fmt.Errorf("failed to repair existing DRS records by path: %w", err)
			}
			log.Printf("Repaired %d existing DocumentReference identifiers by path", repaired)
			repairObjects = mergeMetaObjects(repairObjects, pathObjects)
		}
	}

	repairSHAs, err := documentReferenceSHA256s(outPath)
	if err != nil {
		return err
	}
	if len(repairSHAs) > 0 {
		log.Printf("Repairing existing DocumentReference identifiers from %d SHA256 values", len(repairSHAs))
		shaObjects, err := listProjectObjectsByHashes(context.Background(), sc, repoRemote.Organization, repoRemote.ProjectID, repairSHAs)
		if err != nil {
			return err
		}
		log.Printf("Resolved %d Syfon records by SHA256 for existing metadata repair", len(shaObjects))
		if len(shaObjects) > 0 {
			repaired, err := repairDocumentReferenceIdentifiersBySHA(shaObjects, outPath, sc.Credential().APIEndpoint, repoRemote.ProjectID)
			if err != nil {
				return fmt.Errorf("failed to repair existing DRS records by SHA256: %w", err)
			}
			log.Printf("Repaired %d existing DocumentReference identifiers by SHA256", repaired)
			repairObjects = mergeMetaObjects(repairObjects, shaObjects)
		}
	}

	projectObjects, err := listProjectObjects(context.Background(), sc, repoRemote.Organization, repoRemote.ProjectID)
	if err != nil {
		return err
	}
	if len(repairObjects) > 0 {
		projectObjects = mergeMetaObjects(projectObjects, repairObjects)
	}

	if err := processProjectRecordsAndUpdateFHIR(projectObjects, outPath, sc.Credential().APIEndpoint, repoRemote.ProjectID, rsID); err != nil {
		return fmt.Errorf("failed to process DRS records: %w", err)
	}

	dirPath := filepath.Join(outPath, "Directory"+NDJSON_EXT)
	if err := os.Remove(dirPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove stale directory metadata: %w", err)
	}

	return nil
}

func getResearchStudy(fhirDirectory string, projectID string, endpoint string, marshaller *jsonformat.Marshaller, unmarshaller *jsonformat.Unmarshaller) (string, error) {
	rsPath := filepath.Join(fhirDirectory, RESEARCH_STUDY+NDJSON_EXT)
	if err := os.MkdirAll(filepath.Dir(rsPath), 0o755); err != nil {
		return "", fmt.Errorf("failed to create directory for ResearchStudy file: %w", err)
	}

	if _, err := os.Stat(rsPath); err == nil {
		file, err := os.Open(rsPath)
		if err != nil {
			return "", fmt.Errorf("failed to open ResearchStudy file: %w", err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		var lines [][]byte
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			if len(strings.TrimSpace(string(line))) == 0 {
				continue
			}
			lines = append(lines, line)
		}
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("failed to read ResearchStudy file: %w", err)
		}
		if len(lines) == 0 {
			return "", fmt.Errorf("ResearchStudy file %s is empty", rsPath)
		}

		line := stripRootDirField(lines[0])
		msg, err := unmarshaller.UnmarshalR5(line)
		if err != nil {
			var raw map[string]any
			if err2 := json.Unmarshal(line, &raw); err2 != nil {
				return "", fmt.Errorf("failed to decode ResearchStudy: %w", err)
			}
			id, _ := raw["id"].(string)
			if id == "" {
				return "", fmt.Errorf("existing ResearchStudy missing id")
			}
			lines[0] = line
			return id, writeNDJSON(rsPath, lines)
		}

		rs := msg.GetResearchStudy()
		if rs == nil || rs.GetId() == nil || rs.GetId().GetValue() == "" {
			return "", fmt.Errorf("existing ResearchStudy missing id")
		}
		lines[0] = line
		return rs.GetId().GetValue(), writeNDJSON(rsPath, lines)
	}

	id := createIDFromStrings(endpoint, RESEARCH_STUDY, projectID)
	rs := &rspb.ResearchStudy{
		Id: &dtpb.Id{Value: id},
		Identifier: []*dtpb.Identifier{{
			Use:    &dtpb.Identifier_UseCode{Value: code.IdentifierUseCode_OFFICIAL},
			System: &dtpb.Uri{Value: endpoint + "/" + projectID},
			Value:  &dtpb.String{Value: projectID},
		}},
		Status:      &rspb.ResearchStudy_StatusCode{Value: code.PublicationStatusCode_ACTIVE},
		Description: &dtpb.Markdown{Value: fmt.Sprintf("Skeleton ResearchStudy for %s", projectID)},
	}
	cr := &cprb.ContainedResource{
		OneofResource: &cprb.ContainedResource_ResearchStudy{
			ResearchStudy: rs,
		},
	}
	jsonBytes, err := marshaller.Marshal(cr)
	if err != nil {
		return "", fmt.Errorf("failed to marshal ResearchStudy: %w", err)
	}
	return id, writeNDJSON(rsPath, [][]byte{jsonBytes})
}

func stripRootDirField(line []byte) []byte {
	var raw map[string]any
	if err := json.Unmarshal(line, &raw); err != nil {
		return line
	}
	delete(raw, "rootDir")
	out, err := json.Marshal(raw)
	if err != nil {
		return line
	}
	return out
}

func writeNDJSON(path string, lines [][]byte) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", path, err)
	}
	defer file.Close()
	for _, line := range lines {
		if _, err := file.Write(line); err != nil {
			return fmt.Errorf("failed to write %s: %w", path, err)
		}
		if _, err := file.Write([]byte("\n")); err != nil {
			return fmt.Errorf("failed to write newline to %s: %w", path, err)
		}
	}
	return nil
}

func documentReferenceSHA256s(fhirDirectory string) ([]string, error) {
	return collectDocumentReferenceValues(fhirDirectory, func(docRef *drpb.DocumentReference) []string {
		if sha := docRefSHA256(docRef); sha != "" {
			return []string{sha}
		}
		return nil
	})
}

func documentReferenceDIDsNeedingIdentifierRepair(fhirDirectory string) ([]string, error) {
	return collectDocumentReferenceValues(fhirDirectory, func(docRef *drpb.DocumentReference) []string {
		if !docRefNeedsIdentifierRepair(docRef) || docRef.GetId() == nil {
			return nil
		}
		id := strings.TrimSpace(docRef.GetId().GetValue())
		if id == "" {
			return nil
		}
		return []string{id}
	})
}

func documentReferencePathsNeedingIdentifierRepair(fhirDirectory string) ([]string, error) {
	return collectDocumentReferenceValues(fhirDirectory, func(docRef *drpb.DocumentReference) []string {
		if !docRefNeedsIdentifierRepair(docRef) {
			return nil
		}
		return docRefSyfonPathCandidates(docRef)
	})
}

func collectDocumentReferenceValues(fhirDirectory string, values func(*drpb.DocumentReference) []string) ([]string, error) {
	docRefPath := filepath.Join(fhirDirectory, DOCUMENT_RESOURCE+NDJSON_EXT)
	file, err := os.Open(docRefPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error opening %s: %w", docRefPath, err)
	}
	defer file.Close()

	unmarshaller, err := jsonformat.NewUnmarshallerWithoutValidation("America/Los_Angeles", fver.R5)
	if err != nil {
		return nil, fmt.Errorf("failed to create FHIR unmarshaller: %w", err)
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 || !json.Valid(line) {
			continue
		}
		cr, err := parseDocumentReferenceLine(line, unmarshaller)
		if err != nil {
			continue
		}
		docRef := cr.GetDocumentReference()
		for _, value := range values(docRef) {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if _, exists := seen[value]; exists {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error in %s: %w", docRefPath, err)
	}
	return out, nil
}

func docRefNeedsIdentifierRepair(docRef *drpb.DocumentReference) bool {
	value := documentReferencePrimaryIdentifier(docRef)
	if value == "" {
		return true
	}
	lowerValue := strings.ToLower(value)
	if strings.HasPrefix(lowerValue, "s3://") || strings.HasPrefix(lowerValue, "file://") {
		return true
	}
	if strings.Contains(value, "/") && !looksLikeUUID(value) {
		return true
	}
	return !looksLikeUUID(value)
}

func looksLikeUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for idx, r := range value {
		switch idx {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return false
			}
		}
	}
	return true
}

func docRefSyfonPathCandidates(docRef *drpb.DocumentReference) []string {
	if docRef == nil {
		return nil
	}
	out := make([]string, 0)
	if attachment := firstAttachment(docRef); attachment != nil {
		out = append(out, syfonPathCandidates(attachment.GetUrl().GetValue())...)
		for _, ext := range attachment.GetExtension() {
			if strings.HasSuffix(ext.GetUrl().GetValue(), "/source_path") {
				if value, ok := ext.GetValue().GetChoice().(*dtpb.Extension_ValueX_Url); ok {
					out = append(out, syfonPathCandidates(value.Url.GetValue())...)
				}
			}
		}
	}
	for _, identifier := range docRef.GetIdentifier() {
		if strings.Contains(strings.ToUpper(identifier.GetSystem().GetValue()), "FILE_PATH") {
			out = append(out, syfonPathCandidates(identifier.GetValue().GetValue())...)
		}
	}
	return out
}

func syfonPathCandidates(raw string) []string {
	value := normalizePathForMatch(raw)
	if value == "" {
		return nil
	}
	out := []string{value}
	if idx := strings.Index(value, "/"); idx >= 0 && idx+1 < len(value) {
		out = append(out, value[idx+1:])
	}
	return out
}

func listProjectObjects(ctx context.Context, sc *client.ProfileClient, organization string, projectID string) ([]MetaObject, error) {
	const pageSize = 1000
	page := 1
	seen := make(map[string]struct{})
	out := make([]MetaObject, 0)

	for {
		resp, err := sc.Gen3.SyfonClient().Index().List(ctx, syservices.ListRecordsOptions{
			Organization: organization,
			ProjectID:    projectID,
			Limit:        pageSize,
			Page:         page,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list project records: %w", err)
		}
		if resp.Records == nil || len(*resp.Records) == 0 {
			break
		}
		addedThisPage := 0
		for _, rec := range *resp.Records {
			obj, ok := metaObjectFromIndexRecord(rec)
			if !ok {
				continue
			}
			if _, exists := seen[obj.ID]; exists {
				continue
			}
			seen[obj.ID] = struct{}{}
			out = append(out, obj)
			addedThisPage++
		}
		if len(*resp.Records) < pageSize {
			break
		}
		if addedThisPage == 0 {
			break
		}
		page++
	}
	return out, nil
}

func listProjectObjectsByHashes(ctx context.Context, sc *client.ProfileClient, organization string, projectID string, hashes []string) ([]MetaObject, error) {
	if len(hashes) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{})
	out := make([]MetaObject, 0)

	for start := 0; start < len(hashes); start += bulkHashChunkSize {
		end := start + bulkHashChunkSize
		if end > len(hashes) {
			end = len(hashes)
		}
		records, err := bulkHashRecords(ctx, sc, hashes[start:end])
		if err != nil {
			return nil, fmt.Errorf("failed to bulk lookup records by hash: %w", err)
		}
		for _, rec := range records {
			if !recordMatchesScope(rec, organization, projectID) {
				continue
			}
			obj, ok := metaObjectFromIndexRecord(rec)
			if !ok {
				continue
			}
			if _, exists := seen[obj.ID]; exists {
				continue
			}
			seen[obj.ID] = struct{}{}
			out = append(out, obj)
		}
	}

	return out, nil
}

func listProjectObjectsByDIDs(ctx context.Context, sc *client.ProfileClient, organization string, projectID string, dids []string) ([]MetaObject, error) {
	if len(dids) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{})
	out := make([]MetaObject, 0)
	for start := 0; start < len(dids); start += bulkHashChunkSize {
		end := start + bulkHashChunkSize
		if end > len(dids) {
			end = len(dids)
		}
		records, err := bulkDocumentRecords(ctx, sc, dids[start:end])
		if err != nil {
			return nil, fmt.Errorf("failed to bulk lookup records by DID: %w", err)
		}
		for _, rec := range records {
			if !recordMatchesScope(rec, organization, projectID) {
				continue
			}
			obj, ok := metaObjectFromIndexRecord(rec)
			if !ok {
				continue
			}
			if _, exists := seen[obj.ID]; exists {
				continue
			}
			seen[obj.ID] = struct{}{}
			out = append(out, obj)
		}
	}
	return out, nil
}

func listProjectObjectsByPaths(ctx context.Context, sc *client.ProfileClient, organization string, projectID string, paths []string) ([]MetaObject, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{})
	out := make([]MetaObject, 0)
	for _, candidate := range paths {
		resp, err := sc.Gen3.SyfonClient().Index().List(ctx, syservices.ListRecordsOptions{
			Organization: organization,
			ProjectID:    projectID,
			Path:         candidate,
			Limit:        25,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to lookup records by path %q: %w", candidate, err)
		}
		if resp.Records == nil {
			continue
		}
		for _, rec := range *resp.Records {
			if !recordMatchesScope(rec, organization, projectID) {
				continue
			}
			obj, ok := metaObjectFromIndexRecord(rec)
			if !ok {
				continue
			}
			if _, exists := seen[obj.ID]; exists {
				continue
			}
			seen[obj.ID] = struct{}{}
			out = append(out, obj)
		}
	}
	return out, nil
}

func bulkDocumentRecords(ctx context.Context, sc *client.ProfileClient, dids []string) ([]internalapi.InternalRecord, error) {
	reqDIDs := make([]string, 0, len(dids))
	for _, did := range dids {
		did = strings.TrimSpace(did)
		if did == "" {
			continue
		}
		reqDIDs = append(reqDIDs, did)
	}
	if len(reqDIDs) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(reqDIDs)
	if err != nil {
		return nil, err
	}

	cred := sc.Credential()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, normalizeEndpoint(cred.APIEndpoint)+"/index/bulk/documents", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(cred.AccessToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("POST %s: status %d body=%s", req.URL.String(), resp.StatusCode, string(respBody))
	}
	var out []internalapi.InternalRecord
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func bulkHashRecords(ctx context.Context, sc *client.ProfileClient, hashes []string) ([]internalapi.InternalRecord, error) {
	reqHashes := bulkSHA256Queries(hashes)
	if len(reqHashes) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(internalapi.BulkHashesRequest{Hashes: reqHashes})
	if err != nil {
		return nil, err
	}

	cred := sc.Credential()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, normalizeEndpoint(cred.APIEndpoint)+"/index/bulk/hashes", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(cred.AccessToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("POST %s: status %d body=%s", req.URL.String(), resp.StatusCode, string(respBody))
	}
	return decodeBulkHashRecords(respBody)
}

func bulkSHA256Queries(hashes []string) []string {
	out := make([]string, 0, len(hashes))
	for _, hash := range hashes {
		normalized := strings.ToLower(normalizeChecksum(hash))
		if normalized == "" {
			continue
		}
		out = append(out, "sha256:"+normalized)
	}
	return out
}

func decodeBulkHashRecords(body []byte) ([]internalapi.InternalRecord, error) {
	var payload struct {
		Results map[string][]internalapi.InternalRecord `json:"results"`
		Records []internalapi.InternalRecord            `json:"records"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	out := make([]internalapi.InternalRecord, 0)
	for _, records := range payload.Results {
		out = append(out, records...)
	}
	out = append(out, payload.Records...)
	return out, nil
}

func mergeMetaObjects(primary []MetaObject, extras []MetaObject) []MetaObject {
	if len(extras) == 0 {
		return primary
	}
	seen := make(map[string]struct{}, len(primary)+len(extras))
	out := make([]MetaObject, 0, len(primary)+len(extras))
	for _, obj := range primary {
		if _, exists := seen[obj.ID]; exists {
			continue
		}
		seen[obj.ID] = struct{}{}
		out = append(out, obj)
	}
	for _, obj := range extras {
		if _, exists := seen[obj.ID]; exists {
			continue
		}
		seen[obj.ID] = struct{}{}
		out = append(out, obj)
	}
	return out
}

func recordMatchesScope(rec internalapi.InternalRecord, organization string, projectID string) bool {
	if rec.Organization != nil && strings.TrimSpace(*rec.Organization) != "" {
		if strings.TrimSpace(*rec.Organization) != organization {
			return false
		}
	}
	if rec.Project != nil && strings.TrimSpace(*rec.Project) != "" {
		if strings.TrimSpace(*rec.Project) != projectID {
			return false
		}
	}
	if rec.Organization != nil && strings.TrimSpace(*rec.Organization) != "" &&
		rec.Project != nil && strings.TrimSpace(*rec.Project) != "" {
		return true
	}
	if recordControlledAccessMatchesScope(rec, organization, projectID) {
		return true
	}
	if strings.TrimSpace(organization) == "" && strings.TrimSpace(projectID) == "" {
		return true
	}
	return false
}

func recordControlledAccessMatchesScope(rec internalapi.InternalRecord, organization string, projectID string) bool {
	if rec.ControlledAccess == nil {
		return false
	}
	expected := fmt.Sprintf("/organization/%s/project/%s", strings.TrimSpace(organization), strings.TrimSpace(projectID))
	for _, resource := range *rec.ControlledAccess {
		if strings.TrimSpace(resource) == expected {
			return true
		}
	}
	return false
}

func metaObjectFromIndexRecord(obj internalapi.InternalRecord) (MetaObject, bool) {
	id := strings.TrimSpace(obj.Did)
	if id == "" {
		return MetaObject{}, false
	}
	hashes := map[string]string{}
	if obj.Hashes != nil {
		hashes = normalizeChecksumMap(map[string]string(*obj.Hashes))
	}
	createdTime := ""
	if obj.CreatedTime != nil {
		createdTime = *obj.CreatedTime
	}
	parsedTime, _ := parseOptionalTime(createdTime)
	size := int64(0)
	if obj.Size != nil {
		size = *obj.Size
	}
	name := ""
	if obj.FileName != nil {
		name = strings.TrimSpace(*obj.FileName)
	}
	controlled := []string{}
	if obj.ControlledAccess != nil {
		controlled = append(controlled, (*obj.ControlledAccess)...)
	}
	return MetaObject{
		ID:               id,
		Name:             name,
		Size:             size,
		Checksums:        hashes,
		CreatedTime:      parsedTime,
		AccessURL:        firstAccessURL(obj.AccessMethods),
		ControlledAccess: controlled,
	}, true
}

func normalizeChecksumMap(raw map[string]string) map[string]string {
	if len(raw) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(raw))
	for typ, checksum := range raw {
		out[strings.TrimSpace(typ)] = normalizeChecksum(checksum)
	}
	return out
}

func firstAccessURL(methods *[]drsapi.AccessMethod) string {
	if methods == nil {
		return ""
	}
	for _, method := range *methods {
		if method.AccessUrl != nil && strings.TrimSpace(method.AccessUrl.Url) != "" {
			return strings.TrimSpace(method.AccessUrl.Url)
		}
	}
	return ""
}

func parseOptionalTime(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, nil
	}
	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, trimmed); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time format %q", raw)
}

func processProjectRecordsAndUpdateFHIR(drsRecords []MetaObject, fhirDirectory string, endpoint string, project string, researchStudyID string) error {
	docRefPath := filepath.Join(fhirDirectory, DOCUMENT_RESOURCE+NDJSON_EXT)
	existingFHIRRecords := make([]*cprb.ContainedResource, 0)
	existingBySHA256 := make(map[string][]*cprb.ContainedResource)

	marshaller, err := jsonformat.NewMarshaller(false, "", "", fver.R5)
	if err != nil {
		return fmt.Errorf("failed to create FHIR marshaller: %w", err)
	}
	unmarshaller, err := jsonformat.NewUnmarshallerWithoutValidation("America/Los_Angeles", fver.R5)
	if err != nil {
		return fmt.Errorf("failed to create FHIR unmarshaller: %w", err)
	}

	if file, err := os.Open(docRefPath); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(strings.TrimSpace(string(line))) == 0 || !json.Valid(line) {
				continue
			}
			cr, err := parseDocumentReferenceLine(line, unmarshaller)
			if err != nil {
				continue
			}

			docRef := cr.GetDocumentReference()
			if docRef == nil || docRef.GetId() == nil {
				continue
			}
			existingFHIRRecords = append(existingFHIRRecords, cr)

			if sha := docRefSHA256(docRef); sha != "" {
				existingBySHA256[sha] = append(existingBySHA256[sha], cr)
			}
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("scanner error in %s: %w", docRefPath, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("error opening %s: %w", docRefPath, err)
	}

	for _, obj := range drsRecords {
		sha := checksumValue(obj.Checksums, "sha-256", "sha256")
		if sha == "" {
			continue
		}
		if existingCr, exists := selectExistingRecordForObject(existingBySHA256[sha], &obj); exists {
			syncSyfonIdentifier(existingCr.GetDocumentReference(), &obj, endpoint, project)
			continue
		}
		cr := templateDocRef(&obj, endpoint, project, researchStudyID)
		existingFHIRRecords = append(existingFHIRRecords, cr)
	}

	docFile, err := os.Create(docRefPath)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", docRefPath, err)
	}
	defer docFile.Close()

	written := 0
	for _, record := range existingFHIRRecords {
		recordID := record.GetDocumentReference().GetId().GetValue()
		jsonBytes, err := marshaller.Marshal(record)
		if err != nil {
			log.Printf("error serializing record %s: %v", recordID, err)
			continue
		}
		if _, err := docFile.Write(jsonBytes); err != nil {
			return fmt.Errorf("failed to write %s: %w", docRefPath, err)
		}
		if _, err := docFile.Write([]byte("\n")); err != nil {
			return fmt.Errorf("failed to write newline to %s: %w", docRefPath, err)
		}
		written++
	}

	if written == 0 {
		log.Printf("WARNING: no matching Syfon objects were found for project %q", project)
	}
	return nil
}

func repairDocumentReferenceIdentifiersBySHA(drsRecords []MetaObject, fhirDirectory string, endpoint string, project string) (int, error) {
	objectsBySHA := make(map[string]*MetaObject, len(drsRecords))
	for idx := range drsRecords {
		sha := checksumValue(drsRecords[idx].Checksums, "sha-256", "sha256")
		if sha == "" {
			continue
		}
		if _, exists := objectsBySHA[sha]; exists {
			continue
		}
		objectsBySHA[sha] = &drsRecords[idx]
	}
	return repairDocumentReferenceIdentifiersByMatcher(drsRecords, fhirDirectory, endpoint, project, func(docRef *drpb.DocumentReference, _ []MetaObject) *MetaObject {
		return objectsBySHA[docRefSHA256(docRef)]
	})
}

func repairDocumentReferenceIdentifiersByDID(drsRecords []MetaObject, fhirDirectory string, endpoint string, project string) (int, error) {
	objectsByDID := make(map[string]*MetaObject, len(drsRecords))
	for idx := range drsRecords {
		id := strings.TrimSpace(drsRecords[idx].ID)
		if id == "" {
			continue
		}
		objectsByDID[id] = &drsRecords[idx]
	}
	return repairDocumentReferenceIdentifiersByMatcher(drsRecords, fhirDirectory, endpoint, project, func(docRef *drpb.DocumentReference, _ []MetaObject) *MetaObject {
		if docRef == nil || docRef.GetId() == nil {
			return nil
		}
		return objectsByDID[strings.TrimSpace(docRef.GetId().GetValue())]
	})
}

func repairDocumentReferenceIdentifiersByPath(drsRecords []MetaObject, fhirDirectory string, endpoint string, project string) (int, error) {
	return repairDocumentReferenceIdentifiersByMatcher(drsRecords, fhirDirectory, endpoint, project, func(docRef *drpb.DocumentReference, records []MetaObject) *MetaObject {
		for idx := range records {
			if documentReferenceMatchesObject(docRef, &records[idx]) {
				return &records[idx]
			}
		}
		return nil
	})
}

func repairDocumentReferenceIdentifiersByMatcher(drsRecords []MetaObject, fhirDirectory string, endpoint string, project string, match func(*drpb.DocumentReference, []MetaObject) *MetaObject) (int, error) {
	if len(drsRecords) == 0 {
		return 0, nil
	}
	docRefPath := filepath.Join(fhirDirectory, DOCUMENT_RESOURCE+NDJSON_EXT)
	file, err := os.Open(docRefPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("error opening %s: %w", docRefPath, err)
	}
	defer file.Close()

	marshaller, err := jsonformat.NewMarshaller(false, "", "", fver.R5)
	if err != nil {
		return 0, fmt.Errorf("failed to create FHIR marshaller: %w", err)
	}
	unmarshaller, err := jsonformat.NewUnmarshallerWithoutValidation("America/Los_Angeles", fver.R5)
	if err != nil {
		return 0, fmt.Errorf("failed to create FHIR unmarshaller: %w", err)
	}

	records := make([]*cprb.ContainedResource, 0)
	repaired := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 || !json.Valid(line) {
			continue
		}
		cr, err := parseDocumentReferenceLine(line, unmarshaller)
		if err != nil {
			continue
		}
		docRef := cr.GetDocumentReference()
		if docRef == nil {
			continue
		}
		if obj := match(docRef, drsRecords); obj != nil && documentReferencePrimaryIdentifier(docRef) != obj.ID {
			syncSyfonIdentifier(docRef, obj, endpoint, project)
			repaired++
		}
		records = append(records, cr)
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("scanner error in %s: %w", docRefPath, err)
	}
	if repaired == 0 {
		return 0, nil
	}

	docFile, err := os.Create(docRefPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create %s: %w", docRefPath, err)
	}
	defer docFile.Close()

	for _, record := range records {
		jsonBytes, err := marshaller.Marshal(record)
		if err != nil {
			return 0, fmt.Errorf("failed to serialize repaired DocumentReference: %w", err)
		}
		if _, err := docFile.Write(jsonBytes); err != nil {
			return 0, fmt.Errorf("failed to write %s: %w", docRefPath, err)
		}
		if _, err := docFile.Write([]byte("\n")); err != nil {
			return 0, fmt.Errorf("failed to write newline to %s: %w", docRefPath, err)
		}
	}
	return repaired, nil
}

func parseDocumentReferenceLine(line []byte, unmarshaller *jsonformat.Unmarshaller) (*cprb.ContainedResource, error) {
	cr, err := unmarshaller.UnmarshalR5(line)
	if err == nil && cr.GetDocumentReference() != nil {
		return cr, nil
	}

	docRef := &drpb.DocumentReference{}
	if err2 := json.Unmarshal(line, docRef); err2 == nil && docRef.GetId() != nil && docRef.GetId().GetValue() != "" {
		return &cprb.ContainedResource{
			OneofResource: &cprb.ContainedResource_DocumentReference{
				DocumentReference: docRef,
			},
		}, nil
	}

	var wrapped map[string]json.RawMessage
	if err2 := json.Unmarshal(line, &wrapped); err2 == nil {
		if inner, ok := wrapped["documentReference"]; ok {
			if err2 = json.Unmarshal(inner, docRef); err2 == nil && docRef.GetId() != nil && docRef.GetId().GetValue() != "" {
				return &cprb.ContainedResource{
					OneofResource: &cprb.ContainedResource_DocumentReference{
						DocumentReference: docRef,
					},
				}, nil
			}
		}
	}

	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("document reference not found")
}

func syncSyfonIdentifier(docRef *drpb.DocumentReference, obj *MetaObject, endpoint string, project string) {
	if docRef == nil || obj == nil || strings.TrimSpace(obj.ID) == "" {
		return
	}
	systemValue := normalizeEndpoint(endpoint) + "/" + project
	replacement := &dtpb.Identifier{
		Use:    &dtpb.Identifier_UseCode{Value: code.IdentifierUseCode_OFFICIAL},
		System: &dtpb.Uri{Value: systemValue},
		Value:  &dtpb.String{Value: obj.ID},
	}

	identifiers := docRef.GetIdentifier()
	filtered := make([]*dtpb.Identifier, 0, len(identifiers))
	for _, identifier := range identifiers {
		if identifier == nil {
			continue
		}
		if identifier.GetUse().GetValue() == code.IdentifierUseCode_OFFICIAL ||
			strings.TrimSpace(identifier.GetSystem().GetValue()) == systemValue {
			continue
		}
		filtered = append(filtered, identifier)
	}
	docRef.Identifier = append([]*dtpb.Identifier{replacement}, filtered...)
}

func documentReferencePrimaryIdentifier(docRef *drpb.DocumentReference) string {
	if docRef == nil || len(docRef.GetIdentifier()) == 0 || docRef.GetIdentifier()[0] == nil {
		return ""
	}
	return strings.TrimSpace(docRef.GetIdentifier()[0].GetValue().GetValue())
}

func docRefSHA256(docRef *drpb.DocumentReference) string {
	if docRef == nil {
		return ""
	}
	if attachment := firstAttachment(docRef); attachment != nil {
		for _, ext := range attachment.GetExtension() {
			url := ext.GetUrl().GetValue()
			if strings.HasSuffix(url, "/checksum-sha256") || strings.HasSuffix(url, "/sha256") {
				if sha, ok := ext.GetValue().GetChoice().(*dtpb.Extension_ValueX_StringValue); ok {
					return normalizeChecksum(sha.StringValue.GetValue())
				}
			}
		}
	}
	for _, identifier := range docRef.GetIdentifier() {
		system := strings.ToUpper(strings.TrimSpace(identifier.GetSystem().GetValue()))
		if strings.Contains(system, "SHA256") {
			return normalizeChecksum(identifier.GetValue().GetValue())
		}
	}
	return ""
}

func selectExistingRecordForObject(candidates []*cprb.ContainedResource, obj *MetaObject) (*cprb.ContainedResource, bool) {
	if len(candidates) == 0 || obj == nil {
		return nil, false
	}
	if len(candidates) == 1 {
		cr := candidates[0]
		candidates[0] = nil
		return cr, true
	}

	for idx, candidate := range candidates {
		if candidate == nil {
			continue
		}
		if documentReferenceMatchesObject(candidate.GetDocumentReference(), obj) {
			candidates[idx] = nil
			return candidate, true
		}
	}

	for idx, candidate := range candidates {
		if candidate == nil {
			continue
		}
		candidates[idx] = nil
		return candidate, true
	}
	return nil, false
}

func documentReferenceMatchesObject(docRef *drpb.DocumentReference, obj *MetaObject) bool {
	if docRef == nil || obj == nil {
		return false
	}
	targets := make([]string, 0, 4)
	if obj.AccessURL != "" {
		targets = append(targets, normalizePathForMatch(obj.AccessURL))
	}
	if obj.Name != "" {
		targets = append(targets, normalizePathForMatch(obj.Name))
	}
	if attachment := firstAttachment(docRef); attachment != nil {
		if normalized := normalizePathForMatch(attachment.GetUrl().GetValue()); normalized != "" {
			for _, target := range targets {
				if pathMatchesTarget(normalized, target) {
					return true
				}
			}
		}
		for _, ext := range attachment.GetExtension() {
			if strings.HasSuffix(ext.GetUrl().GetValue(), "/source_path") {
				if value, ok := ext.GetValue().GetChoice().(*dtpb.Extension_ValueX_Url); ok {
					normalized := normalizePathForMatch(value.Url.GetValue())
					for _, target := range targets {
						if pathMatchesTarget(normalized, target) {
							return true
						}
					}
				}
			}
		}
		if normalized := normalizePathForMatch(attachment.GetTitle().GetValue()); normalized != "" {
			for _, target := range targets {
				if pathMatchesTarget(normalized, target) {
					return true
				}
			}
		}
	}
	for _, identifier := range docRef.GetIdentifier() {
		if strings.Contains(strings.ToUpper(identifier.GetSystem().GetValue()), "FILE_PATH") {
			normalized := normalizePathForMatch(identifier.GetValue().GetValue())
			for _, target := range targets {
				if pathMatchesTarget(normalized, target) {
					return true
				}
			}
		}
	}
	return false
}

func normalizePathForMatch(raw string) string {
	normalized := strings.TrimSpace(raw)
	normalized = strings.TrimPrefix(normalized, "file://")
	normalized = strings.TrimPrefix(normalized, "s3://")
	normalized = strings.TrimPrefix(normalized, "/")
	return normalized
}

func pathMatchesTarget(candidate string, target string) bool {
	if candidate == "" || target == "" {
		return false
	}
	if candidate == target {
		return true
	}
	return strings.HasSuffix(candidate, "/"+target) || strings.HasSuffix(target, "/"+candidate)
}

func firstAttachment(docRef *drpb.DocumentReference) *dtpb.Attachment {
	if docRef == nil || len(docRef.GetContent()) == 0 {
		return nil
	}
	return docRef.GetContent()[0].GetAttachment()
}

func checksumValue(checksums map[string]string, acceptedTypes ...string) string {
	typeSet := make(map[string]bool, len(acceptedTypes))
	for _, typ := range acceptedTypes {
		typeSet[strings.ToLower(strings.TrimSpace(typ))] = true
	}
	for typ, checksum := range checksums {
		if typeSet[strings.ToLower(strings.TrimSpace(typ))] {
			return normalizeChecksum(checksum)
		}
	}
	return ""
}

func normalizeChecksum(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "sha256:")
	return strings.TrimSpace(raw)
}

func stringPtr(v string) *string {
	return &v
}
