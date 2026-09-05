package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/timetableprojection"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// CareExitCleanupRepository owns every deliberately cross-schema operation
// that ending a child's care needs: closing the open parent requests across
// the four queues, closing whatever presence record the child still has open
// when the exit takes effect, dropping them from rosters dated after their
// last care day, and ending their offering and activity bookings there.
//
// They span schemas no single domain repository owns (users, active,
// enrollment, schedule, activities), which is the documented exception in
// backend-conventions rule 11 — the raw SQL lives here, never in the service.
// Keeping them together also keeps the counting half (the preview) and the
// writing half (the confirmation) side by side, where a divergence is
// visible.
type CareExitCleanupRepository struct {
	db          *bun.DB
	assignments CareExitAssignments
	periods     CalendarPeriodDirectory
	// rooms re-validates snapshot room references through the Facilities
	// owner on restore (#2665).
	rooms    RoomDirectory
	carePlan CarePlanDirectory
	bookings ActivityBookingDirectory
}

// BindRoomDirectory installs the Facilities directory the restore
// re-validates room references through (#2665).
func (r *CareExitCleanupRepository) BindRoomDirectory(rooms RoomDirectory) {
	r.rooms = rooms
}

// CareOfferingProjection is the narrow owner data used by care-exit cleanup.
type CareOfferingProjection struct {
	ID             int64    `json:"id"`
	TenantID       int64    `json:"tenant_id"`
	Name           string   `json:"name"`
	DaysOfWeekMode string   `json:"days_of_week_mode"`
	AvailableDays  []string `json:"available_days"`
	CountsAsCare   bool     `json:"counts_as_care"`
	SortOrder      int      `json:"sort_order"`
}

// PendingOfferingChange identifies one open request affected by a care exit.
type PendingOfferingChange struct {
	StudentID int64
}

type ActivityBooking struct {
	ID                       int64
	TenantID                 int64
	StudentID                int64
	ActivityGroupID          int64
	ValidFrom                string
	ValidUntil               *string
	CalendarPeriodID         *int64
	EnrollmentRequestChildID *int64
	SelectedWeekdays         []int
	AttendanceStatus         *string
	Weekday                  *int
}

type ActivityBookingCap struct {
	StudentID          int64
	ID                 int64
	PreviousValidUntil *string
}

type ActivityBookingChanges struct {
	Deleted []ActivityBooking
	Capped  []ActivityBookingCap
}

type ActivityBookingRemoval struct {
	ActivityBooking
	WasDeleted         bool
	PreviousValidUntil *string
}

type ActivityBookingDirectory interface {
	LockStudentEnrollmentsForCareExit(context.Context, []int64, string) error
	EndStudentEnrollmentsForCareExit(context.Context, []int64, string) (ActivityBookingChanges, error)
	RestoreStudentEnrollmentsForCareExit(context.Context, []int64, []int64, []ActivityBookingRemoval) (int, error)
}

type CareExitRemoval struct {
	ID                       int64          `json:"id"`
	TenantID                 int64          `json:"tenant_id"`
	StudentID                int64          `json:"student_id"`
	Kind                     string         `json:"kind"`
	InstanceID               *int64         `json:"instance_id"`
	RoomID                   *int64         `json:"room_id"`
	Status                   *string        `json:"status"`
	Substatus                *string        `json:"substatus"`
	Note                     *string        `json:"note"`
	IsUnplanned              *bool          `json:"is_unplanned"`
	NotScheduled             *bool          `json:"not_scheduled"`
	ManualStatusAt           *time.Time     `json:"manual_status_at"`
	StudentStatusDayID       *int64         `json:"student_status_day_id"`
	PickupExceptionID        *int64         `json:"pickup_exception_id"`
	EnrollmentID             *int64         `json:"enrollment_id"`
	WasDeleted               bool           `json:"was_deleted"`
	PreviousValidUntil       *timezone.Date `json:"previous_valid_until"`
	ActivityGroupID          *int64         `json:"activity_group_id"`
	ValidFrom                *timezone.Date `json:"valid_from"`
	CalendarPeriodID         *int64         `json:"calendar_period_id"`
	EnrollmentRequestChildID *int64         `json:"enrollment_request_child_id"`
	SelectedWeekdays         []int          `json:"selected_weekdays"`
	AttendanceStatus         *string        `json:"attendance_status"`
	Weekday                  *int           `json:"weekday"`
	CreatedAt                time.Time      `json:"created_at"`
}

type CareExitSourceRemoval struct {
	ID          int64           `json:"id"`
	TenantID    int64           `json:"tenant_id"`
	StudentID   int64           `json:"student_id"`
	Kind        string          `json:"kind"`
	SourceRowID int64           `json:"source_row_id"`
	WasDeleted  bool            `json:"was_deleted"`
	Snapshot    json.RawMessage `json:"snapshot"`
	CreatedAt   time.Time       `json:"created_at"`
}

const (
	CareExitRemovalRoster          = "roster"
	CareExitRemovalBooking         = "booking"
	CareExitSourceBooking          = "source_booking"
	CareExitSourcePickupSchedule   = "pickup_schedule"
	CareExitSourceArrivalSchedule  = "arrival_schedule"
	CareExitSourcePickupException  = "pickup_exception"
	CareExitSourceArrivalException = "arrival_exception"
)

// CarePlanDirectory exposes only the owner operations care-exit cleanup needs.
type CarePlanDirectory interface {
	ListCareOfferings(context.Context) ([]CareOfferingProjection, error)
	LockCareOfferings(context.Context, []int64) error
	ListPendingOfferingChanges(context.Context, []int64, bool) ([]PendingOfferingChange, error)
	ClosePendingOfferingChanges(context.Context, []int64, string, *int64, time.Time) (int64, error)
	ListCareExitRemovals(context.Context, []int64) ([]CareExitRemoval, error)
	ListCareExitSourceRemovals(context.Context, []int64) ([]CareExitSourceRemoval, error)
	RecordCareExitRemovals(context.Context, []CareExitRemoval) error
	RecordCareExitSourceRemovals(context.Context, []CareExitSourceRemoval) error
	DiscardCareExitRemovals(context.Context, []int64) error
	LockStudentSchedulesForCareExit(context.Context, []int64, string) error
	ListWeeklyPlanPatterns(context.Context, []int64) (map[int64][]string, error)
	EndStudentSchedulesForCareExit(context.Context, []int64, string) (int64, error)
	RestoreStudentSchedulesForCareExit(context.Context, []int64) (int64, error)
	ExistingPickupExceptionIDs(context.Context, []int64) ([]int64, error)
	ExistingStudentStatusDayIDs(context.Context, []int64) ([]int64, error)
	CountOpenCareRequests(context.Context, []int64) (map[int64]int, error)
	LockOpenCareRequests(context.Context, []int64) error
	CloseOpenCareRequests(context.Context, []int64, string, *int64, time.Time) (int64, error)
}

// CalendarPeriodDirectory is the School Calendar query the restore re-validates
// booking period references through. schedule.calendar_periods belongs to that
// owner (#2666); the composition root binds the owner query behind this port
// instead of the former SQL subquery. It fails while unbound.
type CalendarPeriodDirectory interface {
	// ListCalendarPeriodIDs returns the ids of every period of the current
	// tenant.
	ListCalendarPeriodIDs(ctx context.Context) ([]int64, error)
}

