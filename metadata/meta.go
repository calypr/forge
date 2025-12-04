package metadata

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	indexd_client "github.com/calypr/git-drs/client/indexd"
	"github.com/calypr/git-drs/config"
	"github.com/calypr/git-drs/drs"
	"github.com/calypr/git-drs/drslog"
	fver "github.com/google/fhir/go/fhirversion"
	"github.com/google/fhir/go/jsonformat"
	code "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/codes_go_proto"
	dtpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/datatypes_go_proto"
	cprb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/resources/bundle_and_contained_resource_go_proto"
	rspb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/resources/research_study_go_proto"
	"github.com/google/uuid"
)

const (
	META_DIR   = "./META"
	NDJSON_EXT = ".ndjson"
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

var DirectoryCache = make(map[string]*Directory)

// DataStructure represents the overall structure of your JSON.
type MetaStructure struct {
	Aliases  []any    `json:"aliases"` // Can be []string or []interface{} if types vary
	Metadata Metadata `json:"dvc_metadata"`
	Path     string   `json:"path"`
}

func RunMetaInit(outPath string, remote config.Remote) error {
	var rsID string
	var err error
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	logger, err := drslog.NewLogger("", true)
	if err != nil {
		return err
	}

	val, err := cfg.GetRemoteClient(remote, logger)
	if err != nil {
		return err
	}

	idxCl, ok := val.(*indexd_client.IndexDClient)
	if !ok {
		return fmt.Errorf("Config is not IndexDClient")
	}

	marshaller, err := jsonformat.NewMarshaller(false, "", "", fver.R5)
	if err != nil {
		return fmt.Errorf("failed to create FHIR marshaller: %v", err)
	}
	unmarshaller, err := jsonformat.NewUnmarshallerWithoutValidation("America/Los_Angeles", fver.R5)
	if err != nil {
		return fmt.Errorf("failed to create FHIR unmarshaller: %v", err)
	}

	rsID, err = getResearchStudy(META_DIR, idxCl.ProjectId, idxCl.Base.Host, marshaller, unmarshaller)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Fetch all records from the channel into a slice
	recs, err := idxCl.ListObjectsByProject(idxCl.ProjectId)
	if err != nil {
		return fmt.Errorf("error listing indexd records: %v", err)
	}

	collectRecs := []*drs.DRSObject{}
	for rec := range recs {
		if rec.Error != nil {
			return fmt.Errorf("error from record channel: %v", rec.Error)
		}
		collectRecs = append(collectRecs, rec.Object)
	}

	LFSRecords, err := findLFSRecords()
	if err != nil {
		return err
	}

	// Now that we have a channel, we can pass it directly to the merging function
	if err := processDRSRecordsAndUpdateFHIR(collectRecs, LFSRecords, outPath, idxCl.Base.Host, idxCl.ProjectId, rsID); err != nil {
		return fmt.Errorf("failed to process DRS records: %v", err)
	}

	return nil
}

// getOrCreateRootDirectory ensures the root directory ("/") exists in DirectoryCache
func getOrCreateRootDirectory(endpoint string) *Directory {
	cleanPath := "/"
	if dir, ok := DirectoryCache[cleanPath]; ok {
		return dir
	}
	dirUUID := uuid.NewSHA1(uuid.NewSHA1(uuid.NameSpaceDNS, []byte(endpoint)), []byte(cleanPath)).String()
	newDir := &Directory{
		Name:         "/",
		Id:           dirUUID,
		ResourceType: DIRECTORY_RESOURCE,
		Child:        []*dtpb.Reference{},
	}
	DirectoryCache[cleanPath] = newDir
	return newDir
}

// getResearchStudy returns the ResearchStudy id and ensures the NDJSON file contains a top-level rootDir field
func getResearchStudy(fhirDirectory string, projectId string, endpoint string, marshaller *jsonformat.Marshaller, unmarshaller *jsonformat.Unmarshaller) (string, error) {
	rsPath := filepath.Join(fhirDirectory, "ResearchStudy"+NDJSON_EXT)

	// Ensure directory exists for the file path
	if err := os.MkdirAll(filepath.Dir(rsPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create directory for ResearchStudy file: %v", err)
	}

	// Helper to inject rootDir into already-marshalled JSON bytes (which represent a single resource)
	injectRootDir := func(jsonBytes []byte, rootDirRef string) ([]byte, error) {
		// decode into a generic map so we can inject rootDir
		var m map[string]any
		if err := json.Unmarshal(jsonBytes, &m); err != nil {
			return nil, fmt.Errorf("failed to unmarshal marshaller output: %v", err)
		}
		// inject/overwrite rootDir
		m["rootDir"] = map[string]any{"reference": rootDirRef}
		// re-marshal
		out, err := json.Marshal(m)
		if err != nil {
			return nil, fmt.Errorf("failed to re-marshal ResearchStudy with rootDir: %v", err)
		}
		return out, nil
	}

	// If the file exists, read it, update rootDir field, and write back
	if _, err := os.Stat(rsPath); err == nil {
		jsonBytes, err := os.ReadFile(rsPath)
		if err != nil {
			return "", fmt.Errorf("failed to read existing ResearchStudy file: %v", err)
		}

		lines := bytes.Split(jsonBytes, []byte{'\n'})
		unmarshalBytes := jsonBytes
		// If there is more than 1 line in the file, use the first line research study
		if len(lines) > 0 && len(lines[0]) > 0 {
			unmarshalBytes = lines[0]
		}

		// Try to unmarshal existing bytes into FHIR resource to get the ID (use unmarshaller)
		cr, err := unmarshaller.UnmarshalR5(unmarshalBytes)
		if err != nil {
			// If the protobuf unmarshaller fails, attempt to decode the plain JSON to find "id"
			var tmp map[string]any
			if err2 := json.Unmarshal(unmarshalBytes, &tmp); err2 == nil {
				if idv, ok := tmp["id"].(string); ok && idv != "" {
					// we have an ID but couldn't unmarshal via fhir unmarshaller; still inject rootDir
					rootDir := getOrCreateRootDirectory(endpoint)
					newBytes, injErr := injectRootDir(unmarshalBytes, "Directory/"+rootDir.Id)
					if injErr != nil {
						return "", injErr
					}
					if err := os.WriteFile(rsPath, append(newBytes, '\n'), 0644); err != nil {
						return "", fmt.Errorf("error writing updated ResearchStudy file %s: %v", rsPath, err)
					}
					return idv, nil
				}
			}
			return "", fmt.Errorf("failed to decode ResearchStudy from file: %v", err)
		}

		rsID := cr.GetResearchStudy().Id.Value
		fmt.Printf("Loaded existing ResearchStudy from %s with ID %s\n", rsPath, rsID)

		// inject or update rootDir in the existing JSON and write back
		rootDir := getOrCreateRootDirectory(endpoint)
		newFirstLineBytes, err := injectRootDir(unmarshalBytes, "Directory/"+rootDir.Id)
		if err != nil {
			return "", fmt.Errorf("failed to inject rootDir into existing ResearchStudy: %v", err)
		}

		lines[0] = newFirstLineBytes
		contentToWrite := bytes.Join(lines, []byte{'\n'})

		if err := os.WriteFile(rsPath, contentToWrite, 0644); err != nil {
			return "", fmt.Errorf("error writing updated ResearchStudy file %s: %v", rsPath, err)
		}
		fmt.Printf("Updated ResearchStudy at %s with rootDir field\n", rsPath)
		return rsID, nil
	}

	// File does not exist: create a new ResearchStudy contained resource and inject rootDir
	id := createIDFromStrings(endpoint, RESEARCH_STUDY, projectId)

	// Ensure root directory exists
	rootDir := getOrCreateRootDirectory(endpoint)

	rs := &rspb.ResearchStudy{
		Id: &dtpb.Id{Value: id},
		Identifier: []*dtpb.Identifier{{
			Use:    &dtpb.Identifier_UseCode{Value: code.IdentifierUseCode_OFFICIAL},
			System: &dtpb.Uri{Value: endpoint + "/" + projectId},
			Value:  &dtpb.String{Value: projectId},
		}},
		Status:      &rspb.ResearchStudy_StatusCode{Value: code.PublicationStatusCode_ACTIVE},
		Description: &dtpb.Markdown{Value: fmt.Sprintf("Skeleton ResearchStudy for %s", projectId)},
	}

	containedResource := &cprb.ContainedResource{
		OneofResource: &cprb.ContainedResource_ResearchStudy{
			ResearchStudy: rs,
		},
	}

	// Marshal via the FHIR marshaller to get correct FHIR JSON
	jsonBytes, err := marshaller.Marshal(containedResource)
	if err != nil {
		return "", fmt.Errorf("failed to marshal new ResearchStudy: %v", err)
	}

	// inject the non-FHIR top-level rootDir field
	jsonWithRoot, err := injectRootDir(jsonBytes, "Directory/"+rootDir.Id)
	if err != nil {
		return "", fmt.Errorf("failed to inject rootDir into new ResearchStudy: %v", err)
	}

	// write NDJSON
	f, err := os.Create(rsPath)
	if err != nil {
		return "", fmt.Errorf("error creating ResearchStudy file %s: %v", rsPath, err)
	}
	defer f.Close()
	if _, err := f.Write(jsonWithRoot); err != nil {
		return "", fmt.Errorf("error writing ResearchStudy file %s: %v", rsPath, err)
	}
	if _, err := f.Write([]byte("\n")); err != nil {
		return "", fmt.Errorf("error writing newline to ResearchStudy file %s: %v", rsPath, err)
	}
	fmt.Printf("Created new ResearchStudy at %s with ID %s and rootDir field\n", rsPath, id)
	return id, nil
}

type LSFIles struct {
	Files []LFSRecord `json:"files"`
}

type LFSRecord struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	Checkout   bool   `json:"checkout"`
	Downloaded bool   `json:"downloaded"`
	OIDType    string `json:"oid_type"`
	OID        string `json:"oid"`
	Version    string `json:"version"`
}

// findLFSRecords runs ls-files and collects the results into struct
func findLFSRecords() ([]LFSRecord, error) {
	output, err := exec.Command("git-lfs", "ls-files", "--json").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("command execution failed: %w\nstderr: %s", err, exitErr.Stderr)
		}
		return nil, fmt.Errorf("failed to run git-lfs command: %w", err)
	}
	var records LSFIles
	if err := json.Unmarshal(output, &records); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON output: %w", err)
	}
	return records.Files, nil
}

