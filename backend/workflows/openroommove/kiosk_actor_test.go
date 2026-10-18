package openroommove_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/workflows/openroommove"
	"github.com/stretchr/testify/assert"
)

func TestKioskActorSkipsTheResourceChecks(t *testing.T) {
	t.Parallel()
	assert.Equal(t, openroommove.Actor{BypassResourceChecks: true}, openroommove.KioskActor())
}