var errCalendarPeriodDirectoryRequired = errors.New("users repositories: calendar period directory is not bound")

// NewCareExitCleanupRepository builds the repository.
func NewCareExitCleanupRepository(db *bun.DB, assignments CareExitAssignments) userModels.CareExitCleanupRepository {
	if assignments == nil {
		panic("care exit cleanup: timetable assignments are required")
	}
	return &CareExitCleanupRepository{db: db, assignments: assignments}
}

// BindCalendarPeriods installs the School Calendar query the booking restore
// reads surviving period ids through (#2666).
func (r *CareExitCleanupRepository) BindCalendarPeriods(periods CalendarPeriodDirectory) {
	r.periods = periods
}

// BindCarePlan installs the owner query used for care-offering projections
// and row locks.
func (r *CareExitCleanupRepository) BindCarePlan(capability CarePlanDirectory) {
	r.carePlan = capability
}

func (r *CareExitCleanupRepository) BindActivityBookings(capability ActivityBookingDirectory) {
	if capability == nil {
		panic("users repositories: activity booking directory is required")
	}
	r.bookings = capability
}

func (r *CareExitCleanupRepository) requireActivityBookings() error {
	if r.bookings == nil {
		return errors.New("users repositories: activity booking directory is not bound")
	}
	return nil
}

func (r *CareExitCleanupRepository) requireCarePlan() error {
	if r.carePlan == nil {
		return errors.New("care exit cleanup requires the Care Plan capability")
	}
	return nil
}

func withCareExitTransaction(ctx context.Context, mutation func(context.Context) (int64, error)) (int64, error) {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return mutation(ctx)
	}
	var affected int64
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		var mutationErr error
		affected, mutationErr = mutation(txCtx)
		return mutationErr
	})
	return affected, err
}

func (r *CareExitCleanupRepository) careOfferingProjection(ctx context.Context) (string, error) {
	if err := r.requireCarePlan(); err != nil {
		return "", err
	}
	offerings, err := r.carePlan.ListCareOfferings(ctx)
	if err != nil {
		return "", fmt.Errorf("list care offerings for care exit cleanup: %w", err)
	}
	encoded, err := json.Marshal(offerings)
	if err != nil {
		return "", fmt.Errorf("encode care offerings for care exit cleanup: %w", err)
	}
	return string(encoded), nil
}

func (r *CareExitCleanupRepository) careExitRemovalProjection(ctx context.Context, studentIDs []int64) (string, error) {
	if err := r.requireCarePlan(); err != nil {
		return "", err
	}
	removals, err := r.carePlan.ListCareExitRemovals(ctx, studentIDs)
	if err != nil {
		return "", fmt.Errorf("list care exit removals: %w", err)
	}
	encoded, err := json.Marshal(removals)
	if err != nil {
		return "", fmt.Errorf("encode care exit removals: %w", err)
	}
	return string(encoded), nil
}

func (r *CareExitCleanupRepository) careExitSourceRemovalProjection(ctx context.Context, studentIDs []int64) (string, error) {
	if err := r.requireCarePlan(); err != nil {
		return "", err
	}
	removals, err := r.carePlan.ListCareExitSourceRemovals(ctx, studentIDs)
	if err != nil {
		return "", fmt.Errorf("list care exit source removals: %w", err)
	}
	encoded, err := json.Marshal(removals)
	if err != nil {
		return "", fmt.Errorf("encode care exit source removals: %w", err)
	}
	return string(encoded), nil
}

const careExitRemovalRecordset = `jsonb_to_recordset(?::jsonb) AS rm(
	tenant_id bigint, student_id bigint, kind text, instance_id bigint,
	room_id bigint, status text, substatus text, note text,
	is_unplanned boolean, not_scheduled boolean, manual_status_at timestamptz,
	student_status_day_id bigint, pickup_exception_id bigint,
	enrollment_id bigint, was_deleted boolean, previous_valid_until date,
	activity_group_id bigint, valid_from date, calendar_period_id bigint,
	enrollment_request_child_id bigint, selected_weekdays jsonb,
	attendance_status text, weekday smallint
)`

const careExitSourceRemovalRecordset = `jsonb_to_recordset(?::jsonb) AS rm(
	tenant_id bigint, student_id bigint, kind text, source_row_id bigint,
	was_deleted boolean, snapshot jsonb
)`

func (r *CareExitCleanupRepository) CountOpenRequests(
	ctx context.Context, studentIDs []int64,
) (map[int64]int, error) {
	counts := make(map[int64]int, len(studentIDs))
	if len(studentIDs) == 0 {
		return counts, nil
	}
	if r.carePlan == nil {
		return nil, errors.New("care exit cleanup requires the Care Plan capability")
	}
	counts, err := r.carePlan.CountOpenCareRequests(ctx, studentIDs)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "count open parent requests", Err: err}
	}
	changes, err := r.carePlan.ListPendingOfferingChanges(ctx, studentIDs, false)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "count open parent requests", Err: err}
	}
	for _, change := range changes {
		counts[change.StudentID]++
	}
	return counts, nil
}

func (r *CareExitCleanupRepository) LockOpenRequestsForCareExit(ctx context.Context, studentIDs []int64) error {
	if len(studentIDs) == 0 {
		return nil
	}
	if r.carePlan == nil {
		return errors.New("care exit cleanup requires the Care Plan capability")
	}
	if err := r.carePlan.LockOpenCareRequests(ctx, studentIDs); err != nil {
		return &modelBase.DatabaseError{Op: "lock open parent requests for care exit", Err: err}
	}
	if _, err := r.carePlan.ListPendingOfferingChanges(ctx, studentIDs, true); err != nil {
		return &modelBase.DatabaseError{Op: "lock open offering change requests for care exit", Err: err}
	}
	return nil
}

// CloseOpenRequests moves every still-open request of the given children to
// the care_ended terminal state. The decision reason is written in German
// because it is shown to the family verbatim; reviewedBy is the acting account
// for the manual path and nil for the scheduler, whose "reviewer" is nobody.
func (r *CareExitCleanupRepository) CloseOpenRequests(
	ctx context.Context, studentIDs []int64, reviewedBy *int64, at time.Time,
) (int, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	if r.carePlan == nil {
		return 0, errors.New("care exit cleanup requires the Care Plan capability")
	}
	closedCareRequests, err := r.carePlan.CloseOpenCareRequests(ctx, studentIDs, userModels.CareEndedDecisionReason, reviewedBy, at)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "close open parent requests", Err: err}
	}
	total := int(closedCareRequests)
	closed, err := r.carePlan.ClosePendingOfferingChanges(ctx, studentIDs, userModels.CareEndedDecisionReason, reviewedBy, at)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "close open parent requests", Err: err}
	}
	total += int(closed)
	return total, nil
}

