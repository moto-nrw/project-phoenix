package defaults

import config "github.com/moto-nrw/project-phoenix/modules/settings/records"

// registerDefinitions installs ordered groups of owner-owned immutable defaults.
func registerDefinitions(groups ...[]config.Definition) {
	for _, definitions := range groups {
		for _, definition := range definitions {
			config.Register(definition)
		}
	}
}
