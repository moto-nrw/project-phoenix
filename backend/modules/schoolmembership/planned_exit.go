package schoolmembership

import (
	"context"
	"errors"
	"time"
)

// ErrPlannedExitNotFound reports a staff member without a planned exit, or a
// planned exit that moved since the caller read it.
var ErrPlannedExitNotFound = errors.New("staff member has no planned exit")

// PlannedExit is the last working day the Leitung entered for a live staff
// member (#4809). After the end of that day (Berlin) the staff offboarding
// workflow retires the member in the planner's name; the retirement clears
// the planned exit. LastWorkingDay is a calendar date in DateLayout. The
// *SentAt, AccessLockedAt and NextAttemptAt fields are the scheduler's
// progress: a notice is sent once, a blocked offboarding is retried at
// NextAttemptAt, and AccessLockedAt marks that sign-in was closed although
// the offboarding itself was still blocked.
type PlannedExit struct {
	StaffID             int64
	TenantID            int64
	LastWorkingDay      string
	PlannedByAccountID  int64
	PlannedByStaffID    int64
	PlannedByName       string
	PlannedAt           time.Time
	WeekNoticeSentAt    *time.Time
	DayNoticeSentAt     *time.Time
	BlockedNoticeSentAt *time.Time
	AccessLockedAt      *time.Time
	NextAttemptAt       *time.Time
}

// PlanExit enters or moves the last working day of a live staff member. A
// moved day starts the reminders and attempts over.
type PlanExit struct {
	StaffID            int64
	LastWorkingDay     string
	PlannedByAccountID int64
	PlannedByStaffID   int64
	PlannedByName      string
}

// PlannedExitProgress records what the scheduler did for the planned exit
// on LastWorkingDay; a planned exit moved meanwhile is left alone. A nil
// field leaves the stored value unchanged; NextAttemptAt is always written.
type PlannedExitProgress struct {
	StaffID             int64
	LastWorkingDay      string
	WeekNoticeSentAt    *time.Time
	DayNoticeSentAt     *time.Time
	BlockedNoticeSentAt *time.Time
	AccessLockedAt      *time.Time
	NextAttemptAt       *time.Time
}

// PlannedExits reads and writes the planned exits of the school in context;
// every call joins the caller's transaction. The staff offboarding workflow
// decides who may plan which day.
type PlannedExits struct {
	findForMutation func(context.Context, int64) (PlannedExit, error)
	list            func(context.Context) ([]PlannedExit, error)
	save            func(context.Context, PlanExit) (PlannedExit, error)
	remove          func(context.Context, int64) error
	record          func(context.Context, PlannedExitProgress) error
}

// PlannedExitStore binds the queries and commands behind PlannedExits.
type PlannedExitStore struct {
	FindForMutation func(context.Context, int64) (PlannedExit, error)
	List            func(context.Context) ([]PlannedExit, error)
	Save            func(context.Context, PlanExit) (PlannedExit, error)
	Delete          func(context.Context, int64) error
	Record          func(context.Context, PlannedExitProgress) error
}

func NewPlannedExits(store PlannedExitStore) *PlannedExits {
	if store.FindForMutation == nil || store.List == nil || store.Save == nil || store.Delete == nil || store.Record == nil {
		panic("school membership: planned exit queries and commands are required")
	}
	return &PlannedExits{findForMutation: store.FindForMutation, list: store.List, save: store.Save, remove: store.Delete, record: store.Record}
}

// FindPlannedExitForMutation locks the live member and returns its planned
// exit, or ErrPlannedExitNotFound.
func (p *PlannedExits) FindPlannedExitForMutation(ctx context.Context, staffID int64) (PlannedExit, error) {
	if staffID <= 0 {
		return PlannedExit{}, invalid("staff ID is required")
	}
	return p.findForMutation(ctx, staffID)
}

// ListPlannedExits returns the planned exits of the live members.
func (p *PlannedExits) ListPlannedExits(ctx context.Context) ([]PlannedExit, error) {
	return p.list(ctx)
}

// SavePlannedExit enters or moves a planned exit; ErrStaffNotFound when the
// member is not live.
func (p *PlannedExits) SavePlannedExit(ctx context.Context, plan PlanExit) (PlannedExit, error) {
	switch {
	case plan.StaffID <= 0:
		return PlannedExit{}, invalid("staff ID is required")
	case plan.PlannedByAccountID <= 0 || plan.PlannedByStaffID <= 0:
		return PlannedExit{}, invalid("planner is required")
	}
	if _, err := time.Parse(DateLayout, plan.LastWorkingDay); err != nil {
		return PlannedExit{}, invalid("last working day must be a date (YYYY-MM-DD)")
	}
	return p.save(ctx, plan)
}

// DeletePlannedExit withdraws a planned exit; ErrPlannedExitNotFound when
// there is none.
func (p *PlannedExits) DeletePlannedExit(ctx context.Context, staffID int64) error {
	if staffID <= 0 {
		return invalid("staff ID is required")
	}
	return p.remove(ctx, staffID)
}

// RecordPlannedExitProgress stores the scheduler's progress;
// ErrPlannedExitNotFound when the exit is gone or moved.
func (p *PlannedExits) RecordPlannedExitProgress(ctx context.Context, progress PlannedExitProgress) error {
	if progress.StaffID <= 0 || progress.LastWorkingDay == "" {
		return invalid("staff ID and last working day are required")
	}
	return p.record(ctx, progress)
}
