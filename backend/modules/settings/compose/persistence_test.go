package compose

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

type stubSettingsRuntime struct{}

func (stubSettingsRuntime) DB(context.Context) (bun.IDB, error) { return nil, nil }

// The Settings composition builds the setting stores the repository factory
// used to hold (ADR 0049).
func TestSettingsCompositionBuildsItsRepositories(t *testing.T) {
	t.Parallel()

	runtime := stubSettingsRuntime{}
	assert.NotNil(t, NewSettingValueRepository(runtime))
	assert.NotNil(t, NewSettingAuditRepository(runtime))
	assert.NotNil(t, NewHomeLayoutRepository(runtime))
}
