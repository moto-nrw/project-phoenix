package base

import (
	"testing"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  bool
	}{
		{
			name:  "valid email",
			email: "user@example.com",
			want:  true,
		},
		{
			name:  "valid email with subdomain",
			email: "user@mail.example.com",
			want:  true,
		},
		{
			name:  "valid email with dots",
			email: "user.name@example.com",
			want:  true,
		},
		{
			name:  "email with plus (not currently supported)",
			email: "user+tag@example.com",
			want:  false, // Current regex doesn't support + in local part
		},
		{
			name:  "valid email with hyphen",
			email: "user-name@example.com",
			want:  true,
		},
		{
			name:  "valid email with underscore",
			email: "user_name@example.com",
			want:  true,
		},
		{
			name:  "valid email with percent",
			email: "user%name@example.com",
			want:  true,
		},
		{
			name:  "invalid email - no @",
			email: "userexample.com",
			want:  false,
		},
		{
			name:  "invalid email - no domain",
			email: "user@",
			want:  false,
		},
		{
			name:  "invalid email - no TLD",
			email: "user@example",
			want:  false,
		},
		{
			name:  "invalid email - spaces",
			email: "user name@example.com",
			want:  false,
		},
		{
			name:  "empty email",
			email: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateEmail(tt.email); got != tt.want {
				t.Errorf("ValidateEmail(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

func TestValidatePhone(t *testing.T) {
	tests := []struct {
		name  string
		phone string
		want  bool
	}{
		{
			name:  "valid international format",
			phone: "+49 123456789",
			want:  true,
		},
		{
			name:  "valid international format without space",
			phone: "+491234567890",
			want:  true,
		},
		{
			name:  "valid US format",
			phone: "+1 555-123-4567",
			want:  true,
		},
		{
			name:  "valid local format",
			phone: "555 123 4567",
			want:  true,
		},
		{
			name:  "valid local format with dashes",
			phone: "555-123-4567",
			want:  true,
		},
		{
			name:  "valid local format no spaces",
			phone: "5551234567",
			want:  true,
		},
		{
			name:  "valid short phone",
			phone: "1234567",
			want:  true,
		},
		{
			name:  "invalid - too short",
			phone: "123456",
			want:  false,
		},
		{
			name:  "invalid - too long",
			phone: "1234567890123456",
			want:  false,
		},
		{
			name:  "invalid - letters",
			phone: "abc-def-ghij",
			want:  false,
		},
		{
			name:  "empty phone",
			phone: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidatePhone(tt.phone); got != tt.want {
				t.Errorf("ValidatePhone(%q) = %v, want %v", tt.phone, got, tt.want)
			}
		})
	}
}

func TestEmailRegex(t *testing.T) {
	if EmailRegex == nil {
		t.Error("EmailRegex should not be nil")
	}
}

func TestPhoneRegex(t *testing.T) {
	if PhoneRegex == nil {
		t.Error("PhoneRegex should not be nil")
	}
}