// FindOpenPresence returns the ids of the children that still hold an open
// attendance row, an open room visit, or an open roster check-in.
func (r *CareExitCleanupRepository) FindOpenPresence(
	ctx context.Context, studentIDs []int64,
) (map[int64]bool, error) {
	present := make(map[int64]bool, len(studentIDs))
	if len(studentIDs) == 0 {
		return present, nil
	}
	var rows []struct {
		StudentID int64 `bun:"student_id"`
	}
	if err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT student_id FROM active.attendance
		WHERE tenant_id = ? AND student_id IN (?) AND check_out_time IS NULL
		UNION
		SELECT student_id FROM active.visits
		WHERE tenant_id = ? AND student_id IN (?) AND exit_time IS NULL
	`, tenant.FromContext(ctx), bun.List(studentIDs),
		tenant.FromContext(ctx), bun.List(studentIDs)).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find open presence", Err: base.TranslateNotFound(err)}
	}
	for _, row := range rows {
		present[row.StudentID] = true
	}
	rosterIDs, err := r.assignments.ListOpenStudentAssignments(ctx, studentIDs)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find open presence", Err: err}
	}
	for _, id := range rosterIDs {
		present[id] = true
	}
	return present, nil
}

// LockImpactRowsForCareExit freezes rows whose values appear in the binding
// preview but are not part of the planning locks: names/RFID assignment, open
// presence, and source-offering names.
func (r *CareExitCleanupRepository) LockImpactRowsForCareExit(ctx context.Context, studentIDs []int64) error {
	if len(studentIDs) == 0 {
		return nil
	}
	tenantID := tenant.FromContext(ctx)
	db := base.GetDB(ctx, r.db)
	if r.carePlan == nil {
		return errors.New("care exit cleanup requires the Care Plan capability")
	}
	var offeringIDs []int64
	if err := db.NewRaw(`
		SELECT DISTINCT link.care_offering_id
		FROM enrollment.request_child_offerings AS link
		JOIN enrollment.request_children AS child
		  ON child.id = link.request_child_id AND child.tenant_id = link.tenant_id
		WHERE link.tenant_id = ? AND child.created_student_id IN (?)
		ORDER BY link.care_offering_id
	`, tenantID, bun.List(studentIDs)).Scan(ctx, &offeringIDs); err != nil {
		return &modelBase.DatabaseError{Op: "find source offerings for care exit lock", Err: base.TranslateNotFound(err)}
	}
	if len(offeringIDs) > 0 {
		if err := r.carePlan.LockCareOfferings(ctx, offeringIDs); err != nil {
			return &modelBase.DatabaseError{Op: "lock source offerings for care exit", Err: err}
		}
	}
	statements := []struct {
		op  string
		sql string
	}{
		{"lock people for care exit", `SELECT person.id FROM users.persons AS person JOIN users.students AS student ON student.person_id = person.id AND student.tenant_id = person.tenant_id WHERE student.tenant_id = ? AND student.id IN (?) FOR UPDATE OF person`},
		{"lock attendance for care exit", `SELECT id FROM active.attendance WHERE tenant_id = ? AND student_id IN (?) AND check_out_time IS NULL FOR UPDATE`},
		{"lock visits for care exit", `SELECT id FROM active.visits WHERE tenant_id = ? AND student_id IN (?) AND exit_time IS NULL FOR UPDATE`},
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement.sql, tenantID, bun.List(studentIDs)); err != nil {
			return &modelBase.DatabaseError{Op: statement.op, Err: base.TranslateNotFound(err)}
		}
	}
	if err := r.assignments.LockOpenStudentAssignments(ctx, studentIDs); err != nil {
		return &modelBase.DatabaseError{Op: "lock roster presence for care exit", Err: err}
	}
	return nil
}

func (r *CareExitCleanupRepository) LatestAttendanceDate(ctx context.Context, studentID int64) (*timezone.Date, error) {
	var day *timezone.Date
	if err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT MAX(recorded.day) FROM (
			SELECT attendance.date AS day
			FROM active.attendance AS attendance
			WHERE attendance.tenant_id = ? AND attendance.student_id = ?
			UNION ALL
			SELECT (visit.entry_time AT TIME ZONE 'Europe/Berlin')::date AS day
			FROM active.visits AS visit
			WHERE visit.tenant_id = ? AND visit.student_id = ?
		) AS recorded
	`, tenant.FromContext(ctx), studentID,
		tenant.FromContext(ctx), studentID).Scan(ctx, &day); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find latest attendance before care exit", Err: base.TranslateNotFound(err)}
	}
	rosterDay, err := r.assignments.LatestStudentAssignmentAttendanceDate(ctx, studentID)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find latest attendance before care exit", Err: err}
	}
	if rosterDay != nil {
		parsed, parseErr := timezone.ParseDate(*rosterDay)
		if parseErr != nil {
			return nil, &modelBase.DatabaseError{Op: "find latest attendance before care exit", Err: parseErr}
		}
		if day == nil || parsed.After(*day) {
			day = &parsed
		}
	}
	return day, nil
}

// CloseOpenPresence closes whatever the children still have open at the moment
// their care ends: the attendance row, the room visit, and the roster
// check-in. Nothing is deleted — the day that happened stays in the history,
// it just stops being an unfinished one (#2487).
func (r *CareExitCleanupRepository) CloseOpenPresence(ctx context.Context, studentIDs []int64, at time.Time) (int, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	rows, err := withCareExitTransaction(ctx, func(txCtx context.Context) (int64, error) {
		return r.closeOpenPresence(txCtx, studentIDs, at)
	})
	return int(rows), err
}

func (r *CareExitCleanupRepository) closeOpenPresence(ctx context.Context, studentIDs []int64, at time.Time) (int64, error) {
	tenantID := tenant.FromContext(ctx)
	total := 0
	for _, statement := range []string{
		`UPDATE active.attendance SET check_out_time = ?, updated_at = ?
		 WHERE tenant_id = ? AND student_id IN (?) AND check_out_time IS NULL`,
		`UPDATE active.visits SET exit_time = ?, updated_at = ?
		 WHERE tenant_id = ? AND student_id IN (?) AND exit_time IS NULL`,
	} {
		result, err := base.GetDB(ctx, r.db).ExecContext(ctx, statement, at, at, tenantID, bun.List(studentIDs))
		if err != nil {
			return 0, &modelBase.DatabaseError{Op: "close open presence", Err: base.TranslateNotFound(err)}
		}
		affected, _ := result.RowsAffected() // nil-driver-safe: fall through with 0
		total += int(affected)
	}
	rosterRows, err := r.assignments.CloseOpenStudentAssignments(ctx, studentIDs, at)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "close open presence", Err: err}
	}
	return int64(total) + rosterRows, nil
}

// CountPlannedByStudentIDsAfter counts the roster rows the child would lose,
// per student, for the "Betreuung beenden" preview.
//
// It counts against the BASELINE, not against what is left: a child who
// already has a planned exit had their later rows removed then, and those rows
// come back before the new last care day is applied (see RestoreRemovals). So
// the still-live rows and the restorable ledger rows are counted together —
// otherwise moving a planned exit from June to July would promise "0 Termine
// entfallen" while July's rows are restored and then removed again.
func (r *CareExitCleanupRepository) CountPlannedByStudentIDsAfter(ctx context.Context, studentIDs []int64, after timezone.Date) (map[int64]int, error) {
	if len(studentIDs) == 0 {
		return map[int64]int{}, nil
	}
	if err := r.requireCarePlan(); err != nil {
		return nil, err
	}
	removals, err := r.carePlan.ListCareExitRemovals(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("list care exit removals: %w", err)
	}
	counts, err := r.assignments.CountPlannedStudentAssignmentsAfter(ctx, studentIDs, after.String(), removals)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "count planned roster rows after care end", Err: err}
	}
	return counts, nil
}

