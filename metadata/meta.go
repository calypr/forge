package metadata

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/calypr/data-client/drs"
	"github.com/calypr/forge/utils/gitutil"
	"github.com/calypr/git-drs/config"
	"github.com/calypr/git-drs/drslog"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	fver "github.com/google/fhir/go/fhirversion"
	"github.com/google/fhir/go/jsonformat"
	code "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/codes_go_proto"
	dtpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/datatypes_go_proto"
	cprb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/resources/bundle_and_contained_resource_go_proto"
	drpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/resources/document_reference_go_proto"
	rspb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/resources/research_study_go_proto"
	"github.com/google/uuid"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
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

func CreateMeta(outPath string, remote config.Remote) error {
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

	sc, err := cfg.GetRemoteClient(remote, logger)
	if err != nil {
		return err
	}

	// Load existing directories and DocumentReferences if they exist
	dirRefFP := filepath.Join(outPath, DIRECTORY_RESOURCE+NDJSON_EXT)
	if err := LoadDirectories(dirRefFP, sc.GetProjectId()); err != nil {
		logger.Debug(fmt.Sprintf("Warning: could not load existing directories: %v", err))
	}

	marshaller, err := jsonformat.NewMarshaller(false, "", "", fver.R5)
	if err != nil {
		return fmt.Errorf("failed to create FHIR marshaller: %v", err)
	}
	unmarshaller, err := jsonformat.NewUnmarshallerWithoutValidation("America/Los_Angeles", fver.R5)
	if err != nil {
		return fmt.Errorf("failed to create FHIR unmarshaller: %v", err)
	}

	rsID, err = getResearchStudy(META_DIR, sc.GetProjectId(), sc.GetGen3Interface().GetCredential().APIEndpoint, marshaller, unmarshaller)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// 1. Get local files first (LFS and Git)
	LFSRecords, err := findLFSRecords()
	if err != nil {
		return err
	}

	repo, err := gitutil.OpenRepository(".")
	if err != nil {
		return fmt.Errorf("failed to open repository: %v", err)
	}
	gitRecords, err := findGitFiles(repo)
	if err != nil {
		return err
	}

	// 2. Fetch DRS records from Indexd using the fast Project list API
	// but strictly filter to only those that match local LFS hashes.
	uniqueHashes := make(map[string]bool)
	for _, r := range LFSRecords {
		uniqueHashes[r.OID] = true
	}

	collectRecs := []*drs.DRSObject{}
	recs, err := sc.ListObjectsByProject(context.Background(), sc.GetProjectId())
	if err != nil {
		return fmt.Errorf("error listing indexd records: %v", err)
	}

	p := mpb.New(mpb.WithWidth(64))
	bar := p.AddBar(0, // total unknown
		mpb.PrependDecorators(
			decor.Name("Fetching Indexd records: "),
			decor.Elapsed(decor.ET_STYLE_GO),
		),
		mpb.AppendDecorators(
			decor.CountersNoUnit("%d processed"),
		),
	)

	for res := range recs {
		bar.Increment()
		if res.Error != nil {
			logger.Debug(fmt.Sprintf("Note: result channel error: %v", res.Error))
			continue
		}
		if res.Object != nil {
			// Pull only the indexd records that are also in the git-lfs structure
			if uniqueHashes[res.Object.Checksums.SHA256] {
				collectRecs = append(collectRecs, res.Object)
			}
		}
	}
	bar.SetTotal(bar.Current(), true) // mark as done
	p.Wait()

	var githubURL, commitHash string
	hash, err := gitutil.GetLastLocalCommit(repo)
	if err == nil {
		commitHash = hash.String()
	}
	repoRemote, err := repo.Remote(string(remote))
	if err != nil {
		return fmt.Errorf("failed to get remote: %v", err)
	}
	urls := repoRemote.Config().URLs
	if len(urls) > 0 {
		githubURL, err = gitutil.TrimGitURLPrefix(urls[0])
		if err != nil {
			return fmt.Errorf("failed to trim git URL prefix: %v", err)
		}
	}

	// Now that we have the matched records, process and update FHIR files
	if err := processDRSRecordsAndUpdateFHIR(collectRecs, LFSRecords, gitRecords, outPath, sc.GetGen3Interface().GetCredential().APIEndpoint, sc.GetProjectId(), rsID, githubURL, commitHash); err != nil {
		return fmt.Errorf("failed to process DRS records: %v", err)
	}

	return nil
}

