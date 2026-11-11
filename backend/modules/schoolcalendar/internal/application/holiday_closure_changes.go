package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// A change of a school closing day proposes a notice to the parents of the
// booked holiday weeks it touches (#6442, #6457, ADR 0196): in the same
// school transaction and under the school's revision lock, so concurrent
// changes of one school merge in order. The proposal sends nothing; only a
// click confirms it. A change that ends before today proposes nothing.

func (s *Service) lockHolidayClosureRevision(ctx context.Context, stats *domain.OperationStats) (int64, error) {
	revision, lockStats, err := s.store.LockHolidayClosureRevision(ctx)
	stats.Add(lockStats)
	return revision, err
}

func (s *Service) recordHolidayClosureChange(ctx context.Context, stats *domain.OperationStats, closingDayID int64, reason string,
	previous, current *domain.HolidayClosureRange, revision int64,
) error {
	change := domain.HolidayClosureChange{
		SourceKind: domain.HolidayClosureSourceSchoolClosingDay, SourceID: closingDayID, Reason: reason,
		Previous: previous, Current: current, Revision: revision,
	}
	if !change.Touches(calendar.Date(s.administration.Today())) {
		return nil
	}
	recordStats, err := s.store.RecordHolidayClosureChange(ctx, change)
	stats.Add(recordStats)
	return err
}
