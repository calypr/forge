package metadata

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/calypr/calypr-cli/g3client"
	"github.com/calypr/forge/client"
	"github.com/calypr/forge/utils/remoteutil"
	gitinventory "github.com/calypr/git-drs/inventory"
	fver "github.com/google/fhir/go/fhirversion"
	"github.com/google/fhir/go/jsonformat"
	dtpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/datatypes_go_proto"
	drpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/resources/document_reference_go_proto"
)

const fileSHA256System = "https://humantumoratlas.org/FILE_SHA256"

// ReconcileOptions defines one immutable Git snapshot and its authored metadata.
type ReconcileOptions struct {
	RepositoryRoot string
	GitRef         string
	FHIRDirectory  string
	ProfileName    string
	GitRemoteName  string
}

// ReconcileReport is emitted by normal ETL so operators can see exactly what was
// matched, generated, retained, or rejected.
type ReconcileReport struct {
	GitPointers            int
	AuthoredRows           int
	MatchedRows            int
	GeneratedRows          int
	AuthoredDRSIDsSynced   int
	MetadataOnlySHA256     []string
	AuthoredRowsWithoutSHA int
	MissingSyfonRecords    []ReconcileIssue
	AmbiguousSyfonRecords  []ReconcileIssue
}

// ReconcileIssue identifies a Git checksum that could not be mapped to exactly
// one Syfon record in the configured project scope.
type ReconcileIssue struct {
	SHA256       string
	Paths        []string
	SyfonRecords int
}

// CreateMeta preserves the historical Forge entrypoint while switching normal
// imports to Git-SHA-driven reconciliation of the current checkout.
func CreateMeta(outPath string, profileName string, gitRemoteName string) error {
	_, err := ReconcileGitPointers(context.Background(), ReconcileOptions{
		RepositoryRoot: ".",
		GitRef:         "HEAD",
		FHIRDirectory:  outPath,
		ProfileName:    profileName,
		GitRemoteName:  gitRemoteName,
	})
	return err
}

// ReconcileGitPointers performs the normal metadata import reconciliation. Git
// pointers drive the inventory; Syfon is consulted only for those SHA256 values.
// Existing DocumentReference rows are retained verbatim and only missing rows are
// appended.
func ReconcileGitPointers(ctx context.Context, options ReconcileOptions) (ReconcileReport, error) {
	var report ReconcileReport
	if strings.TrimSpace(options.RepositoryRoot) == "" {
		return report, fmt.Errorf("repository root is required")
	}
	if strings.TrimSpace(options.FHIRDirectory) == "" {
		return report, fmt.Errorf("FHIR directory is required")
	}

	pointerRecords, err := gitinventory.List(ctx, gitinventory.Options{
		RepositoryRoot:  options.RepositoryRoot,
		Ref:             options.GitRef,
		ExcludePrefixes: []string{META_DIR, "CONFIG"},
	})
	if err != nil {
		return report, err
	}
	report.GitPointers = len(pointerRecords)
	pointers := make(map[string][]gitinventory.Pointer)
	for _, pointer := range pointerRecords {
		pointers[pointer.SHA256] = append(pointers[pointer.SHA256], pointer)
	}

	sc, closer, err := client.NewGen3Client(options.ProfileName, g3client.WithClients(g3client.SyfonClient))
	if err != nil {
		return report, err
	}
	defer closer()

	repoRemote, err := remoteutil.LoadRemoteOrDefault(options.GitRemoteName)
	if err != nil {
		return report, err
	}

	marshaller, err := jsonformat.NewMarshaller(false, "", "", fver.R5)
	if err != nil {
		return report, err
	}
	unmarshaller, err := jsonformat.NewUnmarshallerWithoutValidation("America/Los_Angeles", fver.R5)
	if err != nil {
		return report, err
	}

	if err := os.MkdirAll(options.FHIRDirectory, 0o755); err != nil {
		return report, fmt.Errorf("create FHIR directory: %w", err)
	}
	researchStudyID, err := getResearchStudy(options.FHIRDirectory, repoRemote.ProjectID, sc.Credential().APIEndpoint, marshaller, unmarshaller)
	if err != nil {
		return report, err
	}

	docRefPath := filepath.Join(options.FHIRDirectory, DOCUMENT_RESOURCE+NDJSON_EXT)
	authored, authoredSHA256, noSHA, err := readAuthoredDocumentReferences(docRefPath, unmarshaller)
	if err != nil {
		return report, err
	}
	report.AuthoredRows = len(authored)
	report.AuthoredRowsWithoutSHA = noSHA

	hashes := make([]string, 0, len(pointers))
	for sha := range pointers {
		hashes = append(hashes, sha)
	}
	sort.Strings(hashes)
	objects, err := listProjectObjectsByHashes(ctx, sc, repoRemote.Organization, repoRemote.ProjectID, hashes)
	if err != nil {
		return report, err
	}
	objectsBySHA := make(map[string][]MetaObject, len(objects))
	for _, object := range objects {
		sha := strings.ToLower(checksumValue(object.Checksums, "sha-256", "sha256"))
		if sha != "" {
			objectsBySHA[sha] = append(objectsBySHA[sha], object)
		}
	}

	generated := make([][]byte, 0)
	for _, sha := range hashes {
		if _, exists := authoredSHA256[sha]; exists {
			report.MatchedRows++
			continue
		}
		candidates := objectsBySHA[sha]
		switch len(candidates) {
		case 0:
			report.MissingSyfonRecords = append(report.MissingSyfonRecords, reconcileIssue(sha, pointers[sha], 0))
			continue
		case 1:
			object := candidates[0]
			pointer := pointers[sha][0]
			object.Size = pointer.Size
			if strings.TrimSpace(object.Name) == "" {
				object.Name = filepath.Base(pointer.Path)
			}
			row := templateDocRef(&object, sc.Credential().APIEndpoint, repoRemote.ProjectID, researchStudyID)
			addSHA256Identifier(row.GetDocumentReference(), sha)
			encoded, err := marshaller.Marshal(row)
			if err != nil {
				return report, fmt.Errorf("serialize generated DocumentReference for %s: %w", sha, err)
			}
			generated = append(generated, encoded)
		default:
			report.AmbiguousSyfonRecords = append(report.AmbiguousSyfonRecords, reconcileIssue(sha, pointers[sha], len(candidates)))
		}
	}
	if len(report.AmbiguousSyfonRecords) > 0 {
		values := make([]string, 0, len(report.AmbiguousSyfonRecords))
		for _, issue := range report.AmbiguousSyfonRecords {
			values = append(values, fmt.Sprintf("%s (%d Syfon records)", issueDescription(issue), issue.SyfonRecords))
		}
		return report, fmt.Errorf("Git/Syfon reconciliation failed: %d Git SHA256 values are ambiguous (%s)", len(values), summarizeValues(values))
	}

	for sha := range authoredSHA256 {
		if _, exists := pointers[sha]; !exists {
			report.MetadataOnlySHA256 = append(report.MetadataOnlySHA256, sha)
		}
	}
	sort.Strings(report.MetadataOnlySHA256)
	if err := appendDocumentReferences(docRefPath, authored, generated); err != nil {
		return report, err
	}
	// Authored metadata often carries a path or a legacy identifier in its
	// first DocumentReference identifier. The Explorer's file-action column
	// consumes that primary identifier as the Syfon DRS object ID. Reuse the
	// SHA256-scoped Syfon result set above to synchronize only Git-matched rows;
	// do not fall back to an unbounded DID/path scan across the project.
	repaired, err := repairDocumentReferenceIdentifiersBySHA(
		objects,
		options.FHIRDirectory,
		sc.Credential().APIEndpoint,
		repoRemote.ProjectID,
	)
	if err != nil {
		return report, fmt.Errorf("synchronize authored DocumentReference DRS identifiers: %w", err)
	}
	report.AuthoredDRSIDsSynced = repaired
	report.GeneratedRows = len(generated)
	if repaired > 0 {
		log.Printf("Git/Syfon reconciliation: synchronized %d authored DocumentReference primary DRS identifiers by SHA256", repaired)
	}
	if len(report.MetadataOnlySHA256) > 0 {
		log.Printf("WARNING: retained %d authored DocumentReference SHA256 values not present in Git: %s", len(report.MetadataOnlySHA256), summarizeValues(report.MetadataOnlySHA256))
	}
	if report.AuthoredRowsWithoutSHA > 0 {
		log.Printf("WARNING: retained %d authored DocumentReference rows without a FILE_SHA256 identifier", report.AuthoredRowsWithoutSHA)
	}
	return report, nil
}

