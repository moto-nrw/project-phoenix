package studentpresence

import (
	"context"
	"errors"
	"time"
)

// Session attendance status values of a planned participant (#2762).
const (
	SessionAttendanceExpected = "expected"
	SessionAttendancePresent  = "present"
	SessionAttendanceAbsent   = "absent"

	SessionAttendanceNoteMaxLength = 500
)

var (
	ErrSessionAttendanceNotFound = errors.New("session attendance not found")
	ErrInvalidSessionAttendance  = errors.New("invalid session attendance")
	ErrSessionAttendanceRoster   = errors.New("session attendance: planned roster is not bound")
	ErrSessionAttendanceCarePlan = errors.New("session attendance: care plan directory is not bound")
)

// SessionAttendance is the owner's view of one
// active.activity_session_attendance row: what was observed or decided for
// one planned participant (a schedule.instance_students row, held by
// Timetable) of an activity instance. A participant without a row is
// expected attendance; ExpectedSessionAttendance builds that default.
type SessionAttendance struct {
	ID, TenantID         int64
	CreatedAt, UpdatedAt time.Time
	ParticipantID        int64
	Status               string
	Substatus            *string
	Note                 *string
	CheckedInAt          *time.Time
	CheckedOutAt         *time.Time
	// IsUnplanned marks a walk-in: a participant the kiosk created because
	// the child was scanned into a block the plan did not list them in.
	IsUnplanned bool
	// NotScheduled records that the care plan did not place the child in the
	// OGS on the block's day (#1747). It is frozen when the block ends.
	NotScheduled bool
	// ManualStatusAt records that a person set the status by hand.
	ManualStatusAt *time.Time
	// StudentStatusDayID and PickupExceptionID name the care-plan record that
	// owns an absence; observed presence and manual decisions clear them.
	StudentStatusDayID *int64
	PickupExceptionID  *int64
}

// ExpectedSessionAttendance is the attendance of a participant no row was
// written for yet.
func ExpectedSessionAttendance(participantID int64) SessionAttendance {
	return SessionAttendance{ParticipantID: participantID, Status: SessionAttendanceExpected}
}

// IsOpen reports a present participant that has not checked out.
func (a SessionAttendance) IsOpen() bool {
	return a.Status == SessionAttendancePresent && a.CheckedInAt != nil && a.CheckedOutAt == nil
}

// SessionAttendancePatch is a manual decision on one participant (#E18): a
// nil pointer leaves the field alone, the Clear flags set it to NULL.
type SessionAttendancePatch struct {
	Status         *string
	Substatus      *string
	SubstatusClear bool
	Note           *string
	NoteClear      bool
}

func (p SessionAttendancePatch) HasChanges() bool {
	return p.Status != nil || p.Substatus != nil || p.SubstatusClear || p.Note != nil || p.NoteClear
}

// SessionAttendanceRestore is one row of a completion snapshot that a reopen
// writes back exactly.
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

// ParticipantPickupException reconnects a restored participant to the
// partial excusal that owned its absence before a care exit.
type ParticipantPickupException struct {
	ParticipantID     int64
	PickupExceptionID int64
}

// PlannedParticipantRef names one planned participant of one instance, the
// pair the planned roster port resolves for the attendance commands.
type PlannedParticipantRef struct {
	StudentID  int64
	InstanceID int64
}

type SessionAttendanceQuery interface {
	// ListSessionAttendance returns the rows that exist for the participants,
	// ordered by participant id. Participants without a row are expected.
	ListSessionAttendance(context.Context, []int64) ([]SessionAttendance, error)
}

// SessionAttendanceCommand writes the attendance of participants the caller
// already resolved through Timetable. Every command reads a missing row as
// expected attendance and returns how many participants it changed.
type SessionAttendanceCommand interface {
	// CheckInParticipants opens observed presence for participants that are
	// expected, owned by a care-plan absence, or present but checked out.
	CheckInParticipants(context.Context, []int64, time.Time) (int64, error)
	// CheckInWalkIn is CheckInParticipants for one participant the plan did
	// not list; the row is marked unplanned and returned.
	CheckInWalkIn(context.Context, int64, time.Time) (SessionAttendance, error)
	// CheckOutParticipants stamps the checkout of present participants that
	// checked in at or before the instant; a later checkout stays.
	CheckOutParticipants(context.Context, []int64, time.Time) (int64, error)
	// CloseOpenParticipants closes present participants that checked in at or
	// before the instant and have not checked out.
	CloseOpenParticipants(context.Context, []int64, time.Time) (int64, error)
	// ReconcileParticipantInterval replaces the interval of one present
	// participant when it still matches the previous one.
	ReconcileParticipantInterval(context.Context, int64, time.Time, *time.Time, time.Time, *time.Time) (bool, error)
	// PatchSessionAttendance applies a manual decision. A status or substatus
	// decision clears the care-plan provenance and the non-booking marker
	// and stamps ManualStatusAt.
	PatchSessionAttendance(context.Context, int64, SessionAttendancePatch) error
	// TransitionParticipants moves participants from one status to another;
	// participants in any other status are left alone.
	TransitionParticipants(context.Context, []int64, string, string, time.Time) (int64, error)
	// MarkParticipantsNotScheduled freezes the non-booking marker onto
	// participants that are still expected or owned by a care-plan absence
	// and were not decided by hand.
	MarkParticipantsNotScheduled(context.Context, []int64) (int64, error)
	// RestoreSessionAttendance writes completion snapshot rows back exactly;
	// a missing participant fails the surrounding recovery transaction.
	RestoreSessionAttendance(context.Context, []SessionAttendanceRestore) error
	// ReconnectParticipantPickupExceptions restores the partial excusal
	// provenance of participants a care exit removed and restored.
	ReconnectParticipantPickupExceptions(context.Context, []ParticipantPickupException) error
	// LockSessionAttendance holds the existing rows of the participants until
	// the caller's tenant transaction ends.
	LockSessionAttendance(context.Context, []int64) error
}

