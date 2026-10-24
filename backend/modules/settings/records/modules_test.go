package records

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The module list holds the twelve additional modules of #4812; Basis has
// no switch.
func TestModuleListHoldsTheTwelveAdditionalModules(t *testing.T) {
	t.Parallel()
	modules := Modules()
	require.Len(t, modules, 12)

	keys := map[ModuleKey]bool{}
	for _, module := range modules {
		assert.False(t, keys[module.Key], "module %s is listed once", module.Key)
		keys[module.Key] = true
		assert.Equal(t, "module."+string(module.Key), module.SettingKey)
		assert.NotEmpty(t, module.Label, module.Key)
		assert.NotEmpty(t, module.Summary, module.Key)
		assert.NotEmpty(t, module.CatalogID, module.Key)
		for _, required := range module.Requires {
			_, ok := LookupModule(required)
			assert.True(t, ok, "%s requires a listed module", module.Key)
			assert.NotEqual(t, module.Key, required, "%s does not require itself", module.Key)
		}
	}
	assert.False(t, keys["basis"], "Basis is always included and has no switch")
}

// Prerequisites decided in #4812 (24.10.2026): Ferienbetreuung needs
// Online-Anmeldung, Elternbeiträge needs Verträge; every other module needs
// only Basis.
func TestModulePrerequisites(t *testing.T) {
	t.Parallel()
	for _, module := range Modules() {
		switch module.Key {
		case ModuleFerienbetreuung:
			assert.Equal(t, []ModuleKey{ModuleOnlineAnmeldung}, module.Requires)
		case ModuleElternbeitraege:
			assert.Equal(t, []ModuleKey{ModuleVertraege}, module.Requires)
		default:
			assert.Empty(t, module.Requires, module.Key)
		}
	}
	assert.Equal(t, []ModuleKey{ModuleFerienbetreuung}, moduleKeys(ModuleDependents(ModuleOnlineAnmeldung)))
	assert.Equal(t, []ModuleKey{ModuleElternbeitraege}, moduleKeys(ModuleDependents(ModuleVertraege)))
	assert.Empty(t, ModuleDependents(ModuleZeiterfassung))
}

func TestModulesReturnsCopies(t *testing.T) {
	t.Parallel()
	modules := Modules()
	for i := range modules {
		if len(modules[i].Requires) > 0 {
			modules[i].Requires[0] = ModuleZeiterfassung
		}
	}
	ferien, ok := LookupModule(ModuleFerienbetreuung)
	require.True(t, ok)
	assert.Equal(t, []ModuleKey{ModuleOnlineAnmeldung}, ferien.Requires)
}

func TestModuleForSettingKey(t *testing.T) {
	t.Parallel()
	module, ok := ModuleForSettingKey(KeyModuleNFCAnwesenheit)
	require.True(t, ok)
	assert.Equal(t, ModuleNFCAnwesenheit, module.Key)
	_, ok = ModuleForSettingKey("attendance.nfc_enabled")
	assert.False(t, ok)
}

func TestModuleDependencyErrorIsAnInvalidValue(t *testing.T) {
	t.Parallel()
	ferien, _ := LookupModule(ModuleFerienbetreuung)
	online, _ := LookupModule(ModuleOnlineAnmeldung)
	booking := &ModuleDependencyError{Key: ferien.SettingKey, Module: ferien, Booking: true, Conflicts: []ModuleDefinition{online}}
	assert.True(t, errors.Is(booking, ErrInvalidValue))
	assert.Contains(t, booking.Error(), "requires online_anmeldung")
	assert.Equal(t, "Online-Anmeldung", booking.ConflictLabels())

	cancel := &ModuleDependencyError{Key: online.SettingKey, Module: online, Conflicts: []ModuleDefinition{ferien}}
	assert.Contains(t, cancel.Error(), "is required by ferienbetreuung")
}

func moduleKeys(modules []ModuleDefinition) []ModuleKey {
	keys := make([]ModuleKey, len(modules))
	for i, module := range modules {
		keys[i] = module.Key
	}
	return keys
}
