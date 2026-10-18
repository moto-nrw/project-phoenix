package config

import (
	"context"

	settingsRecords "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// Prefetch resolves keys in one read and attaches the snapshot to ctx.
func (s *settingsService) Prefetch(ctx context.Context, keys []string) (context.Context, error) {
	snapshot, err := s.ResolveMany(ctx, keys)
	if err != nil {
		return ctx, err
	}
	return settingsRecords.WithSettingsSnapshot(ctx, snapshot), nil
}
