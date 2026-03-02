package metadata

import (
	"testing"
	"time"
)

func TestParseFHIRInstantString(t *testing.T) {
	tests := []struct {
		name     string
		dateStr  string
		expected int64 // microseconds
		isNil    bool
	}{
		{
			name:     "Valid RFC3339",
			dateStr:  "2023-10-27T10:00:00Z",
			expected: time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC).UnixMicro(),
			isNil:    false,
		},
		{
			name:     "Invalid string",
			dateStr:  "not-a-date",
			expected: 0,
			isNil:    true,
		},
		{
			name:     "Empty string",
			dateStr:  "",
			expected: 0,
			isNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFHIRInstantString(tt.dateStr)
			if tt.isNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
			} else {
				if got == nil {
					t.Errorf("expected non-nil")
				} else if got.ValueUs != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, got.ValueUs)
				}
			}
		})
	}
}

func TestParseFHIRDateTimeString(t *testing.T) {
	// Similar logic to parseFHIRInstantString, so we can be brief
	t.Run("Valid", func(t *testing.T) {
		dateStr := "2023-10-27T10:00:00Z"
		got := parseFHIRDateTimeString(dateStr)
		if got == nil {
			t.Fatal("expected non-nil")
		}
		expected := time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC).UnixMicro()
		if got.ValueUs != expected {
			t.Errorf("expected %v, got %v", expected, got.ValueUs)
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		if got := parseFHIRDateTimeString("invalid"); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
}
