package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSystemRoomName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Schulhof is system room", SchulhofRoomName, true},
		{"WC is system room", WCRoomName, true},
		{"Toilette alias is system room", WCRoomAliasName, true},
		{"regular room is not system", "Gruppenraum 1", false},
		{"empty string is not system", "", false},
		{"case sensitive - lowercase schulhof", "schulhof", false},
		{"case sensitive - lowercase wc", "wc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsSystemRoomName(tt.input))
		})
	}
}

// TestIsSchulhofRoomName covers the narrower predicate #2405 split out of
// IsSystemRoomName: the Schulhof keeps its rename/delete protection but its
// colour is admin-configurable, so callers that only care about the toilet
// rooms need to tell the two apart.
func TestIsSchulhofRoomName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"canonical Schulhof", SchulhofRoomName, true},
		{"WC is not the Schulhof", WCRoomName, false},
		{"Toilette is not the Schulhof", WCRoomAliasName, false},
		{"regular room is not the Schulhof", "Gruppenraum 1", false},
		{"empty string is not the Schulhof", "", false},
		{"case sensitive - lowercase schulhof", "schulhof", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsSchulhofRoomName(tt.input))
		})
	}
}

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
