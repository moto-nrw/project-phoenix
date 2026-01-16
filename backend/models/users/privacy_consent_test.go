package users

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestPrivacyConsent_Validate(t *testing.T) {
	now := time.Now()
	future := now.AddDate(0, 0, 30) // 30 days in the future
	past := now.AddDate(0, 0, -30)  // 30 days in the past
	days30 := 30

	tests := []struct {
		name    string
		pc      *PrivacyConsent
		wantErr bool
	}{
		{
			name: "valid",
			pc: &PrivacyConsent{
				StudentID:         1,
				PolicyVersion:     "1.0",
				Accepted:          true,
				AcceptedAt:        &now,
				ExpiresAt:         &future,
				DataRetentionDays: 30,
			},
			wantErr: false,
		},
		{
			name: "missing student ID",
			pc: &PrivacyConsent{
				PolicyVersion: "1.0",
			},
			wantErr: true,
		},
		{
			name: "missing policy version",
			pc: &PrivacyConsent{
				StudentID:         1,
				DataRetentionDays: 30,
			},
			wantErr: true,
		},
		{
			name: "expiration before acceptance",
			pc: &PrivacyConsent{
				StudentID:         1,
				PolicyVersion:     "1.0",
				Accepted:          true,
				AcceptedAt:        &now,
				ExpiresAt:         &past,
				DataRetentionDays: 30,
			},
			wantErr: true,
		},
		{
			name: "details map provided",
			pc: &PrivacyConsent{
				StudentID:         1,
				PolicyVersion:     "1.0",
				Details:           map[string]interface{}{"test": true},
				DataRetentionDays: 30,
			},
			wantErr: false,
		},
		{
			name: "auto-populate accepted_at",
			pc: &PrivacyConsent{
				StudentID:         1,
				PolicyVersion:     "1.0",
				Accepted:          true,
				DataRetentionDays: 30,
			},
			wantErr: false,
		},
		{
			name: "auto-calculate expires_at from duration",
			pc: &PrivacyConsent{
				StudentID:         1,
				PolicyVersion:     "1.0",
				Accepted:          true,
				AcceptedAt:        &now,
				DurationDays:      &days30,
				DataRetentionDays: 30,
			},
			wantErr: false,
		},
		{
			name: "invalid data retention days - too low",
			pc: &PrivacyConsent{
				StudentID:         1,
				PolicyVersion:     "1.0",
				Accepted:          true,
				DataRetentionDays: 0,
			},
			wantErr: true,
		},
		{
			name: "invalid data retention days - too high",
			pc: &PrivacyConsent{
				StudentID:         1,
				PolicyVersion:     "1.0",
				Accepted:          true,
				DataRetentionDays: 32,
			},
			wantErr: true,
		},
		{
			name: "valid data retention days - minimum",
			pc: &PrivacyConsent{
				StudentID:         1,
				PolicyVersion:     "1.0",
				Accepted:          true,
				DataRetentionDays: 1,
			},
			wantErr: false,
		},
		{
			name: "valid data retention days - maximum",
			pc: &PrivacyConsent{
				StudentID:         1,
				PolicyVersion:     "1.0",
				Accepted:          true,
				DataRetentionDays: 31,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pc.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("PrivacyConsent.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check auto-population of fields
			if tt.name == "auto-populate accepted_at" {
				if tt.pc.AcceptedAt == nil {
					t.Errorf("PrivacyConsent.Validate() failed to auto-populate AcceptedAt")
				}
			}

			if tt.name == "auto-calculate expires_at from duration" {
				if tt.pc.ExpiresAt == nil {
					t.Errorf("PrivacyConsent.Validate() failed to auto-calculate ExpiresAt")
				} else {
					expectedExpiry := tt.pc.AcceptedAt.AddDate(0, 0, *tt.pc.DurationDays)
					if !tt.pc.ExpiresAt.Equal(expectedExpiry) {
						t.Errorf("PrivacyConsent.Validate() calculated ExpiresAt = %v, want %v",
							tt.pc.ExpiresAt, expectedExpiry)
					}
				}
			}
		})
	}
}

func TestPrivacyConsent_IsValid(t *testing.T) {
	now := time.Now()
	future := now.AddDate(0, 0, 30) // 30 days in the future
	past := now.AddDate(0, 0, -30)  // 30 days in the past

	tests := []struct {
		name     string
		pc       *PrivacyConsent
		expected bool
	}{
		{
			name: "valid consent",
			pc: &PrivacyConsent{
				StudentID:     1,
				PolicyVersion: "1.0",
				Accepted:      true,
				AcceptedAt:    &now,
				ExpiresAt:     &future,
			},
			expected: true,
		},
		{
			name: "not accepted",
			pc: &PrivacyConsent{
				StudentID:     1,
				PolicyVersion: "1.0",
				Accepted:      false,
				AcceptedAt:    &now,
				ExpiresAt:     &future,
			},
			expected: false,
		},
		{
			name: "expired",
			pc: &PrivacyConsent{
				StudentID:     1,
				PolicyVersion: "1.0",
				Accepted:      true,
				AcceptedAt:    &past,
				ExpiresAt:     &past,
			},
			expected: false,
		},
		{
			name: "no expiration",
			pc: &PrivacyConsent{
				StudentID:     1,
				PolicyVersion: "1.0",
				Accepted:      true,
				AcceptedAt:    &now,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pc.IsValid(); got != tt.expected {
				t.Errorf("PrivacyConsent.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPrivacyConsent_IsExpired(t *testing.T) {
	now := time.Now()
	future := now.AddDate(0, 0, 30) // 30 days in the future
	past := now.AddDate(0, 0, -30)  // 30 days in the past

	tests := []struct {
		name     string
		pc       *PrivacyConsent
		expected bool
	}{
		{
			name: "not expired",
			pc: &PrivacyConsent{
				ExpiresAt: &future,
			},
			expected: false,
		},
		{
			name: "expired",
			pc: &PrivacyConsent{
				ExpiresAt: &past,
			},
			expected: true,
		},
		{
			name: "no expiration",
			pc: &PrivacyConsent{
				ExpiresAt: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pc.IsExpired(); got != tt.expected {
				t.Errorf("PrivacyConsent.IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPrivacyConsent_GetTimeToExpiry(t *testing.T) {
	now := time.Now()
	future := now.AddDate(0, 0, 30) // 30 days in the future
	past := now.AddDate(0, 0, -30)  // 30 days in the past

	tests := []struct {
		name          string
		pc            *PrivacyConsent
		expectedNil   bool
		expectedZero  bool
		expectedValid bool
	}{
		{
			name: "future expiry",
			pc: &PrivacyConsent{
				ExpiresAt: &future,
			},
			expectedNil:   false,
			expectedZero:  false,
			expectedValid: true,
		},
		{
			name: "past expiry",
			pc: &PrivacyConsent{
				ExpiresAt: &past,
			},
			expectedNil:   false,
			expectedZero:  true,
			expectedValid: false,
		},
		{
			name: "no expiry",
			pc: &PrivacyConsent{
				ExpiresAt: nil,
			},
			expectedNil:   true,
			expectedZero:  false,
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pc.GetTimeToExpiry()

			if tt.expectedNil && got != nil {
				t.Errorf("PrivacyConsent.GetTimeToExpiry() = %v, want nil", got)
				return
			}

			if !tt.expectedNil && got == nil {
				t.Errorf("PrivacyConsent.GetTimeToExpiry() = nil, want non-nil")
				return
			}

			if !tt.expectedNil {
				if tt.expectedZero && *got != time.Duration(0) {
					t.Errorf("PrivacyConsent.GetTimeToExpiry() = %v, want 0", got)
				}

				if tt.expectedValid && *got <= time.Duration(0) {
					t.Errorf("PrivacyConsent.GetTimeToExpiry() = %v, want > 0", got)
				}
			}
		})
	}
}

func TestPrivacyConsent_SetStudent(t *testing.T) {
	student := &Student{
		Model: base.Model{
			ID: 123,
		},
		PersonID:    1,
		SchoolClass: "Class A",
	}

	pc := &PrivacyConsent{
		StudentID:     0,
		PolicyVersion: "1.0",
	}

	pc.SetStudent(student)

	if pc.StudentID != 123 {
		t.Errorf("PrivacyConsent.SetStudent() failed to set student ID, got %v", pc.StudentID)
	}

	if pc.Student != student {
		t.Errorf("PrivacyConsent.SetStudent() failed to set student reference")
	}
}

func TestPrivacyConsent_UpdateDetails(t *testing.T) {
	pc := &PrivacyConsent{
		StudentID:     1,
		PolicyVersion: "1.0",
		Details:       map[string]interface{}{},
	}

	// Update details
	newDetails := map[string]interface{}{
		"data_retention": map[string]interface{}{
			"photos":      true,
			"attendance":  true,
			"assessments": false,
		},
		"third_party_sharing": false,
		"research_use":        true,
	}

	err := pc.UpdateDetails(newDetails)
	if err != nil {
		t.Errorf("PrivacyConsent.UpdateDetails() error = %v", err)
	}

	// Check top-level keys directly in the map
	if _, exists := pc.Details["data_retention"]; !exists {
		t.Errorf("PrivacyConsent.UpdateDetails() failed, data_retention key not found")
	}

	if thirdParty, exists := pc.Details["third_party_sharing"].(bool); !exists || thirdParty {
		t.Errorf("PrivacyConsent.UpdateDetails() failed, third_party_sharing incorrect or not found")
	}

	if research, exists := pc.Details["research_use"].(bool); !exists || !research {
		t.Errorf("PrivacyConsent.UpdateDetails() failed, research_use incorrect or not found")
	}

	// Check nested fields
	dataRetention, ok := pc.Details["data_retention"].(map[string]interface{})
	if !ok {
		t.Errorf("PrivacyConsent.UpdateDetails() failed to handle nested structure")
	} else {
		if photos, exists := dataRetention["photos"].(bool); !exists || !photos {
			t.Errorf("PrivacyConsent.UpdateDetails() failed, photos preference incorrect or not found")
		}

		if attendance, exists := dataRetention["attendance"].(bool); !exists || !attendance {
			t.Errorf("PrivacyConsent.UpdateDetails() failed, attendance preference incorrect or not found")
		}

		if assessments, exists := dataRetention["assessments"].(bool); !exists || assessments {
			t.Errorf("PrivacyConsent.UpdateDetails() failed, assessments preference incorrect or not found")
		}
	}
}

func TestPrivacyConsent_Accept(t *testing.T) {
	days30 := 30
	pc := &PrivacyConsent{
		StudentID:     1,
		PolicyVersion: "1.0",
		Accepted:      false,
		DurationDays:  &days30,
	}

	pc.Accept()

	if !pc.Accepted {
		t.Errorf("PrivacyConsent.Accept() failed to set Accepted to true")
	}

	if pc.AcceptedAt == nil {
		t.Errorf("PrivacyConsent.Accept() failed to set AcceptedAt")
	}

	if pc.ExpiresAt == nil {
		t.Errorf("PrivacyConsent.Accept() failed to calculate ExpiresAt")
	} else {
		expectedExpiresAt := pc.AcceptedAt.AddDate(0, 0, *pc.DurationDays)
		if !pc.ExpiresAt.Equal(expectedExpiresAt) {
			t.Errorf("PrivacyConsent.Accept() calculated ExpiresAt = %v, want %v",
				pc.ExpiresAt, expectedExpiresAt)
		}
	}
}

func TestPrivacyConsent_Revoke(t *testing.T) {
	now := time.Now()
	pc := &PrivacyConsent{
		StudentID:     1,
		PolicyVersion: "1.0",
		Accepted:      true,
		AcceptedAt:    &now,
	}

	pc.Revoke()

	if pc.Accepted {
		t.Errorf("PrivacyConsent.Revoke() failed to set Accepted to false")
	}
}

func TestPrivacyConsent_GetDetails(t *testing.T) {
	t.Run("get details when nil", func(t *testing.T) {
		pc := &PrivacyConsent{}

		details := pc.GetDetails()
		if details == nil {
			t.Error("PrivacyConsent.GetDetails() should return initialized map, got nil")
		}
		if len(details) != 0 {
			t.Errorf("PrivacyConsent.GetDetails() should return empty map, got %v", details)
		}
	})

	t.Run("get details when populated", func(t *testing.T) {
		pc := &PrivacyConsent{
			Details: map[string]interface{}{
				"test_key": "test_value",
			},
		}

		details := pc.GetDetails()
		if details == nil {
			t.Error("PrivacyConsent.GetDetails() should return map, got nil")
		}
		if details["test_key"] != "test_value" {
			t.Errorf("PrivacyConsent.GetDetails() = %v, want test_value", details["test_key"])
		}
	})
}

func TestPrivacyConsent_GetDataRetentionDays(t *testing.T) {
	tests := []struct {
		name              string
		dataRetentionDays int
		expected          int
	}{
		{
			name:              "returns set value",
			dataRetentionDays: 15,
			expected:          15,
		},
		{
			name:              "defaults to 30 when zero",
			dataRetentionDays: 0,
			expected:          30,
		},
		{
			name:              "returns minimum value",
			dataRetentionDays: 1,
			expected:          1,
		},
		{
			name:              "returns maximum value",
			dataRetentionDays: 31,
			expected:          31,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &PrivacyConsent{
				DataRetentionDays: tt.dataRetentionDays,
			}

			if got := pc.GetDataRetentionDays(); got != tt.expected {
				t.Errorf("PrivacyConsent.GetDataRetentionDays() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPrivacyConsent_SetDataRetentionDays(t *testing.T) {
	tests := []struct {
		name    string
		days    int
		wantErr bool
	}{
		{
			name:    "valid minimum",
			days:    1,
			wantErr: false,
		},
		{
			name:    "valid maximum",
			days:    31,
			wantErr: false,
		},
		{
			name:    "valid middle",
			days:    15,
			wantErr: false,
		},
		{
			name:    "invalid - too low",
			days:    0,
			wantErr: true,
		},
		{
			name:    "invalid - negative",
			days:    -1,
			wantErr: true,
		},
		{
			name:    "invalid - too high",
			days:    32,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &PrivacyConsent{}

			err := pc.SetDataRetentionDays(tt.days)
			if (err != nil) != tt.wantErr {
				t.Errorf("PrivacyConsent.SetDataRetentionDays() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && pc.DataRetentionDays != tt.days {
				t.Errorf("PrivacyConsent.DataRetentionDays = %v, want %v", pc.DataRetentionDays, tt.days)
			}
		})
	}
}

func TestPrivacyConsent_SetStudent_Nil(t *testing.T) {
	pc := &PrivacyConsent{
		StudentID:     10,
		PolicyVersion: "1.0",
	}

	pc.SetStudent(nil)

	if pc.Student != nil {
		t.Error("PrivacyConsent.SetStudent(nil) should set Student to nil")
	}
	// StudentID should remain unchanged when setting nil
	if pc.StudentID != 10 {
		t.Errorf("PrivacyConsent.SetStudent(nil) should not change StudentID, got %d", pc.StudentID)
	}
}

func TestPrivacyConsent_BeforeAppendModel(t *testing.T) {
	t.Run("handles nil query", func(t *testing.T) {
		pc := &PrivacyConsent{
			StudentID:         1,
			PolicyVersion:     "1.0",
			DataRetentionDays: 30,
		}
		err := pc.BeforeAppendModel(nil)
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})

	t.Run("returns no error for unknown query type", func(t *testing.T) {
		pc := &PrivacyConsent{
			StudentID:         1,
			PolicyVersion:     "1.0",
			DataRetentionDays: 30,
		}
		err := pc.BeforeAppendModel("some string")
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})
}

func TestPrivacyConsent_TableName(t *testing.T) {
	pc := &PrivacyConsent{}
	if got := pc.TableName(); got != "users.privacy_consents" {
		t.Errorf("TableName() = %v, want users.privacy_consents", got)
	}
}

func TestPrivacyConsent_NeedsRenewal(t *testing.T) {
	tests := []struct {
		name            string
		renewalRequired bool
		expected        bool
	}{
		{
			name:            "renewal required",
			renewalRequired: true,
			expected:        true,
		},
		{
			name:            "renewal not required",
			renewalRequired: false,
			expected:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &PrivacyConsent{
				RenewalRequired: tt.renewalRequired,
			}

			if got := pc.NeedsRenewal(); got != tt.expected {
				t.Errorf("PrivacyConsent.NeedsRenewal() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPrivacyConsent_GetID(t *testing.T) {
	pc := &PrivacyConsent{
		Model:             base.Model{ID: 42},
		StudentID:         1,
		PolicyVersion:     "1.0",
		DataRetentionDays: 30,
	}

	if got, ok := pc.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", pc.GetID())
	}
}

func TestPrivacyConsent_GetCreatedAt(t *testing.T) {
	now := time.Now()
	pc := &PrivacyConsent{
		Model:             base.Model{CreatedAt: now},
		StudentID:         1,
		PolicyVersion:     "1.0",
		DataRetentionDays: 30,
	}

	if got := pc.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestPrivacyConsent_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	pc := &PrivacyConsent{
		Model:             base.Model{UpdatedAt: now},
		StudentID:         1,
		PolicyVersion:     "1.0",
		DataRetentionDays: 30,
	}

	if got := pc.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
