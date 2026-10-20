package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (s *Service) DeleteExpiredChildlessGuardians(ctx context.Context, retentionDays int, openInvitations peopledirectory.OpenGuardianInvitations) (int, error) {
	if retentionDays <= 0 {
		return 0, fmt.Errorf("%w, got %d", peopledirectory.ErrGuardianRetentionWindow, retentionDays)
	}
	cutoff := calendar.TodayDate().AddDays(-retentionDays).BerlinMidnight()
	return s.guardianLifecycle.DeleteChildlessGuardiansBefore(ctx, cutoff, openInvitations)
}