// processDRSRecordsAndUpdateFHIR processes DRS records and updates FHIR NDJSON files with UPSERT operation.
func processDRSRecordsAndUpdateFHIR(drsRecords []*drs.DRSObject, LfsRecords []LFSRecord, fhirDirectory string, endpoint string, project string, researchStudyID string) error {
	docRefFP := filepath.Join(fhirDirectory, DOCUMENT_RESOURCE+NDJSON_EXT)
	dirRefFP := filepath.Join(fhirDirectory, DIRECTORY_RESOURCE+NDJSON_EXT) // New Directory file path
	existingFHIRRecords := make(map[string]*cprb.ContainedResource)

	marshaller, err := jsonformat.NewMarshaller(false, "", "", fver.R5)
	if err != nil {
		return fmt.Errorf("failed to create FHIR marshaller: %v", err)
	}

	unmarshaller, err := jsonformat.NewUnmarshallerWithoutValidation("America/Los_Angeles", fver.R5)
	if err != nil {
		return fmt.Errorf("failed to create FHIR unmarshaller: %v", err)
	}

	file, err := os.Open(docRefFP)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("DocumentReference file not found at %s. Creating a new one with new records.", docRefFP)
		} else {
			return fmt.Errorf("error opening FHIR file %s: %v", docRefFP, err)
		}
	} else {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) > 0 {
				if !json.Valid(line) {
					fmt.Printf("Invalid JSON in %s: %s. Skipping record.\n", docRefFP, string(line))
					continue
				}
				dr, err := unmarshaller.UnmarshalR5(line)
				if err != nil {
					fmt.Printf("Invalid FHIR record in %s: %v. Skipping record.\n", docRefFP, err)
					continue
				}
				docRef := dr.GetDocumentReference()
				if docRef != nil {
					existingFHIRRecords[docRef.GetId().Value] = dr
				}
			}
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("scanner error in %s: %v", docRefFP, err)
		}
	}

	count := 0
	for _, rec := range LfsRecords {
		foundMatch := false
		containedResource := &cprb.ContainedResource{}
		for _, drsRecord := range drsRecords {
			for _, sum := range drsRecord.Checksums {
				if sum.Type == drs.ChecksumTypeSHA256 && rec.OID == sum.Checksum {
					drsRecord.Name = rec.Name
					foundMatch = true
					containedResource = templateDocRef(drsRecord, endpoint, project, researchStudyID)
				}
			}
			if foundMatch == true {
				break
			}
		}
		if !foundMatch {
			continue
		}

		count++
		fhirRecord := containedResource.GetDocumentReference()
		recordID := fhirRecord.GetId().GetValue()

		BuildDirectoryTreeFromDocRef(endpoint, fhirRecord)
		if existing := existingFHIRRecords[recordID].GetDocumentReference(); existing != nil {
			existing.Status = fhirRecord.Status
			existing.DocStatus = fhirRecord.DocStatus
			existing.Date = fhirRecord.Date
			existing.Identifier = fhirRecord.Identifier
			if existing.Content != nil && fhirRecord.Content != nil && len(existing.Content) > 0 && len(fhirRecord.Content) > 0 {
				existingAttachment := existing.Content[0].GetAttachment()
				newAttachment := fhirRecord.Content[0].GetAttachment()
				existingAttachment.Creation = newAttachment.Creation
				existingAttachment.Size = newAttachment.Size
				existingAttachment.Title = newAttachment.Title
				existingAttachment.Extension = newAttachment.Extension
				existingAttachment.Url = newAttachment.Url
			} else {
				existing.Content = fhirRecord.Content
			}
			existing.Subject = fhirRecord.Subject
		} else {
			existingFHIRRecords[recordID] = containedResource
		}
	}

	log.Printf("Processed %d records", count)

	docRefFile, err := os.Create(docRefFP)
	if err != nil {
		log.Printf("Error writing to DocumentReference file %s: %v", docRefFP, err)
		return err
	}
	defer docRefFile.Close()

	for recordID, record := range existingFHIRRecords {
		jsonBytes, err := marshaller.Marshal(record)
		if err != nil {
			log.Printf("Error serializing record with id %s: %v. Skipping.", recordID, err)
			continue
		}
		if _, err := docRefFile.Write(jsonBytes); err != nil {
			log.Printf("Error writing to file %s: %v", docRefFP, err)
			break
		}
		if _, err := docRefFile.Write([]byte("\n")); err != nil {
			log.Printf("Error writing newline to file %s: %v", docRefFP, err)
			break
		}
	}
	log.Println("Finished writing all DocumentReference records.")

	dirRefFile, err := os.Create(dirRefFP)
	if err != nil {
		log.Printf("Error creating Directory file %s: %v", dirRefFP, err)
		return err
	}
	defer dirRefFile.Close()

	for recordID, record := range DirectoryCache {
		jsonBytes, err := record.MarshalJSON() // Use standard JSON Marshal since Directory is a custom struct
		if err != nil {
			log.Printf("Error serializing Directory record with id %s: %v. Skipping.", recordID, err)
			continue
		}
		if _, err := dirRefFile.Write(jsonBytes); err != nil {
			log.Printf("Error writing to file %s: %v", dirRefFP, err)
			break
		}
		if _, err := dirRefFile.Write([]byte("\n")); err != nil {
			log.Printf("Error writing newline to file %s: %v", dirRefFP, err)
			break
		}
	}
	log.Println("Finished writing all Directory records.")

	return nil
}
