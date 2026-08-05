package metadata

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/calypr/syfon/apigen/client/internalapi"
	fver "github.com/google/fhir/go/fhirversion"
	"github.com/google/fhir/go/jsonformat"
	dtpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/datatypes_go_proto"
	cprb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/resources/bundle_and_contained_resource_go_proto"
)

func TestAppendDocumentReferencesPreservesAuthoredRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), DOCUMENT_RESOURCE+NDJSON_EXT)
	authored := []byte(`{"resourceType":"DocumentReference", "id":"authored", "identifier":[]}`)
	generated := []byte(`{"resourceType":"DocumentReference","id":"generated"}`)
	if err := appendDocumentReferences(path, [][]byte{authored}, [][]byte{generated}); err != nil {
		t.Fatalf("appendDocumentReferences failed: %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := string(authored) + "\n" + string(generated) + "\n"
	if string(contents) != want {
		t.Fatalf("authored row changed:\nwant %q\n got %q", want, string(contents))
	}
}

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

func TestRepairDocumentReferenceIdentifiersBySHARewritesOnlyExistingRows(t *testing.T) {
	tmpDir := t.TempDir()
	endpoint := "https://calypr-public.ohsu.edu"
	project := "BForePC"
	rsID := "rs-1"

	staleUUID := templateDocRef(&MetaObject{
		ID:        "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		Name:      "stale.txt",
		Size:      10,
		Checksums: map[string]string{"sha-256": "sha-stale"},
		AccessURL: "s3://bucket/stale.txt",
	}, endpoint, project, rsID)
	staleUUID.GetDocumentReference().Identifier[0].Value = &dtpb.String{Value: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"}

	stalePath := templateDocRef(&MetaObject{
		ID:        "cccccccc-cccc-cccc-cccc-cccccccccccc",
		Name:      "path.txt",
		Size:      10,
		Checksums: map[string]string{"sha-256": "sha-path"},
		AccessURL: "s3://bucket/path.txt",
	}, endpoint, project, rsID)
	stalePath.GetDocumentReference().Identifier[0] = &dtpb.Identifier{
		System: &dtpb.Uri{Value: "https://humantumoratlas.org/FILE_PATH"},
		Value:  &dtpb.String{Value: "s3://bucket/path.txt"},
	}

	marshaller, _ := jsonformat.NewMarshaller(false, "", "", fver.R5)
	docRefFP := filepath.Join(tmpDir, DOCUMENT_RESOURCE+NDJSON_EXT)
	file, err := os.Create(docRefFP)
	if err != nil {
		t.Fatalf("failed to create metadata: %v", err)
	}
	for _, cr := range []*cprb.ContainedResource{staleUUID, stalePath} {
		jsonBytes, err := marshaller.Marshal(cr)
		if err != nil {
			t.Fatalf("failed to marshal seed metadata: %v", err)
		}
		if _, err := file.Write(append(jsonBytes, '\n')); err != nil {
			t.Fatalf("failed to write seed metadata: %v", err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatalf("failed to close seed metadata: %v", err)
	}

	repaired, err := repairDocumentReferenceIdentifiersBySHA([]MetaObject{
		{ID: "real-did-for-stale", Checksums: map[string]string{"sha-256": "sha-stale"}},
		{ID: "real-did-for-path", Checksums: map[string]string{"sha-256": "sha-path"}},
		{ID: "new-row-must-not-be-added", Checksums: map[string]string{"sha-256": "sha-new"}},
	}, tmpDir, endpoint, project)
	if err != nil {
		t.Fatalf("repairDocumentReferenceIdentifiersBySHA failed: %v", err)
	}
	if repaired != 2 {
		t.Fatalf("expected 2 repaired rows, got %d", repaired)
	}

	out, err := os.ReadFile(docRefFP)
	if err != nil {
		t.Fatalf("failed to read repaired metadata: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		t.Fatalf("repair should not append new rows, got %d rows", len(lines))
	}

	unmarshaller, _ := jsonformat.NewUnmarshallerWithoutValidation("America/Los_Angeles", fver.R5)
	want := map[string]bool{
		"real-did-for-stale": false,
		"real-did-for-path":  false,
	}
	for _, line := range lines {
		cr, err := unmarshaller.UnmarshalR5([]byte(line))
		if err != nil {
			t.Fatalf("failed to unmarshal repaired row: %v", err)
		}
		got := cr.GetDocumentReference().GetIdentifier()[0].GetValue().GetValue()
		if _, ok := want[got]; !ok {
			t.Fatalf("unexpected primary identifier after repair: %q", got)
		}
		want[got] = true
	}
	for id, seen := range want {
		if !seen {
			t.Fatalf("missing repaired identifier %q", id)
		}
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

func TestProcessProjectRecordsRewritesRawExistingDocumentReferenceRow(t *testing.T) {
	tmpDir := t.TempDir()

	line := `{"author":[{"reference":"Practitioner/ef1a00f0-5410-51a1-a511-281eeb0afc6e"}],"category":[{"coding":[{"code":"HTAN_DATA_FILE_ID","display":"HTA201_3_D0214010500001","system":"https://humantumoratlas.org/HTAN_DATA_FILE_ID"}]},{"coding":[{"code":"FILE_FORMAT","display":"JSON","system":"https://humantumoratlas.org/FILE_FORMAT"}]},{"coding":[{"code":"FILENAME","display":"HTA201_3_1_0214.offsets.json","system":"https://humantumoratlas.org/FILENAME"}]}],"content":[{"attachment":{"contentType":"application/json","extension":[{"url":"http://aced-idp.org/fhir/StructureDefinition/source_path","valueUrl":"s3://bforepc-prod/JHU/ashley_kiemen/hematoxylin_eosin_stain/Level_2/HTA201_3/HTA201_3_ometif/HTA201_3_1_0214.offsets.json"},{"url":"http://aced-idp.org/fhir/StructureDefinition/sha256","valueString":"f8528679a98ceff8576bdbbe4e0d716f824c0ce0fb8a48f744e21e4815cbf280"}],"size":42,"title":"HTA201_3_1_0214.offsets.json","url":"file:///bforepc-prod/JHU/ashley_kiemen/hematoxylin_eosin_stain/Level_2/HTA201_3/HTA201_3_ometif/HTA201_3_1_0214.offsets.json"}}],"id":"803c810d-5188-52b2-b32d-f16097736fcc","identifier":[{"system":"https://humantumoratlas.org/FILE_PATH","use":"secondary","value":"s3://bforepc-prod/JHU/ashley_kiemen/hematoxylin_eosin_stain/Level_2/HTA201_3/HTA201_3_ometif/HTA201_3_1_0214.offsets.json"},{"system":"https://humantumoratlas.org/FILE_SHA256","use":"secondary","value":"f8528679a98ceff8576bdbbe4e0d716f824c0ce0fb8a48f744e21e4815cbf280"},{"system":"https://humantumoratlas.org/HTAN_DATA_FILE_ID","use":"secondary","value":"HTA201_3_D0214010500001"}],"resourceType":"DocumentReference","status":"current","subject":{"reference":"Specimen/806db23a-9673-5ab7-a37b-7e0b750559e7"}}`
	docRefFP := filepath.Join(tmpDir, DOCUMENT_RESOURCE+NDJSON_EXT)
	if err := os.WriteFile(docRefFP, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("failed to seed metadata: %v", err)
	}

	records := []MetaObject{{
		ID:          "15d4dd21-618e-55ca-b325-860f58705d3a",
		Name:        "JHU/ashley_kiemen/hematoxylin_eosin_stain/Level_2/HTA201_3/HTA201_3_ometif/HTA201_3_1_0214.offsets.json",
		Size:        42,
		Checksums:   map[string]string{"sha256": "f8528679a98ceff8576bdbbe4e0d716f824c0ce0fb8a48f744e21e4815cbf280"},
		CreatedTime: time.Date(2026, 3, 13, 20, 32, 55, 0, time.UTC),
		AccessURL:   "s3://bforepc-prod/JHU/ashley_kiemen/hematoxylin_eosin_stain/Level_2/HTA201_3/HTA201_3_ometif/HTA201_3_1_0214.offsets.json",
	}}

	if err := processProjectRecordsAndUpdateFHIR(records, tmpDir, "https://calypr-public.ohsu.edu", "BForePC", "rs-1"); err != nil {
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
	if got := docRef.GetIdentifier()[0].GetValue().GetValue(); got != "15d4dd21-618e-55ca-b325-860f58705d3a" {
		t.Fatalf("expected syfon identifier to be prepended, got %q", got)
	}
}

func TestProcessProjectRecordsDisambiguatesDuplicateSHAByPath(t *testing.T) {
	tmpDir := t.TempDir()

	sha := "same-sha"
	pathA := "s3://bucket/dir/a.offsets.json"
	pathB := "s3://bucket/dir/b.offsets.json"
	lineA := `{"resourceType":"DocumentReference","id":"doc-a","content":[{"attachment":{"extension":[{"url":"http://aced-idp.org/fhir/StructureDefinition/source_path","valueUrl":"` + pathA + `"},{"url":"http://aced-idp.org/fhir/StructureDefinition/sha256","valueString":"` + sha + `"}],"title":"a.offsets.json","url":"file:///bucket/dir/a.offsets.json"}}],"identifier":[{"system":"https://humantumoratlas.org/FILE_PATH","use":"secondary","value":"` + pathA + `"},{"system":"https://humantumoratlas.org/FILE_SHA256","use":"secondary","value":"` + sha + `"}],"status":"current"}`
	lineB := `{"resourceType":"DocumentReference","id":"doc-b","content":[{"attachment":{"extension":[{"url":"http://aced-idp.org/fhir/StructureDefinition/source_path","valueUrl":"` + pathB + `"},{"url":"http://aced-idp.org/fhir/StructureDefinition/sha256","valueString":"` + sha + `"}],"title":"b.offsets.json","url":"file:///bucket/dir/b.offsets.json"}}],"identifier":[{"system":"https://humantumoratlas.org/FILE_PATH","use":"secondary","value":"` + pathB + `"},{"system":"https://humantumoratlas.org/FILE_SHA256","use":"secondary","value":"` + sha + `"}],"status":"current"}`
	docRefFP := filepath.Join(tmpDir, DOCUMENT_RESOURCE+NDJSON_EXT)
	if err := os.WriteFile(docRefFP, []byte(lineA+"\n"+lineB+"\n"), 0o644); err != nil {
		t.Fatalf("failed to seed metadata: %v", err)
	}

	records := []MetaObject{
		{
			ID:        "did-a",
			Name:      "dir/a.offsets.json",
			Checksums: map[string]string{"sha256": sha},
			AccessURL: pathA,
		},
		{
			ID:        "did-b",
			Name:      "dir/b.offsets.json",
			Checksums: map[string]string{"sha256": sha},
			AccessURL: pathB,
		},
	}

	if err := processProjectRecordsAndUpdateFHIR(records, tmpDir, "https://calypr-public.ohsu.edu", "BForePC", "rs-1"); err != nil {
		t.Fatalf("processProjectRecordsAndUpdateFHIR failed: %v", err)
	}

	file, err := os.Open(docRefFP)
	if err != nil {
		t.Fatalf("failed to open output file: %v", err)
	}
	defer file.Close()

	unmarshaller, _ := jsonformat.NewUnmarshallerWithoutValidation("America/Los_Angeles", fver.R5)
	found := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		cr, err := unmarshaller.UnmarshalR5(scanner.Bytes())
		if err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}
		docRef := cr.GetDocumentReference()
		sourcePath := ""
		for _, ext := range docRef.GetContent()[0].GetAttachment().GetExtension() {
			if strings.HasSuffix(ext.GetUrl().GetValue(), "/source_path") {
				if value, ok := ext.GetValue().GetChoice().(*dtpb.Extension_ValueX_Url); ok {
					sourcePath = value.Url.GetValue()
				}
			}
		}
		if sourcePath != "" {
			found[sourcePath] = docRef.GetIdentifier()[0].GetValue().GetValue()
		}
	}

	if got := found[pathA]; got != "did-a" {
		t.Fatalf("expected %s to receive did-a, got %q", pathA, got)
	}
	if got := found[pathB]; got != "did-b" {
		t.Fatalf("expected %s to receive did-b, got %q", pathB, got)
	}
}

func TestProcessProjectRecordsEmitsMultipleSyfonObjectsWithSameSHA(t *testing.T) {
	tmpDir := t.TempDir()

	records := []MetaObject{
		{
			ID:        "did-a",
			Name:      "dir/a.offsets.json",
			Checksums: map[string]string{"sha256": "same-sha"},
			AccessURL: "s3://bucket/dir/a.offsets.json",
		},
		{
			ID:        "did-b",
			Name:      "dir/b.offsets.json",
			Checksums: map[string]string{"sha256": "same-sha"},
			AccessURL: "s3://bucket/dir/b.offsets.json",
		},
	}

	if err := processProjectRecordsAndUpdateFHIR(records, tmpDir, "https://calypr-public.ohsu.edu", "BForePC", "rs-1"); err != nil {
		t.Fatalf("processProjectRecordsAndUpdateFHIR failed: %v", err)
	}

	docRefFP := filepath.Join(tmpDir, DOCUMENT_RESOURCE+NDJSON_EXT)
	file, err := os.Open(docRefFP)
	if err != nil {
		t.Fatalf("failed to open output file: %v", err)
	}
	defer file.Close()

	unmarshaller, _ := jsonformat.NewUnmarshallerWithoutValidation("America/Los_Angeles", fver.R5)
	found := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		cr, err := unmarshaller.UnmarshalR5(scanner.Bytes())
		if err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}
		docRef := cr.GetDocumentReference()
		found[docRef.GetIdentifier()[0].GetValue().GetValue()] = docRef.GetContent()[0].GetAttachment().GetUrl().GetValue()
	}

	if len(found) != 2 {
		t.Fatalf("expected 2 generated rows, got %d", len(found))
	}
	if got := found["did-a"]; got != "s3://bucket/dir/a.offsets.json" {
		t.Fatalf("expected did-a url to be preserved, got %q", got)
	}
	if got := found["did-b"]; got != "s3://bucket/dir/b.offsets.json" {
		t.Fatalf("expected did-b url to be preserved, got %q", got)
	}
}

func TestDocumentReferenceSHA256sReadsAllExistingRows(t *testing.T) {
	tmpDir := t.TempDir()
	docRefFP := filepath.Join(tmpDir, DOCUMENT_RESOURCE+NDJSON_EXT)
	lineA := `{"resourceType":"DocumentReference","id":"doc-a","content":[{"attachment":{"extension":[{"url":"http://aced-idp.org/fhir/StructureDefinition/sha256","valueString":"sha-a"}],"title":"a"}}],"identifier":[{"system":"https://humantumoratlas.org/FILE_PATH","use":"secondary","value":"s3://bucket/a"}],"status":"current"}`
	lineB := `{"resourceType":"DocumentReference","id":"doc-b","content":[{"attachment":{"extension":[{"url":"http://aced-idp.org/fhir/StructureDefinition/sha256","valueString":"sha-b"}],"title":"b"}}],"identifier":[{"system":"https://calypr-public.ohsu.edu/BForePC","use":"official","value":"15d4dd21-618e-55ca-b325-860f58705d3a"}],"status":"current"}`
	lineDup := `{"resourceType":"DocumentReference","id":"doc-c","content":[{"attachment":{"extension":[{"url":"http://aced-idp.org/fhir/StructureDefinition/sha256","valueString":"sha-a"}],"title":"c"}}],"identifier":[{"system":"https://humantumoratlas.org/FILE_PATH","use":"secondary","value":"s3://bucket/c"}],"status":"current"}`
	if err := os.WriteFile(docRefFP, []byte(lineA+"\n"+lineB+"\n"+lineDup+"\n"), 0o644); err != nil {
		t.Fatalf("failed to seed metadata: %v", err)
	}

	got, err := documentReferenceSHA256s(tmpDir)
	if err != nil {
		t.Fatalf("documentReferenceSHA256s returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 unique shas, got %d: %v", len(got), got)
	}
	found := map[string]bool{}
	for _, sha := range got {
		found[sha] = true
	}
	if !found["sha-a"] || !found["sha-b"] {
		t.Fatalf("unexpected shas: %v", got)
	}
}

func TestBulkSHA256QueriesUsesTypedHashForm(t *testing.T) {
	got := bulkSHA256Queries([]string{
		"ABC123",
		"",
		"sha256:def456",
	})
	want := []string{"sha256:abc123", "sha256:def456"}
	if len(got) != len(want) {
		t.Fatalf("expected %d queries, got %d: %v", len(want), len(got), got)
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("query %d = %q, want %q", idx, got[idx], want[idx])
		}
	}
}

func TestDecodeBulkHashRecordsReadsResultsPayload(t *testing.T) {
	body := []byte(`{
		"results": {
			"sha256:aaa": [
				{"did":"did-a","hashes":{"sha256":"aaa"}}
			],
			"sha256:bbb": [
				{"did":"did-b","hashes":{"sha256":"bbb"}}
			]
		}
	}`)
	got, err := decodeBulkHashRecords(body)
	if err != nil {
		t.Fatalf("decodeBulkHashRecords failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 decoded records, got %d: %#v", len(got), got)
	}
	found := map[string]bool{}
	for _, rec := range got {
		found[rec.Did] = true
	}
	if !found["did-a"] || !found["did-b"] {
		t.Fatalf("missing decoded records: %#v", got)
	}
}

func TestRecordMatchesScope(t *testing.T) {
	org := "HTAN_INT"
	project := "BForePC"
	rec := internalapi.InternalRecord{
		Did:          "did-1",
		Organization: &org,
		Project:      &project,
	}
	if !recordMatchesScope(rec, org, project) {
		t.Fatal("expected record to match scope")
	}
	if recordMatchesScope(rec, "OTHER", project) {
		t.Fatal("expected organization mismatch")
	}
	if recordMatchesScope(rec, org, "OTHER") {
		t.Fatal("expected project mismatch")
	}

	controlled := []string{"/organization/HTAN_INT/project/BForePC"}
	rec = internalapi.InternalRecord{
		Did:              "did-2",
		ControlledAccess: &controlled,
	}
	if !recordMatchesScope(rec, org, project) {
		t.Fatal("expected controlled_access scope to match")
	}
	if recordMatchesScope(rec, "OTHER", project) {
		t.Fatal("expected controlled_access organization mismatch")
	}

	rec = internalapi.InternalRecord{Did: "did-3"}
	if recordMatchesScope(rec, org, project) {
		t.Fatal("expected unscoped bulk hash record to be rejected")
	}
}

func TestMergeMetaObjectsKeepsPrimaryAndAppendsMissing(t *testing.T) {
	primary := []MetaObject{
		{ID: "did-1", Name: "a"},
		{ID: "did-2", Name: "b"},
	}
	extras := []MetaObject{
		{ID: "did-2", Name: "b-new"},
		{ID: "did-3", Name: "c"},
	}
	got := mergeMetaObjects(primary, extras)
	if len(got) != 3 {
		t.Fatalf("expected 3 merged objects, got %d", len(got))
	}
	if got[0].ID != "did-1" || got[1].ID != "did-2" || got[2].ID != "did-3" {
		t.Fatalf("unexpected merge order: %#v", got)
	}
}

func TestStripRootDirField(t *testing.T) {
	line := []byte(`{"resourceType":"ResearchStudy","id":"rs1","rootDir":{"reference":"Directory/x"}}`)
	out := stripRootDirField(line)
	if strings.Contains(string(out), "rootDir") {
		t.Fatalf("rootDir should have been removed: %s", string(out))
	}
}
