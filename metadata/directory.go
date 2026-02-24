package metadata

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

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
	// Path: The full POSIX path to the directory (e.g., "/a/b/c").
	Path string `json:"path"`
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
		"path":         d.Path,
		"child":        children, // The new flattened array of references
	}
	return json.Marshal(finalMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for the Directory struct.
func (d *Directory) UnmarshalJSON(data []byte) error {
	type Alias Directory
	aux := &struct {
		Child []map[string]string `json:"child"`
		*Alias
	}{
		Alias: (*Alias)(d),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	d.Child = nil
	for _, c := range aux.Child {
		refStr := c["reference"]
		if refStr == "" {
			continue
		}

		var ref *dtpb.Reference
		if strings.HasPrefix(refStr, DOCUMENT_RESOURCE+"/") {
			id := strings.TrimPrefix(refStr, DOCUMENT_RESOURCE+"/")
			ref = CreateDocReferenceReference(id)
		} else if strings.HasPrefix(refStr, "Directory/") {
			id := strings.TrimPrefix(refStr, "Directory/")
			ref = CreateResourceReference(id)
		} else {
			// Handle other potential reference prefixes or raw IDs
			ref = CreateResourceReference(refStr)
		}
		d.Child = append(d.Child, ref)
	}
	return nil
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
		// ResourceId can be either Directory or other resource types.
		// If it's a Directory, we assume it's just the ID value.
		return "Directory/" + r.ResourceId.Value, nil
	}
	return "", fmt.Errorf("unsupported or empty reference type in Protobuf: %T", ref.Reference)
}

// LoadDirectories loads existing Directory records from an NDJSON file into DirectoryCache.
func LoadDirectories(path string, project string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No existing directories is fine
		}
		return fmt.Errorf("error opening Directory file %s: %v", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var dir Directory
		if err := json.Unmarshal(line, &dir); err != nil {
			log.Printf("Warning: Failed to unmarshal Directory from %s: %v", path, err)
			continue
		}

		// Rebuild the cache key using the path if available, otherwise just project:id
		cacheKey := project + ":" + dir.Id
		if dir.Path != "" {
			cacheKey = project + ":" + dir.Path
		}
		DirectoryCache[cacheKey] = &dir
	}
	return scanner.Err()
}

// EnsureDirectoryPathExists recursively checks and creates all parent directories
// for a given POSIX path (e.g., "/a/b/c"). It returns the Directory
// object for the target path.
func EnsureDirectoryPathExists(endpoint string, project string, posixPath string) *Directory {
	// Clean and normalize the path to standard POSIX separators (/)
	cleanPath := filepath.Clean(posixPath)
	// Use the clean path as the unique key in our cache.
	if cleanPath == "." {
		cleanPath = "/"
	}

	cacheKey := project + ":" + cleanPath
	// If the directory already exists in our cache, return it
	if dir, ok := DirectoryCache[cacheKey]; ok {
		return dir
	}

	dirUUID := uuid.NewSHA1(uuid.NewSHA1(uuid.NameSpaceDNS, []byte(endpoint)), []byte(project+cleanPath)).String()
	// Base case: Handle the root path "/"
	if cleanPath == "/" {
		DirectoryCache[cacheKey] = &Directory{
			Name:         "/",
			Id:           dirUUID,
			Path:         "/",
			ResourceType: DIRECTORY_RESOURCE,
		}
		return DirectoryCache[cacheKey]
	}

	// Determine the parent path and recursively ensure it exists
	dirName := filepath.Base(cleanPath)
	parentPath := filepath.Dir(cleanPath)
	parentDir := EnsureDirectoryPathExists(endpoint, project, parentPath)

	// Create the current Directory object with its deterministic UUID
	currentDir := &Directory{
		Name:         dirName,
		Id:           dirUUID,
		Path:         cleanPath,
		ResourceType: DIRECTORY_RESOURCE,
	}

	// Add the new directory to the cache
	DirectoryCache[cacheKey] = currentDir

	// Link the current directory to its parent
	if parentDir != nil {
		// Create a Resource Reference pointing to the child directory's full UUID-based ID
		childRef := CreateResourceReference(dirUUID)

		// Check if the link already exists (avoiding duplicates)
		isAlreadyLinked := false
		for _, link := range parentDir.Child {
			refStr, _ := extractReferenceString(link)
			if refStr == DIR_ID_PREFIX+dirUUID {
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
func BuildDirectoryTreeFromDocRef(endpoint string, project string, docRef *drpb.DocumentReference) {
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
	parentDir := EnsureDirectoryPathExists(endpoint, project, dirPath)

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

// RefreshDirectoryChildren removes stale DocumentReference links from all directories in the cache.
// It keeps references that are NOT DocumentReferences (manual linkages) or are in the provided validIDs map.
func RefreshDirectoryChildren(validDocRefIDs map[string]bool) {
	for _, dir := range DirectoryCache {
		var newChildren []*dtpb.Reference
		for _, child := range dir.Child {
			refStr, err := extractReferenceString(child)
			if err != nil {
				continue
			}

			if strings.HasPrefix(refStr, DOCUMENT_RESOURCE+"/") {
				docID := strings.TrimPrefix(refStr, DOCUMENT_RESOURCE+"/")
				if validDocRefIDs[docID] {
					newChildren = append(newChildren, child)
				}
				// If not in validDocRefIDs, it's a stale DocumentReference, so we drop it.
			} else {
				// Keep non-DocumentReference links (e.g. manual linkages to other resources)
				newChildren = append(newChildren, child)
			}
		}
		dir.Child = newChildren
	}
}

// ClearDocRefLinks removes all DocumentReference references from all Directory objects in the cache.
// This allows the directory tree to be rebuilt cleanly from current DocumentReference states.
func ClearDocRefLinks() {
	for _, dir := range DirectoryCache {
		var newChildren []*dtpb.Reference
		for _, child := range dir.Child {
			refStr, err := extractReferenceString(child)
			if err != nil {
				continue
			}

			// Keep only references that are NOT DocumentReferences
			if !strings.HasPrefix(refStr, DOCUMENT_RESOURCE+"/") {
				newChildren = append(newChildren, child)
			}
		}
		dir.Child = newChildren
	}
}
