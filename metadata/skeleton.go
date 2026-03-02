package metadata

import (
	"fmt"
	"strings"

	"github.com/calypr/data-client/drs"
	code "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/codes_go_proto"
	dtpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/datatypes_go_proto"
	cprb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/resources/bundle_and_contained_resource_go_proto"
	drpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/resources/document_reference_go_proto"
	"github.com/google/uuid"
)

const (
	FHIR_STRUCTURE_DEFINITION = "/fhir/StructureDefinition"
	FILE_PREFIX               = "file://"
	RESEARCH_STUDY            = "ResearchStudy"
	DOCUMENT_RESOURCE         = "DocumentReference"
	DIRECTORY_RESOURCE        = "Directory"
	DIR_ID_PREFIX             = DIRECTORY_RESOURCE + "/"
	SOURCE_EXTENSION_URL      = "/fhir/StructureDefinition/source"
	GITHUB_SOURCE             = "github"
	S3_SOURCE                 = "s3"
)

func CreateDocReferenceReference(resourceId string) *dtpb.Reference {
	return &dtpb.Reference{
		Reference: &dtpb.Reference_DocumentReferenceId{
			DocumentReferenceId: &dtpb.ReferenceId{
				Value: resourceId,
			},
		},
	}
}

// General Resource reference
func CreateResourceReference(resourceId string) *dtpb.Reference {
	return &dtpb.Reference{
		Reference: &dtpb.Reference_ResourceId{
			ResourceId: &dtpb.ReferenceId{
				Value: resourceId,
			},
		},
	}
}

func templateDocRef(obj *drs.DRSObject, endpoint string, project string, rSID string) *cprb.ContainedResource {
	id := uuid.NewSHA1(
		uuid.NewSHA1(uuid.NameSpaceDNS, []byte(endpoint)),
		fmt.Appendf(nil, "%s/%s", project, obj.Name),
	).String()

	var extensions []*dtpb.Extension
	if obj.Checksums.MD5 != "" {
		extensions = append(extensions, &dtpb.Extension{
			Url: &dtpb.Uri{Value: endpoint + FHIR_STRUCTURE_DEFINITION + "/checksum-md5"},
			Value: &dtpb.Extension_ValueX{
				Choice: &dtpb.Extension_ValueX_StringValue{StringValue: &dtpb.String{Value: obj.Checksums.MD5}},
			},
		})
	}
	if obj.Checksums.SHA256 != "" {
		extensions = append(extensions, &dtpb.Extension{
			Url: &dtpb.Uri{Value: "http://" + endpoint + FHIR_STRUCTURE_DEFINITION + "/checksum-sha256"},
			Value: &dtpb.Extension_ValueX{
				Choice: &dtpb.Extension_ValueX_StringValue{StringValue: &dtpb.String{Value: obj.Checksums.SHA256}},
			},
		})
	}

	extensions = append(extensions, &dtpb.Extension{
		Url: &dtpb.Uri{Value: endpoint + SOURCE_EXTENSION_URL},
		Value: &dtpb.Extension_ValueX{
			Choice: &dtpb.Extension_ValueX_StringValue{StringValue: &dtpb.String{Value: S3_SOURCE}},
		},
	})

	var url *dtpb.Url
	if len(obj.AccessMethods) > 0 {
		// TODO: Big assumption here assuming that there exists only one url per FHIR attachment
		url = &dtpb.Url{Value: obj.AccessMethods[0].AccessURL.URL}
	}

	dr := &drpb.DocumentReference{
		Id:        &dtpb.Id{Value: id},
		Status:    &drpb.DocumentReference_StatusCode{Value: code.DocumentReferenceStatusCode_CURRENT},
		DocStatus: &drpb.DocumentReference_DocStatusCode{Value: code.CompositionStatusCode_FINAL},
		Date:      parseFHIRInstantString(obj.CreatedTime),
		Identifier: []*dtpb.Identifier{
			{
				Use:    &dtpb.Identifier_UseCode{Value: code.IdentifierUseCode_OFFICIAL},
				System: &dtpb.Uri{Value: "http://" + endpoint + "/" + project},
				Value:  &dtpb.String{Value: obj.Id},
			},
		},
		Content: []*drpb.DocumentReference_Content{
			{
				Attachment: &dtpb.Attachment{
					Creation:  parseFHIRDateTimeString(obj.CreatedTime),
					Size:      &dtpb.Integer64{Value: obj.Size},
					Title:     &dtpb.String{Value: obj.Name},
					Extension: extensions,
					Url:       url,
				},
			},
		},
		Subject: &dtpb.Reference{
			Reference: &dtpb.Reference_ResearchStudyId{
				ResearchStudyId: &dtpb.ReferenceId{
					Value: rSID,
				},
			},
		},
	}

	return &cprb.ContainedResource{
		OneofResource: &cprb.ContainedResource_DocumentReference{
			DocumentReference: dr,
		},
	}
}

