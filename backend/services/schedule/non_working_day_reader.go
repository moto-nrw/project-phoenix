package schedule

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/holidays"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// nonWorkingDayResolver unions public holidays (3a) and tenant closing days
// (3b) into one date set. It implements HolidayService so it can replace the
// plain holiday service at the Soll-computation injection points without any
// change to the consuming loops — a day that is both a holiday and a closing
// day collapses into a single boolean key, so it can never be subtracted
// twice.
//
// HolidaysInRange stays a pure holiday passthrough: the /holidays endpoint
// and the Feiertagsarbeit warning must not start reporting closing days.
type nonWorkingDayResolver struct {
	holidays HolidayService
	closing  ClosingDayService
}

// NewNonWorkingDayResolver combines a holiday service and a closing day
// service into a HolidayService whose HolidayDates returns the union of both
// date sets.
func NewNonWorkingDayResolver(h HolidayService, c ClosingDayService) HolidayService {
	return &nonWorkingDayResolver{holidays: h, closing: c}
}

func (r *nonWorkingDayResolver) HolidaysInRange(ctx context.Context, from, to timezone.Date) ([]holidays.Holiday, error) {
	return r.holidays.HolidaysInRange(ctx, from, to)
}

func (r *nonWorkingDayResolver) HolidayDates(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, error) {
	set, err := r.holidays.HolidayDates(ctx, from, to)
	if err != nil {
		return nil, err
	}
	closingSet, err := r.closing.ClosingDayDates(ctx, from, to)
	if err != nil {
		return nil, err
	}
	for d := range closingSet {
		set[d] = true
	}
	return set, nil
}
