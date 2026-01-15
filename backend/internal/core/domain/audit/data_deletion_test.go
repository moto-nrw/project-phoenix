package audit

import (
	"testing"
	"time"
)

func TestDataDeletion_Validate(t *testing.T) {
	tests := []struct {
		name    string
		dd      *DataDeletion
		wantErr bool
	}{
		{
			name: "valid data deletion",
			dd: &DataDeletion{
				StudentID:      1,
				DeletionType:   DeletionTypeVisitRetention,
				RecordsDeleted: 10,
				DeletedBy:      "system",
			},
			wantErr: false,
		},
		{
			name: "valid with manual deletion type",
			dd: &DataDeletion{
				StudentID:      1,
				DeletionType:   DeletionTypeManual,
				RecordsDeleted: 5,
				DeletedBy:      "admin@example.com",
				DeletionReason: "User requested deletion",
			},
			wantErr: false,
		},
		{
			name: "valid with GDPR request",
			dd: &DataDeletion{
				StudentID:      1,
				DeletionType:   DeletionTypeGDPRRequest,
				RecordsDeleted: 100,
				DeletedBy:      "admin@example.com",
				DeletionReason: "GDPR Article 17 request",
			},
			wantErr: false,
		},
		{
			name: "valid with zero records deleted",
			dd: &DataDeletion{
				StudentID:      1,
				DeletionType:   DeletionTypeVisitRetention,
				RecordsDeleted: 0,
				DeletedBy:      "system",
			},
			wantErr: false,
		},
		{
			name: "zero student ID",
			dd: &DataDeletion{
				StudentID:      0,
				DeletionType:   DeletionTypeManual,
				RecordsDeleted: 10,
				DeletedBy:      "system",
			},
			wantErr: true,
		},
		{
			name: "negative student ID",
			dd: &DataDeletion{
				StudentID:      -1,
				DeletionType:   DeletionTypeManual,
				RecordsDeleted: 10,
				DeletedBy:      "system",
			},
			wantErr: true,
		},
		{
			name: "empty deletion type",
			dd: &DataDeletion{
				StudentID:      1,
				DeletionType:   "",
				RecordsDeleted: 10,
				DeletedBy:      "system",
			},
			wantErr: true,
		},
		{
			name: "invalid deletion type",
			dd: &DataDeletion{
				StudentID:      1,
				DeletionType:   "unknown_type",
				RecordsDeleted: 10,
				DeletedBy:      "system",
			},
			wantErr: true,
		},
		{
			name: "negative records deleted",
			dd: &DataDeletion{
				StudentID:      1,
				DeletionType:   DeletionTypeManual,
				RecordsDeleted: -1,
				DeletedBy:      "system",
			},
			wantErr: true,
		},
		{
			name: "empty deleted by",
			dd: &DataDeletion{
				StudentID:      1,
				DeletionType:   DeletionTypeManual,
				RecordsDeleted: 10,
				DeletedBy:      "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dd.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("DataDeletion.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDataDeletion_Validate_SetsDefaultDeletedAt(t *testing.T) {
	dd := &DataDeletion{
		StudentID:      1,
		DeletionType:   DeletionTypeManual,
		RecordsDeleted: 5,
		DeletedBy:      "system",
		DeletedAt:      time.Time{}, // Zero time
	}

	before := time.Now()
	err := dd.Validate()
	after := time.Now()

	if err != nil {
		t.Fatalf("DataDeletion.Validate() unexpected error = %v", err)
	}

	if dd.DeletedAt.IsZero() {
		t.Error("DataDeletion.Validate() should set DeletedAt when zero")
	}

	if dd.DeletedAt.Before(before) || dd.DeletedAt.After(after) {
		t.Errorf("DataDeletion.DeletedAt = %v, want between %v and %v", dd.DeletedAt, before, after)
	}
}

func TestDataDeletion_Metadata(t *testing.T) {
	t.Run("GetMetadata initializes nil map", func(t *testing.T) {
		dd := &DataDeletion{
			Metadata: nil,
		}

		got := dd.GetMetadata()
		if got == nil {
			t.Error("DataDeletion.GetMetadata() should not return nil")
		}

		// Verify it's the same reference
		if dd.Metadata == nil {
			t.Error("DataDeletion.GetMetadata() should initialize Metadata field")
		}
	})

	t.Run("GetMetadata returns existing map", func(t *testing.T) {
		dd := &DataDeletion{
			Metadata: map[string]interface{}{"key": "value"},
		}

		got := dd.GetMetadata()
		if got["key"] != "value" {
			t.Errorf("DataDeletion.GetMetadata() = %v, want map with key=value", got)
		}
	})

	t.Run("SetMetadata initializes nil map", func(t *testing.T) {
		dd := &DataDeletion{
			Metadata: nil,
		}

		dd.SetMetadata("test_key", "test_value")

		if dd.Metadata == nil {
			t.Error("DataDeletion.SetMetadata() should initialize Metadata")
		}

		if dd.Metadata["test_key"] != "test_value" {
			t.Errorf("DataDeletion.Metadata[test_key] = %v, want test_value", dd.Metadata["test_key"])
		}
	})

	t.Run("SetMetadata adds to existing map", func(t *testing.T) {
		dd := &DataDeletion{
			Metadata: map[string]interface{}{"existing": "value"},
		}

		dd.SetMetadata("new_key", 42)

		if dd.Metadata["existing"] != "value" {
			t.Error("SetMetadata should preserve existing values")
		}
		if dd.Metadata["new_key"] != 42 {
			t.Errorf("DataDeletion.Metadata[new_key] = %v, want 42", dd.Metadata["new_key"])
		}
	})
}

func TestNewDataDeletion(t *testing.T) {
	before := time.Now()
	dd := NewDataDeletion(123, DeletionTypeGDPRRequest, 50, "admin@example.com")
	after := time.Now()

	if dd.StudentID != 123 {
		t.Errorf("NewDataDeletion().StudentID = %v, want 123", dd.StudentID)
	}

	if dd.DeletionType != DeletionTypeGDPRRequest {
		t.Errorf("NewDataDeletion().DeletionType = %v, want %v", dd.DeletionType, DeletionTypeGDPRRequest)
	}

	if dd.RecordsDeleted != 50 {
		t.Errorf("NewDataDeletion().RecordsDeleted = %v, want 50", dd.RecordsDeleted)
	}

	if dd.DeletedBy != "admin@example.com" {
		t.Errorf("NewDataDeletion().DeletedBy = %v, want admin@example.com", dd.DeletedBy)
	}

	if dd.DeletedAt.Before(before) || dd.DeletedAt.After(after) {
		t.Errorf("NewDataDeletion().DeletedAt = %v, want between %v and %v", dd.DeletedAt, before, after)
	}

	if dd.Metadata == nil {
		t.Error("NewDataDeletion().Metadata should not be nil")
	}
}

func TestDeletionTypeConstants(t *testing.T) {
	// Verify constants have expected values
	if DeletionTypeVisitRetention != "visit_retention" {
		t.Errorf("DeletionTypeVisitRetention = %q, want visit_retention", DeletionTypeVisitRetention)
	}
	if DeletionTypeManual != "manual" {
		t.Errorf("DeletionTypeManual = %q, want manual", DeletionTypeManual)
	}
	if DeletionTypeGDPRRequest != "gdpr_request" {
		t.Errorf("DeletionTypeGDPRRequest = %q, want gdpr_request", DeletionTypeGDPRRequest)
	}
}