func templateGitHubDocRef(name string, size int64, endpoint string, project string, rSID string, githubURL string, commitHash string) *cprb.ContainedResource {
	id := uuid.NewSHA1(
		uuid.NewSHA1(uuid.NameSpaceDNS, []byte(endpoint)),
		fmt.Appendf(nil, "%s/%s", project, name),
	).String()

	var extensions []*dtpb.Extension
	extensions = append(extensions, &dtpb.Extension{
		Url: &dtpb.Uri{Value: endpoint + SOURCE_EXTENSION_URL},
		Value: &dtpb.Extension_ValueX{
			Choice: &dtpb.Extension_ValueX_StringValue{StringValue: &dtpb.String{Value: GITHUB_SOURCE}},
		},
	})

	// Construct repository link
	// We use the /blob/<commit>/<file> format to link to the file in the web UI.
	// This avoids exposing raw tokens in URLs and relies on the user's browser session for auth.
	cleanURL := strings.TrimSuffix(githubURL, ".git")
	fileURL := fmt.Sprintf("https://%s/blob/%s/%s", cleanURL, commitHash, name)

	dr := &drpb.DocumentReference{
		Id:        &dtpb.Id{Value: id},
		Status:    &drpb.DocumentReference_StatusCode{Value: code.DocumentReferenceStatusCode_CURRENT},
		DocStatus: &drpb.DocumentReference_DocStatusCode{Value: code.CompositionStatusCode_FINAL},
		Identifier: []*dtpb.Identifier{
			{
				Use:    &dtpb.Identifier_UseCode{Value: code.IdentifierUseCode_OFFICIAL},
				System: &dtpb.Uri{Value: "http://" + endpoint + "/" + project},
				Value:  &dtpb.String{Value: name},
			},
		},
		Content: []*drpb.DocumentReference_Content{
			{
				Attachment: &dtpb.Attachment{
					Size:      &dtpb.Integer64{Value: size},
					Title:     &dtpb.String{Value: name},
					Extension: extensions,
					Url:       &dtpb.Url{Value: fileURL},
				},
			},
		},
		Subject: &dtpb.Reference{
			Reference: &dtpb.Reference_ResearchStudyId{
				ResearchStudyId: &dtpb.ReferenceId{
					Value: rSID,
				},
			},
		},
	}

	return &cprb.ContainedResource{
		OneofResource: &cprb.ContainedResource_DocumentReference{
			DocumentReference: dr,
		},
	}
}

func createIDFromStrings(apiEndpoint string, resourceType string, projectID string) string {
	// Use uuid.Nil as the namespace and the apiEndpoint string to create a consistent namespace UUID.
	// This is necessary because `uuid.NewSHA1` requires a UUID namespace, not a string.
	endpointNamespace := uuid.NewSHA1(uuid.NameSpaceDNS, []byte(apiEndpoint))

	system := getSystem(apiEndpoint, resourceType, projectID)

	// Generate the final UUID using the endpoint-specific namespace.
	return uuid.NewSHA1(endpointNamespace, fmt.Appendf(nil, "%s/%s", projectID, system)).String()
}

func getSystem(apiEndpoint string, identifier string, projectID string) string {
	if strings.Contains(identifier, "#") {
		return strings.Split(identifier, "#")[0]
	}
	if strings.Contains(identifier, "|") {
		return strings.Split(identifier, "|")[0]
	}
	return fmt.Sprintf("%s/%s", apiEndpoint, projectID)
}
