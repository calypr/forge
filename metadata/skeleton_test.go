package metadata

import (
	"strings"
	"testing"

	"github.com/calypr/data-client/drs"
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
		t.Errorf("expected deterministic IDs, got %s and %s", id1, id2)
	}

	// Changing endpoint should change ID
	id3 := createIDFromStrings("http://other.com", resourceType, projectID)
	if id1 == id3 {
		t.Errorf("expected different IDs for different endpoints")
	}
}

func TestGetSystem(t *testing.T) {
	tests := []struct {
		name       string
		endpoint   string
		identifier string
		project    string
		expected   string
	}{
		{
			name:       "Simple",
			endpoint:   "http://api",
			identifier: "Res",
			project:    "proj",
			expected:   "http://api/proj",
		},
		{
			name:       "With Hash",
			endpoint:   "http://api",
			identifier: "Type#id",
			project:    "proj",
			expected:   "Type",
		},
		{
			name:       "With Pipe",
			endpoint:   "http://api",
			identifier: "Type|id",
			project:    "proj",
			expected:   "Type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSystem(tt.endpoint, tt.identifier, tt.project)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestCreateResourceReference(t *testing.T) {
	id := "uuid-123"
	ref := CreateResourceReference(id)
	if ref.GetResourceId().Value != id {
		t.Errorf("expected %s, got %s", id, ref.GetResourceId().Value)
	}
}

func TestTemplateDocRef(t *testing.T) {
	obj := &drs.DRSObject{
		Id:   "drs-1",
		Name: "test.txt",
		Size: 100,
		Checksums: drs.HashInfo{
			SHA256: "abc",
			MD5:    "def",
		},
		CreatedTime: "2023-10-27T10:00:00Z",
		AccessMethods: []drs.AccessMethod{
			{
				AccessURL: drs.AccessURL{URL: "s3://bucket/test.txt"},
			},
		},
	}
	endpoint := "localhost"
	project := "test-proj"
	rsID := "rs-1"

	res := templateDocRef(obj, endpoint, project, rsID)
	dr := res.GetDocumentReference()

	if dr == nil {
		t.Fatal("expected DocumentReference")
	}

	if dr.Content[0].Attachment.Size.Value != 100 {
		t.Errorf("expected size 100, got %d", dr.Content[0].Attachment.Size.Value)
	}

	if dr.Content[0].Attachment.Title.Value != "test.txt" {
		t.Errorf("expected title test.txt, got %s", dr.Content[0].Attachment.Title.Value)
	}

	// Check for extensions
	foundSHA256 := false
	foundMD5 := false
	for _, ext := range dr.Content[0].Attachment.Extension {
		if strings.Contains(ext.Url.Value, "checksum-sha256") {
			foundSHA256 = true
			if val, ok := ext.Value.Choice.(*dtpb.Extension_ValueX_StringValue); ok {
				if val.StringValue.Value != "abc" {
					t.Errorf("expected sha256 abc, got %s", val.StringValue.Value)
				}
			} else {
				t.Error("expected string value for sha256 extension")
			}
		}
		if strings.Contains(ext.Url.Value, "checksum-md5") {
			foundMD5 = true
			if val, ok := ext.Value.Choice.(*dtpb.Extension_ValueX_StringValue); ok {
				if val.StringValue.Value != "def" {
					t.Errorf("expected md5 def, got %s", val.StringValue.Value)
				}
			} else {
				t.Error("expected string value for md5 extension")
			}
		}
	}

	if !foundSHA256 {
		t.Error("sha256 extension not found")
	}
	if !foundMD5 {
		t.Error("md5 extension not found")
	}

	if dr.Subject.GetResearchStudyId().Value != rsID {
		t.Errorf("expected subject rs-1, got %s", dr.Subject.GetResearchStudyId().Value)
	}
}

func TestTemplateGitHubDocRef(t *testing.T) {
	name := "README.md"
	size := int64(500)
	endpoint := "localhost"
	project := "test-proj"
	rsID := "rs-1"
	githubURL := "github.com/user/repo"
	commitHash := "abcd123"

	res := templateGitHubDocRef(name, size, endpoint, project, rsID, githubURL, commitHash)
	dr := res.GetDocumentReference()

	if dr == nil {
		t.Fatal("expected DocumentReference")
	}

	expectedURL := "https://github.com/user/repo/blob/abcd123/README.md"
	gotURL := dr.Content[0].Attachment.Url.Value
	if gotURL != expectedURL {
		t.Errorf("expected URL %s, got %s", expectedURL, gotURL)
	}

	// Test non-github URL
	githubURL2 := "source.ohsu.edu/user/repo"
	res2 := templateGitHubDocRef(name, size, endpoint, project, rsID, githubURL2, commitHash)
	dr2 := res2.GetDocumentReference()
	expectedURL2 := "https://source.ohsu.edu/user/repo/blob/abcd123/README.md"
	gotURL2 := dr2.Content[0].Attachment.Url.Value
	if gotURL2 != expectedURL2 {
		t.Errorf("expected URL %s, got %s", expectedURL2, gotURL2)
	}
}
