package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// MaterializationRun reports one materialization of a school's timetable
// window.
type MaterializationRun struct {
	InstancesCreated          int
	CandidatesSkippedExisting int
	CandidatesRaced           int
	DurationMS                int64
}

// TimetableMaterializer materializes the planned timetable blocks of the
// school in ctx (Timetable). ResolveWindow returns the window the weekly job
// fills: from is the next Monday strictly after baseDate, to is from +
// weeksAhead*7 − 1. MaterializeScheduledWindow creates the missing blocks of
// [from, to] as the Worker's scheduled run.
type TimetableMaterializer interface {
	ResolveWindow(baseDate calendar.Date, weeksAhead int) (from, to calendar.Date)
	MaterializeScheduledWindow(ctx context.Context, from, to calendar.Date) (*MaterializationRun, error)
}

// TimetableCleanupRun reports the retention sweep of a school's past
// timetable data.
type TimetableCleanupRun struct {
	InstancesDeleted  int
	ExceptionsDeleted int
	StudentsAffected  int
	RetentionDays     int
	DurationMS        int64
}

// TimetableCleanup sweeps the school's timetable data past its retention
// (Timetable).
type TimetableCleanup interface {
	CleanupExpiredTimetableData(ctx context.Context) (*TimetableCleanupRun, error)
}

// AutoStartRun reports one automatic start of a school's planned blocks.
type AutoStartRun struct {
	Checked         int
	Started         int
	SkippedNoStaff  int
	SkippedDuty     int
	SkippedConflict int
	SkippedMoved    int
	Failed          int
	DurationMS      int64
}

// InstanceAutoStart starts the school's planned blocks whose window is open
// (Timetable).
type InstanceAutoStart interface {
	RunForTenant(ctx context.Context, now time.Time) (*AutoStartRun, error)
}

// AutoEndRun reports one automatic end of a school's running blocks.
type AutoEndRun struct {
	Checked           int
	Completed         int
	SkippedConcurrent int
	Failed            int
	DurationMS        int64
}

// InstanceAutoEnd ends the school's running blocks past their end and the
// grace (Timetable).
type InstanceAutoEnd interface {
	RunForTenant(ctx context.Context, now time.Time, grace time.Duration) (*AutoEndRun, error)
}

// AppointmentReminderCommand queues the guardian appointment reminders of
// [from, to) (Reminder Delivery).
type AppointmentReminderCommand interface {
	EnqueueDueAppointmentReminders(ctx context.Context, from, to time.Time) (int, error)
}

// ParentAnnouncementReminderCommand sends the due parent announcement
// reminders (Reminder Delivery).
type ParentAnnouncementReminderCommand interface {
	SendDueParentAnnouncementReminders(ctx context.Context, notBefore, dueBefore time.Time) (sent int, retryFrom *time.Time, err error)
}

// ReminderCommands are the scheduled reminders of Reminder Delivery.
type ReminderCommands interface {
	AppointmentReminderCommand
	ParentAnnouncementReminderCommand
}
