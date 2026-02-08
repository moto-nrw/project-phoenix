package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParseAllowedOrigins tests the parseAllowedOrigins function
func TestParseAllowedOrigins(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected []string
	}{
		{
			name:     "empty env var returns wildcard",
			envValue: "",
			expected: []string{"*"},
		},
		{
			name:     "single origin",
			envValue: "http://localhost:3000",
			expected: []string{"http://localhost:3000"},
		},
		{
			name:     "multiple origins with spaces",
			envValue: "http://localhost:3000, https://example.com, https://app.example.com",
			expected: []string{"http://localhost:3000", "https://example.com", "https://app.example.com"},
		},
		{
			name:     "multiple origins without spaces",
			envValue: "http://localhost:3000,https://example.com,https://app.example.com",
			expected: []string{"http://localhost:3000", "https://example.com", "https://app.example.com"},
		},
		{
			name:     "origins with excessive whitespace",
			envValue: "  http://localhost:3000  ,  https://example.com  ",
			expected: []string{"http://localhost:3000", "https://example.com"},
		},
		{
			name:     "wildcard explicitly set",
			envValue: "*",
			expected: []string{"*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable for this test
			t.Setenv("CORS_ALLOWED_ORIGINS", tt.envValue)

			// Call function under test
			result := parseAllowedOrigins()

			// Assert result
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestParsePositiveInt tests the parsePositiveInt function
func TestParsePositiveInt(t *testing.T) {
	tests := []struct {
		name         string
		envVar       string
		envValue     string
		defaultValue int
		expected     int
	}{
		{
			name:         "empty env var returns default",
			envVar:       "TEST_INT_EMPTY",
			envValue:     "",
			defaultValue: 60,
			expected:     60,
		},
		{
			name:         "valid positive int",
			envVar:       "TEST_INT_VALID",
			envValue:     "100",
			defaultValue: 60,
			expected:     100,
		},
		{
			name:         "zero returns default",
			envVar:       "TEST_INT_ZERO",
			envValue:     "0",
			defaultValue: 60,
			expected:     60,
		},
		{
			name:         "negative returns default",
			envVar:       "TEST_INT_NEGATIVE",
			envValue:     "-5",
			defaultValue: 60,
			expected:     60,
		},
		{
			name:         "non-numeric string returns default",
			envVar:       "TEST_INT_INVALID",
			envValue:     "invalid",
			defaultValue: 60,
			expected:     60,
		},
		{
			name:         "float string returns default",
			envVar:       "TEST_INT_FLOAT",
			envValue:     "12.5",
			defaultValue: 60,
			expected:     60,
		},
		{
			name:         "very large positive int",
			envVar:       "TEST_INT_LARGE",
			envValue:     "999999",
			defaultValue: 60,
			expected:     999999,
		},
		{
			name:         "whitespace returns default",
			envVar:       "TEST_INT_WHITESPACE",
			envValue:     "  ",
			defaultValue: 60,
			expected:     60,
		},
		{
			name:         "number with leading/trailing spaces returns default",
			envVar:       "TEST_INT_SPACES",
			envValue:     " 100 ",
			defaultValue: 60,
			expected:     60, // strconv.Atoi doesn't trim, so this fails
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable for this test
			if tt.envValue != "" {
				t.Setenv(tt.envVar, tt.envValue)
			}

			// Call function under test
			result := parsePositiveInt(tt.envVar, tt.defaultValue)

			// Assert result
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestParsePositiveInt_DifferentDefaults tests parsePositiveInt with various default values
func TestParsePositiveInt_DifferentDefaults(t *testing.T) {
	tests := []struct {
		name         string
		defaultValue int
	}{
		{"default 1", 1},
		{"default 10", 10},
		{"default 60", 60},
		{"default 100", 100},
		{"default 1000", 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Don't set any env var, so it uses default
			result := parsePositiveInt("NONEXISTENT_ENV_VAR", tt.defaultValue)
			assert.Equal(t, tt.defaultValue, result)
		})
	}
}
