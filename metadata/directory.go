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

	// Determine the best scanner buffer size based on actual file size
	maxCapacity := 10 * 1024 * 1024 // 10MB default
	if info, err := file.Stat(); err == nil && info.Size() > int64(maxCapacity) {
		maxCapacity = int(info.Size())
	}

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxCapacity)
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

		// Always use filepath.Clean for consistent cache keys (Standardizes root as "." and removes leading slashes)
		cleanPath := filepath.Clean(dir.Path)
		cacheKey := project + ":" + cleanPath
		DirectoryCache[cacheKey] = &dir
	}
	return scanner.Err()
}

// EnsureDirectoryPathExists recursively checks and creates all parent directories
// for a given POSIX path (e.g., "/a/b/c"). It returns the Directory
// object for the target path.
func EnsureDirectoryPathExists(endpoint string, project string, posixPath string) *Directory {
	// Clean and normalize the path. Relative paths become ".", "a/b", etc.
	cleanPath := filepath.Clean(posixPath)

	cacheKey := project + ":" + cleanPath
	// If the directory already exists in our cache, return it
	if dir, ok := DirectoryCache[cacheKey]; ok {
		return dir
	}

	dirUUID := uuid.NewSHA1(uuid.NewSHA1(uuid.NameSpaceDNS, []byte(endpoint)), []byte(project+cleanPath)).String()
	// Base case: Handle the root path "." (result of filepath.Clean on empty or "/")
	if cleanPath == "." || cleanPath == "/" {
		// Standardize root to "." in cache but display as "/" in the tree
		rootKey := project + ":."
		if dir, ok := DirectoryCache[rootKey]; ok {
			return dir
		}

		DirectoryCache[rootKey] = &Directory{
			Name:         "/",
			Id:           dirUUID,
			Path:         ".",
			ResourceType: DIRECTORY_RESOURCE,
		}
		return DirectoryCache[rootKey]
	}

	// For relative paths that hit ".", treat them as root
	if cleanPath == "." {
		return EnsureDirectoryPathExists(endpoint, project, "/")
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
	if len(docRef.Content) == 0 || docRef.Content[0].GetAttachment() == nil {
		log.Println("DocumentReference missing attachment.")
		return
	}

	posixPath := logicalDocRefPath(docRef)
	if posixPath == "" {
		log.Printf("Skipping DocumentReference %s: no logical path available", docRef.GetId().GetValue())
		return
	}

	// Standardize to relative path (no leading slash) for consistent caching
	posixPath = strings.TrimPrefix(posixPath, "/")
	posixPath = filepath.Clean(posixPath)

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

func logicalDocRefPath(docRef *drpb.DocumentReference) string {
	if docRef == nil || len(docRef.Content) == 0 || docRef.Content[0].GetAttachment() == nil {
		return ""
	}

	attachment := docRef.Content[0].GetAttachment()
	source := attachmentSource(attachment)
	title := normalizeLogicalPath(attachment.GetTitle().GetValue())
	titleHasHierarchy := strings.Contains(title, "/")

	// Keep GitHub paths aligned with repository structure.
	if source == GITHUB_SOURCE {
		if title != "" {
			return title
		}
		if raw := attachment.GetUrl().GetValue(); raw != "" {
			if path := normalizeLogicalPath(raw); path != "" {
				return path
			}
		}
		return ""
	}

	// For S3 sources, prefer title when it already carries hierarchy.
	if source == S3_SOURCE && titleHasHierarchy {
		return title
	}

	// Prefer source_path extension when available.
	for _, ext := range attachment.GetExtension() {
		if !strings.HasSuffix(ext.GetUrl().GetValue(), SOURCE_PATH_EXTENSION_URL) {
			continue
		}
		if raw := ext.GetValue().GetUrl().GetValue(); raw != "" {
			if path := normalizeLogicalPath(raw); path != "" {
				return path
			}
		}
		if raw := ext.GetValue().GetUri().GetValue(); raw != "" {
			if path := normalizeLogicalPath(raw); path != "" {
				return path
			}
		}
		if raw := ext.GetValue().GetStringValue().GetValue(); raw != "" {
			if path := normalizeLogicalPath(raw); path != "" {
				return path
			}
		}
	}

	if raw := attachment.GetUrl().GetValue(); raw != "" {
		if path := normalizeLogicalPath(raw); path != "" {
			return path
		}
	}

	if title != "" {
		return title
	}

	return ""
}

func attachmentSource(attachment *dtpb.Attachment) string {
	for _, ext := range attachment.GetExtension() {
		if strings.HasSuffix(ext.GetUrl().GetValue(), SOURCE_EXTENSION_URL) {
			if v := ext.GetValue().GetStringValue().GetValue(); v != "" {
				return strings.ToLower(strings.TrimSpace(v))
			}
		}
	}
	return ""
}

func normalizeLogicalPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return cleanRelativePath(raw)
		}

		switch strings.ToLower(parsed.Scheme) {
		case "s3":
			// s3://bucket/key -> key
			return cleanRelativePath(parsed.Path)
		case "file":
			// file:///bucket/key -> key
			path := strings.TrimPrefix(parsed.Path, "/")
			parts := strings.Split(path, "/")
			if len(parts) > 1 {
				path = strings.Join(parts[1:], "/")
			}
			return cleanRelativePath(path)
		default:
			path := strings.TrimPrefix(parsed.Path, "/")
			parts := strings.Split(path, "/")
			// https://github.com/org/repo/blob/<sha>/path/to/file -> path/to/file
			for i, p := range parts {
				if p == "blob" && i+2 < len(parts) {
					return cleanRelativePath(strings.Join(parts[i+2:], "/"))
				}
			}
			// For generic HTTP(S) URLs, avoid using storage-like paths as directory keys.
			return ""
		}
	}

	return cleanRelativePath(raw)
}

func cleanRelativePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return ""
	}
	clean := filepath.Clean(path)
	if clean == "." {
		return ""
	}
	return clean
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
