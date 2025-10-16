package metadata

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"path/filepath"

	dtpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/datatypes_go_proto"
	drpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/resources/document_reference_go_proto"
	"github.com/google/uuid"
)

// Directory represents the POSIX directory node structure.
type Directory struct {
	// Name: The name of the directory component.
	Name string `json:"name"`
	// Child: A reference to a downstream node (Directory or DocumentReference).
	Child []*dtpb.Reference `json:"child"`
	// Id: The unique ID for the Directory instance (e.g., Directory/path/to/dir).
	Id string `json:"id"`
	// ResourceType: Custom type to identify the structure in the NDJSON.
	ResourceType string `json:"resourceType"`
}

// MarshalJSON implements the json.Marshaler interface for the Directory struct.
func (d *Directory) MarshalJSON() ([]byte, error) {
	children := make([]map[string]string, 0, len(d.Child))
	for _, childRef := range d.Child {
		refStr, err := extractReferenceString(childRef)
		if err != nil {
			log.Printf("Warning: Skipping bad reference in Directory %s: %v", d.Id, err)
			continue
		}
		flatRef := map[string]string{
			"reference": refStr,
		}
		children = append(children, flatRef)
	}
	finalMap := map[string]any{
		"resourceType": d.ResourceType,
		"id":           d.Id,
		"name":         d.Name,
		"child":        children, // The new flattened array of references
	}
	return json.Marshal(finalMap)
}

func extractReferenceString(ref *dtpb.Reference) (string, error) {
	if ref == nil {
		return "", fmt.Errorf("reference is nil")
	}

	// Determine which oneof field is set and construct the full reference string.
	switch r := ref.Reference.(type) {
	case *dtpb.Reference_DocumentReferenceId:
		return DOCUMENT_RESOURCE + "/" + r.DocumentReferenceId.Value, nil
	case *dtpb.Reference_ResourceId:
		// Assume the ID value already contains the full reference (e.g., "Directory/path/to/dir").
		return "Directory/" + r.ResourceId.Value, nil
	}
	return "", fmt.Errorf("unsupported or empty reference type in Protobuf: %T", ref.Reference)
}

// EnsureDirectoryPathExists recursively checks and creates all parent directories
// for a given POSIX path (e.g., "/a/b/c"). It returns the Directory
// object for the target path.
func EnsureDirectoryPathExists(posixPath string) *Directory {
	// Clean and normalize the path to standard POSIX separators (/)
	cleanPath := filepath.Clean(posixPath)
	// Use the clean path as the unique key in our cache.
	if cleanPath == "." {
		cleanPath = "/"
	}

	// If the directory already exists in our cache, return it
	if dir, ok := DirectoryCache[cleanPath]; ok {
		return dir
	}

	dirUUID := uuid.NewSHA1(DirectoryNamespaceUUID, []byte(cleanPath)).String()
	// Base case: Handle the root path "/"
	if cleanPath == "/" {
		DirectoryCache[cleanPath] = &Directory{
			Name:         "/",
			Id:           dirUUID,
			ResourceType: DIRECTORY_RESOURCE,
		}
		return DirectoryCache[cleanPath]
	}

	// Determine the parent path and recursively ensure it exists
	dirName := filepath.Base(cleanPath)
	parentPath := filepath.Dir(cleanPath)
	parentDir := EnsureDirectoryPathExists(parentPath)

	// Create the current Directory object with its deterministic UUID
	currentDir := &Directory{
		Name:         dirName,
		Id:           dirUUID,
		ResourceType: DIRECTORY_RESOURCE,
	}

	// Add the new directory to the cache
	DirectoryCache[cleanPath] = currentDir

	// Link the current directory to its parent
	if parentDir != nil {
		// Create a Resource Reference pointing to the child directory's full UUID-based ID
		childRef := CreateResourceReference(dirUUID)

		// Check if the link already exists (avoiding duplicates)
		isAlreadyLinked := false
		for _, link := range parentDir.Child {
			refStr, _ := extractReferenceString(link)
			if refStr == dirUUID {
				isAlreadyLinked = true
				break
			}
		}

		if !isAlreadyLinked {
			parentDir.Child = append(parentDir.Child, childRef)
		}
	}

	return currentDir
}

// BuildDirectoryTreeFromDocRef extracts the path from a DocumentReference and builds the tree.
// It ensures all necessary Directory nodes are created and linked in the global DirectoryCache.
func BuildDirectoryTreeFromDocRef(docRef *drpb.DocumentReference) {
	if len(docRef.Content) == 0 || docRef.Content[0].GetAttachment().GetUrl().GetValue() == "" {
		log.Println("DocumentReference missing URL attachment.")
		return
	}

	rawURL := docRef.Content[0].GetAttachment().GetTitle().GetValue()
	u, err := url.Parse(rawURL)
	if err != nil {
		log.Printf("Error parsing URL %s for DocRef %s: %v\n", rawURL, docRef.GetId().GetValue(), err)
		return
	}

	posixPath := u.Path
	dirPath := filepath.Dir(posixPath)

	// Recursively create all directories up to the file's parent
	parentDir := EnsureDirectoryPathExists(dirPath)

	if parentDir != nil {
		docRefID := docRef.GetId().GetValue()
		fileRef := CreateDocReferenceReference(docRefID)

		// Link the DocumentReference to its parent directory
		isAlreadyLinked := false
		docRefFull := DOCUMENT_RESOURCE + "/" + docRefID

		for _, link := range parentDir.Child {
			refStr, _ := extractReferenceString(link)
			if refStr == docRefFull {
				isAlreadyLinked = true
				break
			}
		}

		if !isAlreadyLinked {
			parentDir.Child = append(parentDir.Child, fileRef)
		}
	}
}