// DeletePlannedByStudentIDsAfter drops the children from every roster dated
// after their last care day and records each row in the care-exit ledger in
// the same transaction.
//
// The snapshot is what makes a planned exit reversible (#2487): a cancellation
// that left the child active with an emptied plan would not be a cancellation.
// It is a verbatim copy rather than a note to rebuild from enrollments, for the
// reason the graduation path already learned (#405): an occurrence a supervisor
// customised by hand would otherwise come back plain.
//
// The ledger is dropped unreplayed once the exit takes effect and on a resume —
// the criteria require a returning child to be planned again by hand.
func (r *CareExitCleanupRepository) DeletePlannedByStudentIDsAfter(
	ctx context.Context, studentIDs []int64, after timezone.Date,
) (int, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	if err := r.requireCarePlan(); err != nil {
		return 0, err
	}
	affected, err := withCareExitTransaction(ctx, func(txCtx context.Context) (int64, error) {
		return r.deletePlannedByStudentIDsAfter(txCtx, studentIDs, after)
	})
	return int(affected), err
}

func (r *CareExitCleanupRepository) deletePlannedByStudentIDsAfter(
	ctx context.Context, studentIDs []int64, after timezone.Date,
) (int64, error) {
	removed, err := r.assignments.RemovePlannedStudentAssignmentsAfter(ctx, studentIDs, after.String())
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete planned roster rows after care end", Err: err}
	}
	if err := r.carePlan.RecordCareExitRemovals(ctx, removed); err != nil {
		return 0, &modelBase.DatabaseError{Op: "record planned roster rows after care end", Err: err}
	}
	return int64(len(removed)), nil
}

// LockPlanningForCareExit locks the live roster rows and bookings that a care
// exit can remove. The confirmation takes these locks before rebuilding its
// token, so a plan row cannot change between the shown preview and mutation.
func (r *CareExitCleanupRepository) LockPlanningForCareExit(
	ctx context.Context, studentIDs []int64, after timezone.Date,
) error {
	if len(studentIDs) == 0 {
		return nil
	}
	tenantID := tenant.FromContext(ctx)
	db := base.GetDB(ctx, r.db)
	if err := r.assignments.LockPlannedStudentAssignmentsAfter(ctx, studentIDs, after.String()); err != nil {
		return &modelBase.DatabaseError{Op: "lock planned roster rows for care exit", Err: err}
	}
	if err := r.requireActivityBookings(); err != nil {
		return err
	}
	if err := r.bookings.LockStudentEnrollmentsForCareExit(ctx, studentIDs, after.AddDays(1).String()); err != nil {
		return &modelBase.DatabaseError{Op: "lock bookings for care exit", Err: base.TranslateNotFound(err)}
	}
	if _, err := db.ExecContext(ctx, `
		SELECT rco.id
		FROM enrollment.request_child_offerings AS rco
		JOIN enrollment.request_children AS rc
		  ON rc.id = rco.request_child_id AND rc.tenant_id = rco.tenant_id
		WHERE rco.tenant_id = ? AND rc.created_student_id IN (?)
		  AND (rco.valid_until IS NULL OR rco.valid_until > ?)
		FOR UPDATE OF rco
	`, tenantID, bun.List(studentIDs), after.AddDays(1)); err != nil {
		return &modelBase.DatabaseError{Op: "lock source bookings for care exit", Err: base.TranslateNotFound(err)}
	}
	if err := r.requireCarePlan(); err != nil {
		return err
	}
	if err := r.carePlan.LockStudentSchedulesForCareExit(ctx, studentIDs, after.AddDays(1).String()); err != nil {
		return &modelBase.DatabaseError{Op: "lock weekly plan for care exit", Err: err}
	}
	return nil
}

// CountRunningByStudentIDsAfter counts the bookings CapByStudentIDs would
// touch, per student, for the preview. valid_until is an EXCLUSIVE upper
// bound, so the caller passes the day AFTER the last care day.
//
// Like the roster count this counts against the BASELINE: a booking a previous
// exit already capped or deleted is restored before the new cutoff is applied,
// so it belongs in the number the preview shows.
func (r *CareExitCleanupRepository) CountRunningByStudentIDsAfter(
	ctx context.Context, studentIDs []int64, validUntil timezone.Date,
) (map[int64]int, error) {
	counts := make(map[int64]int, len(studentIDs))
	if len(studentIDs) == 0 {
		return counts, nil
	}
	removals, err := r.careExitRemovalProjection(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	tenantID := tenant.FromContext(ctx)
	counts, err = timetableprojection.CountRunningEnrollmentsAfter(
		ctx, base.GetDB(ctx, r.db), tenantID, studentIDs, validUntil, removals,
	)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "count running bookings after care end", Err: base.TranslateNotFound(err)}
	}
	return counts, nil
}

func (r *CareExitCleanupRepository) ListSourceOfferingsAfter(
	ctx context.Context,
	studentIDs []int64,
	validUntil timezone.Date,
) (map[int64][]userModels.CareExitSourceOffering, error) {
	result := make(map[int64][]userModels.CareExitSourceOffering, len(studentIDs))
	if len(studentIDs) == 0 {
		return result, nil
	}
	offerings, err := r.careOfferingProjection(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		StudentID int64    `bun:"student_id"`
		Name      string   `bun:"name"`
		Days      []string `bun:"days,type:jsonb"`
	}
	if err := base.GetDB(ctx, r.db).NewRaw(`
		WITH care_offerings AS (
			SELECT * FROM jsonb_to_recordset(?::jsonb) AS offering(
				id bigint, tenant_id bigint, name text, days_of_week_mode text,
				available_days jsonb, counts_as_care boolean, sort_order integer
			)
		)
		SELECT rc.created_student_id AS student_id, co.name,
		       CASE WHEN co.days_of_week_mode = 'fixed' THEN co.available_days ELSE rco.selected_days END AS days
		FROM enrollment.request_child_offerings AS rco
		JOIN enrollment.request_children AS rc
		  ON rc.id = rco.request_child_id AND rc.tenant_id = rco.tenant_id
		JOIN care_offerings AS co
		  ON co.id = rco.care_offering_id AND co.tenant_id = rco.tenant_id
		WHERE rco.tenant_id = ? AND rc.created_student_id IN (?)
		  AND (rco.valid_until IS NULL OR rco.valid_until > ?)
		ORDER BY rc.created_student_id, co.sort_order, co.id
	`, offerings, tenant.FromContext(ctx), bun.List(studentIDs), validUntil).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list source offerings for care exit preview", Err: base.TranslateNotFound(err)}
	}
	for _, row := range rows {
		result[row.StudentID] = append(result[row.StudentID], userModels.CareExitSourceOffering{Name: row.Name, Days: row.Days})
	}
	return result, nil
}