func reconcileIssue(sha string, pointers []gitinventory.Pointer, syfonRecords int) ReconcileIssue {
	paths := make([]string, 0, len(pointers))
	seen := make(map[string]struct{}, len(pointers))
	for _, pointer := range pointers {
		path := strings.TrimSpace(pointer.Path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return ReconcileIssue{SHA256: sha, Paths: paths, SyfonRecords: syfonRecords}
}

func issueDescription(issue ReconcileIssue) string {
	if len(issue.Paths) == 0 {
		return issue.SHA256
	}
	return fmt.Sprintf("%s (%s)", issue.SHA256, strings.Join(issue.Paths, ", "))
}

// DiscoverGitPointers reads pointer files from a checkout. META and CONFIG are
// intentionally excluded: they are ETL inputs, not data inventory.
func readAuthoredDocumentReferences(path string, unmarshaller *jsonformat.Unmarshaller) ([][]byte, map[string]struct{}, int, error) {
	rows := make([][]byte, 0)
	sha256s := make(map[string]struct{})
	withoutSHA := 0
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return rows, sha256s, withoutSHA, nil
	}
	if err != nil {
		return nil, nil, 0, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		rows = append(rows, line)
		if !json.Valid(line) {
			withoutSHA++
			continue
		}
		cr, err := parseDocumentReferenceLine(line, unmarshaller)
		if err != nil || cr.GetDocumentReference() == nil {
			withoutSHA++
			continue
		}
		sha := strings.ToLower(docRefSHA256(cr.GetDocumentReference()))
		if sha == "" {
			withoutSHA++
			continue
		}
		sha256s[sha] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, 0, err
	}
	return rows, sha256s, withoutSHA, nil
}

func appendDocumentReferences(path string, authored, generated [][]byte) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	for _, row := range append(authored, generated...) {
		if _, err := file.Write(row); err != nil {
			return err
		}
		if _, err := file.Write([]byte("\n")); err != nil {
			return err
		}
	}
	return nil
}

func addSHA256Identifier(docRef *drpb.DocumentReference, sha string) {
	docRef.Identifier = append(docRef.Identifier, &dtpb.Identifier{
		Use:    &dtpb.Identifier_UseCode{},
		System: &dtpb.Uri{Value: fileSHA256System},
		Value:  &dtpb.String{Value: sha},
	})
}

func pointerDescription(sha string, pointers []gitinventory.Pointer) string {
	paths := make([]string, 0, len(pointers))
	for _, pointer := range pointers {
		paths = append(paths, pointer.Path)
	}
	sort.Strings(paths)
	return fmt.Sprintf("%s (%s)", sha, summarizeValues(paths))
}

func summarizeValues(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	const max = 5
	if len(values) <= max {
		return strings.Join(values, ", ")
	}
	return strings.Join(values[:max], ", ") + fmt.Sprintf(" … and %d more", len(values)-max)
}
