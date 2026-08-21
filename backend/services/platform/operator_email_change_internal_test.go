package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "NormalEmail",
			input:    "operator@example.com",
			expected: "o***@example.com",
		},
		{
			name:     "ShortLocalPart_TwoChars",
			input:    "ab@example.com",
			expected: "***@example.com",
		},
		{
			name:     "ShortLocalPart_OneChar",
			input:    "a@example.com",
			expected: "***@example.com",
		},
		{
			name:     "EmptyString",
			input:    "",
			expected: "***",
		},
		{
			name:     "NoAtSign",
			input:    "invalid-email",
			expected: "***",
		},
		{
			name:     "EmptyLocalPart",
			input:    "@example.com",
			expected: "***",
		},
		{
			name:     "LongLocalPart",
			input:    "verylonglocalpart@domain.org",
			expected: "v***@domain.org",
		},
		{
			name:     "ThreeCharLocalPart",
			input:    "abc@domain.org",
			expected: "a***@domain.org",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := maskEmail(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
