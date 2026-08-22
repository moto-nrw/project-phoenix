package users

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGermanGuardianValidationMessage verifies every English model-validation
// message used by the student-create guardian flow is translated to German for
// the user-facing 400 response, and that unknown messages fall through verbatim
// rather than being masked.
func TestGermanGuardianValidationMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in       error
		expected string
	}{
		{
			name:     "invalid email",
			in:       errors.New("invalid email format"),
			expected: "ungültiges E-Mail-Format",
		},
		{
			name:     "invalid contact method",
			in:       errors.New("invalid preferred contact method"),
			expected: "ungültige bevorzugte Kontaktmethode",
		},
		{
			name:     "phone required",
			in:       errors.New("phone number is required"),
			expected: "Telefonnummer ist erforderlich",
		},
		{
			name:     "invalid phone format",
			in:       errors.New("invalid phone number format"),
			expected: "ungültiges Telefonnummer-Format",
		},
		{
			name:     "phone too few digits",
			in:       errors.New("phone number must contain at least 3 digits"),
			expected: "Telefonnummer muss mindestens 3 Ziffern enthalten",
		},
		{
			name:     "unknown message falls through verbatim",
			in:       errors.New("some unexpected validation failure"),
			expected: "some unexpected validation failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, germanGuardianValidationMessage(tt.in))
		})
	}
}
