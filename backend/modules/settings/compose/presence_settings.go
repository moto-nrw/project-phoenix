package compose

import config "github.com/moto-ogs/project-phoenix/modules/settings/internal/application"

// PresenceSettings binds the owner's named settings reads.
func PresenceSettings(source config.PresenceSettingsSource) config.PresenceSettings {
	return config.NewPresenceSettings(source)
}
