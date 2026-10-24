package users

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/ptrtest"
)

func TestGuardianProfile_Validate(t *testing.T) {
	t.Parallel()

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
				Email:     ptrtest.Ptr("john@example.com"),
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
				Email:                  ptrtest.Ptr("john@example.com"),
				PreferredContactMethod: "email",
			},
			wantErr: false,
		},
		{
			name: "empty first name is allowed",
			profile: &GuardianProfile{
				FirstName: "",
				LastName:  "Doe",
				Email:     ptrtest.Ptr("john@example.com"),
			},
			wantErr: false,
		},
		{
			name: "whitespace only first name is allowed",
			profile: &GuardianProfile{
				FirstName: "   ",
				LastName:  "Doe",
				Email:     ptrtest.Ptr("john@example.com"),
			},
			wantErr: false,
		},
		{
			name: "empty last name is allowed",
			profile: &GuardianProfile{
				FirstName: "John",
				LastName:  "",
				Email:     ptrtest.Ptr("john@example.com"),
			},
			wantErr: false,
		},
		{
			name: "whitespace only last name is allowed",
			profile: &GuardianProfile{
				FirstName: "John",
				LastName:  "   ",
				Email:     ptrtest.Ptr("john@example.com"),
			},
			wantErr: false,
		},
		{
			name: "both names empty is allowed",
			profile: &GuardianProfile{
				Email: ptrtest.Ptr("john@example.com"),
			},
			wantErr: false,
		},
		{
			name: "invalid email format",
			profile: &GuardianProfile{
				FirstName: "John",
				LastName:  "Doe",
				Email:     ptrtest.Ptr("not-an-email"),
			},
			wantErr: true,
		},
		{
			name: "invalid preferred contact method",
			profile: &GuardianProfile{
				FirstName:              "John",
				LastName:               "Doe",
				Email:                  ptrtest.Ptr("john@example.com"),
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
				Email:     ptrtest.Ptr("john@example.com"),
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