func (r *CareExitCleanupRepository) ListWeeklyPlanPatterns(ctx context.Context, studentIDs []int64) (map[int64][]string, error) {
	if len(studentIDs) == 0 {
		return map[int64][]string{}, nil
	}
	if err := r.requireCarePlan(); err != nil {
		return nil, err
	}
	patterns, err := r.carePlan.ListWeeklyPlanPatterns(ctx, studentIDs)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list recurring weekly plans for care exit preview", Err: err}
	}
	return patterns, nil
}

// CapByStudentIDs ends every offering and activity booking of the given
// children at validUntil (exclusive), deleting the ones that would be left
// with no interval at all. Both halves update the ledger atomically so a
// cancelled or re-dated exit can put the bookings back (#2487).
//
// Unlike CapActiveByGroup this deliberately ignores provenance: the child
// leaves the school, so a booking materialized from an approved enrollment
// request has to end too.
func (r *CareExitCleanupRepository) CapByStudentIDs(
	ctx context.Context, studentIDs []int64, validUntil timezone.Date,
) (int64, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	if err := r.requireCarePlan(); err != nil {
		return 0, err
	}
	return withCareExitTransaction(ctx, func(txCtx context.Context) (int64, error) {
		return r.capByStudentIDs(txCtx, studentIDs, validUntil)
	})
}

func (r *CareExitCleanupRepository) capByStudentIDs(
	ctx context.Context, studentIDs []int64, validUntil timezone.Date,
) (int64, error) {
	if err := r.requireActivityBookings(); err != nil {
		return 0, err
	}
	changes, err := r.bookings.EndStudentEnrollmentsForCareExit(ctx, studentIDs, validUntil.String())
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "end activity bookings after care end", Err: err}
	}
	if err := r.carePlan.RecordCareExitRemovals(ctx, deletedBookingRemovals(changes.Deleted)); err != nil {
		return 0, &modelBase.DatabaseError{Op: "record deleted future bookings after care end", Err: err}
	}
	if err := r.carePlan.RecordCareExitRemovals(ctx, cappedBookingRemovals(changes.Capped)); err != nil {
		return 0, &modelBase.DatabaseError{Op: "record capped bookings after care end", Err: err}
	}
	return int64(len(changes.Deleted) + len(changes.Capped)), nil
}

func deletedBookingRemovals(rows []ActivityBooking) []CareExitRemoval {
	result := make([]CareExitRemoval, 0, len(rows))
	for _, row := range rows {
		enrollmentID, groupID := row.ID, row.ActivityGroupID
		from := timezone.Date(row.ValidFrom)
		result = append(result, CareExitRemoval{
			StudentID: row.StudentID, Kind: CareExitRemovalBooking, EnrollmentID: &enrollmentID,
			WasDeleted: true, PreviousValidUntil: carePlanDateString(row.ValidUntil), ActivityGroupID: &groupID,
			ValidFrom: &from, CalendarPeriodID: row.CalendarPeriodID,
			EnrollmentRequestChildID: row.EnrollmentRequestChildID,
			SelectedWeekdays:         row.SelectedWeekdays, AttendanceStatus: row.AttendanceStatus, Weekday: row.Weekday,
		})
	}
	return result
}

func cappedBookingRemovals(rows []ActivityBookingCap) []CareExitRemoval {
	result := make([]CareExitRemoval, 0, len(rows))
	for _, row := range rows {
		enrollmentID := row.ID
		result = append(result, CareExitRemoval{
			StudentID: row.StudentID, Kind: CareExitRemovalBooking, EnrollmentID: &enrollmentID,
			PreviousValidUntil: carePlanDateString(row.PreviousValidUntil),
		})
	}
	return result
}

func carePlanDateString(value *string) *timezone.Date {
	if value == nil {
		return nil
	}
	date := timezone.Date(*value)
	return &date
}

func activityBookingRemovals(encoded string) ([]ActivityBookingRemoval, error) {
	var ledger []CareExitRemoval
	if err := json.Unmarshal([]byte(encoded), &ledger); err != nil {
		return nil, fmt.Errorf("decode activity booking removals: %w", err)
	}
	result := make([]ActivityBookingRemoval, 0)
	for _, removal := range ledger {
		if removal.Kind != CareExitRemovalBooking || removal.EnrollmentID == nil {
			continue
		}
		result = append(result, activityBookingRemoval(removal))
	}
	return result, nil
}

func activityBookingRemoval(removal CareExitRemoval) ActivityBookingRemoval {
	booking := ActivityBooking{ID: *removal.EnrollmentID, TenantID: removal.TenantID, StudentID: removal.StudentID}
	if removal.ActivityGroupID != nil {
		booking.ActivityGroupID = *removal.ActivityGroupID
	}
	if removal.ValidFrom != nil {
		booking.ValidFrom = removal.ValidFrom.String()
	}
	booking.ValidUntil = dateString(removal.PreviousValidUntil)
	booking.CalendarPeriodID = removal.CalendarPeriodID
	booking.EnrollmentRequestChildID = removal.EnrollmentRequestChildID
	booking.SelectedWeekdays = removal.SelectedWeekdays
	booking.AttendanceStatus = removal.AttendanceStatus
	booking.Weekday = removal.Weekday
	return ActivityBookingRemoval{
		ActivityBooking: booking, WasDeleted: removal.WasDeleted,
		PreviousValidUntil: dateString(removal.PreviousValidUntil),
	}
}

func dateString(value *timezone.Date) *string {
	if value == nil {
		return nil
	}
	result := value.String()
	return &result
}

func (r *CareExitCleanupRepository) EndSourceBookingsAndSchedules(
	ctx context.Context,
	studentIDs []int64,
	validUntil timezone.Date,
	sourceRequestChildID *int64,
) (int64, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	if err := r.requireCarePlan(); err != nil {
		return 0, err
	}
	return withCareExitTransaction(ctx, func(txCtx context.Context) (int64, error) {
		bookings, err := r.endSourceBookings(txCtx, studentIDs, validUntil, sourceRequestChildID)
		if err != nil {
			return 0, err
		}
		plans, err := r.endCarePlanRows(txCtx, studentIDs, validUntil)
		return bookings + plans, err
	})
}

func (r *CareExitCleanupRepository) EndSourceBookings(
	ctx context.Context, studentIDs []int64, validUntil timezone.Date, sourceRequestChildID *int64,
) (int64, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	if err := r.requireCarePlan(); err != nil {
		return 0, err
	}
	return withCareExitTransaction(ctx, func(txCtx context.Context) (int64, error) {
		return r.endSourceBookings(txCtx, studentIDs, validUntil, sourceRequestChildID)
	})
}

func (r *CareExitCleanupRepository) endSourceBookings(
	ctx context.Context, studentIDs []int64, validUntil timezone.Date, sourceRequestChildID *int64,
) (int64, error) {
	tenantID := tenant.FromContext(ctx)
	if err := r.snapshotSourceBookings(ctx, studentIDs, validUntil, tenantID, sourceRequestChildID); err != nil {
		return 0, err
	}
	return r.endSourceBookingRows(ctx, studentIDs, validUntil, tenantID, sourceRequestChildID)
}

