package compose_test

import (
	"context"
	"fmt"

	configModels "github.com/moto-ogs/project-phoenix/modules/settings"
)

// This test-only adapter preserves the former root binding's scalar getter,
// shared-lock and error-wrapping behavior for the existing Classday stubs.
// Production composition coverage belongs to the Settings suite.
// settingsReader is the slice of the settings service the slot lists
// read.
type settingsReader interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
	// LockSlotListCutoffPairShared acquires the SHARED advisory lock guarding
	// the Ganztag pickup-cutoff pair so both cutoffs come from one consistent
	// snapshot (#1565 review pass 12).
	LockSlotListCutoffPairShared(ctx context.Context) error
}

// settingsAdapter resolves the slot lists' timetable flag and
// Ganztag pickup cutoffs.
type settingsAdapter struct{ Settings settingsReader }

func (b settingsAdapter) TimetableEnabled(ctx context.Context) (bool, error) {
	return b.Settings.ResolveBool(ctx, configModels.KeyTimetableEnabled)
}

// PickupCutoffs resolves both Ganztag cutoffs under the SHARED variant of the
// advisory lock the settings writer takes exclusively. Without it the two
// reads run as separate statements under READ COMMITTED, and an administrator
// lowering BOTH boundaries through two individually valid writes could commit
// between them and hand the reader an inverted pair (#1565 review pass 12).
func (b settingsAdapter) PickupCutoffs(ctx context.Context) (string, string, error) {
	if err := b.Settings.LockSlotListCutoffPairShared(ctx); err != nil {
		return "", "", err
	}
	short, err := b.Settings.ResolveString(ctx, configModels.KeySlotListShortDayCutoff)
	if err != nil {
		return "", "", fmt.Errorf("resolve short-day pickup cutoff: %w", err)
	}
	long, err := b.Settings.ResolveString(ctx, configModels.KeySlotListLongDayCutoff)
	if err != nil {
		return "", "", fmt.Errorf("resolve long-day pickup cutoff: %w", err)
	}
	return short, long, nil
}
