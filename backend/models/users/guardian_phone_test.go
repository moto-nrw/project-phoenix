package users

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestGuardianPhoneNumber_Validate(t *testing.T) {
	tests := []struct {
		name    string
		phone   *GuardianPhoneNumber
		wantErr bool
	}{
		{
			name: "valid phone number",
			phone: &GuardianPhoneNumber{
				GuardianProfileID: 1,
				PhoneNumber:       "+49 30 123456",
				PhoneType:         PhoneTypeHome,
			},
			wantErr: false,
		},
		{
			name: "valid mobile phone",
			phone: &GuardianPhoneNumber{
				GuardianProfileID: 1,
				PhoneNumber:       "0170-1234567",
				PhoneType:         PhoneTypeMobile,
			},
			wantErr: false,
		},
		{
			name: "valid work phone with label",
			phone: &GuardianPhoneNumber{
				GuardianProfileID: 1,
				PhoneNumber:       "+49 30 9876543",
				PhoneType:         PhoneTypeWork,
				Label:             base.StringPtr("Büro"),
			},
			wantErr: false,
		},
		{
			name: "valid other phone type",
			phone: &GuardianPhoneNumber{
				GuardianProfileID: 1,
				PhoneNumber:       "030-999999",
				PhoneType:         PhoneTypeOther,
			},
			wantErr: false,
		},
		{
			name: "missing guardian profile ID",
			phone: &GuardianPhoneNumber{
				GuardianProfileID: 0,
				PhoneNumber:       "+49 30 123456",
				PhoneType:         PhoneTypeHome,
			},
			wantErr: true,
		},
		{
			name: "negative guardian profile ID",
			phone: &GuardianPhoneNumber{
				GuardianProfileID: -1,
				PhoneNumber:       "+49 30 123456",
				PhoneType:         PhoneTypeHome,
			},
			wantErr: true,
		},
		{
			name: "empty phone number",
			phone: &GuardianPhoneNumber{
				GuardianProfileID: 1,
				PhoneNumber:       "",
				PhoneType:         PhoneTypeHome,
			},
			wantErr: true,
		},
		{
			name: "whitespace only phone number",
			phone: &GuardianPhoneNumber{
				GuardianProfileID: 1,
				PhoneNumber:       "   ",
				PhoneType:         PhoneTypeHome,
			},
			wantErr: true,
		},
		{
			name: "invalid phone number format - letters",
			phone: &GuardianPhoneNumber{
				GuardianProfileID: 1,
				PhoneNumber:       "ABC123",
				PhoneType:         PhoneTypeHome,
			},
			wantErr: true,
		},
		{
			name: "invalid phone number format - special chars",
			phone: &GuardianPhoneNumber{
				GuardianProfileID: 1,
				PhoneNumber:       "123#456",
				PhoneType:         PhoneTypeHome,
			},
			wantErr: true,
		},
		{
			name: "phone number too short",
			phone: &GuardianPhoneNumber{
				GuardianProfileID: 1,
				PhoneNumber:       "12",
				PhoneType:         PhoneTypeHome,
			},
			wantErr: true,
		},
		{
			name: "invalid phone type",
			phone: &GuardianPhoneNumber{
				GuardianProfileID: 1,
				PhoneNumber:       "+49 30 123456",
				PhoneType:         "invalid",
			},
			wantErr: true,
		},
		{
			name: "empty phone type",
			phone: &GuardianPhoneNumber{
				GuardianProfileID: 1,
				PhoneNumber:       "+49 30 123456",
				PhoneType:         "",
			},
			wantErr: true,
		},
		{
			name: "trims phone number whitespace",
			phone: &GuardianPhoneNumber{
				GuardianProfileID: 1,
				PhoneNumber:       "  +49 30 123456  ",
				PhoneType:         PhoneTypeHome,
			},
			wantErr: false,
		},
		{
			name: "trims empty label to nil",
			phone: &GuardianPhoneNumber{
				GuardianProfileID: 1,
				PhoneNumber:       "+49 30 123456",
				PhoneType:         PhoneTypeHome,
				Label:             base.StringPtr("   "),
			},
			wantErr: false,
		},
		{
			name: "corrects negative priority to 1",
			phone: &GuardianPhoneNumber{
				GuardianProfileID: 1,
				PhoneNumber:       "+49 30 123456",
				PhoneType:         PhoneTypeHome,
				Priority:          -5,
			},
			wantErr: false,
		},
		{
			name: "corrects zero priority to 1",
			phone: &GuardianPhoneNumber{
				GuardianProfileID: 1,
				PhoneNumber:       "+49 30 123456",
				PhoneType:         PhoneTypeHome,
				Priority:          0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.phone.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("GuardianPhoneNumber.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check side effects of validation
			if !tt.wantErr {
				// Verify phone number was trimmed
				if tt.name == "trims phone number whitespace" {
					if tt.phone.PhoneNumber != "+49 30 123456" {
						t.Errorf("GuardianPhoneNumber.Validate() did not trim phone number, got %q", tt.phone.PhoneNumber)
					}
				}

				// Verify empty label was set to nil
				if tt.name == "trims empty label to nil" {
					if tt.phone.Label != nil {
						t.Errorf("GuardianPhoneNumber.Validate() did not set empty label to nil, got %v", tt.phone.Label)
					}
				}

				// Verify priority was corrected
				if tt.name == "corrects negative priority to 1" || tt.name == "corrects zero priority to 1" {
					if tt.phone.Priority != 1 {
						t.Errorf("GuardianPhoneNumber.Validate() did not correct priority to 1, got %d", tt.phone.Priority)
					}
				}
			}
		})
	}
}

func TestGuardianPhoneNumber_TableName(t *testing.T) {
	phone := &GuardianPhoneNumber{}
	if got := phone.TableName(); got != "users.guardian_phone_numbers" {
		t.Errorf("TableName() = %v, want users.guardian_phone_numbers", got)
	}
}

func TestGuardianPhoneNumber_GetID(t *testing.T) {
	phone := &GuardianPhoneNumber{
		Model:             base.Model{ID: 42},
		GuardianProfileID: 1,
		PhoneNumber:       "+49 30 123456",
		PhoneType:         PhoneTypeHome,
	}

	if got, ok := phone.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", phone.GetID())
	}
}

