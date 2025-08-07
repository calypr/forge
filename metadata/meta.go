package metadata

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/calypr/data-client/data-client/jwt"
	drsConfig "github.com/calypr/git-drs/config"
	fver "github.com/google/fhir/go/fhirversion"
	"github.com/google/fhir/go/jsonformat"
)

const (
	META_DIR      = "./META"
	NDJSON_EXT    = ".ndjson"
	FILE_META_EXT = ".meta"
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
		var data MetaStructure
		data.Path = filepath.Base(filePath)
		content, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file %q: %v\n", filePath, err)
			continue
		}

		err = json.Unmarshal(content, &data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error unmarshaling JSON from file %q: %v\n", filePath, err)
			continue
		}
		dataStructures = append(dataStructures, &data)
	}

	return dataStructures, nil
}

func RunMetaInit(dirPath, outPath string, rebuild bool) error {
	var fileFlags int
	cfg, err := drsConfig.LoadConfig()
	if err != nil {
		return err
	}
	// get the gen3Profile and endpoint
	profile := cfg.Servers.Gen3.Auth.Profile
	if profile == "" {
		return fmt.Errorf("No gen3 profile specified. Please provide a gen3Profile key in your .drsconfig")
	}
	var conf jwt.Configure
	cred, err := conf.ParseConfig(profile)
	if err != nil {
		return err
	}

	metaFilePaths, err := findMetaFiles(dirPath, rebuild)
	if err != nil {
		return fmt.Errorf("error walking directory %q: %v", dirPath, err)
	}
	if len(metaFilePaths) == 0 {
		fmt.Printf("No %s files found to process\n", FILE_META_EXT)
		return nil
	}
	processedData, err := processMetaFiles(metaFilePaths)
	if err != nil {
		return fmt.Errorf("error processing meta files: %v", err)
	}

	if err := os.MkdirAll(outPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}
	filename := filepath.Join(outPath, "DocumentReference"+NDJSON_EXT) // .ndjson is a common extension

	if rebuild {
		fileFlags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	} else {
		fileFlags = os.O_APPEND | os.O_CREATE | os.O_WRONLY
	}
	file, err := os.OpenFile(filename, fileFlags, 0644)
	if err != nil {
		return fmt.Errorf("failed to open/create file %s: %v", filename, err)
	}
	defer file.Close()

	marshaller, err := jsonformat.NewMarshaller(false, "", "", fver.R5)
	if err != nil {
		return fmt.Errorf("failed to create FHIR marshaller: %v", err)
	}

	for _, v := range processedData {
		docRef := templateDocRef(v, cred.APIEndpoint, cfg.Servers.Gen3.Auth.ProjectID)
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
