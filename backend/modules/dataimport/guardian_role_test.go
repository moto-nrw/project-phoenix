package dataimport

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapGuardianRole(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"":                     "",
		"Hauptsorgeberechtigt": "primary_guardian",
		"sorgeberechtigte":     "legal_guardian",
		"Mitsorgeberechtigt":   "co_guardian",
		"Notfallkontakt":       "emergency_contact",
		"Nur Abholung":         "pickup_only",
		"pickup_only":          "pickup_only",
		"Sozialarbeit":         "social_worker",
	}
	for in, want := range cases {
		got, ok := MapGuardianRole(in)
		assert.True(t, ok, in)
		assert.Equal(t, want, got, in)
	}
	_, ok := MapGuardianRole("Chef")
	assert.False(t, ok)
}