func TestGuardianPhoneNumber_GetCreatedAt(t *testing.T) {
	now := time.Now()
	phone := &GuardianPhoneNumber{
		Model:             base.Model{CreatedAt: now},
		GuardianProfileID: 1,
		PhoneNumber:       "+49 30 123456",
		PhoneType:         PhoneTypeHome,
	}

	if got := phone.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestGuardianPhoneNumber_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	phone := &GuardianPhoneNumber{
		Model:             base.Model{UpdatedAt: now},
		GuardianProfileID: 1,
		PhoneNumber:       "+49 30 123456",
		PhoneType:         PhoneTypeHome,
	}

	if got := phone.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}

func TestGuardianPhoneNumber_GetDisplayString(t *testing.T) {
	tests := []struct {
		name     string
		phone    *GuardianPhoneNumber
		expected string
	}{
		{
			name: "phone without label",
			phone: &GuardianPhoneNumber{
				PhoneNumber: "+49 30 123456",
				PhoneType:   PhoneTypeHome,
			},
			expected: "+49 30 123456",
		},
		{
			name: "phone with label",
			phone: &GuardianPhoneNumber{
				PhoneNumber: "+49 30 9876543",
				PhoneType:   PhoneTypeWork,
				Label:       base.StringPtr("Büro"),
			},
			expected: "+49 30 9876543 (Büro)",
		},
		{
			name: "phone with empty label",
			phone: &GuardianPhoneNumber{
				PhoneNumber: "+49 30 123456",
				PhoneType:   PhoneTypeHome,
				Label:       base.StringPtr(""),
			},
			expected: "+49 30 123456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.phone.GetDisplayString(); got != tt.expected {
				t.Errorf("GetDisplayString() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGuardianPhoneNumber_PhoneTypeLabel(t *testing.T) {
	tests := []struct {
		name      string
		phoneType PhoneType
		expected  string
	}{
		{"mobile", PhoneTypeMobile, "Mobil"},
		{"home", PhoneTypeHome, "Telefon"},
		{"work", PhoneTypeWork, "Dienstlich"},
		{"other", PhoneTypeOther, "Sonstige"},
		{"unknown", PhoneType("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phone := &GuardianPhoneNumber{PhoneType: tt.phoneType}
			if got := phone.PhoneTypeLabel(); got != tt.expected {
				t.Errorf("PhoneTypeLabel() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGuardianPhoneNumber_BeforeAppendModel(t *testing.T) {
	t.Run("handles nil query", func(t *testing.T) {
		phone := &GuardianPhoneNumber{
			GuardianProfileID: 1,
			PhoneNumber:       "+49 30 123456",
			PhoneType:         PhoneTypeHome,
		}
		err := phone.BeforeAppendModel(nil)
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})

	t.Run("returns no error for unknown query type", func(t *testing.T) {
		phone := &GuardianPhoneNumber{
			GuardianProfileID: 1,
			PhoneNumber:       "+49 30 123456",
			PhoneType:         PhoneTypeHome,
		}
		err := phone.BeforeAppendModel("some string")
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})
}

func TestValidPhoneTypes(t *testing.T) {
	expected := map[PhoneType]bool{
		PhoneTypeMobile: true,
		PhoneTypeHome:   true,
		PhoneTypeWork:   true,
		PhoneTypeOther:  true,
	}

	if len(ValidPhoneTypes) != len(expected) {
		t.Errorf("ValidPhoneTypes has %d entries, want %d", len(ValidPhoneTypes), len(expected))
	}

	for pt, valid := range expected {
		if ValidPhoneTypes[pt] != valid {
			t.Errorf("ValidPhoneTypes[%q] = %v, want %v", pt, ValidPhoneTypes[pt], valid)
		}
	}

	// Verify invalid type returns false
	if ValidPhoneTypes["invalid"] {
		t.Error("ValidPhoneTypes[\"invalid\"] should be false")
	}
}

func TestPhoneTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant PhoneType
		expected string
	}{
		{"PhoneTypeMobile", PhoneTypeMobile, "mobile"},
		{"PhoneTypeHome", PhoneTypeHome, "home"},
		{"PhoneTypeWork", PhoneTypeWork, "work"},
		{"PhoneTypeOther", PhoneTypeOther, "other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.constant) != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.expected)
			}
		})
	}
}