// SessionAttendanceRules are the attendance decisions that resolve their own
// participants through the planned roster and the care plan: a reported day
// status, a partial excusal, and the block end of a live group.
type SessionAttendanceRules interface {
	// ApplyStatusDay marks the student's planned participants of the day absent
	// for the reported day status when it is the latest active one.
	ApplyStatusDay(context.Context, int64, string, int64, string) (int, error)
	// ReleaseStatusDay returns the participants the day status owned to the
	// next active day status, or to expected (absent when the block ended).
	ReleaseStatusDay(context.Context, int64) (int, error)
	// ApplyActiveStatusDaysForInstance marks the expected participants of one
	// instance absent for the day statuses reported for its date.
	ApplyActiveStatusDaysForInstance(context.Context, int64, string) (int, error)
	// ApplyPartialAbsence marks the student's participants of blocks starting
	// at or after the excusal absent and excused.
	ApplyPartialAbsence(context.Context, int64) (int, error)
	// ReleasePartialAbsence returns the participants the excusal owned to the
	// active day status or to expected.
	ReleasePartialAbsence(context.Context, int64) (int, error)
	// ApplyActivePartialAbsencesForInstance marks the participants of one
	// instance absent for the excusals reported for its date.
	ApplyActivePartialAbsencesForInstance(context.Context, int64, string) (int, error)
	// MarkExpectedAbsentByActiveGroupIDs marks every still expected
	// participant of the running blocks on the live groups absent, except
	// the given (student, instance) pairs.
	MarkExpectedAbsentByActiveGroupIDs(context.Context, []int64, time.Time, []PlannedParticipantRef) error
	// CloseOpenCheckoutsByActiveGroupIDs closes the open presence of the
	// participants of the blocks on the live groups.
	CloseOpenCheckoutsByActiveGroupIDs(context.Context, []int64, time.Time) (int, error)
}

func (m *Module) ListSessionAttendance(ctx context.Context, participantIDs []int64) ([]SessionAttendance, error) {
	return m.engine.ListSessionAttendance(ctx, participantIDs)
}

func (m *Module) CheckInParticipants(ctx context.Context, participantIDs []int64, at time.Time) (int64, error) {
	return m.engine.CheckInParticipants(ctx, participantIDs, at)
}

func (m *Module) CheckInWalkIn(ctx context.Context, participantID int64, at time.Time) (SessionAttendance, error) {
	return m.engine.CheckInWalkIn(ctx, participantID, at)
}

func (m *Module) CheckOutParticipants(ctx context.Context, participantIDs []int64, at time.Time) (int64, error) {
	return m.engine.CheckOutParticipants(ctx, participantIDs, at)
}

func (m *Module) CloseOpenParticipants(ctx context.Context, participantIDs []int64, at time.Time) (int64, error) {
	return m.engine.CloseOpenParticipants(ctx, participantIDs, at)
}

func (m *Module) ReconcileParticipantInterval(ctx context.Context, participantID int64, previousCheckIn time.Time, previousCheckOut *time.Time, checkIn time.Time, checkOut *time.Time) (bool, error) {
	return m.engine.ReconcileParticipantInterval(ctx, participantID, previousCheckIn, previousCheckOut, checkIn, checkOut)
}

func (m *Module) PatchSessionAttendance(ctx context.Context, participantID int64, patch SessionAttendancePatch) error {
	return m.engine.PatchSessionAttendance(ctx, participantID, patch)
}

func (m *Module) TransitionParticipants(ctx context.Context, participantIDs []int64, from, to string, at time.Time) (int64, error) {
	return m.engine.TransitionParticipants(ctx, participantIDs, from, to, at)
}

