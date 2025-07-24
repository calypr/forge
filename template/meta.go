package template

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	fver "github.com/google/fhir/go/fhirversion"
	"github.com/google/fhir/go/jsonformat"
)

// DVCMetadata represents the structure of the "dvc_metadata" object in your JSON.
type Metadata struct {
	CRC       *string   `json:"crc"`  // Use *string for nullable fields
	ETag      *string   `json:"etag"` // Use *string for nullable fields
	Hash      string    `json:"hash"`
	IsSymlink bool      `json:"is_symlink"`
	MD5       string    `json:"md5"`
	MIME      string    `json:"mime"`
	Modified  time.Time `json:"modified"` // time.Time for ISO 8601 formatted date-time
	ObjectID  string    `json:"object_id"`
	Realpath  string    `json:"realpath"`
	SHA1      *string   `json:"sha1"`   // Use *string for nullable fields
	SHA256    *string   `json:"sha256"` // Use *string for nullable fields
	SHA512    *string   `json:"sha512"` // Use *string for nullable fields
	Size      int64     `json:"size"`
	SourceURL *string   `json:"source_url"` // Use *string for nullable fields
}

// DataStructure represents the overall structure of your JSON.
type MetaStructure struct {
	Aliases  []any    `json:"aliases"` // Can be []string or []interface{} if types vary
	Metadata Metadata `json:"dvc_metadata"`
	Path     string   `json:"path"`
}

func processMetaFiles(filePaths []string) ([]*MetaStructure, error) {
	var dataStructures []*MetaStructure
	for _, filePath := range filePaths {
		content, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file %q: %v\n", filePath, err)
			continue
		}

		var data MetaStructure
		err = json.Unmarshal(content, &data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error unmarshaling JSON from file %q: %v\n", filePath, err)
			continue
		}
		data.Path = filePath
		dataStructures = append(dataStructures, &data)
	}

	return dataStructures, nil
}

func RunMetaInit(dirPath string) error {
	metaFilePaths, err := findMetaFiles(dirPath)
	if err != nil {
		return fmt.Errorf("error walking directory %q: %v", dirPath, err)
	}
	if len(metaFilePaths) == 0 {
		fmt.Println("No .meta files found to process.")
		return nil
	}
	processedData, err := processMetaFiles(metaFilePaths)
	if err != nil {
		return fmt.Errorf("error processing meta files: %v", err)
	}

	outputDir := "./META"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}
	filename := filepath.Join(outputDir, "document_references.ndjson") // .ndjson is a common extension

	// Open the file in write mode. It will be created if it doesn't exist,
	// and truncated if it does.
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %v", filename, err)
	}
	defer file.Close() // Make sure the file is closed at the end of the function.

	marshaller, err := jsonformat.NewMarshaller(true, "", "  ", fver.R5)
	if err != nil {
		return fmt.Errorf("failed to create FHIR marshaller: %v", err)
	}

	for _, v := range processedData {
		docRef := templateDocRef(v)
		jsonBytes, err := marshaller.Marshal(docRef)
		if err != nil {
			log.Fatalf("Failed to marshal DocumentReference to JSON: %v", err)
		}

		if _, err := file.Write(jsonBytes); err != nil {
			log.Printf("Failed to write JSON for DocumentReference %s to file: %v", v.Metadata.ObjectID, err)
			continue
		}

		// Write a newline character after each JSON object.
		if _, err := file.WriteString("\n"); err != nil {
			log.Printf("Failed to write newline for DocumentReference %s to file: %v", v.Metadata.ObjectID, err)
			continue
		}

	}
	return nil
}
