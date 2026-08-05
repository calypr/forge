package metadata

import (
	"reflect"
	"testing"

	gitinventory "github.com/calypr/git-drs/inventory"
)

func TestReconcileIssueSortsAndDeduplicatesPaths(t *testing.T) {
	issue := reconcileIssue("abc", []gitinventory.Pointer{
		{Path: "z/file.txt"},
		{Path: "a/file.txt"},
		{Path: "z/file.txt"},
	}, 0)
	if issue.SHA256 != "abc" || issue.SyfonRecords != 0 {
		t.Fatalf("unexpected issue: %+v", issue)
	}
	if want := []string{"a/file.txt", "z/file.txt"}; !reflect.DeepEqual(issue.Paths, want) {
		t.Fatalf("paths = %#v, want %#v", issue.Paths, want)
	}
}
