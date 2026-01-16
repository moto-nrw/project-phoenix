package users

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
)

// Test helpers - local to avoid external dependencies
func stringPtr(s string) *string     { return &s }
func int64Ptr(i int64) *int64        { return &i }
func timePtr(t time.Time) *time.Time { return &t }

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
				Email:     stringPtr("john@example.com"),
			},
			wantErr: false,
		},
		{
			name: "valid with phone only",
			profile: &GuardianProfile{
				FirstName: "John",
				LastName:  "Doe",
				Phone:     stringPtr("+49 123 456789"),
			},
			wantErr: false,
		},
		{
			name: "valid with mobile phone only",
			profile: &GuardianProfile{
				FirstName:   "John",
				LastName:    "Doe",
				MobilePhone: stringPtr("+49 171 1234567"),
			},
			wantErr: false,
		},
		{
			name: "valid with all contact methods",
			profile: &GuardianProfile{
				FirstName:   "John",
				LastName:    "Doe",
				Email:       stringPtr("john@example.com"),
				Phone:       stringPtr("+49 123 456789"),
				MobilePhone: stringPtr("+49 171 1234567"),
			},
			wantErr: false,
		},
		{
			name: "valid with preferred contact method",
			profile: &GuardianProfile{
				FirstName:              "John",
				LastName:               "Doe",
				Email:                  stringPtr("john@example.com"),
				PreferredContactMethod: "email",
			},
			wantErr: false,
		},
		{
			name: "missing first name",
			profile: &GuardianProfile{
				FirstName: "",
				LastName:  "Doe",
				Email:     stringPtr("john@example.com"),
			},
			wantErr: true,
		},
		{
			name: "whitespace only first name",
			profile: &GuardianProfile{
				FirstName: "   ",
				LastName:  "Doe",
				Email:     stringPtr("john@example.com"),
			},
			wantErr: true,
		},
		{
			name: "missing last name",
			profile: &GuardianProfile{
				FirstName: "John",
				LastName:  "",
				Email:     stringPtr("john@example.com"),
			},
			wantErr: true,
		},
		{
			name: "whitespace only last name",
			profile: &GuardianProfile{
				FirstName: "John",
				LastName:  "   ",
				Email:     stringPtr("john@example.com"),
			},
			wantErr: true,
		},
		{
			name: "missing all contact methods",
			profile: &GuardianProfile{
				FirstName: "John",
				LastName:  "Doe",
			},
			wantErr: true,
		},
		{
			name: "empty email string",
			profile: &GuardianProfile{
				FirstName: "John",
				LastName:  "Doe",
				Email:     stringPtr(""),
			},
			wantErr: true,
		},
		{
			name: "whitespace only email",
			profile: &GuardianProfile{
				FirstName: "John",
				LastName:  "Doe",
				Email:     stringPtr("   "),
			},
			wantErr: true,
		},
		{
			name: "invalid email format",
			profile: &GuardianProfile{
				FirstName: "John",
				LastName:  "Doe",
				Email:     stringPtr("not-an-email"),
			},
			wantErr: true,
		},
		{
			name: "invalid preferred contact method",
			profile: &GuardianProfile{
				FirstName:              "John",
				LastName:               "Doe",
				Email:                  stringPtr("john@example.com"),
				PreferredContactMethod: "invalid",
			},
			wantErr: true,
		},
		{
			name: "valid sms preferred contact method",
			profile: &GuardianProfile{
				FirstName:              "John",
				LastName:               "Doe",
				MobilePhone:            stringPtr("+49 171 1234567"),
				PreferredContactMethod: "sms",
			},
			wantErr: false,
		},
		{
			name: "valid mobile preferred contact method",
			profile: &GuardianProfile{
				FirstName:              "John",
				LastName:               "Doe",
				MobilePhone:            stringPtr("+49 171 1234567"),
				PreferredContactMethod: "mobile",
			},
			wantErr: false,
		},
		{
			name: "trim whitespace from names",
			profile: &GuardianProfile{
				FirstName: "  John  ",
				LastName:  "  Doe  ",
				Email:     stringPtr("john@example.com"),
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
	tests := []struct {
		name     string
		profile  *GuardianProfile
		expected string
	}{
		{
			name: "preferred email returns email",
			profile: &GuardianProfile{
				Email:                  stringPtr("john@example.com"),
				Phone:                  stringPtr("+49 123 456789"),
				PreferredContactMethod: "email",
			},
			expected: "john@example.com",
		},
		{
			name: "preferred phone returns phone",
			profile: &GuardianProfile{
				Email:                  stringPtr("john@example.com"),
				Phone:                  stringPtr("+49 123 456789"),
				PreferredContactMethod: "phone",
			},
			expected: "+49 123 456789",
		},
		{
			name: "preferred mobile returns mobile",
			profile: &GuardianProfile{
				Email:                  stringPtr("john@example.com"),
				MobilePhone:            stringPtr("+49 171 1234567"),
				PreferredContactMethod: "mobile",
			},
			expected: "+49 171 1234567",
		},
		{
			name: "preferred sms returns mobile",
			profile: &GuardianProfile{
				Email:                  stringPtr("john@example.com"),
				MobilePhone:            stringPtr("+49 171 1234567"),
				PreferredContactMethod: "sms",
			},
			expected: "+49 171 1234567",
		},
		{
			name: "fallback to mobile when preferred not available",
			profile: &GuardianProfile{
				Email:                  stringPtr("john@example.com"),
				MobilePhone:            stringPtr("+49 171 1234567"),
				Phone:                  stringPtr("+49 123 456789"),
				PreferredContactMethod: "email",
			},
			expected: "john@example.com",
		},
		{
			name: "fallback priority mobile > phone > email",
			profile: &GuardianProfile{
				Email:       stringPtr("john@example.com"),
				Phone:       stringPtr("+49 123 456789"),
				MobilePhone: stringPtr("+49 171 1234567"),
			},
			expected: "+49 171 1234567",
		},
		{
			name: "fallback to phone when no mobile",
			profile: &GuardianProfile{
				Email: stringPtr("john@example.com"),
				Phone: stringPtr("+49 123 456789"),
			},
			expected: "+49 123 456789",
		},
		{
			name: "fallback to email when no phone",
			profile: &GuardianProfile{
				Email: stringPtr("john@example.com"),
			},
			expected: "john@example.com",
		},
		{
			name: "unknown preferred method uses fallback",
			profile: &GuardianProfile{
				MobilePhone:            stringPtr("+49 171 1234567"),
				PreferredContactMethod: "unknown",
			},
			expected: "+49 171 1234567",
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

func TestGuardianProfile_CanInvite(t *testing.T) {
	tests := []struct {
		name     string
		profile  *GuardianProfile
		expected bool
	}{
		{
			name: "can invite - has email and no account",
			profile: &GuardianProfile{
				Email:      stringPtr("john@example.com"),
				HasAccount: false,
			},
			expected: true,
		},
		{
			name: "cannot invite - has account",
			profile: &GuardianProfile{
				Email:      stringPtr("john@example.com"),
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
				Email:      stringPtr(""),
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
			email:    stringPtr("john@example.com"),
			expected: true,
		},
		{
			name:     "no email - nil",
			email:    nil,
			expected: false,
		},
		{
			name:     "no email - empty string",
			email:    stringPtr(""),
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
			Email:     stringPtr("john@example.com"),
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
			Email:     stringPtr("john@example.com"),
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
		Email:     stringPtr("john@example.com"),
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
		Email:     stringPtr("john@example.com"),
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
		Email:     stringPtr("john@example.com"),
	}

	if got := profile.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
