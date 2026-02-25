package metadata

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/fhir/go/fhirversion"
	"github.com/google/fhir/go/jsonformat"
)

// VisualizeTree reads NDJSON files and prints a tree structure to the provided writer.
func VisualizeTree(out io.Writer, metaDir string, maxDepth int) error {
	dirFP := filepath.Join(metaDir, DIRECTORY_RESOURCE+NDJSON_EXT)
	docRefFP := filepath.Join(metaDir, DOCUMENT_RESOURCE+NDJSON_EXT)

	// Maps for building the tree
	// dirID -> Directory object
	dirs := make(map[string]*Directory)
	// docRefID -> DocumentReference title
	docRefs := make(map[string]string)

	// 1. Load Directories
	if err := loadNDJSON(dirFP, func(line []byte) error {
		var dir Directory
		if err := json.Unmarshal(line, &dir); err != nil {
			return err
		}
		dirs[dir.Id] = &dir
		return nil
	}); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error loading directories: %v", err)
	}

	// 2. Load DocumentReferences
	unmarshaller, err := jsonformat.NewUnmarshallerWithoutValidation("UTC", fhirversion.R5)
	if err != nil {
		return fmt.Errorf("failed to create FHIR unmarshaller: %v", err)
	}

	if err := loadNDJSON(docRefFP, func(line []byte) error {
		dr, err := unmarshaller.UnmarshalR5(line)
		if err != nil {
			return err
		}
		docRef := dr.GetDocumentReference()
		if docRef != nil {
			id := docRef.GetId().GetValue()
			title := ""
			if len(docRef.Content) > 0 {
				title = docRef.Content[0].GetAttachment().GetTitle().GetValue()
			}
			if title == "" {
				title = id
			}
			// Only keep the filename for the tree display
			docRefs[id] = filepath.Base(title)
		}
		return nil
	}); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error loading document references: %v", err)
	}

	// 3. Find the root directory(ies)
	// Usually there is one root directory with path "/"
	var roots []*Directory
	for _, d := range dirs {
		if d.Path == "/" || d.Name == "/" {
			roots = append(roots, d)
		}
	}

	if len(roots) == 0 && len(dirs) > 0 {
		// If no clear root, might be a disconnected tree, find nodes that aren't children of others?
		// For now just pick any if "/" is missing
		for _, d := range dirs {
			roots = append(roots, d)
			break
		}
	}

	if len(roots) == 0 {
		if len(docRefs) > 0 {
			fmt.Fprintln(out, "No directory structure found, but found these files:")
			for _, title := range docRefs {
				fmt.Fprintf(out, "  %s\n", title)
			}
			return nil
		}
		fmt.Fprintln(out, "No metadata records found in", metaDir)
		return nil
	}

	// 4. Print the tree
	fmt.Fprintf(out, "Metadata Tree Structure (from %s):\n", metaDir)
	for _, root := range roots {
		printNode(out, root, dirs, docRefs, "", true, 0, maxDepth)
	}

	return nil
}

func loadNDJSON(path string, processor func([]byte) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if err := processor(line); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func printNode(out io.Writer, dir *Directory, dirs map[string]*Directory, docRefs map[string]string, prefix string, isLast bool, currentDepth int, maxDepth int) {
	if maxDepth >= 0 && currentDepth > maxDepth {
		return
	}

	connector := "├── "
	if isLast {
		connector = "└── "
	}

	fmt.Fprintf(out, "%s%s%s/\n", prefix, connector, dir.Name)

	if maxDepth >= 0 && currentDepth == maxDepth {
		// If there are more children, show an ellipsis or something?
		if len(dir.Child) > 0 {
			newPrefix := prefix + "    "
			if !isLast {
				newPrefix = prefix + "│   "
			}
			fmt.Fprintf(out, "%s└── ... (%d children)\n", newPrefix, len(dir.Child))
		}
		return
	}

	newPrefix := prefix + "│   "
	if isLast {
		newPrefix = prefix + "    "
	}

	// Sort children for deterministic output
	type childInfo struct {
		name   string
		isDir  bool
		nodeID string
	}
	var children []childInfo

	for _, childRef := range dir.Child {
		refStr, _ := extractReferenceString(childRef)
		if strings.HasPrefix(refStr, "Directory/") {
			id := strings.TrimPrefix(refStr, "Directory/")
			if d, ok := dirs[id]; ok {
				children = append(children, childInfo{name: d.Name, isDir: true, nodeID: id})
			}
		} else if strings.HasPrefix(refStr, DOCUMENT_RESOURCE+"/") {
			id := strings.TrimPrefix(refStr, DOCUMENT_RESOURCE+"/")
			if name, ok := docRefs[id]; ok {
				children = append(children, childInfo{name: name, isDir: false, nodeID: id})
			}
		}
	}

	sort.Slice(children, func(i, j int) bool {
		if children[i].isDir != children[j].isDir {
			return children[i].isDir // Directories first
		}
		return children[i].name < children[j].name
	})

	for i, child := range children {
		lastChild := i == len(children)-1
		if child.isDir {
			printNode(out, dirs[child.nodeID], dirs, docRefs, newPrefix, lastChild, currentDepth+1, maxDepth)
		} else {
			childConnector := "├── "
			if lastChild {
				childConnector = "└── "
			}
			fmt.Fprintf(out, "%s%s%s\n", newPrefix, childConnector, child.name)
		}
	}
}
