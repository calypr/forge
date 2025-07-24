package template

import (
	"time"

	dtpb "github.com/google/fhir/go/proto/google/fhir/proto/r5/core/datatypes_go_proto" // This will now house all primitives and complex datatypes
)

func parseFHIRInstantString(t time.Time) *dtpb.Instant {
	timezoneName := t.Location().String()
	if timezoneName == "" {
		timezoneName = "UTC"
	}

	return &dtpb.Instant{
		ValueUs:   t.UnixMicro(),
		Timezone:  timezoneName,
		Precision: dtpb.Instant_MILLISECOND,
	}
}

func parseFHIRDateTimeString(t time.Time) *dtpb.DateTime {
	timezoneName := t.Location().String()
	if timezoneName == "" {
		timezoneName = "UTC"
	}

	return &dtpb.DateTime{
		ValueUs:   t.UnixMicro(),
		Timezone:  timezoneName,
		Precision: dtpb.DateTime_MILLISECOND,
	}
}
