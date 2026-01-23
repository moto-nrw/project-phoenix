package users

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestGuardianProfile_Validate(t *testing.T) {
	tests := []struct {
		name    string
		profile *GuardianProfile
		wantErr bool
	}{
		{
			name: "valid with email only",
			profile: &GuardianProfile{
				FirstName: "John",
				LastName:  "Doe",
				Email:     base.StringPtr("john@example.com"),
			},
			wantErr: false,
		},
		{
			name: "valid without email (phone numbers are separate)",
			profile: &GuardianProfile{
				FirstName: "John",
				LastName:  "Doe",
			},
			wantErr: false,
		},
		{
			name: "valid with preferred contact method",
			profile: &GuardianProfile{
				FirstName:              "John",
				LastName:               "Doe",
				Email:                  base.StringPtr("john@example.com"),
				PreferredContactMethod: "email",
			},
			wantErr: false,
		},
		{
			name: "missing first name",
			profile: &GuardianProfile{
				FirstName: "",
				LastName:  "Doe",
				Email:     base.StringPtr("john@example.com"),
			},
			wantErr: true,
		},
		{
			name: "whitespace only first name",
			profile: &GuardianProfile{
				FirstName: "   ",
				LastName:  "Doe",
				Email:     base.StringPtr("john@example.com"),
			},
			wantErr: true,
		},
		{
			name: "missing last name",
			profile: &GuardianProfile{
				FirstName: "John",
				LastName:  "",
				Email:     base.StringPtr("john@example.com"),
			},
			wantErr: true,
		},
		{
			name: "whitespace only last name",
			profile: &GuardianProfile{
				FirstName: "John",
				LastName:  "   ",
				Email:     base.StringPtr("john@example.com"),
			},
			wantErr: true,
		},
		{
			name: "invalid email format",
			profile: &GuardianProfile{
				FirstName: "John",
				LastName:  "Doe",
				Email:     base.StringPtr("not-an-email"),
			},
			wantErr: true,
		},
		{
			name: "invalid preferred contact method",
			profile: &GuardianProfile{
				FirstName:              "John",
				LastName:               "Doe",
				Email:                  base.StringPtr("john@example.com"),
				PreferredContactMethod: "invalid",
			},
			wantErr: true,
		},
		{
			name: "valid sms preferred contact method",
			profile: &GuardianProfile{
				FirstName:              "John",
				LastName:               "Doe",
				PreferredContactMethod: "sms",
			},
			wantErr: false,
		},
		{
			name: "valid mobile preferred contact method",
			profile: &GuardianProfile{
				FirstName:              "John",
				LastName:               "Doe",
				PreferredContactMethod: "mobile",
			},
			wantErr: false,
		},
		{
			name: "trim whitespace from names",
			profile: &GuardianProfile{
				FirstName: "  John  ",
				LastName:  "  Doe  ",
				Email:     base.StringPtr("john@example.com"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.profile.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("GuardianProfile.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check that names are trimmed on successful validation
			if !tt.wantErr && tt.name == "trim whitespace from names" {
				if tt.profile.FirstName != "John" {
					t.Errorf("GuardianProfile.Validate() failed to trim FirstName, got %q", tt.profile.FirstName)
				}
				if tt.profile.LastName != "Doe" {
					t.Errorf("GuardianProfile.Validate() failed to trim LastName, got %q", tt.profile.LastName)
				}
			}
		})
	}
}

func TestGuardianProfile_GetFullName(t *testing.T) {
	tests := []struct {
		name      string
		firstName string
		lastName  string
		expected  string
	}{
		{
			name:      "normal names",
			firstName: "John",
			lastName:  "Doe",
			expected:  "John Doe",
		},
		{
			name:      "names with spaces",
			firstName: "Jean Pierre",
			lastName:  "Van Der Berg",
			expected:  "Jean Pierre Van Der Berg",
		},
		{
			name:      "empty first name",
			firstName: "",
			lastName:  "Doe",
			expected:  " Doe",
		},
		{
			name:      "empty last name",
			firstName: "John",
			lastName:  "",
			expected:  "John ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := &GuardianProfile{
				FirstName: tt.firstName,
				LastName:  tt.lastName,
			}

			if got := profile.GetFullName(); got != tt.expected {
				t.Errorf("GuardianProfile.GetFullName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGuardianProfile_GetPreferredContact(t *testing.T) {
	// Helper to create phone number pointers
	makePhone := func(number string, phoneType PhoneType, isPrimary bool) *GuardianPhoneNumber {
		return &GuardianPhoneNumber{
			PhoneNumber: number,
			PhoneType:   phoneType,
			IsPrimary:   isPrimary,
		}
	}

	tests := []struct {
		name     string
		profile  *GuardianProfile
		expected string
	}{
		{
			name: "preferred email returns email",
			profile: &GuardianProfile{
				Email:                  base.StringPtr("john@example.com"),
				PhoneNumbers:           []*GuardianPhoneNumber{makePhone("+49 123 456789", PhoneTypeHome, true)},
				PreferredContactMethod: "email",
			},
			expected: "john@example.com",
		},
		{
			name: "preferred phone returns home phone",
			profile: &GuardianProfile{
				Email:                  base.StringPtr("john@example.com"),
				PhoneNumbers:           []*GuardianPhoneNumber{makePhone("+49 123 456789", PhoneTypeHome, true)},
				PreferredContactMethod: "phone",
			},
			expected: "+49 123 456789",
		},
		{
			name: "preferred mobile returns mobile phone",
			profile: &GuardianProfile{
				Email: base.StringPtr("john@example.com"),
				PhoneNumbers: []*GuardianPhoneNumber{
					makePhone("+49 171 1234567", PhoneTypeMobile, true),
				},
				PreferredContactMethod: "mobile",
			},
			expected: "+49 171 1234567",
		},
		{
			name: "preferred sms returns mobile phone",
			profile: &GuardianProfile{
				Email: base.StringPtr("john@example.com"),
				PhoneNumbers: []*GuardianPhoneNumber{
					makePhone("+49 171 1234567", PhoneTypeMobile, true),
				},
				PreferredContactMethod: "sms",
			},
			expected: "+49 171 1234567",
		},
		{
			name: "fallback to primary phone when no preferred",
			profile: &GuardianProfile{
				Email: base.StringPtr("john@example.com"),
				PhoneNumbers: []*GuardianPhoneNumber{
					makePhone("+49 171 1234567", PhoneTypeMobile, true),
					makePhone("+49 123 456789", PhoneTypeHome, false),
				},
			},
			expected: "+49 171 1234567",
		},
		{
			name: "fallback to first phone when none primary",
			profile: &GuardianProfile{
				Email: base.StringPtr("john@example.com"),
				PhoneNumbers: []*GuardianPhoneNumber{
					makePhone("+49 123 456789", PhoneTypeHome, false),
					makePhone("+49 171 1234567", PhoneTypeMobile, false),
				},
			},
			expected: "+49 123 456789",
		},
		{
			name: "fallback to email when no phone",
			profile: &GuardianProfile{
				Email: base.StringPtr("john@example.com"),
			},
			expected: "john@example.com",
		},
		{
			name: "empty string when no contact",
			profile: &GuardianProfile{
				PreferredContactMethod: "email",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.profile.GetPreferredContact(); got != tt.expected {
				t.Errorf("GuardianProfile.GetPreferredContact() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGuardianProfile_GetPrimaryPhone(t *testing.T) {
	makePhone := func(number string, phoneType PhoneType, isPrimary bool) *GuardianPhoneNumber {
		return &GuardianPhoneNumber{
			PhoneNumber: number,
			PhoneType:   phoneType,
			IsPrimary:   isPrimary,
		}
	}

	tests := []struct {
		name     string
		phones   []*GuardianPhoneNumber
		expected string
	}{
		{
			name:     "empty phones returns empty",
			phones:   nil,
			expected: "",
		},
		{
			name: "returns primary phone",
			phones: []*GuardianPhoneNumber{
				makePhone("+49 123 456789", PhoneTypeHome, false),
				makePhone("+49 171 1234567", PhoneTypeMobile, true),
			},
			expected: "+49 171 1234567",
		},
		{
			name: "returns first phone if no primary",
			phones: []*GuardianPhoneNumber{
				makePhone("+49 123 456789", PhoneTypeHome, false),
				makePhone("+49 171 1234567", PhoneTypeMobile, false),
			},
			expected: "+49 123 456789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := &GuardianProfile{PhoneNumbers: tt.phones}
			if got := profile.GetPrimaryPhone(); got != tt.expected {
				t.Errorf("GuardianProfile.GetPrimaryPhone() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGuardianProfile_GetPhoneByType(t *testing.T) {
	makePhone := func(number string, phoneType PhoneType, isPrimary bool) *GuardianPhoneNumber {
		return &GuardianPhoneNumber{
			PhoneNumber: number,
			PhoneType:   phoneType,
			IsPrimary:   isPrimary,
		}
	}

	phones := []*GuardianPhoneNumber{
		makePhone("+49 123 456789", PhoneTypeHome, false),
		makePhone("+49 171 1234567", PhoneTypeMobile, true),
		makePhone("+49 30 123456", PhoneTypeWork, false),
	}

	profile := &GuardianProfile{PhoneNumbers: phones}

	tests := []struct {
		name      string
		phoneType PhoneType
		expected  string
	}{
		{"home phone", PhoneTypeHome, "+49 123 456789"},
		{"mobile phone", PhoneTypeMobile, "+49 171 1234567"},
		{"work phone", PhoneTypeWork, "+49 30 123456"},
		{"other phone (not found)", PhoneTypeOther, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := profile.GetPhoneByType(tt.phoneType); got != tt.expected {
				t.Errorf("GuardianProfile.GetPhoneByType(%v) = %q, want %q", tt.phoneType, got, tt.expected)
			}
		})
	}
}

func TestGuardianProfile_CanInvite(t *testing.T) {
	tests := []struct {
		name     string
		profile  *GuardianProfile
		expected bool
	}{
		{
			name: "can invite - has email and no account",
			profile: &GuardianProfile{
				Email:      base.StringPtr("john@example.com"),
				HasAccount: false,
			},
			expected: true,
		},
		{
			name: "cannot invite - has account",
			profile: &GuardianProfile{
				Email:      base.StringPtr("john@example.com"),
				HasAccount: true,
			},
			expected: false,
		},
		{
			name: "cannot invite - no email",
			profile: &GuardianProfile{
				HasAccount: false,
			},
			expected: false,
		},
		{
			name: "cannot invite - empty email",
			profile: &GuardianProfile{
				Email:      base.StringPtr(""),
				HasAccount: false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.profile.CanInvite(); got != tt.expected {
				t.Errorf("GuardianProfile.CanInvite() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGuardianProfile_HasEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    *string
		expected bool
	}{
		{
			name:     "has email",
			email:    base.StringPtr("john@example.com"),
			expected: true,
		},
		{
			name:     "no email - nil",
			email:    nil,
			expected: false,
		},
		{
			name:     "no email - empty string",
			email:    base.StringPtr(""),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := &GuardianProfile{Email: tt.email}
			if got := profile.HasEmail(); got != tt.expected {
				t.Errorf("GuardianProfile.HasEmail() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGuardianProfile_BeforeAppendModel(t *testing.T) {
	t.Run("handles nil query", func(t *testing.T) {
		profile := &GuardianProfile{
			FirstName: "John",
			LastName:  "Doe",
			Email:     base.StringPtr("john@example.com"),
		}
		err := profile.BeforeAppendModel(nil)
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})

	t.Run("returns no error for unknown query type", func(t *testing.T) {
		profile := &GuardianProfile{
			FirstName: "John",
			LastName:  "Doe",
			Email:     base.StringPtr("john@example.com"),
		}
		err := profile.BeforeAppendModel("some string")
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})
}

func TestGuardianProfile_TableName(t *testing.T) {
	profile := &GuardianProfile{}
	if got := profile.TableName(); got != "users.guardian_profiles" {
		t.Errorf("TableName() = %v, want users.guardian_profiles", got)
	}
}

func TestGuardianProfile_GetID(t *testing.T) {
	profile := &GuardianProfile{
		Model:     base.Model{ID: 42},
		FirstName: "John",
		LastName:  "Doe",
		Email:     base.StringPtr("john@example.com"),
	}

	if got, ok := profile.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", profile.GetID())
	}
}

func TestGuardianProfile_GetCreatedAt(t *testing.T) {
	now := time.Now()
	profile := &GuardianProfile{
		Model:     base.Model{CreatedAt: now},
		FirstName: "John",
		LastName:  "Doe",
		Email:     base.StringPtr("john@example.com"),
	}

	if got := profile.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestGuardianProfile_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	profile := &GuardianProfile{
		Model:     base.Model{UpdatedAt: now},
		FirstName: "John",
		LastName:  "Doe",
		Email:     base.StringPtr("john@example.com"),
	}

	if got := profile.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
