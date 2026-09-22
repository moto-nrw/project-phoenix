package timetable

import (
	"context"
	"errors"
	"regexp"
	"slices"
)

// Pickup extensions (#3261) are the mirror of the automatic partial absence
// for earlier pickups (#2360): a child now stays longer than before, and the
// Leitung decides which Betreuungsblock covers the extra time. moto never
// adds the child on its own; it keeps an open task until someone decides.
//
// A day task belongs to one date. A weekday task belongs to a lasting weekday
// change and its choice applies to every future block of that weekday.
const (
	PickupExtensionKindDay     = "day"
	PickupExtensionKindWeekday = "weekday"
)

var (
	ErrInvalidPickupExtension  = errors.New("invalid pickup extension")
	ErrPickupExtensionNotFound = errors.New("pickup extension task not found")
	// ErrPickupExtensionBlockGone rejects a chosen block that is no longer a
	// candidate: cancelled, moved, or the child was added meanwhile.
	ErrPickupExtensionBlockGone = errors.New("pickup extension block is no longer available")
)

var pickupClockPattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// PickupDayExtension records that the day pickup exception moves the pickup
// later than the weekly time. Times are wall-clock "HH:MM".
type PickupDayExtension struct {
	StudentID         int64
	PickupExceptionID int64
	Date              string
	PreviousPickup    string
	Pickup            string
}

// PickupWeekdayExtension records that a lasting weekday pickup moved later.
type PickupWeekdayExtension struct {
	StudentID      int64
	Weekday        int
	EffectiveFrom  string
	PreviousPickup string
	Pickup         string
}

// PickupExtensionBlock is one block the child can be added to. For a day task
// ID is the activity instance; for a weekday task it is the template.
type PickupExtensionBlock struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

// PickupExtensionTask is an open decision. Blocks lists the current choices;
// a task without choices is not open and is never returned.
type PickupExtensionTask struct {
	ID             int64
	StudentID      int64
	Kind           string
	Date           string
	Weekday        int
	EffectiveFrom  string
	PreviousPickup string
	Pickup         string
	Blocks         []PickupExtensionBlock
}

// PickupExtensionResolution reports what a decision changed.
type PickupExtensionResolution struct {
	StudentID      int64
	Kind           string
	AssignedBlocks []PickupExtensionBlock
	// InstanceIDs are the concrete blocks that got the child, including the
	// already planned future blocks of a weekday task.
	InstanceIDs []int64
	// Instances pairs those blocks with their dates. The child joins them as
	// expected; Student Presence re-applies the reported day statuses and
	// partial absences of each date, which the caller asks for.
	Instances []PickupExtensionInstance
}

// PickupExtensionInstance is one concrete block a decision added the child to.
type PickupExtensionInstance struct {
	ID   int64
	Date string
}

type PickupExtensionRecorder interface {
	// RecordPickupDayExtension needs a pickup later than the previous time.
	RecordPickupDayExtension(context.Context, PickupDayExtension) error
	ClearPickupDayExtension(context.Context, int64, string) error
	// RecordPickupWeekdayExtension takes any weekday change. The task opens
	// while the pickup is later than the earliest previous time recorded for
	// that weekday, and closes once it is not.
	RecordPickupWeekdayExtension(context.Context, PickupWeekdayExtension) error
	ClearPickupWeekdayExtension(context.Context, int64, int) error
}

type PickupExtensionQuery interface {
	// FindPickupExtensionStudent returns the child affected by one task. It is
	// used to authorize a resolve request before the command locks or changes
	// the roster.
	FindPickupExtensionStudent(context.Context, int64) (int64, error)
	// ListOpenPickupExtensions returns open tasks, for one child when the
	// student ID is positive, otherwise for the whole school.
	ListOpenPickupExtensions(context.Context, int64) ([]PickupExtensionTask, error)
}

type PickupExtensionCommand interface {
	PickupExtensionRecorder
	// ResolvePickupExtension adds the child to the chosen blocks and closes
	// the task. No block IDs closes it without a change ("keinem Block").
	// Weekday decisions change the template roster: callers must hold the
	// tenant recurrence lock inside the request transaction.
	ResolvePickupExtension(context.Context, int64, []int64) (PickupExtensionResolution, error)
}

type PickupExtensionCapability interface {
	PickupExtensionQuery
	PickupExtensionCommand
}

func validPickupClock(value string) bool { return pickupClockPattern.MatchString(value) }

func (m *Module) RecordPickupDayExtension(ctx context.Context, input PickupDayExtension) error {
	if input.StudentID <= 0 || input.PickupExceptionID <= 0 || !validDate(input.Date) ||
		!validPickupClock(input.PreviousPickup) || !validPickupClock(input.Pickup) || input.Pickup <= input.PreviousPickup {
		return m.reject("record_pickup_day_extension", ErrInvalidPickupExtension)
	}
	return m.engine.RecordPickupDayExtension(ctx, input)
}

func (m *Module) ClearPickupDayExtension(ctx context.Context, studentID int64, date string) error {
	if studentID <= 0 || !validDate(date) {
		return m.reject("clear_pickup_day_extension", ErrInvalidPickupExtension)
	}
	return m.engine.ClearPickupDayExtension(ctx, studentID, date)
}

func (m *Module) RecordPickupWeekdayExtension(ctx context.Context, input PickupWeekdayExtension) error {
	if input.StudentID <= 0 || input.Weekday < 1 || input.Weekday > 5 || !validDate(input.EffectiveFrom) ||
		!validPickupClock(input.PreviousPickup) || !validPickupClock(input.Pickup) || input.Pickup == input.PreviousPickup {
		return m.reject("record_pickup_weekday_extension", ErrInvalidPickupExtension)
	}
	return m.engine.RecordPickupWeekdayExtension(ctx, input)
}

func (m *Module) ClearPickupWeekdayExtension(ctx context.Context, studentID int64, weekday int) error {
	if studentID <= 0 || weekday < 1 || weekday > 5 {
		return m.reject("clear_pickup_weekday_extension", ErrInvalidPickupExtension)
	}
	return m.engine.ClearPickupWeekdayExtension(ctx, studentID, weekday)
}

func (m *Module) ListOpenPickupExtensions(ctx context.Context, studentID int64) ([]PickupExtensionTask, error) {
	if studentID < 0 {
		return nil, m.reject("list_open_pickup_extensions", ErrInvalidPickupExtension)
	}
	return m.engine.ListOpenPickupExtensions(ctx, studentID)
}

func (m *Module) FindPickupExtensionStudent(ctx context.Context, taskID int64) (int64, error) {
	if taskID <= 0 {
		return 0, m.reject("find_pickup_extension_student", ErrInvalidPickupExtension)
	}
	return m.engine.FindPickupExtensionStudent(ctx, taskID)
}

func (m *Module) ResolvePickupExtension(ctx context.Context, taskID int64, blockIDs []int64) (PickupExtensionResolution, error) {
	if taskID <= 0 || len(blockIDs) > 20 || slices.ContainsFunc(blockIDs, func(id int64) bool { return id <= 0 }) {
		return PickupExtensionResolution{}, m.reject("resolve_pickup_extension", ErrInvalidPickupExtension)
	}
	return m.engine.ResolvePickupExtension(ctx, taskID, blockIDs)
}
