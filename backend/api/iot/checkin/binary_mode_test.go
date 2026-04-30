package checkin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBinaryModeGreeting(t *testing.T) {
	tests := []struct {
		name   string
		action string
		first  string
		want   string
	}{
		{"check-in greets by first name", "checked_in", "Max", "Willkommen, Max!"},
		{"check-out farewells by first name", "checked_out", "Lena", "Tschüss, Lena!"},
		{"unknown action falls back to neutral message", "no_action", "Max", "Anwesenheit aktualisiert"},
		{"empty action falls back to neutral message", "", "Max", "Anwesenheit aktualisiert"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, binaryModeGreeting(tc.action, tc.first))
		})
	}
}
