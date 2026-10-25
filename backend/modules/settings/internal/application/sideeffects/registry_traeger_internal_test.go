package sideeffects

import (
	"context"
	"testing"

	_ "github.com/moto-nrw/project-phoenix/modules/settings/internal/defaults" // registers the definitions
	settingsRecords "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/stretchr/testify/assert"
)

// TestNewRegistryWithRefusesASideEffectOnATraegerLevelKey: a side effect runs
// in one school's transaction; a Träger value would bypass it (ADR 0086
// section 3).
func TestNewRegistryWithRefusesASideEffectOnATraegerLevelKey(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	definition := settingsRecords.GetDefinition(settingsRecords.KeyParentNewsEnabled)
	is.NotNil(definition)
	is.True(definition.Levels.AtTraeger())
	_, err := NewRegistryWith(func(string) {}, Handler{Key: settingsRecords.KeyParentNewsEnabled, Handle: func(context.Context, int64, any) (func(), error) { return nil, nil }})
	is.Error(err)
}
