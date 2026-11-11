package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// HolidayGuestGroups lists the Feriengruppen of the school whose week ends
// today or later (ADR 0120, Nachtrag 2026-11-11). A group whose weeks have
// all ended is no Feriengruppe for the lists any more.
func (l HolidayGuestBadgeList) HolidayGuestGroups(ctx context.Context, schoolID int64) ([]enrollment.HolidayGuestGroup, error) {
	if schoolID <= 0 {
		return nil, errors.New("enrollment: the hosting school is required")
	}
	groups, err := l.store.ListGuestGroups(ctx, schoolID, l.today())
	if err != nil {
		return nil, err
	}
	if groups == nil {
		groups = []enrollment.HolidayGuestGroup{}
	}
	return groups, nil
}
