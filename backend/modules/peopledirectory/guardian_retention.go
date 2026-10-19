package peopledirectory

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ChildlessGuardianRetention deletes a school's parent contacts that have
// had no child for longer than the retention window (#4043). The window
// counts from users.guardian_profiles.childless_since in whole Berlin days,
// starting the day after; a contact with an open invitation stays.
type ChildlessGuardianRetention interface {
	DeleteExpiredChildlessGuardians(ctx context.Context, retentionDays int, openInvitations OpenGuardianInvitations) (int, error)
}

// OpenGuardianInvitations reports which of the contacts an open invitation
// still names. Identity & Access owns the invitations; the composition root
// binds this port.
type OpenGuardianInvitations func(ctx context.Context, guardianIDs []int64) (map[int64]bool, error)

// childlessGuardianPurge is the engine's deletion behind
// ChildlessGuardianRetention.
type childlessGuardianPurge interface {
	DeleteChildlessGuardiansBefore(ctx context.Context, cutoff time.Time, openInvitations OpenGuardianInvitations) (int, error)
}

// ErrGuardianRetentionWindow refuses a retention window that is not
// positive: nothing is deleted on a wrong period, and the run is caught up
// once the setting is fixed.
var ErrGuardianRetentionWindow = errors.New("guardian retention window must be positive")

// DeleteExpiredChildlessGuardians deletes the contacts childless since before
// the first Berlin calendar day a window of retentionDays keeps.
func (m *Module) DeleteExpiredChildlessGuardians(ctx context.Context, retentionDays int, openInvitations OpenGuardianInvitations) (int, error) {
	if retentionDays <= 0 {
		return 0, fmt.Errorf("%w, got %d", ErrGuardianRetentionWindow, retentionDays)
	}
	cutoff := calendar.TodayDate().AddDays(-retentionDays).BerlinMidnight()
	return m.engine.DeleteChildlessGuardiansBefore(ctx, cutoff, openInvitations)
}