func (r *CareExitCleanupRepository) FindCareWithdrawalBookingExpiries(
	ctx context.Context, _ timezone.Date,
) ([]userModels.CareWithdrawalBookingChange, error) {
	offerings, err := r.careOfferingProjection(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]userModels.CareWithdrawalBookingChange, 0)
	err = base.GetDB(ctx, r.db).NewRaw(`
	WITH care_offerings AS (
		SELECT * FROM jsonb_to_recordset(?::jsonb) AS offering(
			id bigint, tenant_id bigint, name text, days_of_week_mode text,
			available_days jsonb, counts_as_care boolean, sort_order integer
		)
	)
	SELECT rc.created_student_id AS student_id,
	       rco.valid_until AS first_bookingless_day,
	       rco.request_child_id AS source_request_child_id,
		       jsonb_agg(jsonb_build_object(
				'name', co.name,
				'days', CASE WHEN co.days_of_week_mode = 'fixed' THEN co.available_days ELSE rco.selected_days END
			)) AS source_offerings
		FROM enrollment.request_child_offerings AS rco
		JOIN enrollment.request_children AS rc
		  ON rc.id = rco.request_child_id AND rc.tenant_id = rco.tenant_id
		JOIN care_offerings AS co
		  ON co.id = rco.care_offering_id AND co.tenant_id = rco.tenant_id
		JOIN users.students AS student
		  ON student.id = rc.created_student_id AND student.tenant_id = rc.tenant_id
		WHERE rco.tenant_id = ? AND rco.valid_until IS NOT NULL AND co.counts_as_care
		  AND ((co.days_of_week_mode = 'fixed' AND jsonb_array_length(co.available_days) > 0)
		    OR (co.days_of_week_mode <> 'fixed' AND jsonb_array_length(rco.selected_days) > 0))
		  AND student.status = 'active'
		  AND (student.enrolled_until IS NULL OR student.enrolled_until >= rco.valid_until)
		  AND NOT EXISTS (
			SELECT 1 FROM enrollment.request_child_offerings AS later
			JOIN care_offerings AS later_offering
			  ON later_offering.id = later.care_offering_id AND later_offering.tenant_id = later.tenant_id
			JOIN enrollment.request_children AS later_child
			  ON later_child.id = later.request_child_id AND later_child.tenant_id = later.tenant_id
			WHERE later.tenant_id = rco.tenant_id AND later_child.created_student_id = rc.created_student_id
			  AND later_offering.counts_as_care
			  AND ((later_offering.days_of_week_mode = 'fixed' AND jsonb_array_length(later_offering.available_days) > 0)
			    OR (later_offering.days_of_week_mode <> 'fixed' AND jsonb_array_length(later.selected_days) > 0))
			  AND COALESCE(later.valid_from, '-infinity'::date) <= rco.valid_until
			  AND (later.valid_until IS NULL OR later.valid_until > rco.valid_until)
		  )
		  AND NOT EXISTS (
			SELECT 1 FROM users.care_withdrawal_completions AS completion
			WHERE completion.tenant_id = rco.tenant_id
			  AND completion.student_id = rc.created_student_id
			  AND completion.first_bookingless_day = rco.valid_until
		  )
		GROUP BY rc.created_student_id, rco.valid_until, rco.request_child_id
	`, offerings, tenant.FromContext(ctx)).Scan(ctx, &rows)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find expired final care bookings", Err: base.TranslateNotFound(err)}
	}
	for index := range rows {
		rows[index].WasCompleteWithdrawal = true
	}
	return rows, nil
}

type careBookingPeriodRow struct {
	StudentID            int64          `bun:"student_id"`
	ValidFrom            *timezone.Date `bun:"valid_from"`
	ValidUntil           *timezone.Date `bun:"valid_until"`
	SourceRequestChildID int64          `bun:"source_request_child_id"`
	OfferingName         string         `bun:"offering_name"`
	Days                 []string       `bun:"days,type:jsonb"`
}

// ListCareBookingFacts reads facts without interpreting them. Date-window
// merging and the completion decision stay in the users service so mutation,
// setting, scheduler, and reader paths cannot acquire separate SQL rules.
func (r *CareExitCleanupRepository) ListCareBookingFacts(
	ctx context.Context, on timezone.Date, studentIDs []int64,
) ([]userModels.CareBookingFacts, error) {
	facts, err := r.listCareBookingStudents(ctx, on, studentIDs)
	if err != nil || len(facts) == 0 {
		return facts, err
	}
	if err := r.attachCareBookingPeriods(ctx, facts); err != nil {
		return nil, err
	}
	return facts, nil
}

func (r *CareExitCleanupRepository) listCareBookingStudents(
	ctx context.Context, on timezone.Date, studentIDs []int64,
) ([]userModels.CareBookingFacts, error) {
	facts := make([]userModels.CareBookingFacts, 0)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&facts).
		ModelTableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".id AS student_id`).
		ColumnExpr(`"person".first_name, "person".last_name`).
		ColumnExpr(`"student".school_class, "student".enrolled_until`).
		Join(`JOIN users.persons AS "person" ON "person".id = "student".person_id AND "person".tenant_id = "student".tenant_id`).
		Where(`"student".tenant_id = ?`, tenant.FromContext(ctx)).
		Where(`"student".status <> 'alumnus'`).
		OrderExpr(`"student".id`)
	if len(studentIDs) > 0 {
		query = query.Where(`"student".id IN (?)`, bun.List(studentIDs))
	} else {
		query = query.
			Where(`NOT ("student".enrolled_from IS NULL AND "student".enrolled_until IS NULL AND "student".status = ?)`, userModels.StudentStatusInactive).
			Where(`("student".enrolled_from IS NULL OR "student".enrolled_from <= ? OR "student".status = ?)`, on, userModels.StudentStatusActive).
			Where(`("student".enrolled_until IS NULL OR "student".enrolled_until >= ?)`, on)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list current care students for booking evaluation", Err: base.TranslateNotFound(err)}
	}
	return facts, nil
}

func (r *CareExitCleanupRepository) attachCareBookingPeriods(
	ctx context.Context, facts []userModels.CareBookingFacts,
) error {
	loadedIDs := make([]int64, len(facts))
	byStudent := make(map[int64]*userModels.CareBookingFacts, len(facts))
	for index := range facts {
		loadedIDs[index] = facts[index].StudentID
		byStudent[facts[index].StudentID] = &facts[index]
	}
	rows, err := r.listCareBookingPeriods(ctx, loadedIDs)
	if err != nil {
		return err
	}
	for _, row := range rows {
		child := byStudent[row.StudentID]
		if child == nil {
			continue
		}
		child.Periods = append(child.Periods, userModels.CareBookingPeriod{
			ValidFrom:            row.ValidFrom,
			ValidUntil:           row.ValidUntil,
			Days:                 row.Days,
			SourceRequestChildID: row.SourceRequestChildID,
			SourceOfferings: []userModels.CareExitSourceOffering{{
				Name: row.OfferingName,
				Days: row.Days,
			}},
		})
	}
	return nil
}

