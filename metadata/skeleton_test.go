package metadata

import (
	"strings"
	"testing"
	"time"

	dtpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/datatypes_go_proto"
)

func TestCreateIDFromStrings(t *testing.T) {
	endpoint := "http://example.com"
	resourceType := "ResearchStudy"
	projectID := "test-project"

	id1 := createIDFromStrings(endpoint, resourceType, projectID)
	id2 := createIDFromStrings(endpoint, resourceType, projectID)

	if id1 == "" {
		t.Fatal("expected non-empty ID")
	}
	if id1 != id2 {
		t.Fatalf("expected deterministic IDs, got %s and %s", id1, id2)
	}
	if id1 == createIDFromStrings("http://other.com", resourceType, projectID) {
		t.Fatal("expected different IDs for different endpoints")
	}
}

func TestGetSystem(t *testing.T) {
	tests := []struct {
		identifier string
		expected   string
	}{
		{identifier: "Res", expected: "http://api/proj"},
		{identifier: "Type#id", expected: "Type"},
		{identifier: "Type|id", expected: "Type"},
	}

	for _, tt := range tests {
		got := getSystem("http://api", tt.identifier, "proj")
		if got != tt.expected {
			t.Fatalf("expected %s got %s", tt.expected, got)
		}
	}
}

func TestCreateResourceReference(t *testing.T) {
	id := "uuid-123"
	ref := CreateResourceReference(id)
	if ref.GetResourceId().GetValue() != id {
		t.Fatalf("expected %s got %s", id, ref.GetResourceId().GetValue())
	}
}

func TestTemplateDocRef(t *testing.T) {
	obj := &MetaObject{
		ID:          "drs-1",
		Name:        "test.txt",
		Size:        100,
		Checksums:   map[string]string{"sha-256": "abc", "md5": "def"},
		CreatedTime: time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC),
		AccessURL:   "s3://bucket/test.txt",
	}

	res := templateDocRef(obj, "localhost", "test-proj", "rs-1")
	dr := res.GetDocumentReference()
	if dr == nil {
		t.Fatal("expected DocumentReference")
	}
	if dr.GetContent()[0].GetAttachment().GetSize().GetValue() != 100 {
		t.Fatalf("expected size 100 got %d", dr.GetContent()[0].GetAttachment().GetSize().GetValue())
	}
	if dr.GetContent()[0].GetAttachment().GetTitle().GetValue() != "test.txt" {
		t.Fatalf("unexpected title")
	}

	foundSHA256 := false
	foundMD5 := false
	for _, ext := range dr.GetContent()[0].GetAttachment().GetExtension() {
		if strings.Contains(ext.GetUrl().GetValue(), "checksum-sha256") {
			foundSHA256 = true
			val := ext.GetValue().GetChoice().(*dtpb.Extension_ValueX_StringValue)
			if val.StringValue.GetValue() != "abc" {
				t.Fatalf("unexpected sha256 value")
			}
		}
		if strings.Contains(ext.GetUrl().GetValue(), "checksum-md5") {
			foundMD5 = true
			val := ext.GetValue().GetChoice().(*dtpb.Extension_ValueX_StringValue)
			if val.StringValue.GetValue() != "def" {
				t.Fatalf("unexpected md5 value")
			}
		}
	}
	if !foundSHA256 || !foundMD5 {
		t.Fatal("missing checksum extensions")
	}
	if dr.GetSubject().GetResearchStudyId().GetValue() != "rs-1" {
		t.Fatal("unexpected subject")
	}
}

func TestTemplateDocRefIdentityUsesProjectScopedSHA256(t *testing.T) {
	first := &MetaObject{
		ID:        "drs-a",
		Name:      "shared-name.json",
		Checksums: map[string]string{"sha-256": "AAAAAAAA"},
	}
	second := &MetaObject{
		ID:        "drs-b",
		Name:      "shared-name.json",
		Checksums: map[string]string{"sha256": "bbbbbbbb"},
	}
	renamed := &MetaObject{
		ID:        "drs-c",
		Name:      "renamed.json",
		Checksums: map[string]string{"sha-256": "aaaaaaaa"},
	}

	firstID := templateDocRef(first, "localhost", "test-proj", "rs-1").GetDocumentReference().GetId().GetValue()
	secondID := templateDocRef(second, "localhost", "test-proj", "rs-1").GetDocumentReference().GetId().GetValue()
	renamedID := templateDocRef(renamed, "localhost", "test-proj", "rs-1").GetDocumentReference().GetId().GetValue()
	otherProjectID := templateDocRef(first, "localhost", "other-proj", "rs-1").GetDocumentReference().GetId().GetValue()

	if firstID == secondID {
		t.Fatal("different SHA256 values with the same basename produced the same DocumentReference ID")
	}
	if firstID != renamedID {
		t.Fatal("the same SHA256 produced different DocumentReference IDs after a filename change")
	}
	if firstID == otherProjectID {
		t.Fatal("the same SHA256 produced the same DocumentReference ID across projects")
	}
}

func TestTemplateGitHubDocRef(t *testing.T) {
	res := templateGitHubDocRef("README.md", 500, "localhost", "test-proj", "rs-1", "github.com/user/repo", "abcd123")
	dr := res.GetDocumentReference()
	if dr == nil {
		t.Fatal("expected DocumentReference")
	}
	if got := dr.GetContent()[0].GetAttachment().GetUrl().GetValue(); got != "https://github.com/user/repo/blob/abcd123/README.md" {
		t.Fatalf("unexpected URL %s", got)
	}
}
