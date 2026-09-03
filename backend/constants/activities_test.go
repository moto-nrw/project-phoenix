package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSystemActivityName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Schulhof Freispiel is system activity", SchulhofActivityName, true},
		{"WC is system activity", WCActivityName, true},
		{"regular activity is not system", "Basteln", false},
		{"empty string is not system", "", false},
		{"partial match is not system", "Schulhof", false},
		{"case sensitive - lowercase", "schulhof freispiel", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsSystemActivityName(tt.input))
		})
	}
}
