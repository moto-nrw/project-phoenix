package application

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/timetable/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// cancelledBlock names a block a cancellation freezes: its id and date.
type cancelledBlock struct {
	id   int64
	date calendar.Date
}

// lockCancelledBlockStaff takes the staff rows of the people of every future
// block the caller cancels, in one ascending batch after the day locks and
// before the attendance and block rows (#6702). The cancellation's block
// guard re-locks exactly these people (held, skipped): it locks the staff of
// a block that stays operational, and a cancelled block stays operational
// only on a later day. A bulk cancellation therefore never holds one
// block's rows while it waits for the next block's people.
func (s *InstanceLifecycleService) lockCancelledBlockStaff(ctx context.Context, blocks []cancelledBlock, op string) error {
	today := calendar.TodayDate()
	ids := make([]int64, 0, len(blocks))
	for _, block := range blocks {
		if block.date.After(today) {
			ids = append(ids, block.id)
		}
	}
	rows, err := staffOfInstances(ctx, s.deps.InstanceStaffRepo, ids)
	if err != nil {
		return &ports.ScheduleError{Op: op + ": load staff", Err: err}
	}
	staff := make([]int64, 0, len(rows))
	for _, row := range rows {
		staff = append(staff, row.StaffID)
	}
	if err := lockStaffRows(ctx, s.deps.InstanceStaffRepo, staff); err != nil {
		return &ports.ScheduleError{Op: op + ": lock staff", Err: err}
	}
	return nil
}
