package users

import (
	"errors"
	"testing"
)

func TestValidateOptionalEmail(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"empty is allowed", "", false},
		{"whitespace only is allowed", "   ", false},
		{"standard address", "anna@example.com", false},
		{"dotted local + subdomain", "anna.beispiel@sub.example.de", false},
		{"surrounding whitespace is trimmed", "  anna@example.com  ", false},
		// `+` in local part — sub-addressing, RFC 5322 valid. The
		// enrollment rate-limit test (request_service_test.go) uses
		// `anna+0@example.com` etc to exercise per-IP throttling.
		{"plus subaddressing", "anna+0@example.com", false},
		{"plus subaddressing complex", "user+tag@sub.example.com", false},
		// Regression: net/mail.ParseAddress accepts these, but student
		// creation at enrollment approval rejects them (the domain has no
		// dot), which used to leave enrollment requests permanently stuck.
		// Submit and approval must share this one rule.
		{"no dot in domain - localhost", "test@localhost", true},
		{"no dot in domain - single label", "a@b", true},
		{"missing @", "not-an-email", true},
		{"missing local part", "@example.com", true},
		{"missing domain", "user@", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateOptionalEmail(tc.email)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ValidateOptionalEmail(%q) = nil, want error", tc.email)
				}
				if !errors.Is(err, ErrInvalidEmailFormat) {
					t.Fatalf("ValidateOptionalEmail(%q) error = %v, want ErrInvalidEmailFormat", tc.email, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateOptionalEmail(%q) = %v, want nil", tc.email, err)
			}
		})
	}
}
