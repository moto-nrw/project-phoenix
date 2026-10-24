package settings

import (
	"fmt"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// ModuleKey identifies one additional module of the module catalog (#4812).
// moto Basis is always included and has no key.
type ModuleKey = configModel.ModuleKey

// ModuleDefinition is one entry of the central module list.
type ModuleDefinition = configModel.ModuleDefinition

// The additional modules a school can book.
const (
	ModuleNFCAnwesenheit      = configModel.ModuleNFCAnwesenheit
	ModuleMittagessen         = configModel.ModuleMittagessen
	ModuleElternApp           = configModel.ModuleElternApp
	ModuleOnlineAnmeldung     = configModel.ModuleOnlineAnmeldung
	ModuleUnterrichtVormittag = configModel.ModuleUnterrichtVormittag
	ModuleDienstplan          = configModel.ModuleDienstplan
	ModuleZeiterfassung       = configModel.ModuleZeiterfassung
	ModulePersonalakte        = configModel.ModulePersonalakte
	ModuleTraegerUebersicht   = configModel.ModuleTraegerUebersicht
	ModuleFerienbetreuung     = configModel.ModuleFerienbetreuung
	ModuleVertraege           = configModel.ModuleVertraege
	ModuleElternbeitraege     = configModel.ModuleElternbeitraege
)

// ModuleSettingKeys lists the setting key of every module switch, for a
// caller that batch-resolves them (PrefetchSettings, ResolveManyForTenant).
func ModuleSettingKeys() []string {
	modules := configModel.Modules()
	keys := make([]string, len(modules))
	for i, module := range modules {
		keys[i] = module.SettingKey
	}
	return keys
}

// ModuleDependencyError rejects a module switch that breaks a prerequisite.
type ModuleDependencyError = configModel.ModuleDependencyError

// BookedModules lists the booked modules in catalog order from one batch
// of resolved settings, for a reader that already holds a snapshot.
func BookedModules(readBool func(key string) (bool, error)) ([]ModuleKey, error) {
	booked := []ModuleKey{}
	for _, module := range configModel.Modules() {
		ok, err := readBool(module.SettingKey)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", module.SettingKey, err)
		}
		if ok {
			booked = append(booked, module.Key)
		}
	}
	return booked, nil
}
