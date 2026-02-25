package metadata

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/calypr/data-client/drs"
	fver "github.com/google/fhir/go/fhirversion"
	"github.com/google/fhir/go/jsonformat"
	dtpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/datatypes_go_proto"
)

func TestSmartMergeIDPreservation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "meta-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	endpoint := "localhost"
	project := "test-project"
	rsID := "rs-1"

	// 1. Create initial metadata
	initialPath := "old/path/file.txt"
	initialOID := "sha256-content-123"
	obj := &drs.DRSObject{
		Id:   "drs-1",
		Name: initialPath,
		Size: 100,
		Checksums: drs.HashInfo{
			SHA256: initialOID,
		},
		CreatedTime: "2023-10-27T10:00:00Z",
	}

	initialCr := templateDocRef(obj, endpoint, project, rsID)
	initialID := initialCr.GetDocumentReference().Id.Value

	marshaller, _ := jsonformat.NewMarshaller(false, "", "", fver.R5)
	jsonBytes, _ := marshaller.Marshal(initialCr)

	docRefFP := filepath.Join(tmpDir, DOCUMENT_RESOURCE+NDJSON_EXT)
	err = os.WriteFile(docRefFP, append(jsonBytes, '\n'), 0644)
	if err != nil {
		t.Fatalf("failed to write initial metadata: %v", err)
	}

	// 2. Simulate a rename in LFS records
	newPath := "new/path/renamed_file.txt"
	lfsRecords := []LFSRecord{
		{
			Name: newPath,
			OID:  initialOID,
		},
	}

	// Mock DRS records (content matched)
	drsRecords := []*drs.DRSObject{
		{
			Id:   "drs-1",
			Name: newPath,
			Size: 100,
			Checksums: drs.HashInfo{
				SHA256: initialOID,
			},
			CreatedTime: "2023-10-27T10:00:00Z",
		},
	}

	// 3. Run the smart merge
	err = processDRSRecordsAndUpdateFHIR(drsRecords, lfsRecords, nil, tmpDir, endpoint, project, rsID, "", "")
	if err != nil {
		t.Fatalf("processDRSRecordsAndUpdateFHIR failed: %v", err)
	}

	// 4. Validate results
	file, err := os.Open(docRefFP)
	if err != nil {
		t.Fatalf("failed to open output file: %v", err)
	}
	defer file.Close()

	unmarshaller, _ := jsonformat.NewUnmarshallerWithoutValidation("America/Los_Angeles", fver.R5)
	foundRecord := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		dr, err := unmarshaller.UnmarshalR5(line)
		if err != nil {
			t.Errorf("failed to unmarshal result: %v", err)
			continue
		}

		docRef := dr.GetDocumentReference()
		if docRef.Content[0].Attachment.Title.Value == newPath {
			foundRecord = true
			if docRef.Id.Value != initialID {
				t.Errorf("ID mismatch after rename! Expected %s, got %s", initialID, docRef.Id.Value)
			}
		}
	}

	if !foundRecord {
		t.Error("renamed record not found in output")
	}

	// 5. Validate Directory structure (should point to preserved ID)
	dirRefFP := filepath.Join(tmpDir, DIRECTORY_RESOURCE+NDJSON_EXT)
	dirFile, err := os.Open(dirRefFP)
	if err != nil {
		t.Fatalf("failed to open directory file: %v", err)
	}
	defer dirFile.Close()

	foundInDir := false
	scanner = bufio.NewScanner(dirFile)
	for scanner.Scan() {
		var dir Directory
		json.Unmarshal(scanner.Bytes(), &dir)
		if dir.Path == "new/path" {
			for _, child := range dir.Child {
				refStr, _ := extractReferenceString(child)
				if refStr == DOCUMENT_RESOURCE+"/"+initialID {
					foundInDir = true
				}
			}
		}
		// Confirm it's NOT in the old path
		if dir.Path == "old/path" {
			for _, child := range dir.Child {
				refStr, _ := extractReferenceString(child)
				if refStr == DOCUMENT_RESOURCE+"/"+initialID {
					t.Errorf("Stale reference found in old directory path: %s", dir.Path)
				}
			}
		}
	}

	if !foundInDir {
		t.Error("preserved ID not found in new directory path")
	}
}

func TestMergeCustomExtensions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "meta-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	endpoint := "localhost"
	project := "test-project"
	rsID := "rs-1"

	// 1. Create initial metadata with a custom extension
	path := "file.txt"
	oid := "sha256-abc"
	customExtURL := "http://example.com/custom-ext"
	customExtVal := "custom-value"

	obj := &drs.DRSObject{
		Id:   "drs-1",
		Name: path,
		Size: 100,
		Checksums: drs.HashInfo{
			SHA256: oid,
		},
		CreatedTime: "2023-10-27T10:00:00Z",
	}

	initialCr := templateDocRef(obj, endpoint, project, rsID)
	initialCr.GetDocumentReference().Content[0].Attachment.Extension = append(
		initialCr.GetDocumentReference().Content[0].Attachment.Extension,
		&dtpb.Extension{
			Url: &dtpb.Uri{Value: customExtURL},
			Value: &dtpb.Extension_ValueX{
				Choice: &dtpb.Extension_ValueX_StringValue{StringValue: &dtpb.String{Value: customExtVal}},
			},
		},
	)

	marshaller, _ := jsonformat.NewMarshaller(false, "", "", fver.R5)
	jsonBytes, _ := marshaller.Marshal(initialCr)

	docRefFP := filepath.Join(tmpDir, DOCUMENT_RESOURCE+NDJSON_EXT)
	os.WriteFile(docRefFP, append(jsonBytes, '\n'), 0644)

	// 2. Prepare update
	lfsRecords := []LFSRecord{{Name: path, OID: oid}}
	drsRecords := []*drs.DRSObject{obj}

	// 3. Run merge
	processDRSRecordsAndUpdateFHIR(drsRecords, lfsRecords, nil, tmpDir, endpoint, project, rsID, "", "")

	// 4. Verify custom extension is still there
	file, _ := os.Open(docRefFP)
	defer file.Close()
	unmarshaller, _ := jsonformat.NewUnmarshallerWithoutValidation("America/Los_Angeles", fver.R5)
	scanner := bufio.NewScanner(file)
	foundCustomExt := false
	for scanner.Scan() {
		dr, _ := unmarshaller.UnmarshalR5(scanner.Bytes())
		for _, ext := range dr.GetDocumentReference().Content[0].Attachment.Extension {
			if ext.Url.Value == customExtURL {
				foundCustomExt = true
				if val, ok := ext.Value.Choice.(*dtpb.Extension_ValueX_StringValue); ok {
					if val.StringValue.Value != customExtVal {
						t.Errorf("Custom extension value mismatch: expected %s, got %s", customExtVal, val.StringValue.Value)
					}
				}
			}
		}
	}

	if !foundCustomExt {
		t.Error("custom extension lost during merge")
	}
}