// getOrCreateRootDirectory ensures the root directory (".") exists in DirectoryCache
func getOrCreateRootDirectory(endpoint string, project string) *Directory {
	// Root is stored as "." in cache keys to match filepath.Clean behavior
	cacheKey := project + ":."
	if dir, ok := DirectoryCache[cacheKey]; ok {
		return dir
	}

	// Canonical clean path to root is "." for internal logic
	dirUUID := uuid.NewSHA1(uuid.NewSHA1(uuid.NameSpaceDNS, []byte(endpoint)), []byte(project+".")).String()
	newDir := &Directory{
		Name:         "/", // Display as "/"
		Id:           dirUUID,
		Path:         ".",
		ResourceType: DIRECTORY_RESOURCE,
		Child:        []*dtpb.Reference{},
	}
	DirectoryCache[cacheKey] = newDir
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
					rootDir := getOrCreateRootDirectory(endpoint, projectId)
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
		rootDir := getOrCreateRootDirectory(endpoint, projectId)
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
	rootDir := getOrCreateRootDirectory(endpoint, projectId)

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

type LSFiles struct {
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
	var records LSFiles
	if err := json.Unmarshal(output, &records); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON output: %w", err)
	}
	return records.Files, nil
}

// findGitFiles runs git ls-tree and collects the results into struct
func findGitFiles(repo *git.Repository) ([]LFSRecord, error) {
	var records []LFSRecord

	// Get the HEAD reference
	ref, err := repo.Head()
	if err != nil {
		return nil, err
	}

	// Get the commit object
	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}

	// Get the tree from the commit
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	// Walk() is the direct replacement for 'ls-tree -r'
	err = tree.Files().ForEach(func(f *object.File) error {
		records = append(records, LFSRecord{
			Name: f.Name,
			Size: f.Size,
		})
		return nil
	})

	return records, err
}

