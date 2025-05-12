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
				StudentID:     1,
				PolicyVersion: "1.0",
				Accepted:      true,
				AcceptedAt:    &now,
				ExpiresAt:     &future,
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
				StudentID: 1,
			},
			wantErr: true,
		},
		{
			name: "expiration before acceptance",
			pc: &PrivacyConsent{
				StudentID:     1,
				PolicyVersion: "1.0",
				Accepted:      true,
				AcceptedAt:    &now,
				ExpiresAt:     &past,
			},
			wantErr: true,
		},
		{
			name: "details map provided",
			pc: &PrivacyConsent{
				StudentID:     1,
				PolicyVersion: "1.0",
				Details:       map[string]interface{}{"test": true},
			},
			wantErr: false,
		},
		{
			name: "auto-populate accepted_at",
			pc: &PrivacyConsent{
				StudentID:     1,
				PolicyVersion: "1.0",
				Accepted:      true,
			},
			wantErr: false,
		},
		{
			name: "auto-calculate expires_at from duration",
			pc: &PrivacyConsent{
				StudentID:     1,
				PolicyVersion: "1.0",
				Accepted:      true,
				AcceptedAt:    &now,
				DurationDays:  &days30,
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

func TestPrivacyConsent_NeedsRenewal(t *testing.T) {
	tests := []struct {
		name     string
		pc       *PrivacyConsent
		expected bool
	}{
		{
			name: "needs renewal",
			pc: &PrivacyConsent{
				RenewalRequired: true,
			},
			expected: true,
		},
		{
			name: "doesn't need renewal",
			pc: &PrivacyConsent{
				RenewalRequired: false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pc.NeedsRenewal(); got != tt.expected {
				t.Errorf("PrivacyConsent.NeedsRenewal() = %v, want %v", got, tt.expected)
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

func TestPrivacyConsent_EntityInterface(t *testing.T) {
	now := time.Now()
	pc := &PrivacyConsent{
		Model: base.Model{
			ID:        123,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	// Test GetID
	if pc.GetID() != int64(123) {
		t.Errorf("PrivacyConsent.GetID() = %v, want %v", pc.GetID(), int64(123))
	}

	// Test GetCreatedAt
	if pc.GetCreatedAt() != now {
		t.Errorf("PrivacyConsent.GetCreatedAt() = %v, want %v", pc.GetCreatedAt(), now)
	}

	// Test GetUpdatedAt
	if pc.GetUpdatedAt() != now {
		t.Errorf("PrivacyConsent.GetUpdatedAt() = %v, want %v", pc.GetUpdatedAt(), now)
	}
}
