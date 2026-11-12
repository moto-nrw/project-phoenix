package compose

import (
	"github.com/moto-ogs/project-phoenix/modules/settings"
	"github.com/moto-ogs/project-phoenix/modules/settings/records"
)

// NewSlotListSettings binds the lists to the Settings-owned read protocol.
func NewSlotListSettings(source records.BatchSettingsService) settings.SlotListSettings {
	if source == nil {
		panic("slot list settings: source is required")
	}
	return source
}
