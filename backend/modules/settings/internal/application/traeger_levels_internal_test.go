package config

import (
	"testing"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateTraegerLevelsRefusesSchoolBoundKeys: a key whose write takes a
// school lock or a check over several fields cannot get a Träger level (ADR
// 0086 section 3); the registry of the process passes.
func TestValidateTraegerLevelsRefusesSchoolBoundKeys(t *testing.T) {
	t.Parallel()
	require.NoError(t, ValidateTraegerLevels(config.DefaultRegistry()))
	for _, key := range []string{config.KeyMFAMode, config.KeyParentPickupChangeEnabled, config.KeyParentSickNoteEnabled, config.KeySlotListShortDayCutoff} {
		registry := config.NewRegistry()
		registry.Register(config.Definition{Key: key, Type: config.FieldText, Default: "", Tab: "operations", Category: "test", Levels: config.LevelBoth})
		assert.Error(t, ValidateTraegerLevels(registry), key)
	}
}

// TestTraegerLevelRequiresASharedNonPasswordDefinition pins the checks the
// definition itself shows.
func TestTraegerLevelRequiresASharedNonPasswordDefinition(t *testing.T) {
	t.Parallel()
	for name, definition := range map[string]config.Definition{
		"operator only":    {Key: "a.b", Type: config.FieldText, Tab: "x", Category: "y", Levels: config.LevelBoth, AccessPolicy: config.AccessOperatorOnly},
		"operator managed": {Key: "a.b", Type: config.FieldText, Tab: "x", Category: "y", Levels: config.LevelTraeger, AccessPolicy: config.AccessOperatorManaged},
		"password":         {Key: "a.b", Type: config.FieldPassword, Tab: "x", Category: "y", Levels: config.LevelBoth, AccessPolicy: config.AccessShared},
		"unknown level":    {Key: "a.b", Type: config.FieldText, Tab: "x", Category: "y", Levels: "land", AccessPolicy: config.AccessShared},
		"right at school":  {Key: "a.b", Type: config.FieldText, Tab: "x", Category: "y", AccessPolicy: config.AccessShared, TraegerWriteRight: config.TraegerRightStaff},
	} {
		assert.Error(t, definition.Validate(), name)
	}
	admin := config.Definition{Key: "a.b", Type: config.FieldText, Tab: "x", Category: "y", Levels: config.LevelBoth, AccessPolicy: config.AccessAdminOnly}
	assert.NoError(t, admin.Validate())
}
