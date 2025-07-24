package template

import (
	code "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/codes_go_proto"
	dtpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/datatypes_go_proto"
	cprb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/resources/bundle_and_contained_resource_go_proto"
	drpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/resources/document_reference_go_proto"
)

func templateDocRef(metaStruc *MetaStructure) *cprb.ContainedResource {
	obj := metaStruc.Metadata
	dr := &drpb.DocumentReference{
		Id: &dtpb.Id{Value: obj.ObjectID},
		Status: &drpb.DocumentReference_StatusCode{
			Value: code.DocumentReferenceStatusCode_CURRENT,
		},
		Identifier: []*dtpb.Identifier{
			{
				Use:    &dtpb.Identifier_UseCode{Value: code.IdentifierUseCode_OFFICIAL},
				System: &dtpb.Uri{Value: "sample-project"},
				Value:  &dtpb.String{Value: obj.ObjectID},
			},
		},
		DocStatus: &drpb.DocumentReference_DocStatusCode{Value: code.CompositionStatusCode_FINAL},
		Date:      parseFHIRInstantString(obj.Modified),
		Content: []*drpb.DocumentReference_Content{
			{
				Attachment: &dtpb.Attachment{
					ContentType: &dtpb.Attachment_ContentTypeCode{Value: obj.MIME},
					Url:         &dtpb.Url{Value: obj.Realpath},
					Title:       &dtpb.String{Value: metaStruc.Path},
					Size:        &dtpb.Integer64{Value: obj.Size},
					Creation:    parseFHIRDateTimeString(obj.Modified),
				},
			},
		},
	}

	if obj.MD5 != "" || obj.SourceURL != nil {
		dr.Extension = []*dtpb.Extension{}
		if obj.MD5 != "" {
			dr.Extension = append(dr.Extension, &dtpb.Extension{
				Url: &dtpb.Uri{Value: ""},
				Value: &dtpb.Extension_ValueX{
					Choice: &dtpb.Extension_ValueX_StringValue{StringValue: &dtpb.String{Value: ""}},
				},
			})
		} else if obj.SourceURL != nil {
			dr.Extension = append(dr.Extension, &dtpb.Extension{
				Url: &dtpb.Uri{Value: ""},
				Value: &dtpb.Extension_ValueX{
					Choice: &dtpb.Extension_ValueX_Url{Url: &dtpb.Url{Value: ""}},
				},
			})
		}
	}

	return &cprb.ContainedResource{
		OneofResource: &cprb.ContainedResource_DocumentReference{
			DocumentReference: dr,
		},
	}
}