// processDRSRecordsAndUpdateFHIR processes DRS records and updates FHIR NDJSON files with UPSERT operation.
func processDRSRecordsAndUpdateFHIR(drsRecords []*drs.DRSObject, LfsRecords []LFSRecord, gitRecords []LFSRecord, fhirDirectory string, endpoint string, project string, researchStudyID string, githubURL string, commitHash string) error {
	docRefFP := filepath.Join(fhirDirectory, DOCUMENT_RESOURCE+NDJSON_EXT)
	existingFHIRRecords := make(map[string]*cprb.ContainedResource)

	// Maps for matching
	existingByPath := make(map[string]*cprb.ContainedResource)
	existingBySHA256 := make(map[string]*cprb.ContainedResource)

	processedPaths := make(map[string]bool)
	matchedDrsOids := make(map[string]bool)

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
			if len(line) > 0 {
				if !json.Valid(line) {
					fmt.Printf("Invalid JSON in %s: %s. Skipping record.\n", docRefFP, string(line))
					continue
				}
				// Robust unmarshal: Handle both wrapped (Gen3 style) and unwrapped (Raw FHIR) formats
				var dr *cprb.ContainedResource
				// Attempt to unmarshal line. UnmarshalR5 returns specifically what's in 'resourceType'
				msg, err := unmarshaller.UnmarshalR5(line)
				if err != nil || msg == nil {
					// Fallback: Check for Gen3-wrapped record: {"documentReference": { ... }}
					var m map[string]json.RawMessage
					if err2 := json.Unmarshal(line, &m); err2 == nil {
						if inner, ok := m["documentReference"]; ok {
							msg, err = unmarshaller.UnmarshalR5(inner)
						}
					}
				}

				if err != nil {
					snippet := string(line)
					if len(snippet) > 80 {
						snippet = snippet[:80] + "..."
					}
					fmt.Printf("Warning: Failed to unmarshal record in %s: %v. Skipping line: %s\n", docRefFP, err, snippet)
					continue
				}

				// Safely normalize to *cprb.ContainedResource using an interface switch
				switch m := (interface{})(msg).(type) {
				case *cprb.ContainedResource:
					dr = m
				case *drpb.DocumentReference:
					// Wrap raw DocumentReference into ContainedResource for consistent processing
					dr = &cprb.ContainedResource{
						OneofResource: &cprb.ContainedResource_DocumentReference{
							DocumentReference: m,
						},
					}
				default:
					fmt.Printf("Warning: Skipping record in %s of unsupported type %T\n", docRefFP, msg)
					continue
				}
				docRef := dr.GetDocumentReference()
				if docRef != nil {
					id := docRef.GetId().Value
					existingFHIRRecords[id] = dr

					// Populate matching maps
					if len(docRef.Content) > 0 && docRef.Content[0].GetAttachment() != nil {
						path := logicalDocRefPath(docRef)
						if path != "" {
							existingByPath[path] = dr
						}
						title := docRef.Content[0].GetAttachment().GetTitle().GetValue()
						if title != "" {
							existingByPath[title] = dr
						}

						for _, ext := range docRef.Content[0].GetAttachment().GetExtension() {
							if strings.HasSuffix(ext.GetUrl().GetValue(), "/checksum-sha256") {
								if sha, ok := ext.GetValue().GetChoice().(*dtpb.Extension_ValueX_StringValue); ok {
									existingBySHA256[sha.StringValue.Value] = dr
								}
							}
						}

						// Also match by file_sha256 in category codings
						for _, cat := range docRef.Category {
							for _, coding := range cat.Coding {
								if coding.GetCode().GetValue() == "file_sha256" || coding.GetSystem().GetValue() == "https://humantumoratlas.org/file_sha256" {
									sha := coding.GetDisplay().GetValue()
									if sha == "" {
										sha = cat.GetText().GetValue()
									}
									if sha != "" {
										existingBySHA256[sha] = dr
									}
								}
							}
						}
					}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("scanner error in %s: %v", docRefFP, err)
		}
	}

	// Helper to merge records
	mergeDocRef := func(existing, new *drpb.DocumentReference) {
		existing.Status = new.Status
		existing.DocStatus = new.DocStatus
		existing.Date = new.Date

		// Merge identifiers
		existingIds := make(map[string]bool)
		for _, id := range new.Identifier {
			key := fmt.Sprintf("%s|%s", id.GetSystem().GetValue(), id.GetValue().GetValue())
			existingIds[key] = true
		}
		for _, id := range existing.Identifier {
			key := fmt.Sprintf("%s|%s", id.GetSystem().GetValue(), id.GetValue().GetValue())
			if !existingIds[key] {
				new.Identifier = append(new.Identifier, id)
			}
		}
		existing.Identifier = new.Identifier

		if existing.Content != nil && new.Content != nil && len(existing.Content) > 0 && len(new.Content) > 0 {
			existingAttachment := existing.Content[0].GetAttachment()
			newAttachment := new.Content[0].GetAttachment()
			existingAttachment.Creation = newAttachment.Creation
			existingAttachment.Size = newAttachment.Size
			existingAttachment.Title = newAttachment.Title

			// Merge extensions
			extMap := make(map[string]*dtpb.Extension)
			for _, ext := range existingAttachment.Extension {
				extMap[ext.GetUrl().GetValue()] = ext
			}
			for _, ext := range newAttachment.Extension {
				extMap[ext.GetUrl().GetValue()] = ext
			}
			var mergedExt []*dtpb.Extension
			for _, ext := range extMap {
				mergedExt = append(mergedExt, ext)
			}
			existingAttachment.Extension = mergedExt
			existingAttachment.Url = newAttachment.Url
		} else if new.Content != nil {
			existing.Content = new.Content
		}

		// Merge categories to preserve rich metadata (Assay, Level, file_sha256, etc.)
		existingCatKeys := make(map[string]bool)
		for _, cat := range existing.Category {
			for _, coding := range cat.Coding {
				key := fmt.Sprintf("%s|%s", coding.GetSystem().GetValue(), coding.GetCode().GetValue())
				existingCatKeys[key] = true
			}
		}
		for _, cat := range new.Category {
			addCat := true
			for _, coding := range cat.Coding {
				key := fmt.Sprintf("%s|%s", coding.GetSystem().GetValue(), coding.GetCode().GetValue())
				if existingCatKeys[key] {
					addCat = false
					break
				}
			}
			if addCat {
				existing.Category = append(existing.Category, cat)
			}
		}

		existing.Subject = new.Subject
	}

	finalDocRefIDs := make(map[string]bool)

	count := 0
	for _, rec := range LfsRecords {
		foundMatch := false
		containedResource := &cprb.ContainedResource{}
		for _, drsRecord := range drsRecords {
			if drsRecord.Checksums.SHA256 == rec.OID {
				drsRecord.Name = rec.Name
				foundMatch = true
				containedResource = templateDocRef(drsRecord, endpoint, project, researchStudyID)
				processedPaths[rec.Name] = true
				matchedDrsOids[drsRecord.Checksums.SHA256] = true
			}

			if foundMatch {
				break
			}
		}
		if !foundMatch {
			continue
		}

		count++
		fhirRecord := containedResource.GetDocumentReference()
		recordID := fhirRecord.GetId().GetValue()

		if existingCr, ok := existingByPath[rec.Name]; ok {
			mergeDocRef(existingCr.GetDocumentReference(), fhirRecord)
			finalDocRefIDs[existingCr.GetDocumentReference().GetId().Value] = true
		} else if existingCr, ok := existingBySHA256[rec.OID]; ok {
			// Content matched but path changed (rename)
			mergeDocRef(existingCr.GetDocumentReference(), fhirRecord)
			// Update the path in the attachment
			existingCr.GetDocumentReference().Content[0].GetAttachment().Title = &dtpb.String{Value: rec.Name}
			finalDocRefIDs[existingCr.GetDocumentReference().GetId().Value] = true
		} else {
			if existingCr, ok := existingFHIRRecords[recordID]; ok {
				mergeDocRef(existingCr.GetDocumentReference(), fhirRecord)
			} else {
				existingFHIRRecords[recordID] = containedResource
			}
			finalDocRefIDs[recordID] = true
		}
	}

	for _, rec := range gitRecords {
		if processedPaths[rec.Name] {
			continue // Already handled by LFS/DRS matching
		}
		// Also check if it's an LFS file (even if no DRS match)
		isLFS := false
		for _, lfs := range LfsRecords {
			if lfs.Name == rec.Name {
				isLFS = true
				break
			}
		}
		if isLFS {
			continue // Skip LFS files that didn't match DRS records
		}

		if githubURL != "" && commitHash != "" {
			containedResource := templateGitHubDocRef(rec.Name, rec.Size, endpoint, project, researchStudyID, githubURL, commitHash)
			fhirRecord := containedResource.GetDocumentReference()
			recordID := fhirRecord.GetId().GetValue()

			if existingCr, ok := existingByPath[rec.Name]; ok {
				mergeDocRef(existingCr.GetDocumentReference(), fhirRecord)
				finalDocRefIDs[existingCr.GetDocumentReference().GetId().Value] = true
			} else {
				if existingCr, ok := existingFHIRRecords[recordID]; ok {
					mergeDocRef(existingCr.GetDocumentReference(), fhirRecord)
				} else {
					existingFHIRRecords[recordID] = containedResource
				}
				finalDocRefIDs[recordID] = true
			}
			count++
		}
	}

	if count == 0 {
		log.Printf("WARNING: Processed 0 records. This means no matches were found between DRS objects on remote and local LFS files for project '%s'.", project)
		log.Printf("Verify that project ID '%s' is correct and that files are indexed on the remote DRS server '%s'.", project, endpoint)
	} else {
		log.Printf("Processed %d records", count)
	}

	docRefFile, err := os.Create(docRefFP)
	if err != nil {
		log.Printf("Error writing to DocumentReference file %s: %v", docRefFP, err)
		return err
	}
	defer docRefFile.Close()

	// Reset DirectoryCache to rebuild the tree cleanly from the current ground-truth DocumentReferences.
	// This ensures ghost directories from previous runs or bucket implementation details (like S3/GitHub web paths)
	// are completely purged if they are no longer reachable from any actual logical record.
	DirectoryCache = make(map[string]*Directory)

	for recordID, record := range existingFHIRRecords {

		// Rebuild directory tree for each final record
		BuildDirectoryTreeFromDocRef(endpoint, project, record.GetDocumentReference())

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

	// Directory children are already refreshed by BuildDirectoryTreeFromDocRef

	log.Println("Finished writing all DocumentReference records.")

	dirRefFP := filepath.Join(fhirDirectory, DIRECTORY_RESOURCE+NDJSON_EXT)
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