func (r *CareExitCleanupRepository) listCareBookingPeriods(
	ctx context.Context, studentIDs []int64,
) ([]careBookingPeriodRow, error) {
	offerings, err := r.careOfferingProjection(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]careBookingPeriodRow, 0)
	err = base.GetDB(ctx, r.db).NewRaw(`
		WITH care_offerings AS (
			SELECT * FROM jsonb_to_recordset(?::jsonb) AS offering(
				id bigint, tenant_id bigint, name text, days_of_week_mode text,
				available_days jsonb, counts_as_care boolean, sort_order integer
			)
		)
		SELECT COALESCE(rc.created_student_id, rc.matched_student_id) AS student_id, rco.valid_from, rco.valid_until,
		       rco.request_child_id AS source_request_child_id, co.name AS offering_name,
		       CASE WHEN co.days_of_week_mode = 'fixed' THEN co.available_days ELSE rco.selected_days END AS days
		FROM enrollment.request_child_offerings AS rco
		JOIN enrollment.request_children AS rc
		  ON rc.id = rco.request_child_id AND rc.tenant_id = rco.tenant_id
		JOIN care_offerings AS co
		  ON co.id = rco.care_offering_id AND co.tenant_id = rco.tenant_id
		WHERE rco.tenant_id = ? AND COALESCE(rc.created_student_id, rc.matched_student_id) IN (?)
		  AND rc.status = ? AND co.counts_as_care
		  AND ((co.days_of_week_mode = 'fixed' AND jsonb_array_length(co.available_days) > 0)
		    OR (co.days_of_week_mode <> 'fixed' AND jsonb_array_length(rco.selected_days) > 0))
		ORDER BY COALESCE(rc.created_student_id, rc.matched_student_id), rco.valid_from NULLS FIRST, rco.valid_until NULLS LAST, rco.id
	`, offerings, tenant.FromContext(ctx), bun.List(studentIDs), enrollmentModels.ChildStatusApproved).Scan(ctx, &rows)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list care booking periods for evaluation", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

