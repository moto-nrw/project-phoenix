package ports

import (
	"context"
	"errors"
	"time"
)

var ErrSessionAttendanceNotFound = errors.New("session attendance not found")

// SessionAttendance is the persistence-independent attendance of one planned
// participant. A participant without a stored row is expected attendance.
type SessionAttendance struct {
	ID, TenantID         int64
	CreatedAt, UpdatedAt time.Time
	ParticipantID        int64
	Status               string
	Substatus            *string
	Note                 *string
	CheckedInAt          *time.Time
	CheckedOutAt         *time.Time
	IsUnplanned          bool
	NotScheduled         bool
	ManualStatusAt       *time.Time
	StudentStatusDayID   *int64
	PickupExceptionID    *int64
}

type SessionAttendancePatch struct {
	Status         *string
	Substatus      *string
	SubstatusClear bool
	Note           *string
	NoteClear      bool
}

type SessionAttendanceRestore struct {
	ParticipantID      int64
	Status             string
	Substatus          *string
	Note               *string
	CheckedInAt        *time.Time
	CheckedOutAt       *time.Time
	IsUnplanned        bool
	NotScheduled       bool
	ManualStatusAt     *time.Time
	StudentStatusDayID *int64
	PickupExceptionID  *int64
}

type ParticipantPickupException struct {
	ParticipantID     int64
	PickupExceptionID int64
}

// StatusDayAbsence marks one participant absent for a reported day status.
type StatusDayAbsence struct {
	ParticipantID int64
	StatusDayID   int64
	Substatus     string
}

// StatusDayRelease returns one participant the day status owned to its next
// state: absent under the replacement, absent because the block ended, or
// expected.
type StatusDayRelease struct {
	ParticipantID    int64
	Status           string
	Substatus        *string
	ReplacementDayID *int64
}

// PartialAbsence marks one participant absent and excused for an excusal.
type PartialAbsence struct {
	ParticipantID     int64
	PickupExceptionID int64
}

// SessionAttendanceStore persists active.activity_session_attendance. Commands
// take participant ids the application resolved through the planned roster;
// each treats a missing row as expected attendance.
type SessionAttendanceStore interface {
	ListSessionAttendance(context.Context, []int64) ([]SessionAttendance, Stats, error)
	ListSessionAttendanceByStatusDay(context.Context, int64) ([]SessionAttendance, Stats, error)
	ListSessionAttendanceByPickupException(context.Context, int64) ([]SessionAttendance, Stats, error)
	CheckInParticipants(context.Context, []int64, time.Time, bool) (Stats, error)
	CheckOutParticipants(context.Context, []int64, time.Time) (Stats, error)
	CloseOpenParticipants(context.Context, []int64, time.Time) (Stats, error)
	ReconcileParticipantInterval(context.Context, int64, time.Time, *time.Time, time.Time, *time.Time) (Stats, error)
	PatchSessionAttendance(context.Context, int64, SessionAttendancePatch, time.Time) (Stats, error)
	// TransitionParticipants stamps updated_at with the given instant, or with
	// the transaction time when nil.
	TransitionParticipants(context.Context, []int64, string, string, *time.Time) (Stats, error)
	MarkParticipantsNotScheduled(context.Context, []int64) (Stats, error)
	RestoreSessionAttendance(context.Context, SessionAttendanceRestore) (Stats, error)
	ReconnectParticipantPickupExceptions(context.Context, []ParticipantPickupException) (Stats, error)
	LockSessionAttendance(context.Context, []int64) (Stats, error)
	ApplyStatusDayAbsences(context.Context, []StatusDayAbsence, time.Time) (Stats, error)
	ReleaseStatusDay(context.Context, int64, []StatusDayRelease, time.Time) (Stats, error)
	ApplyPartialAbsences(context.Context, []PartialAbsence, time.Time) (Stats, error)
	ReleasePartialAbsence(context.Context, int64, []StatusDayRelease, time.Time) (Stats, error)
	FindSessionAttendance(context.Context, int64) (SessionAttendance, bool, Stats, error)
}

// PlannedParticipant is one planned participant of one instance as the
// Timetable owner lists it, with the planning day and block start
// (HH:MM:SS) the attendance rules compare against.
type PlannedParticipant struct {
	ID         int64
	InstanceID int64
	StudentID  int64
	Date       string
	StartTime  string
}

// PlannedRosterFilter selects planned participants. Date and FromClock name
// the planning day and the earliest block start (HH:MM:SS); ExcludeCancelled
// drops cancelled instances.
type PlannedRosterFilter struct {
	IDs              []int64
	InstanceIDs      []int64
	StudentIDs       []int64
	Date             string
	FromClock        string
	ExcludeCancelled bool
}

// PlannedRoster is the consumer-owned port to the Timetable owner's planned
// participants; the composition root binds it.
type PlannedRoster interface {
	ListPlannedParticipants(context.Context, PlannedRosterFilter) ([]PlannedParticipant, error)
}

type PickupException struct {
	ID            int64
	StudentID     int64
	ExceptionDate string
	ExcusedFrom   *time.Time
	ExcusedAuto   bool
}

type PickupExceptionFilter struct {
	IDs        []int64
	StudentIDs []int64
	Date       string
	From       string
}

type StudentStatusDay struct {
	ID        int64
	StudentID int64
	Date      string
	Status    string
}

type StudentStatusDayFilter struct {
	IDs        []int64
	StudentIDs []int64
	Date       string
	From       string
	ActiveOnly bool
	LatestOnly bool
}

// CarePlanDirectory is the consumer-owned port to the Care Plan owner's
// reported day statuses and partial excusals.
type CarePlanDirectory interface {
	FindPickupException(context.Context, int64) (*PickupException, error)
	ListPickupExceptions(context.Context, PickupExceptionFilter) ([]PickupException, error)
	FindStudentStatusDay(context.Context, int64, bool) (*StudentStatusDay, error)
	ListStudentStatusDays(context.Context, StudentStatusDayFilter) ([]StudentStatusDay, error)
}

// CareDayLocker serializes the attendance decisions of one child's day with
// the care-plan writers of that day.
type CareDayLocker interface {
	LockStudentAndExceptionDay(context.Context, int64, string) error
	LockExceptionDay(context.Context, int64, string) error
}

// AttendanceRulePorts are the consumer-owned ports the attendance rules
// resolve their participants and care-plan facts through.
type AttendanceRulePorts struct {
	Roster   PlannedRoster
	CarePlan CarePlanDirectory
	CareDays CareDayLocker
}
