package metadata

import (
	"fmt"
	"strings"

	"github.com/calypr/git-drs/drs"
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
		fmt.Appendf(nil, "%s/%s", project, obj.Name)).String()

	// Create the extensions for hashes
	var extensions []*dtpb.Extension
	for _, sum := range obj.Checksums {
		if sum.Type == drs.ChecksumTypeMD5 {
			extensions = append(extensions, &dtpb.Extension{
				Url: &dtpb.Uri{Value: endpoint + FHIR_STRUCTURE_DEFINITION + "/checksum-md5"},
				Value: &dtpb.Extension_ValueX{
					Choice: &dtpb.Extension_ValueX_StringValue{StringValue: &dtpb.String{Value: sum.Checksum}},
				},
			})
		}
		if sum.Type == drs.ChecksumTypeSHA256 {
			extensions = append(extensions, &dtpb.Extension{
				Url: &dtpb.Uri{Value: "http://" + endpoint + FHIR_STRUCTURE_DEFINITION + "/checksum-sha256"},
				Value: &dtpb.Extension_ValueX{
					Choice: &dtpb.Extension_ValueX_StringValue{StringValue: &dtpb.String{Value: sum.Checksum}},
				},
			})
		}
	}

	// Determine the URL for the attachment
	var url *dtpb.Url
	if len(obj.AccessMethods) > 0 {
		// TODO: Big assumption here assuming that there exists only one url per FHIR attachment
		url = &dtpb.Url{Value: obj.AccessMethods[0].AccessURL.URL}
	}

	// Create the DocumentReference
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
