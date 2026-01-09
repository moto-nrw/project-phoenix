package users

import (
	"testing"

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
			name: "valid with phone only",
			profile: &GuardianProfile{
				FirstName: "John",
				LastName:  "Doe",
				Phone:     base.StringPtr("+49 123 456789"),
			},
			wantErr: false,
		},
		{
			name: "valid with mobile phone only",
			profile: &GuardianProfile{
				FirstName:   "John",
				LastName:    "Doe",
				MobilePhone: base.StringPtr("+49 171 1234567"),
			},
			wantErr: false,
		},
		{
			name: "valid with all contact methods",
			profile: &GuardianProfile{
				FirstName:   "John",
				LastName:    "Doe",
				Email:       base.StringPtr("john@example.com"),
				Phone:       base.StringPtr("+49 123 456789"),
				MobilePhone: base.StringPtr("+49 171 1234567"),
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
				Email:     base.StringPtr(""),
			},
			wantErr: true,
		},
		{
			name: "whitespace only email",
			profile: &GuardianProfile{
				FirstName: "John",
				LastName:  "Doe",
				Email:     base.StringPtr("   "),
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
				MobilePhone:            base.StringPtr("+49 171 1234567"),
				PreferredContactMethod: "sms",
			},
			wantErr: false,
		},
		{
			name: "valid mobile preferred contact method",
			profile: &GuardianProfile{
				FirstName:              "John",
				LastName:               "Doe",
				MobilePhone:            base.StringPtr("+49 171 1234567"),
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
	tests := []struct {
		name     string
		profile  *GuardianProfile
		expected string
	}{
		{
			name: "preferred email returns email",
			profile: &GuardianProfile{
				Email:                  base.StringPtr("john@example.com"),
				Phone:                  base.StringPtr("+49 123 456789"),
				PreferredContactMethod: "email",
			},
			expected: "john@example.com",
		},
		{
			name: "preferred phone returns phone",
			profile: &GuardianProfile{
				Email:                  base.StringPtr("john@example.com"),
				Phone:                  base.StringPtr("+49 123 456789"),
				PreferredContactMethod: "phone",
			},
			expected: "+49 123 456789",
		},
		{
			name: "preferred mobile returns mobile",
			profile: &GuardianProfile{
				Email:                  base.StringPtr("john@example.com"),
				MobilePhone:            base.StringPtr("+49 171 1234567"),
				PreferredContactMethod: "mobile",
			},
			expected: "+49 171 1234567",
		},
		{
			name: "preferred sms returns mobile",
			profile: &GuardianProfile{
				Email:                  base.StringPtr("john@example.com"),
				MobilePhone:            base.StringPtr("+49 171 1234567"),
				PreferredContactMethod: "sms",
			},
			expected: "+49 171 1234567",
		},
		{
			name: "fallback to mobile when preferred not available",
			profile: &GuardianProfile{
				Email:                  base.StringPtr("john@example.com"),
				MobilePhone:            base.StringPtr("+49 171 1234567"),
				Phone:                  base.StringPtr("+49 123 456789"),
				PreferredContactMethod: "email",
			},
			expected: "john@example.com",
		},
		{
			name: "fallback priority mobile > phone > email",
			profile: &GuardianProfile{
				Email:       base.StringPtr("john@example.com"),
				Phone:       base.StringPtr("+49 123 456789"),
				MobilePhone: base.StringPtr("+49 171 1234567"),
			},
			expected: "+49 171 1234567",
		},
		{
			name: "fallback to phone when no mobile",
			profile: &GuardianProfile{
				Email: base.StringPtr("john@example.com"),
				Phone: base.StringPtr("+49 123 456789"),
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
			name: "unknown preferred method uses fallback",
			profile: &GuardianProfile{
				MobilePhone:            base.StringPtr("+49 171 1234567"),
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
