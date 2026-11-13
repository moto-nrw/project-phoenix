package application

import (
	"context"
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// HolidayGuestGroups lists the Feriengruppen of the school whose week ends
// today or later by the calendar day of the service clock (ADR 0120,
// Nachtrag 2026-11-11). A group whose weeks have all ended is no
// Feriengruppe for the lists any more: from the day after its last week it
// is an ordinary group again.
func (l HolidayGuestBadgeList) HolidayGuestGroups(ctx context.Context, schoolID int64) ([]enrollment.HolidayGuestGroup, error) {
	if schoolID <= 0 {
		return nil, errors.New("enrollment: the hosting school is required")
	}
	all, err := l.store.ListGuestGroups(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	today := l.today()
	current := make([]enrollment.HolidayGuestGroup, 0, len(all))
	for _, group := range all {
		if !group.EndsOn.Before(today) {
			current = append(current, group)
		}
	}
	return current, nil
}