func (r *CareExitCleanupRepository) snapshotSourceBookings(ctx context.Context, studentIDs []int64, validUntil timezone.Date, tenantID int64, sourceRequestChildID *int64) error {
	removals := make([]CareExitSourceRemoval, 0)
	if err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT rco.tenant_id, rc.created_student_id AS student_id,
		       ?::text AS kind, rco.id AS source_row_id,
		       COALESCE(rco.valid_from, '-infinity'::date) >= ? AS was_deleted,
		       to_jsonb(rco) AS snapshot
		FROM enrollment.request_child_offerings AS rco
		JOIN enrollment.request_children AS rc
		  ON rc.id = rco.request_child_id AND rc.tenant_id = rco.tenant_id
		WHERE rco.tenant_id = ? AND rc.created_student_id IN (?)
		  AND (? IS NULL OR rco.request_child_id = ?)
		  AND (rco.valid_until IS NULL OR rco.valid_until > ?)
		`, CareExitSourceBooking, validUntil, tenantID, bun.List(studentIDs), sourceRequestChildID, sourceRequestChildID, validUntil).Scan(ctx, &removals); err != nil {
		return &modelBase.DatabaseError{Op: "snapshot source bookings before care exit", Err: base.TranslateNotFound(err)}
	}
	if err := r.carePlan.RecordCareExitSourceRemovals(ctx, removals); err != nil {
		return &modelBase.DatabaseError{Op: "record source bookings before care exit", Err: err}
	}
	return nil
}

func (r *CareExitCleanupRepository) endSourceBookingRows(ctx context.Context, studentIDs []int64, validUntil timezone.Date, tenantID int64, sourceRequestChildID *int64) (int64, error) {
	db := base.GetDB(ctx, r.db)
	deleted, err := db.ExecContext(ctx, `
		DELETE FROM enrollment.request_child_offerings AS rco
		USING enrollment.request_children AS rc
		WHERE rc.id = rco.request_child_id AND rc.tenant_id = rco.tenant_id
		  AND rco.tenant_id = ? AND rc.created_student_id IN (?)
		  AND (? IS NULL OR rco.request_child_id = ?)
		  AND COALESCE(rco.valid_from, '-infinity'::date) >= ?
		  AND (rco.valid_until IS NULL OR rco.valid_until > ?)
	`, tenantID, bun.List(studentIDs), sourceRequestChildID, sourceRequestChildID, validUntil, validUntil)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete future source bookings after care exit", Err: base.TranslateNotFound(err)}
	}
	deletedRows, _ := deleted.RowsAffected()
	capped, err := db.ExecContext(ctx, `
		UPDATE enrollment.request_child_offerings AS rco
		SET valid_until = ?, updated_at = NOW()
		FROM enrollment.request_children AS rc
		WHERE rc.id = rco.request_child_id AND rc.tenant_id = rco.tenant_id
		  AND rco.tenant_id = ? AND rc.created_student_id IN (?)
		  AND (? IS NULL OR rco.request_child_id = ?)
		  AND COALESCE(rco.valid_from, '-infinity'::date) < ?
		  AND (rco.valid_until IS NULL OR rco.valid_until > ?)
	`, validUntil, tenantID, bun.List(studentIDs), sourceRequestChildID, sourceRequestChildID, validUntil, validUntil)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "cap source bookings after care exit", Err: base.TranslateNotFound(err)}
	}
	cappedRows, _ := capped.RowsAffected()
	return deletedRows + cappedRows, nil
}

func (r *CareExitCleanupRepository) endCarePlanRows(ctx context.Context, studentIDs []int64, validUntil timezone.Date) (int64, error) {
	if err := r.requireCarePlan(); err != nil {
		return 0, err
	}
	rows, err := r.carePlan.EndStudentSchedulesForCareExit(ctx, studentIDs, validUntil.String())
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "end weekly plans after care exit", Err: err}
	}
	return rows, nil
}

// RestoreRemovals puts back everything the children's current care exit took
// away and empties their ledger (#2487).
//
// It is the inverse of DeletePlannedByStudentIDsAfter + CapByStudentIDs, and it
// runs in two places: when a planned exit is CANCELLED, and at the start of
// every re-run of "Betreuung beenden" over the same child, so changing the last
// care day always applies to the untouched plan instead of to the remains of
// the previous attempt.
//
// It is deliberately forgiving. A roster row somebody re-created by hand, a
// room or status day deleted since, an instance that has meanwhile completed:
// none of those may fail the restore, because the alternative is a child stuck
// in a state nobody can leave. Skipped rows are simply not restored — the ledger
// is cleared either way, since it describes one exit and that exit is over.
func (r *CareExitCleanupRepository) RestoreRemovals(
	ctx context.Context, studentIDs []int64,
) (int, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		var restored int
		err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
			var restoreErr error
			restored, restoreErr = r.restoreRemovals(txCtx, studentIDs)
			return restoreErr
		})
		return restored, err
	}
	return r.restoreRemovals(ctx, studentIDs)
}

func (r *CareExitCleanupRepository) restoreRemovals(ctx context.Context, studentIDs []int64) (int, error) {
	tenantID := tenant.FromContext(ctx)
	db := base.GetDB(ctx, r.db)
	restored := 0
	removals, err := r.careExitRemovalProjection(ctx, studentIDs)
	if err != nil {
		return 0, err
	}
	sourceRemovals, err := r.careExitSourceRemovalProjection(ctx, studentIDs)
	if err != nil {
		return 0, err
	}

	// Rosters. room_id / student_status_day_id / pickup_exception_id are
	// re-validated instead of trusted: all three are ON DELETE SET NULL on the
	// live table, so a snapshot may point at something that is gone, and a bare
	// insert would fail the whole restore over a deleted room. Rooms are
	// validated through their owner (#2665), the other two in place.
	var archivedRoomIDs []int64
	if err := db.NewRaw(`SELECT DISTINCT rm.room_id FROM `+careExitRemovalRecordset+`
		WHERE rm.kind = 'roster' AND rm.tenant_id = ?
		  AND rm.student_id IN (?) AND rm.room_id IS NOT NULL`,
		removals, tenantID, bun.List(studentIDs)).Scan(ctx, &archivedRoomIDs); err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore roster rows after care exit change", Err: base.TranslateNotFound(err)}
	}
	roomIDs, err := validRoomIDs(ctx, r.rooms, tenantID, archivedRoomIDs)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore roster rows after care exit change", Err: err}
	}
	var archivedPickupExceptionIDs []int64
	if err := db.NewRaw(`SELECT DISTINCT rm.pickup_exception_id FROM `+careExitRemovalRecordset+`
		WHERE rm.kind = 'roster' AND rm.tenant_id = ?
		  AND rm.student_id IN (?) AND rm.pickup_exception_id IS NOT NULL`,
		removals, tenantID, bun.List(studentIDs)).Scan(ctx, &archivedPickupExceptionIDs); err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore roster rows after care exit change", Err: base.TranslateNotFound(err)}
	}
	validPickupExceptionIDs, err := r.carePlan.ExistingPickupExceptionIDs(ctx, archivedPickupExceptionIDs)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore roster rows after care exit change", Err: err}
	}
	var archivedStatusDayIDs []int64
	if err := db.NewRaw(`SELECT DISTINCT rm.student_status_day_id FROM `+careExitRemovalRecordset+`
		WHERE rm.kind = 'roster' AND rm.tenant_id = ?
		  AND rm.student_id IN (?) AND rm.student_status_day_id IS NOT NULL`,
		removals, tenantID, bun.List(studentIDs)).Scan(ctx, &archivedStatusDayIDs); err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore roster rows after care exit change", Err: base.TranslateNotFound(err)}
	}
	validStatusDayIDs, err := r.carePlan.ExistingStudentStatusDayIDs(ctx, archivedStatusDayIDs)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore roster rows after care exit change", Err: err}
	}
	var rosterRemovals []CareExitRemoval
	if err := json.Unmarshal([]byte(removals), &rosterRemovals); err != nil {
		return 0, fmt.Errorf("decode care exit roster removals: %w", err)
	}
	rosterRows, err := r.assignments.RestoreCareExitStudentAssignments(ctx, studentIDs, roomIDs, validStatusDayIDs, validPickupExceptionIDs, rosterRemovals)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore roster rows after care exit change", Err: err}
	}
	restored += int(rosterRows)

	// The Timetable owner restores capped and deleted bookings. It keeps the
	// original ids, re-validates the enrollment provenance, and treats duplicate
	// recreation as an idempotent no-op.
	if r.periods == nil {
		return 0, errCalendarPeriodDirectoryRequired
	}
	periodIDs, err := r.periods.ListCalendarPeriodIDs(ctx)
	if err != nil {
		return 0, err
	}
	if err := r.requireActivityBookings(); err != nil {
		return 0, err
	}
	bookingRemovals, err := activityBookingRemovals(removals)
	if err != nil {
		return 0, err
	}
	bookingRows, err := r.bookings.RestoreStudentEnrollmentsForCareExit(ctx, studentIDs, periodIDs, bookingRemovals)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore activity bookings after care exit change", Err: err}
	}
	restored += bookingRows

	// Source bookings capped by the exit recover their original exclusive end.
	if _, err := db.ExecContext(ctx, `
		UPDATE enrollment.request_child_offerings AS rco
		SET valid_until = NULLIF(rm.snapshot->>'valid_until', '')::date,
		    updated_at = NOW()
		FROM `+careExitSourceRemovalRecordset+`
		WHERE rm.kind = 'source_booking' AND rm.was_deleted = FALSE
		  AND rm.tenant_id = ? AND rm.student_id IN (?)
		  AND rco.tenant_id = rm.tenant_id AND rco.id = rm.source_row_id
	`, sourceRemovals, tenantID, bun.List(studentIDs)); err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore capped source bookings after care exit", Err: base.TranslateNotFound(err)}
	}

	// Deleted source bookings retain their original ids. The Care Plan owner
	// restores its weekly schedules and exceptions from the same ledger.
	sourceResult, err := db.ExecContext(ctx, `INSERT INTO enrollment.request_child_offerings
		SELECT (jsonb_populate_record(NULL::enrollment.request_child_offerings, rm.snapshot)).*
		FROM `+careExitSourceRemovalRecordset+`
		WHERE rm.kind = 'source_booking' AND rm.was_deleted = TRUE
		  AND rm.tenant_id = ? AND rm.student_id IN (?)
		ON CONFLICT DO NOTHING`, sourceRemovals, tenantID, bun.List(studentIDs))
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore source booking after care exit", Err: base.TranslateNotFound(err)}
	}
	sourceRows, _ := sourceResult.RowsAffected()
	restored += int(sourceRows)
	planRows, err := r.carePlan.RestoreStudentSchedulesForCareExit(ctx, studentIDs)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore weekly plans after care exit", Err: err}
	}
	restored += int(planRows)

	// Rosters are restored before their pickup exceptions because the original
	// roster ledger is shared with older exits. Reconnect the FK now that the
	// exception snapshots are back; otherwise cancellation would silently turn
	// an exception-bound roster row into an ordinary row.
	validPickupExceptionIDs, err = r.carePlan.ExistingPickupExceptionIDs(ctx, archivedPickupExceptionIDs)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "reconnect restored roster pickup exception", Err: err}
	}
	if err := r.assignments.ReconnectCareExitAssignmentPickupExceptions(ctx, studentIDs, validPickupExceptionIDs, rosterRemovals); err != nil {
		return 0, &modelBase.DatabaseError{Op: "reconnect restored roster pickup exception", Err: err}
	}

	if err := r.DiscardRemovals(ctx, studentIDs); err != nil {
		return 0, err
	}
	return restored, nil
}

// DiscardRemovals drops the ledger without replaying it. Used when the exit
// becomes final (the effect day) and on a resume, where the acceptance criteria
// explicitly require the school to plan the returning child again by hand
// rather than have last term's plan switch itself back on.
func (r *CareExitCleanupRepository) DiscardRemovals(
	ctx context.Context, studentIDs []int64,
) error {
	if len(studentIDs) == 0 {
		return nil
	}
	if r.carePlan == nil {
		return errors.New("care exit cleanup requires the Care Plan capability")
	}
	if err := r.carePlan.DiscardCareExitRemovals(ctx, studentIDs); err != nil {
		return &modelBase.DatabaseError{Op: "discard care exit removals", Err: err}
	}
	return nil
}
