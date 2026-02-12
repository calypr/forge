package metadata

import (
	"encoding/json"
	"strings"
	"testing"

	dtpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/datatypes_go_proto"
	drpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/resources/document_reference_go_proto"
)

func resetCache() {
	for k := range DirectoryCache {
		delete(DirectoryCache, k)
	}
}

func TestEnsureDirectoryPathExists(t *testing.T) {
	endpoint := "localhost"

	t.Run("Root", func(t *testing.T) {
		resetCache()
		dir := EnsureDirectoryPathExists(endpoint, "test-project", "/")
		if dir.Name != "/" {
			t.Errorf("expected /, got %s", dir.Name)
		}
		if len(DirectoryCache) != 1 {
			t.Errorf("expected 1 entry in cache, got %d", len(DirectoryCache))
		}
	})

	t.Run("Nested", func(t *testing.T) {
		resetCache()
		dir := EnsureDirectoryPathExists(endpoint, "test-project", "/a/b/c")
		if dir.Name != "c" {
			t.Errorf("expected c, got %s", dir.Name)
		}
		// Should have /, /a, /a/b, /a/b/c
		if len(DirectoryCache) != 4 {
			t.Errorf("expected 4 entries in cache, got %d", len(DirectoryCache))
		}

		// Verify parent pointers
		parentB := DirectoryCache["test-project:/a/b"]
		foundC := false
		for _, child := range parentB.Child {
			if strings.Contains(child.GetResourceId().Value, dir.Id) {
				foundC = true
				break
			}
		}
		if !foundC {
			t.Error("directory c not found in parent b children")
		}
	})
}

func TestBuildDirectoryTreeFromDocRef(t *testing.T) {
	resetCache()
	endpoint := "localhost"
	docRef := &drpb.DocumentReference{
		Id: &dtpb.Id{Value: "doc-1"},
		Content: []*drpb.DocumentReference_Content{
			{
				Attachment: &dtpb.Attachment{
					Title: &dtpb.String{Value: "s3://bucket/data/file.txt"},
					Url:   &dtpb.Url{Value: "s3://bucket/data/file.txt"},
				},
			},
		},
	}

	BuildDirectoryTreeFromDocRef(endpoint, "test-project", docRef)

	// Path /data should exist
	dir, ok := DirectoryCache["test-project:/data"]
	if !ok {
		t.Fatal("/data not found in cache")
	}

	foundDoc := false
	for _, child := range dir.Child {
		if child.GetDocumentReferenceId().Value == "doc-1" {
			foundDoc = true
			break
		}
	}
	if !foundDoc {
		t.Error("document-1 not found in /data children")
	}
}

func TestDirectoryMarshalJSON(t *testing.T) {
	dir := &Directory{
		Name:         "test",
		Id:           "uuid-1",
		ResourceType: "Directory",
		Child: []*dtpb.Reference{
			{
				Reference: &dtpb.Reference_DocumentReferenceId{
					DocumentReferenceId: &dtpb.ReferenceId{Value: "doc-1"},
				},
			},
		},
	}

	data, err := dir.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if m["name"] != "test" {
		t.Errorf("expected test, got %v", m["name"])
	}

	children := m["child"].([]any)
	if len(children) != 1 {
		t.Errorf("expected 1 child, got %d", len(children))
	}

	child := children[0].(map[string]any)
	if child["reference"] != "DocumentReference/doc-1" {
		t.Errorf("expected DocumentReference/doc-1, got %v", child["reference"])
	}
}

func TestDirectoryProjectIsolation(t *testing.T) {
	resetCache()
	endpoint := "localhost"
	path := "/shared/data"

	// Project A
	dirA := EnsureDirectoryPathExists(endpoint, "project-A", path)
	// Project B
	dirB := EnsureDirectoryPathExists(endpoint, "project-B", path)

	if dirA.Id == dirB.Id {
		t.Errorf("expected different IDs for different projects, but both got %s", dirA.Id)
	}

	if len(DirectoryCache) != 6 { // Root, shared, data for each project = 3 * 2
		t.Errorf("expected 6 entries in cache (3 per project), got %d", len(DirectoryCache))
	}

	// Verify they are both in the cache under their respective keys
	if _, ok := DirectoryCache["project-A:/shared/data"]; !ok {
		t.Error("project-A path missing from cache")
	}
	if _, ok := DirectoryCache["project-B:/shared/data"]; !ok {
		t.Error("project-B path missing from cache")
	}
}
