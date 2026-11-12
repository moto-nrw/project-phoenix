package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

var _ schoolcalendar.HolidayClosureWatchdog = (*HolidayClosureEventStore)(nil)

// OverdueHolidayClosureEvents is an administrative, read-only aggregate over
// confirmed commands. It also sees blocked work absent from worker discovery,
// including commands whose school has moved out of the original Träger.
func (s *HolidayClosureEventStore) OverdueHolidayClosureEvents(ctx context.Context, overdueFor time.Duration) (int, error) {
	raw, ok := tenant.TransactionFromContext(ctx)
	if !ok || !tenant.IsAdminTx(ctx) {
		return 0, errors.New("school calendar postgres: watchdog requires an administrative transaction")
	}
	db, ok := raw.(bun.Tx)
	if !ok {
		return 0, fmt.Errorf("school calendar postgres: unsupported transaction %T", raw)
	}
	if overdueFor <= 0 {
		return 0, errors.New("school calendar postgres: watchdog threshold must be positive")
	}
	var count int
	err := db.NewRaw(`SELECT count(*) FROM calendar.holiday_closure_events
  WHERE state = 'blocked'
   OR (state = 'pending' AND next_attempt_at < CURRENT_TIMESTAMP - ?::interval)
   OR (state = 'leased' AND lease_until < CURRENT_TIMESTAMP - ?::interval)
   OR (state IN ('prepared', 'unknown') AND processed_at < CURRENT_TIMESTAMP - ?::interval)`,
		overdueFor.String(), overdueFor.String(), overdueFor.String()).Scan(ctx, &count)
	if err != nil {
		return 0, fmt.Errorf("school calendar postgres: inspect overdue holiday closure events: %w", err)
	}
	return count, nil
}
