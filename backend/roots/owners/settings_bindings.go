package owners

import (
	"context"

	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// StaffSickNoteRetentionSettings resolves the staff sick note retention
// window of the school in context.
type StaffSickNoteRetentionSettings struct {
	Settings interface {
		ResolveInt(ctx context.Context, key string) (int, error)
	}
}

func (s StaffSickNoteRetentionSettings) StaffSickNoteRetentionDays(ctx context.Context) (int, error) {
	return s.Settings.ResolveInt(ctx, configModels.KeyGDPRStaffSickNoteRetentionDays)
}
