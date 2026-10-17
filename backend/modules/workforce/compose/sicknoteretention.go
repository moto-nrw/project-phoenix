package compose

import (
	"context"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application"
)

// SickNoteRetentionResolver resolves the retention windows of the school in
// context.
type SickNoteRetentionResolver interface {
	ResolveInt(ctx context.Context, key string) (int, error)
}

// NewSickNoteRetention composes the sick note retention of the nightly
// cleanup (#3961). It runs on the tenant transaction of the school in
// context and resolves the school's window itself.
func NewSickNoteRetention(settings SickNoteRetentionResolver) workforce.SickNoteRetention {
	if settings == nil {
		panic("workforce compose: sick note retention requires settings")
	}
	return application.NewSickNoteRetention(
		postgres.New(retentionDatabase), sickNoteRetentionWindow{settings: settings})
}

type sickNoteRetentionWindow struct{ settings SickNoteRetentionResolver }

func (w sickNoteRetentionWindow) StaffSickNoteRetentionDays(ctx context.Context) (int, error) {
	return w.settings.ResolveInt(ctx, configModels.KeyGDPRStaffSickNoteRetentionDays)
}
