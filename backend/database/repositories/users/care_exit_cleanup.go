package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment"
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
// Enrollment's application and offering-link facts arrive as owner
// projections and commands (#2695); the remaining SQL joins those recordsets.
// Keeping them together also keeps the counting half (the preview) and the
// writing half (the confirmation) side by side, where a divergence is
// visible.
type CareExitCleanupRepository struct {
	db          *bun.DB
	assignments CareExitAssignments
	presence    CareExitPresence
	periods     CalendarPeriodDirectory
	carePlan    CarePlanDirectory
	bookings    ActivityBookingDirectory
	enrollment  CareExitEnrollmentQueries
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
	LockPlannedRosterForCareExit(context.Context, []int64, string) error
	RemovePlannedRosterForCareExit(context.Context, []int64, string) ([]CareExitRemoval, error)
	RestoreRosterForCareExit(context.Context, []int64, []CareExitRemoval) (int, error)
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
func NewCareExitCleanupRepository(db *bun.DB, enrollment CareExitEnrollmentQueries, assignments CareExitAssignments, presence CareExitPresence) userModels.CareExitCleanupRepository {
	if enrollment == nil {
		panic("care exit cleanup requires Enrollment queries")
	}
	if assignments == nil {
		panic("care exit cleanup: timetable assignments are required")
	}
	if presence == nil {
		panic("care exit cleanup: student presence is required")
	}
	return &CareExitCleanupRepository{db: db, enrollment: enrollment, assignments: assignments, presence: presence}
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

// careExitOfferingLinkRecordset is the Enrollment offering-link projection
// (#2695) the care-exit reads join instead of enrollment.request_child_offerings.
const careExitOfferingLinkRecordset = `offering_links AS (
 SELECT * FROM jsonb_to_recordset(?::jsonb) AS link(
 id bigint, tenant_id bigint, request_child_id bigint, care_offering_id bigint,
 selected_days jsonb, valid_from date, valid_until date
 )
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
	ids, err := r.presence.ListOpenPresence(ctx, studentIDs)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find open presence", Err: base.TranslateNotFound(err)}
	}
	for _, id := range ids {
		present[id] = true
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
	childIDs, err := r.enrollment.CreatedStudentRequestChildIDs(ctx, studentIDs)
	if err != nil {
		return &modelBase.DatabaseError{Op: "find source applications for care exit lock", Err: err}
	}
	links, err := r.enrollment.CareExitOfferingLinks(ctx, studentIDs)
	if err != nil {
		return &modelBase.DatabaseError{Op: "find source offerings for care exit lock", Err: err}
	}
	offeringIDs := sourceOfferingIDs(links, childIDs)
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
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement.sql, tenantID, bun.List(studentIDs)); err != nil {
			return &modelBase.DatabaseError{Op: statement.op, Err: base.TranslateNotFound(err)}
		}
	}
	if err := r.presence.LockOpenPresence(ctx, studentIDs); err != nil {
		return &modelBase.DatabaseError{Op: "lock presence for care exit", Err: err}
	}
	if err := r.assignments.LockOpenStudentAssignments(ctx, studentIDs); err != nil {
		return &modelBase.DatabaseError{Op: "lock roster presence for care exit", Err: err}
	}
	return nil
}

func (r *CareExitCleanupRepository) LatestAttendanceDate(ctx context.Context, studentID int64) (*timezone.Date, error) {
	rosterDay, err := timetableprojection.LatestRosterAttendanceDate(ctx, base.GetDB(ctx, r.db), tenant.FromContext(ctx), studentID)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find latest attendance before care exit", Err: base.TranslateNotFound(err)}
	}
	value, err := r.presence.LatestPresenceDate(ctx, studentID)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find latest attendance before care exit", Err: base.TranslateNotFound(err)}
	}
	var day *timezone.Date
	if value != nil {
		parsed, err := timezone.ParseDate(*value)
		if err != nil {
			return nil, &modelBase.DatabaseError{Op: "find latest attendance before care exit", Err: err}
		}
		day = &parsed
	}
	if rosterDay != nil && (day == nil || rosterDay.After(*day)) {
		day = rosterDay
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
	total, err := r.presence.CloseOpenPresence(ctx, studentIDs, at)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "close open presence", Err: base.TranslateNotFound(err)}
	}
	rosterRows, err := r.assignments.CloseOpenStudentAssignments(ctx, studentIDs, at)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "close open presence", Err: err}
	}
	return total + rosterRows, nil
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
func (r *CareExitCleanupRepository) CountPlannedByStudentIDsAfter(
	ctx context.Context, studentIDs []int64, after timezone.Date,
) (map[int64]int, error) {
	counts := make(map[int64]int, len(studentIDs))
	if len(studentIDs) == 0 {
		return counts, nil
	}
	removals, err := r.careExitRemovalProjection(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	counts, err = timetableprojection.CountPlannedRosterAfter(ctx, base.GetDB(ctx, r.db), tenant.FromContext(ctx), studentIDs, after, removals)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "count planned roster rows after care end", Err: base.TranslateNotFound(err)}
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
	if err := r.requireActivityBookings(); err != nil {
		return 0, err
	}
	removed, err := r.bookings.RemovePlannedRosterForCareExit(ctx, studentIDs, after.String())
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
	if err := r.requireActivityBookings(); err != nil {
		return err
	}
	if err := r.bookings.LockPlannedRosterForCareExit(ctx, studentIDs, after.String()); err != nil {
		return &modelBase.DatabaseError{Op: "lock planned roster rows for care exit", Err: base.TranslateNotFound(err)}
	}
	if err := r.requireActivityBookings(); err != nil {
		return err
	}
	if err := r.bookings.LockStudentEnrollmentsForCareExit(ctx, studentIDs, after.AddDays(1).String()); err != nil {
		return &modelBase.DatabaseError{Op: "lock bookings for care exit", Err: base.TranslateNotFound(err)}
	}
	childIDs, err := r.enrollment.CreatedStudentRequestChildIDs(ctx, studentIDs)
	if err != nil {
		return &modelBase.DatabaseError{Op: "find source applications for care exit lock", Err: err}
	}
	if err := r.enrollment.LockCareExitOfferingLinks(ctx, childIDs, enrollment.Date(after.AddDays(1))); err != nil {
		return &modelBase.DatabaseError{Op: "lock source bookings for care exit", Err: err}
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
	links, linksErr := r.careExitApplicationProjection(ctx, studentIDs)
	if linksErr != nil {
		return nil, linksErr
	}
	result := make(map[int64][]userModels.CareExitSourceOffering, len(studentIDs))
	if len(studentIDs) == 0 {
		return result, nil
	}
	offerings, err := r.careOfferingProjection(ctx)
	if err != nil {
		return nil, err
	}
	offeringLinks, err := r.careExitOfferingLinkProjection(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		StudentID int64    `bun:"student_id"`
		Name      string   `bun:"name"`
		Days      []string `bun:"days,type:jsonb"`
	}
	if err := base.GetDB(ctx, r.db).NewRaw(`
		WITH application_links AS (
 SELECT * FROM jsonb_to_recordset(?::jsonb) AS application(
 id bigint, tenant_id bigint, created_student_id bigint, matched_student_id bigint, status text
 )
), care_offerings AS (
			SELECT * FROM jsonb_to_recordset(?::jsonb) AS offering(
				id bigint, tenant_id bigint, name text, days_of_week_mode text,
				available_days jsonb, counts_as_care boolean, sort_order integer
			)
		), `+careExitOfferingLinkRecordset+`
		SELECT rc.created_student_id AS student_id, co.name,
		       CASE WHEN co.days_of_week_mode = 'fixed' THEN co.available_days ELSE rco.selected_days END AS days
		FROM offering_links AS rco
		JOIN application_links AS rc
		  ON rc.id = rco.request_child_id AND rc.tenant_id = rco.tenant_id
		JOIN care_offerings AS co
		  ON co.id = rco.care_offering_id AND co.tenant_id = rco.tenant_id
		WHERE rco.tenant_id = ? AND rc.created_student_id IN (?)
		  AND (rco.valid_until IS NULL OR rco.valid_until > ?)
		ORDER BY rc.created_student_id, co.sort_order, co.id
	`, links, offerings, offeringLinks, tenant.FromContext(ctx), bun.List(studentIDs), validUntil).Scan(ctx, &rows); err != nil {
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
	links, linksErr := r.careExitApplicationProjection(ctx, nil)
	if linksErr != nil {
		return nil, linksErr
	}
	offerings, err := r.careOfferingProjection(ctx)
	if err != nil {
		return nil, err
	}
	offeringLinks, err := r.careExitOfferingLinkProjection(ctx, nil)
	if err != nil {
		return nil, err
	}
	rows := make([]userModels.CareWithdrawalBookingChange, 0)
	err = base.GetDB(ctx, r.db).NewRaw(`
	WITH application_links AS (
 SELECT * FROM jsonb_to_recordset(?::jsonb) AS application(
 id bigint, tenant_id bigint, created_student_id bigint, matched_student_id bigint, status text
 )
), care_offerings AS (
		SELECT * FROM jsonb_to_recordset(?::jsonb) AS offering(
			id bigint, tenant_id bigint, name text, days_of_week_mode text,
			available_days jsonb, counts_as_care boolean, sort_order integer
		)
	), `+careExitOfferingLinkRecordset+`
	SELECT rc.created_student_id AS student_id,
	       rco.valid_until AS first_bookingless_day,
	       rco.request_child_id AS source_request_child_id,
		       jsonb_agg(jsonb_build_object(
				'name', co.name,
				'days', CASE WHEN co.days_of_week_mode = 'fixed' THEN co.available_days ELSE rco.selected_days END
			)) AS source_offerings
		FROM offering_links AS rco
		JOIN application_links AS rc
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
			SELECT 1 FROM offering_links AS later
			JOIN care_offerings AS later_offering
			  ON later_offering.id = later.care_offering_id AND later_offering.tenant_id = later.tenant_id
			JOIN application_links AS later_child
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
	`, links, offerings, offeringLinks, tenant.FromContext(ctx)).Scan(ctx, &rows)
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
	links, linksErr := r.careExitApplicationProjection(ctx, studentIDs)
	if linksErr != nil {
		return nil, linksErr
	}
	offerings, err := r.careOfferingProjection(ctx)
	if err != nil {
		return nil, err
	}
	offeringLinks, err := r.careExitOfferingLinkProjection(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	rows := make([]careBookingPeriodRow, 0)
	err = base.GetDB(ctx, r.db).NewRaw(`
		WITH application_links AS (
 SELECT * FROM jsonb_to_recordset(?::jsonb) AS application(
 id bigint, tenant_id bigint, created_student_id bigint, matched_student_id bigint, status text
 )
), care_offerings AS (
			SELECT * FROM jsonb_to_recordset(?::jsonb) AS offering(
				id bigint, tenant_id bigint, name text, days_of_week_mode text,
				available_days jsonb, counts_as_care boolean, sort_order integer
			)
		), `+careExitOfferingLinkRecordset+`
		SELECT COALESCE(rc.created_student_id, rc.matched_student_id) AS student_id, rco.valid_from, rco.valid_until,
		       rco.request_child_id AS source_request_child_id, co.name AS offering_name,
		       CASE WHEN co.days_of_week_mode = 'fixed' THEN co.available_days ELSE rco.selected_days END AS days
		FROM offering_links AS rco
		JOIN application_links AS rc
		  ON rc.id = rco.request_child_id AND rc.tenant_id = rco.tenant_id
		JOIN care_offerings AS co
		  ON co.id = rco.care_offering_id AND co.tenant_id = rco.tenant_id
		WHERE rco.tenant_id = ? AND COALESCE(rc.created_student_id, rc.matched_student_id) IN (?)
		  AND rc.status = ? AND co.counts_as_care
		  AND ((co.days_of_week_mode = 'fixed' AND jsonb_array_length(co.available_days) > 0)
		    OR (co.days_of_week_mode <> 'fixed' AND jsonb_array_length(rco.selected_days) > 0))
		ORDER BY COALESCE(rc.created_student_id, rc.matched_student_id), rco.valid_from NULLS FIRST, rco.valid_until NULLS LAST, rco.id
	`, links, offerings, offeringLinks, tenant.FromContext(ctx), bun.List(studentIDs), enrollmentModels.ChildStatusApproved).Scan(ctx, &rows)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list care booking periods for evaluation", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

func (r *CareExitCleanupRepository) snapshotSourceBookings(ctx context.Context, studentIDs []int64, validUntil timezone.Date, tenantID int64, sourceRequestChildID *int64) error {
	snapshots, err := r.enrollment.CareExitOfferingSnapshots(ctx, studentIDs, enrollment.Date(validUntil), sourceRequestChildID)
	if err != nil {
		return &modelBase.DatabaseError{Op: "snapshot source bookings before care exit", Err: err}
	}
	removals := make([]CareExitSourceRemoval, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot.TenantID != tenantID {
			continue
		}
		removals = append(removals, CareExitSourceRemoval{
			TenantID: snapshot.TenantID, StudentID: snapshot.StudentID, Kind: CareExitSourceBooking,
			SourceRowID: snapshot.SourceRowID, WasDeleted: snapshot.WasDeleted, Snapshot: snapshot.Snapshot,
		})
	}
	if err := r.carePlan.RecordCareExitSourceRemovals(ctx, removals); err != nil {
		return &modelBase.DatabaseError{Op: "record source bookings before care exit", Err: err}
	}
	return nil
}

func (r *CareExitCleanupRepository) endSourceBookingRows(ctx context.Context, studentIDs []int64, validUntil timezone.Date, _ int64, sourceRequestChildID *int64) (int64, error) {
	childIDs, err := r.enrollment.CreatedStudentRequestChildIDs(ctx, studentIDs)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "find source applications for care exit", Err: err}
	}
	if len(childIDs) == 0 {
		return 0, nil
	}
	changed, err := r.enrollment.EndCareExitOfferingLinks(ctx, childIDs, sourceRequestChildID, enrollment.Date(validUntil))
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "end source bookings after care exit", Err: err}
	}
	return changed, nil
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
	restored := 0
	removals, err := r.careExitRemovalProjection(ctx, studentIDs)
	if err != nil {
		return 0, err
	}
	sourceRemovals, err := r.carePlan.ListCareExitSourceRemovals(ctx, studentIDs)
	if err != nil {
		return 0, fmt.Errorf("list care exit source removals: %w", err)
	}

	// Timetable validates the surviving room and care-plan references and
	// restores its roster in this same outer transaction.
	if err := r.requireActivityBookings(); err != nil {
		return 0, err
	}
	var ledger []CareExitRemoval
	if err := json.Unmarshal([]byte(removals), &ledger); err != nil {
		return 0, fmt.Errorf("decode care exit roster removals: %w", err)
	}
	rosterRows, err := r.bookings.RestoreRosterForCareExit(ctx, studentIDs, ledger)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore roster rows after care exit change", Err: err}
	}
	restored += rosterRows
	var archivedPickupExceptionIDs []int64
	for _, removal := range ledger {
		if removal.Kind == CareExitRemovalRoster && removal.PickupExceptionID != nil {
			archivedPickupExceptionIDs = append(archivedPickupExceptionIDs, *removal.PickupExceptionID)
		}
	}

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

	// Source bookings capped by the exit recover their original exclusive end
	// and deleted ones retain their original ids; Enrollment owns both writes.
	// The Care Plan owner restores its weekly schedules and exceptions from the
	// same ledger.
	sourceRows, err := r.enrollment.RestoreCareExitOfferingLinks(ctx, sourceBookingRestores(sourceRemovals, tenantID, studentIDs))
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore source booking after care exit", Err: err}
	}
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
	validPickupExceptionIDs, err := r.carePlan.ExistingPickupExceptionIDs(ctx, archivedPickupExceptionIDs)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "reconnect restored roster pickup exception", Err: err}
	}
	if err := r.assignments.ReconnectCareExitAssignmentPickupExceptions(ctx, studentIDs, validPickupExceptionIDs, ledger); err != nil {
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

// CareExitEnrollmentQueries is the Enrollment capability care-exit cleanup
// reads source applications and their offering links through. The offering
// link writes (#2695) are Enrollment commands on the caller's transaction.
type CareExitEnrollmentQueries interface {
	CreatedStudentRequestChildIDs(context.Context, []int64) ([]int64, error)
	CareExitApplicationLinks(context.Context, []int64) ([]enrollment.CareExitApplicationLink, error)
	CareExitOfferingLinks(context.Context, []int64) ([]enrollment.CareOfferingLink, error)
	LockCareExitOfferingLinks(context.Context, []int64, enrollment.Date) error
	CareExitOfferingSnapshots(context.Context, []int64, enrollment.Date, *int64) ([]enrollment.CareExitOfferingSnapshot, error)
	EndCareExitOfferingLinks(context.Context, []int64, *int64, enrollment.Date) (int64, error)
	RestoreCareExitOfferingLinks(context.Context, []enrollment.CareExitOfferingSnapshotRestore) (int64, error)
}

func (r *CareExitCleanupRepository) careExitOfferingLinkProjection(ctx context.Context, studentIDs []int64) (string, error) {
	links, err := r.enrollment.CareExitOfferingLinks(ctx, studentIDs)
	if err != nil {
		return "", fmt.Errorf("load care-exit offering links: %w", err)
	}
	encoded, err := json.Marshal(links)
	if err != nil {
		return "", fmt.Errorf("encode care-exit offering links: %w", err)
	}
	return string(encoded), nil
}

// sourceOfferingIDs names the distinct offerings the given source applications
// select, in ascending order.
func sourceOfferingIDs(links []enrollment.CareOfferingLink, requestChildIDs []int64) []int64 {
	children := make(map[int64]struct{}, len(requestChildIDs))
	for _, id := range requestChildIDs {
		children[id] = struct{}{}
	}
	seen := make(map[int64]struct{})
	ids := make([]int64, 0)
	for _, link := range links {
		if _, ok := children[link.RequestChildID]; !ok {
			continue
		}
		if _, ok := seen[link.CareOfferingID]; ok {
			continue
		}
		seen[link.CareOfferingID] = struct{}{}
		ids = append(ids, link.CareOfferingID)
	}
	slices.Sort(ids)
	return ids
}

// sourceBookingRestores selects the source-booking ledger entries of the given
// students that belong to the current tenant.
func sourceBookingRestores(removals []CareExitSourceRemoval, tenantID int64, studentIDs []int64) []enrollment.CareExitOfferingSnapshotRestore {
	students := make(map[int64]struct{}, len(studentIDs))
	for _, id := range studentIDs {
		students[id] = struct{}{}
	}
	restores := make([]enrollment.CareExitOfferingSnapshotRestore, 0, len(removals))
	for _, removal := range removals {
		if removal.Kind != CareExitSourceBooking || removal.TenantID != tenantID {
			continue
		}
		if _, ok := students[removal.StudentID]; !ok {
			continue
		}
		restores = append(restores, enrollment.CareExitOfferingSnapshotRestore{
			SourceRowID: removal.SourceRowID, WasDeleted: removal.WasDeleted, Snapshot: removal.Snapshot,
		})
	}
	return restores
}

func (r *CareExitCleanupRepository) careExitApplicationProjection(ctx context.Context, studentIDs []int64) (string, error) {
	links, err := r.enrollment.CareExitApplicationLinks(ctx, studentIDs)
	if err != nil {
		return "", fmt.Errorf("load care-exit application links: %w", err)
	}
	encoded, err := json.Marshal(links)
	if err != nil {
		return "", fmt.Errorf("encode care-exit application links: %w", err)
	}
	return string(encoded), nil
}
