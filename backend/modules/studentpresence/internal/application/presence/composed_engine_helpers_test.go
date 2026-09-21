package presence_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
	"github.com/stretchr/testify/require"
)

// composedEngine returns the application service the services factory
// composed behind the public presence facades, so the behaviour tests keep
// driving the service with the retained session rows.
func composedEngine(tb testing.TB, value studentpresence.Presence) presence.Service {
	tb.Helper()
	engine, ok := presenceservice.PresenceEngine(value)
	require.True(tb, ok, "the factory presence must be the composed presence service")
	return engine
}
