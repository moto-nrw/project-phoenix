package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The parents portal accepts a new contact with its signed pickup permission
// in at most 8 KB (#4780); the drawn signature must leave room for the rest.
func TestDemoSignatureFitsTheContactRequestLimit(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"Sabine Schneider", "Klaus-Dieter Menzel", "Constantin Gutiérrez"} {
		assert.Less(t, len(demoSignaturePNG(name)), 6<<10, name)
	}
}
