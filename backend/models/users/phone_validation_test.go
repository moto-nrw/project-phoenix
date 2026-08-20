package users

import (
	"errors"
	"testing"
)

func TestValidateOptionalPhone(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		phone   string
		wantErr bool
	}{
		{"empty is allowed", "", false},
		{"whitespace only is allowed", "   ", false},
		{"international format", "+49 123 456789", false},
		{"dashes", "123-456-7890", false},
		{"simple digits", "1234567890", false},
		{"minimum seven chars", "1234567", false},
		// The exact value from issue #1465: 5 digits is too short and must
		// be rejected at submit/edit so it never blocks approval.
		{"too short - issue 1465", "12345", true},
		{"contains letters", "123-ABC-7890", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateOptionalPhone(tc.phone)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ValidateOptionalPhone(%q) = nil, want error", tc.phone)
				}
				if !errors.Is(err, ErrInvalidPhoneFormat) {
					t.Fatalf("ValidateOptionalPhone(%q) error = %v, want ErrInvalidPhoneFormat", tc.phone, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateOptionalPhone(%q) = %v, want nil", tc.phone, err)
			}
		})
	}
}
