package defaults

import config "github.com/moto-nrw/project-phoenix/modules/settings/records"

// Module switches (#4892, decisions in #4812 of 24.10.2026): one per
// additional module, per school. The moto team books them by contract
// (AccessOperatorManaged); school admins see them read-only. Every switch
// defaults to booked so that no existing school loses a function with the
// deployment; the moto team then aligns them with the contracts.
func init() {
	registerDefinitions(moduleDefinitions())
}

func moduleDefinitions() []config.Definition {
	modules := config.Modules()
	definitions := make([]config.Definition, len(modules))
	for i, module := range modules {
		definitions[i] = config.Definition{
			Key:             module.SettingKey,
			Label:           module.Label,
			Description:     module.Summary,
			Type:            config.FieldBoolean,
			Default:         true,
			ReadPermission:  "config:read",
			WritePermission: "config:manage",
			Tab:             "modules",
			Category:        "gebuchte-module",
			SortOrder:       i,
			AccessPolicy:    config.AccessOperatorManaged,
		}
	}
	return definitions
}
