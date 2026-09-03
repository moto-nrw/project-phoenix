package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsWCRoomName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"WC is accepted", WCRoomName, true},
		{"Toilette alias is accepted", WCRoomAliasName, true},
		{"lowercase wc is rejected", "wc", false},
		{"lowercase toilette is rejected", "toilette", false},
		{"mixed-case Wc is rejected", "Wc", false},
		{"regular room is rejected", "Gruppenraum 1", false},
		{"empty string is rejected", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsWCRoomName(tt.input))
		})
	}
}
