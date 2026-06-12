package metadata

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	META_DIR   = "./META"
	NDJSON_EXT = ".ndjson"
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
	_ = gitRemoteName
	sc, closer, err := client.NewGen3Client(profileName, g3client.WithClients(g3client.SyfonClient))
	if err != nil {
		return err
	}
	defer closer()

	repoRemote, err := remoteutil.LoadRemoteOrDefault("")
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

	projectObjects, err := listProjectObjects(context.Background(), sc, repoRemote.Organization, repoRemote.ProjectID)
	if err != nil {
		return err
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
		}
		if len(*resp.Records) < pageSize {
			break
		}
		page++
	}
	return out, nil
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
	existingBySHA256 := make(map[string]*cprb.ContainedResource)

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
			cr, err := unmarshaller.UnmarshalR5(line)
			if err != nil {
				var wrapped map[string]json.RawMessage
				if err2 := json.Unmarshal(line, &wrapped); err2 == nil {
					if inner, ok := wrapped["documentReference"]; ok {
						docRef := &drpb.DocumentReference{}
						if err = json.Unmarshal(inner, docRef); err == nil {
							cr = &cprb.ContainedResource{
								OneofResource: &cprb.ContainedResource_DocumentReference{
									DocumentReference: docRef,
								},
							}
						}
					}
				}
			}
			if err != nil {
				continue
			}

			docRef := cr.GetDocumentReference()
			if docRef == nil || docRef.GetId() == nil {
				continue
			}
			existingFHIRRecords = append(existingFHIRRecords, cr)

			if sha := docRefSHA256(docRef); sha != "" {
				if _, exists := existingBySHA256[sha]; !exists {
					existingBySHA256[sha] = cr
				}
			}
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("scanner error in %s: %w", docRefPath, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("error opening %s: %w", docRefPath, err)
	}

	addedBySHA := make(map[string]struct{})
	for _, obj := range drsRecords {
		sha := checksumValue(obj.Checksums, "sha-256", "sha256")
		if sha == "" {
			continue
		}
		if _, exists := existingBySHA256[sha]; exists {
			continue
		}
		if _, exists := addedBySHA[sha]; exists {
			continue
		}
		cr := templateDocRef(&obj, endpoint, project, researchStudyID)
		existingFHIRRecords = append(existingFHIRRecords, cr)
		addedBySHA[sha] = struct{}{}
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
