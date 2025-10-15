package metadata

import (
	"time"

	dtpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/datatypes_go_proto" // This will now house all primitives and complex datatypes
)

func parseFHIRInstantString(dateStr string) *dtpb.Instant {
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return nil
	}
	return &dtpb.Instant{ValueUs: t.UnixMicro()}
}

func parseFHIRDateTimeString(dateStr string) *dtpb.DateTime {
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return nil
	}
	return &dtpb.DateTime{ValueUs: t.UnixMicro()}
}
