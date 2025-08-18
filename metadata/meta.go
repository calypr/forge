package metadata

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	idxClient "github.com/calypr/git-drs/client"
	fver "github.com/ohsu-comp-bio/fhir/go/fhirversion"
	"github.com/ohsu-comp-bio/fhir/go/jsonformat"
	code "github.com/ohsu-comp-bio/fhir/go/proto/google/fhir/proto/r5/core/codes_go_proto"
	dtpb "github.com/ohsu-comp-bio/fhir/go/proto/google/fhir/proto/r5/core/datatypes_go_proto"
	cprb "github.com/ohsu-comp-bio/fhir/go/proto/google/fhir/proto/r5/core/resources/bundle_and_contained_resource_go_proto"
	rspb "github.com/ohsu-comp-bio/fhir/go/proto/google/fhir/proto/r5/core/resources/research_study_go_proto"
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

// DataStructure represents the overall structure of your JSON.
type MetaStructure struct {
	Aliases  []any    `json:"aliases"` // Can be []string or []interface{} if types vary
	Metadata Metadata `json:"dvc_metadata"`
	Path     string   `json:"path"`
}

func RunMetaInit(outPath string) error {
	var rsID string
	cfg, err := idxClient.NewIndexDClient(&idxClient.NoOpLogger{})
	if err != nil {
		return err
	}

	idxCl, ok := cfg.(*idxClient.IndexDClient)
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

	// Now that we have a channel, we can pass it directly to the merging function
	if err := processDRSRecordsAndUpdateFHIR(recs, outPath, idxCl.Base.Host, idxCl.ProjectId, rsID); err != nil {
		return fmt.Errorf("failed to process DRS records: %v", err)
	}

	return nil
}

func getResearchStudy(fhirDirectory string, projectId string, endpoint string, marshaller *jsonformat.Marshaller, unmarshaller *jsonformat.Unmarshaller) (string, error) {
	rsPath := filepath.Join(fhirDirectory, "ResearchStudy"+NDJSON_EXT)
	if _, err := os.Stat(rsPath); err == nil {
		jsonBytes, err := os.ReadFile(rsPath)
		if err != nil {
			return "", fmt.Errorf("failed to read existing ResearchStudy file: %v", err)
		}
		cr, err := unmarshaller.UnmarshalR5(jsonBytes)
		if err != nil {
			return "", fmt.Errorf("failed to decode ResearchStudy from file: %v", err)
		}
		rsID := cr.GetResearchStudy().Id.Value
		fmt.Printf("Loaded existing ResearchStudy from %s with ID %s\n", rsPath, rsID)
		return rsID, nil
	}

	id := createIDFromStrings(endpoint, RESEARCH_STUDY, projectId, projectId)
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

	jsonBytes, err := marshaller.Marshal(containedResource)
	if err != nil {
		return "", fmt.Errorf("failed to marshal new ResearchStudy: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(rsPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create directory for ResearchStudy file: %v", err)
	}

	f, err := os.Create(rsPath)
	if err != nil {
		return "", fmt.Errorf("error creating ResearchStudy file %s: %v", rsPath, err)
	}
	defer f.Close()

	if _, err := f.Write(jsonBytes); err != nil {
		return "", fmt.Errorf("error writing ResearchStudy file %s: %v", rsPath, err)
	}
	if _, err := f.Write([]byte("\n")); err != nil {
		return "", fmt.Errorf("error writing newline to ResearchStudy file %s: %v", rsPath, err)
	}

	fmt.Printf("Created new ResearchStudy at %s with ID %s\n", rsPath, id)
	return id, nil
}

// processDRSRecordsAndUpdateFHIR processes DRS records and updates FHIR NDJSON files with UPSERT operation.
func processDRSRecordsAndUpdateFHIR(drsRecordsChan chan idxClient.ListRecordsResult, fhirDirectory string, endpoint string, project string, researchStudyID string) error {
	docRefFP := filepath.Join(fhirDirectory, "DocumentReference"+NDJSON_EXT)
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
				if !json.Valid(line) { // Check if line is valid JSON
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
		if err := scanner.Err(); err != nil { // Check for scanner errors
			return fmt.Errorf("scanner error in %s: %v", docRefFP, err)
		}
	}

	for drsRecord := range drsRecordsChan {
		if drsRecord.Error != nil {
			return fmt.Errorf("error from record channel: %v", drsRecord.Error)
		}
		containedResource := templateDocRef(drsRecord, endpoint, project, researchStudyID)
		fhirRecord := containedResource.GetDocumentReference()
		recordID := fhirRecord.GetId().GetValue()
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
			fmt.Printf("Merged and updated record: %s\n", recordID)
		} else {
			existingFHIRRecords[recordID] = containedResource
			fmt.Printf("Added new record: %s\n", recordID)
		}
	}

	outputFilePath := docRefFP
	f, err := os.Create(outputFilePath)
	if err != nil {
		log.Printf("Error writing to FHIR file %s: %v", outputFilePath, err)
		return err
	}
	defer f.Close()

	for recordID, record := range existingFHIRRecords {
		jsonBytes, err := marshaller.Marshal(record)
		if err != nil {
			log.Printf("Error serializing record with id %s: %v. Skipping.", recordID, err)
			continue
		}
		if _, err := f.Write(jsonBytes); err != nil {
			log.Printf("Error writing to file %s: %v", docRefFP, err)
			break
		}
		if _, err := f.Write([]byte("\n")); err != nil {
			log.Printf("Error writing newline to file %s: %v", docRefFP, err)
			break
		}
	}
	log.Println("Finished writing all records to files.")
	return nil
}
