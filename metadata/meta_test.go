package metadata

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	fver "github.com/google/fhir/go/fhirversion"
	"github.com/google/fhir/go/jsonformat"
	dtpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/datatypes_go_proto"
)

func TestProcessProjectRecordsPreservesExistingAndAddsMissing(t *testing.T) {
	tmpDir := t.TempDir()

	endpoint := "localhost"
	project := "test-project"
	rsID := "rs-1"

	existing := &MetaObject{
		ID:          "drs-existing",
		Name:        "existing.txt",
		Size:        100,
		Checksums:   map[string]string{"sha-256": "sha-existing"},
		CreatedTime: time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC),
		AccessURL:   "s3://bucket/existing",
	}
	existingCr := templateDocRef(existing, endpoint, project, rsID)
	existingCr.GetDocumentReference().Identifier = append(existingCr.GetDocumentReference().Identifier,
		&dtpb.Identifier{
			Use:    &dtpb.Identifier_UseCode{},
			System: &dtpb.Uri{Value: "https://humantumoratlas.org/FILE_SHA256"},
			Value:  &dtpb.String{Value: "sha-existing"},
		},
	)

	marshaller, _ := jsonformat.NewMarshaller(false, "", "", fver.R5)
	jsonBytes, _ := marshaller.Marshal(existingCr)
	docRefFP := filepath.Join(tmpDir, DOCUMENT_RESOURCE+NDJSON_EXT)
	if err := os.WriteFile(docRefFP, append(jsonBytes, '\n'), 0o644); err != nil {
		t.Fatalf("failed to write initial metadata: %v", err)
	}

	records := []MetaObject{
		*existing,
		{
			ID:          "drs-new",
			Name:        "new.txt",
			Size:        200,
			Checksums:   map[string]string{"sha-256": "sha-new"},
			CreatedTime: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
			AccessURL:   "s3://bucket/new",
		},
	}

	if err := processProjectRecordsAndUpdateFHIR(records, tmpDir, endpoint, project, rsID); err != nil {
		t.Fatalf("processProjectRecordsAndUpdateFHIR failed: %v", err)
	}

	file, err := os.Open(docRefFP)
	if err != nil {
		t.Fatalf("failed to open output file: %v", err)
	}
	defer file.Close()

	unmarshaller, _ := jsonformat.NewUnmarshallerWithoutValidation("America/Los_Angeles", fver.R5)
	var existingCount, newCount int
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		cr, err := unmarshaller.UnmarshalR5(line)
		if err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}
		docRef := cr.GetDocumentReference()
		switch docRef.GetContent()[0].GetAttachment().GetTitle().GetValue() {
		case "existing.txt":
			existingCount++
		case "new.txt":
			newCount++
		}
	}
	if existingCount != 1 {
		t.Fatalf("expected exactly one existing row, got %d", existingCount)
	}
	if newCount != 1 {
		t.Fatalf("expected exactly one new row, got %d", newCount)
	}
}

func TestProcessProjectRecordsRewritesStaleSyfonIdentifier(t *testing.T) {
	tmpDir := t.TempDir()

	endpoint := "localhost"
	project := "test-project"
	rsID := "rs-1"

	existing := &MetaObject{
		ID:          "stale-id",
		Name:        "existing.txt",
		Size:        100,
		Checksums:   map[string]string{"sha-256": "sha-existing"},
		CreatedTime: time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC),
		AccessURL:   "s3://bucket/existing",
	}
	existingCr := templateDocRef(existing, endpoint, project, rsID)
	existingCr.GetDocumentReference().Identifier[0].Value = &dtpb.String{Value: "wrong-syfon-did"}
	existingCr.GetDocumentReference().Identifier = append(existingCr.GetDocumentReference().Identifier,
		&dtpb.Identifier{
			System: &dtpb.Uri{Value: "https://humantumoratlas.org/FILE_SHA256"},
			Value:  &dtpb.String{Value: "sha-existing"},
		},
	)

	marshaller, _ := jsonformat.NewMarshaller(false, "", "", fver.R5)
	jsonBytes, _ := marshaller.Marshal(existingCr)
	docRefFP := filepath.Join(tmpDir, DOCUMENT_RESOURCE+NDJSON_EXT)
	if err := os.WriteFile(docRefFP, append(jsonBytes, '\n'), 0o644); err != nil {
		t.Fatalf("failed to write initial metadata: %v", err)
	}

	records := []MetaObject{{
		ID:          "correct-syfon-did",
		Name:        "existing.txt",
		Size:        100,
		Checksums:   map[string]string{"sha-256": "sha-existing"},
		CreatedTime: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		AccessURL:   "s3://bucket/existing",
	}}

	if err := processProjectRecordsAndUpdateFHIR(records, tmpDir, endpoint, project, rsID); err != nil {
		t.Fatalf("processProjectRecordsAndUpdateFHIR failed: %v", err)
	}

	file, err := os.Open(docRefFP)
	if err != nil {
		t.Fatalf("failed to open output file: %v", err)
	}
	defer file.Close()

	unmarshaller, _ := jsonformat.NewUnmarshallerWithoutValidation("America/Los_Angeles", fver.R5)
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("expected one document reference row")
	}
	cr, err := unmarshaller.UnmarshalR5(scanner.Bytes())
	if err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	docRef := cr.GetDocumentReference()
	if got := docRef.GetIdentifier()[0].GetValue().GetValue(); got != "correct-syfon-did" {
		t.Fatalf("expected syfon identifier to be rewritten, got %q", got)
	}
	if got := docRef.GetIdentifier()[1].GetValue().GetValue(); got != "sha-existing" {
		t.Fatalf("expected existing secondary identifier to be preserved after syfon identifier, got %q", got)
	}
}

func TestDocRefSHA256ReadsIdentifierFallback(t *testing.T) {
	docRef := templateDocRef(&MetaObject{
		ID:        "drs-1",
		Name:      "file.txt",
		Size:      10,
		Checksums: map[string]string{},
	}, "localhost", "proj", "rs-1").GetDocumentReference()
	docRef.Identifier = append(docRef.Identifier, &dtpb.Identifier{
		System: &dtpb.Uri{Value: "https://humantumoratlas.org/FILE_SHA256"},
		Value:  &dtpb.String{Value: "abc123"},
	})

	if got := docRefSHA256(docRef); got != "abc123" {
		t.Fatalf("expected identifier fallback sha256, got %q", got)
	}
}

func TestStripRootDirField(t *testing.T) {
	line := []byte(`{"resourceType":"ResearchStudy","id":"rs1","rootDir":{"reference":"Directory/x"}}`)
	out := stripRootDirField(line)
	if strings.Contains(string(out), "rootDir") {
		t.Fatalf("rootDir should have been removed: %s", string(out))
	}
}