func (m *Module) MarkParticipantsNotScheduled(ctx context.Context, participantIDs []int64) (int64, error) {
	return m.engine.MarkParticipantsNotScheduled(ctx, participantIDs)
}

func (m *Module) RestoreSessionAttendance(ctx context.Context, rows []SessionAttendanceRestore) error {
	return m.engine.RestoreSessionAttendance(ctx, rows)
}

func (m *Module) ReconnectParticipantPickupExceptions(ctx context.Context, links []ParticipantPickupException) error {
	return m.engine.ReconnectParticipantPickupExceptions(ctx, links)
}

func (m *Module) LockSessionAttendance(ctx context.Context, participantIDs []int64) error {
	return m.engine.LockSessionAttendance(ctx, participantIDs)
}

func (m *Module) ApplyStatusDay(ctx context.Context, studentID int64, date string, statusDayID int64, substatus string) (int, error) {
	return m.engine.ApplyStatusDay(ctx, studentID, date, statusDayID, substatus)
}

func (m *Module) ReleaseStatusDay(ctx context.Context, statusDayID int64) (int, error) {
	return m.engine.ReleaseStatusDay(ctx, statusDayID)
}

func (m *Module) ApplyActiveStatusDaysForInstance(ctx context.Context, instanceID int64, date string) (int, error) {
	return m.engine.ApplyActiveStatusDaysForInstance(ctx, instanceID, date)
}

func (m *Module) ApplyPartialAbsence(ctx context.Context, pickupExceptionID int64) (int, error) {
	return m.engine.ApplyPartialAbsence(ctx, pickupExceptionID)
}

func (m *Module) ReleasePartialAbsence(ctx context.Context, pickupExceptionID int64) (int, error) {
	return m.engine.ReleasePartialAbsence(ctx, pickupExceptionID)
}

func (m *Module) ApplyActivePartialAbsencesForInstance(ctx context.Context, instanceID int64, date string) (int, error) {
	return m.engine.ApplyActivePartialAbsencesForInstance(ctx, instanceID, date)
}

func (m *Module) MarkExpectedAbsentByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, at time.Time, exclusions []PlannedParticipantRef) error {
	return m.engine.MarkExpectedAbsentByActiveGroupIDs(ctx, activeGroupIDs, at, exclusions)
}

func (m *Module) CloseOpenCheckoutsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, at time.Time) (int, error) {
	return m.engine.CloseOpenCheckoutsByActiveGroupIDs(ctx, activeGroupIDs, at)
}

// PlannedParticipant is one planned participant of one instance as Timetable
// lists it: the participant row, its instance, the child, and the planning
// day and block start (HH:MM:SS) the attendance rules compare against.
type PlannedParticipant struct {
	ID         int64
	InstanceID int64
	StudentID  int64
	Date       string
	StartTime  string
}

// PlannedRosterFilter selects planned participants. Date and FromClock name
// the planning day and the earliest block start; ExcludeCancelled drops
// cancelled instances.
type PlannedRosterFilter struct {
	IDs              []int64
	InstanceIDs      []int64
	StudentIDs       []int64
	Date             string
	FromClock        string
	ExcludeCancelled bool
}

// PlannedRoster is the consumer-owned port to the Timetable owner's planned
// participants. The composition root binds it; without it the attendance
// rules fail with ErrSessionAttendanceRoster.
type PlannedRoster interface {
	ListPlannedParticipants(context.Context, PlannedRosterFilter) ([]PlannedParticipant, error)
}

type SessionPickupException struct {
	ID            int64
	StudentID     int64
	ExceptionDate string
	ExcusedFrom   *time.Time
	ExcusedAuto   bool
}

type SessionPickupExceptionFilter struct {
	IDs        []int64
	StudentIDs []int64
	Date       string
	From       string
}

type SessionStudentStatusDay struct {
	ID        int64
	StudentID int64
	Date      string
	Status    string
}

type SessionStudentStatusDayFilter struct {
	IDs        []int64
	StudentIDs []int64
	Date       string
	From       string
	ActiveOnly bool
	LatestOnly bool
}

// SessionCarePlanDirectory is the consumer-owned port to the Care Plan
// owner's reported day statuses and partial excusals.
type SessionCarePlanDirectory interface {
	FindPickupException(context.Context, int64) (*SessionPickupException, error)
	ListPickupExceptions(context.Context, SessionPickupExceptionFilter) ([]SessionPickupException, error)
	FindStudentStatusDay(context.Context, int64, bool) (*SessionStudentStatusDay, error)
	ListStudentStatusDays(context.Context, SessionStudentStatusDayFilter) ([]SessionStudentStatusDay, error)
}

// SessionCareDayLocker serializes one child's attendance decisions of a day
// with the care-plan writers of that day.
type SessionCareDayLocker interface {
	LockStudentAndExceptionDay(context.Context, int64, string) error
	LockExceptionDay(context.Context, int64, string) error
}
