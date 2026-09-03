package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
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
	db      *bun.DB
	periods CalendarPeriodDirectory
	// rooms re-validates snapshot room references through the Facilities
	// owner on restore (#2665).
	rooms    RoomDirectory
	carePlan CarePlanDirectory
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

type CareExitRemoval struct {
	ID                       int64
	TenantID                 int64
	StudentID                int64
	Kind                     string
	InstanceID               *int64
	RoomID                   *int64
	Status                   *string
	Substatus                *string
	Note                     *string
	IsUnplanned              *bool
	NotScheduled             *bool
	ManualStatusAt           *time.Time
	StudentStatusDayID       *int64
	PickupExceptionID        *int64
	EnrollmentID             *int64
	WasDeleted               bool
	PreviousValidUntil       *timezone.Date
	ActivityGroupID          *int64
	ValidFrom                *timezone.Date
	CalendarPeriodID         *int64
	EnrollmentRequestChildID *int64
	SelectedWeekdays         []int
	AttendanceStatus         *string
	Weekday                  *int
	CreatedAt                time.Time
}

type CareExitSourceRemoval struct {
	ID          int64
	TenantID    int64
	StudentID   int64
	Kind        string
	SourceRowID int64
	WasDeleted  bool
	Snapshot    json.RawMessage
	CreatedAt   time.Time
}

const (
	CareExitRemovalRoster          = "roster"
	CareExitRemovalBooking         = "booking"
	CareExitSourceBooking          = "booking"
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
func NewCareExitCleanupRepository(db *bun.DB) userModels.CareExitCleanupRepository {
	return &CareExitCleanupRepository{db: db}
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

// openRequestQueues names the four parent request tables and the status value
// each considers "still open". Kept as data so counting and closing can never
// disagree about which rows the preview promised and the confirmation touches.
var openRequestQueues = []struct {
	Table   string
	Pending string
}{
	{"users.student_data_change_requests", userModels.DataChangeStatusPending},
	{"active.excused_absence_requests", activeModels.ExcusedRequestStatusPending},
	{"schedule.care_schedule_change_requests", scheduleModels.CareRequestStatusPending},
}

type careExitPlanTable struct {
	Kind, Table, DateColumn string
}

type bookingRemoval struct {
	ID                       int64          `bun:"id"`
	TenantID                 int64          `bun:"tenant_id"`
	StudentID                int64          `bun:"student_id"`
	ActivityGroupID          int64          `bun:"activity_group_id"`
	ValidFrom                timezone.Date  `bun:"valid_from"`
	ValidUntil               *timezone.Date `bun:"valid_until"`
	CalendarPeriodID         *int64         `bun:"calendar_period_id"`
	EnrollmentRequestChildID *int64         `bun:"enrollment_request_child_id"`
	SelectedWeekdays         []int          `bun:"selected_weekdays,type:jsonb"`
	AttendanceStatus         *string        `bun:"attendance_status"`
	Weekday                  *int           `bun:"weekday"`
}

type bookingCapSnapshot struct {
	StudentID  int64          `bun:"student_id"`
	ID         int64          `bun:"id"`
	ValidUntil *timezone.Date `bun:"valid_until"`
}

var careExitPlanTables = []careExitPlanTable{
	{Kind: CareExitSourcePickupSchedule, Table: "schedule.student_pickup_schedules"}, {Kind: CareExitSourceArrivalSchedule, Table: "schedule.student_arrival_schedules"}, {Kind: CareExitSourcePickupException, Table: "schedule.student_pickup_exceptions", DateColumn: "exception_date"}, {Kind: CareExitSourceArrivalException, Table: "schedule.student_arrival_exceptions", DateColumn: "exception_date"},
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
	tenantID := tenant.FromContext(ctx)
	for _, queue := range openRequestQueues {
		var rows []struct {
			StudentID int64 `bun:"student_id"`
			Total     int   `bun:"total"`
		}
		sql := `SELECT student_id, COUNT(*)::int AS total FROM ` + queue.Table + `
			WHERE tenant_id = ? AND student_id IN (?) AND status = ?
			GROUP BY student_id`
		if err := base.GetDB(ctx, r.db).NewRaw(sql, tenantID, bun.List(studentIDs), queue.Pending).Scan(ctx, &rows); err != nil {
			return nil, &modelBase.DatabaseError{Op: "count open parent requests", Err: base.TranslateNotFound(err)}
		}
		for _, row := range rows {
			counts[row.StudentID] += row.Total
		}
	}
	if r.carePlan == nil {
		return nil, errors.New("care exit cleanup requires the Care Plan capability")
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
	tenantID := tenant.FromContext(ctx)
	for _, queue := range openRequestQueues {
		sql := `SELECT id FROM ` + queue.Table + ` WHERE tenant_id = ? AND student_id IN (?) AND status = ? FOR UPDATE`
		if _, err := base.GetDB(ctx, r.db).ExecContext(ctx, sql, tenantID, bun.List(studentIDs), queue.Pending); err != nil {
			return &modelBase.DatabaseError{Op: "lock open parent requests for care exit", Err: base.TranslateNotFound(err)}
		}
	}
	if r.carePlan == nil {
		return errors.New("care exit cleanup requires the Care Plan capability")
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
	tenantID := tenant.FromContext(ctx)
	total := 0

	// The Stammdaten queue names its decision column review_reason and has no
	// applied_at semantics for a non-decision; the other three share the
	// decision_reason / applied_at shape.
	statements := []struct {
		SQL  string
		Args []any
	}{
		{
			`UPDATE users.student_data_change_requests
			 SET status = ?, review_reason = ?, reviewed_by = ?, reviewed_at = ?, updated_at = ?
			 WHERE tenant_id = ? AND student_id IN (?) AND status = ?`,
			[]any{userModels.DataChangeStatusCareEnded, userModels.CareEndedDecisionReason, reviewedBy, at, at,
				tenantID, bun.List(studentIDs), userModels.DataChangeStatusPending},
		},
		{
			`UPDATE active.excused_absence_requests
			 SET status = ?, decision_reason = ?, reviewed_by = ?, reviewed_at = ?, updated_at = ?
			 WHERE tenant_id = ? AND student_id IN (?) AND status = ?`,
			[]any{activeModels.ExcusedRequestStatusCareEnded, userModels.CareEndedDecisionReason, reviewedBy, at, at,
				tenantID, bun.List(studentIDs), activeModels.ExcusedRequestStatusPending},
		},
		{
			`UPDATE schedule.care_schedule_change_requests
			 SET status = ?, decision_reason = ?, reviewed_by = ?, reviewed_at = ?, updated_at = ?
			 WHERE tenant_id = ? AND student_id IN (?) AND status = ?`,
			[]any{scheduleModels.CareRequestStatusCareEnded, userModels.CareEndedDecisionReason, reviewedBy, at, at,
				tenantID, bun.List(studentIDs), scheduleModels.CareRequestStatusPending},
		},
	}

	for _, statement := range statements {
		result, err := base.GetDB(ctx, r.db).ExecContext(ctx, statement.SQL, statement.Args...)
		if err != nil {
			return 0, &modelBase.DatabaseError{Op: "close open parent requests", Err: base.TranslateNotFound(err)}
		}
		affected, _ := result.RowsAffected() // nil-driver-safe: fall through with 0
		total += int(affected)
	}
	if r.carePlan == nil {
		return 0, errors.New("care exit cleanup requires the Care Plan capability")
	}
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
		UNION
		SELECT student_id FROM schedule.instance_students
		WHERE tenant_id = ? AND student_id IN (?)
		  AND checked_in_at IS NOT NULL AND checked_out_at IS NULL
	`, tenant.FromContext(ctx), bun.List(studentIDs),
		tenant.FromContext(ctx), bun.List(studentIDs),
		tenant.FromContext(ctx), bun.List(studentIDs)).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find open presence", Err: base.TranslateNotFound(err)}
	}
	for _, row := range rows {
		present[row.StudentID] = true
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
		{"lock roster presence for care exit", `SELECT id FROM schedule.instance_students WHERE tenant_id = ? AND student_id IN (?) AND checked_in_at IS NOT NULL AND checked_out_at IS NULL FOR UPDATE`},
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement.sql, tenantID, bun.List(studentIDs)); err != nil {
			return &modelBase.DatabaseError{Op: statement.op, Err: base.TranslateNotFound(err)}
		}
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
			UNION ALL
			SELECT instance.date AS day
			FROM schedule.instance_students AS roster
			JOIN schedule.activity_instances AS instance
			  ON instance.tenant_id = roster.tenant_id AND instance.id = roster.instance_id
			WHERE roster.tenant_id = ? AND roster.student_id = ?
			  AND roster.checked_in_at IS NOT NULL
		) AS recorded
	`, tenant.FromContext(ctx), studentID,
		tenant.FromContext(ctx), studentID,
		tenant.FromContext(ctx), studentID).Scan(ctx, &day); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find latest attendance before care exit", Err: base.TranslateNotFound(err)}
	}
	return day, nil
}

// CloseOpenPresence closes whatever the children still have open at the moment
// their care ends: the attendance row, the room visit, and the roster
// check-in. Nothing is deleted — the day that happened stays in the history,
// it just stops being an unfinished one (#2487).
func (r *CareExitCleanupRepository) CloseOpenPresence(
	ctx context.Context, studentIDs []int64, at time.Time,
) (int, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	tenantID := tenant.FromContext(ctx)
	total := 0
	for _, statement := range []string{
		`UPDATE active.attendance SET check_out_time = ?, updated_at = ?
		 WHERE tenant_id = ? AND student_id IN (?) AND check_out_time IS NULL`,
		`UPDATE active.visits SET exit_time = ?, updated_at = ?
		 WHERE tenant_id = ? AND student_id IN (?) AND exit_time IS NULL`,
		`UPDATE schedule.instance_students SET checked_out_at = ?, updated_at = ?
		 WHERE tenant_id = ? AND student_id IN (?) AND checked_in_at IS NOT NULL AND checked_out_at IS NULL`,
	} {
		result, err := base.GetDB(ctx, r.db).ExecContext(ctx, statement, at, at, tenantID, bun.List(studentIDs))
		if err != nil {
			return 0, &modelBase.DatabaseError{Op: "close open presence", Err: base.TranslateNotFound(err)}
		}
		affected, _ := result.RowsAffected() // nil-driver-safe: fall through with 0
		total += int(affected)
	}
	return total, nil
}

// carePlannedRosterPredicate is the shared WHERE tail for the two roster
// methods below. "Still planned" means the row records no event: no check-in
// stamp, no checkout stamp. Every affected instance is dated strictly after
// the child's last care day and therefore lies in the future, so a status
// somebody set by hand there is a plan too and goes with it — leaving it would
// keep the departed child on future slot lists, staffing ratios and exports,
// which is exactly what ending the care has to stop.
//
// Cancelled and completed instances are skipped: a cancelled block plans
// nothing, and a completed one is history.
const carePlannedRosterPredicate = `
	  AND s.checked_in_at IS NULL
	  AND s.checked_out_at IS NULL
	  AND ai.date > ?
	  AND ai.status NOT IN ('completed', 'cancelled')
	  AND s.tenant_id = ?`

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
	tenantID := tenant.FromContext(ctx)
	var rows []struct {
		StudentID int64 `bun:"student_id"`
		Total     int   `bun:"total"`
	}
	sql := `
		SELECT student_id, COUNT(*)::int AS total FROM (
			SELECT s.student_id AS student_id
			FROM schedule.instance_students AS s
			JOIN schedule.activity_instances AS ai
			  ON ai.id = s.instance_id AND ai.tenant_id = s.tenant_id
			WHERE s.student_id IN (?)` + carePlannedRosterPredicate + `
			UNION ALL
			SELECT rm.student_id AS student_id
			FROM jsonb_to_recordset(?::jsonb) AS rm(
			     tenant_id bigint, student_id bigint, kind text, instance_id bigint
			)
			JOIN schedule.activity_instances AS ai
			  ON ai.id = rm.instance_id AND ai.tenant_id = rm.tenant_id
			WHERE rm.kind = 'roster'
			  AND rm.tenant_id = ?
			  AND rm.student_id IN (?)
			  AND ai.date > ?
			  AND ai.status NOT IN ('completed', 'cancelled')
			  -- Somebody may have put the child back on that block by hand
			  -- since. Then the live branch above already counted it, and the
			  -- restore will skip it.
			  AND NOT EXISTS (
			        SELECT 1 FROM schedule.instance_students AS live
			         WHERE live.instance_id = rm.instance_id
			           AND live.student_id = rm.student_id
			           AND live.tenant_id = rm.tenant_id
			      )
		) AS baseline
		GROUP BY student_id`
	if err := base.GetDB(ctx, r.db).NewRaw(sql,
		bun.List(studentIDs), after, tenantID,
		removals, tenantID, bun.List(studentIDs), after,
	).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "count planned roster rows after care end", Err: base.TranslateNotFound(err)}
	}
	for _, row := range rows {
		counts[row.StudentID] = row.Total
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
	tenantID := tenant.FromContext(ctx)
	removed := make([]CareExitRemoval, 0)
	err := base.GetDB(ctx, r.db).NewRaw(`
		DELETE FROM schedule.instance_students AS s
		USING schedule.activity_instances AS ai
		WHERE s.instance_id = ai.id
		  AND s.tenant_id = ai.tenant_id
		  AND s.student_id IN (?)`+carePlannedRosterPredicate+`
		RETURNING s.tenant_id, s.student_id, ?::text AS kind,
		          s.instance_id, s.room_id, s.status, s.substatus, s.note,
		          s.is_unplanned, s.not_scheduled, s.manual_status_at,
		          s.student_status_day_id, s.pickup_exception_id`,
		bun.List(studentIDs), after, tenantID, CareExitRemovalRoster,
	).Scan(ctx, &removed)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete planned roster rows after care end", Err: base.TranslateNotFound(err)}
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
	if _, err := db.ExecContext(ctx, `
		SELECT s.instance_id
		FROM schedule.instance_students AS s
		JOIN schedule.activity_instances AS ai ON ai.id = s.instance_id AND ai.tenant_id = s.tenant_id
		WHERE s.student_id IN (?)`+carePlannedRosterPredicate+`
		FOR UPDATE OF s`, bun.List(studentIDs), after, tenantID); err != nil {
		return &modelBase.DatabaseError{Op: "lock planned roster rows for care exit", Err: base.TranslateNotFound(err)}
	}
	if _, err := db.ExecContext(ctx, `
		SELECT e.id
		FROM activities.student_enrollments AS e
		WHERE e.tenant_id = ? AND e.student_id IN (?)
		  AND (e.valid_until IS NULL OR e.valid_until > ?)
		FOR UPDATE OF e`, tenantID, bun.List(studentIDs), after.AddDays(1)); err != nil {
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
	for _, statement := range []string{
		`SELECT id FROM schedule.student_pickup_schedules WHERE tenant_id = ? AND student_id IN (?) FOR UPDATE`,
		`SELECT id FROM schedule.student_arrival_schedules WHERE tenant_id = ? AND student_id IN (?) FOR UPDATE`,
		`SELECT id FROM schedule.student_pickup_exceptions WHERE tenant_id = ? AND student_id IN (?) AND exception_date > ? FOR UPDATE`,
		`SELECT id FROM schedule.student_arrival_exceptions WHERE tenant_id = ? AND student_id IN (?) AND exception_date > ? FOR UPDATE`,
	} {
		args := []any{tenantID, bun.List(studentIDs)}
		if strings.Contains(statement, "exception_date") {
			args = append(args, after)
		}
		if _, err := db.ExecContext(ctx, statement, args...); err != nil {
			return &modelBase.DatabaseError{Op: "lock weekly plan for care exit", Err: base.TranslateNotFound(err)}
		}
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
	var rows []struct {
		StudentID int64 `bun:"student_id"`
		Total     int   `bun:"total"`
	}
	if err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT student_id, COUNT(*)::int AS total FROM (
			-- Still live, and either open-ended or running past the cutoff. A
			-- row a previous exit capped is joined to its ledger entry so it is
			-- judged by the interval it will be RESTORED to, not by the capped
			-- one.
			SELECT e.student_id AS student_id
			FROM activities.student_enrollments AS e
			LEFT JOIN jsonb_to_recordset(?::jsonb) AS rm(
			     tenant_id bigint, student_id bigint, kind text,
			     enrollment_id bigint, was_deleted boolean,
			     previous_valid_until date
			)
			       ON rm.kind = 'booking'
			      AND rm.tenant_id = e.tenant_id
			      AND rm.enrollment_id = e.id
			      AND rm.was_deleted = FALSE
			WHERE e.tenant_id = ? AND e.student_id IN (?)
			  AND (
			        (rm.enrollment_id IS NULL AND (e.valid_until IS NULL OR e.valid_until > ?))
			        OR (rm.enrollment_id IS NOT NULL AND (rm.previous_valid_until IS NULL OR rm.previous_valid_until > ?))
			      )
			UNION ALL
			-- Deleted outright by a previous exit; the restore writes it back.
			SELECT rm.student_id AS student_id
			FROM jsonb_to_recordset(?::jsonb) AS rm(
			     tenant_id bigint, student_id bigint, kind text,
			     enrollment_id bigint, was_deleted boolean,
			     previous_valid_until date
			)
			WHERE rm.kind = 'booking'
			  AND rm.was_deleted = TRUE
			  AND rm.tenant_id = ?
			  AND rm.student_id IN (?)
			  AND (rm.previous_valid_until IS NULL OR rm.previous_valid_until > ?)
			  -- Same guard as above: a booking re-created under that id in the
			  -- meantime is already counted by the live branch.
			  AND NOT EXISTS (
			        SELECT 1 FROM activities.student_enrollments AS live
			         WHERE live.id = rm.enrollment_id
			           AND live.tenant_id = rm.tenant_id
			      )
		) AS baseline
		GROUP BY student_id
	`, removals, tenantID, bun.List(studentIDs), validUntil, validUntil,
		removals, tenantID, bun.List(studentIDs), validUntil).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "count running bookings after care end", Err: base.TranslateNotFound(err)}
	}
	for _, row := range rows {
		counts[row.StudentID] = row.Total
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
	patterns := make(map[int64][]string, len(studentIDs))
	if len(studentIDs) == 0 {
		return patterns, nil
	}
	var rows []struct {
		StudentID int64  `bun:"student_id"`
		Pattern   string `bun:"pattern"`
	}
	if err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT student_id, pattern FROM (
			SELECT student_id,
			       'Ankunft am ' || CASE weekday
			         WHEN 1 THEN 'Montag' WHEN 2 THEN 'Dienstag' WHEN 3 THEN 'Mittwoch'
			         WHEN 4 THEN 'Donnerstag' WHEN 5 THEN 'Freitag' END ||
			       COALESCE(': ' || TO_CHAR(expected_arrival, 'HH24:MI'), '') AS pattern
			FROM schedule.student_arrival_schedules
			WHERE tenant_id = ? AND student_id IN (?)
			UNION ALL
			SELECT student_id,
			       'Abholung am ' || CASE weekday
			         WHEN 1 THEN 'Montag' WHEN 2 THEN 'Dienstag' WHEN 3 THEN 'Mittwoch'
			         WHEN 4 THEN 'Donnerstag' WHEN 5 THEN 'Freitag' END ||
			       ': ' || TO_CHAR(pickup_time, 'HH24:MI') AS pattern
			FROM schedule.student_pickup_schedules
			WHERE tenant_id = ? AND student_id IN (?)
		) AS patterns ORDER BY student_id, pattern
	`, tenant.FromContext(ctx), bun.List(studentIDs), tenant.FromContext(ctx), bun.List(studentIDs)).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list recurring weekly plans for care exit preview", Err: base.TranslateNotFound(err)}
	}
	for _, row := range rows {
		patterns[row.StudentID] = append(patterns[row.StudentID], row.Pattern)
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
	db, tenantID := base.GetDB(ctx, r.db), tenant.FromContext(ctx)
	deleted, err := deleteFutureBookings(ctx, db, tenantID, studentIDs, validUntil)
	if err != nil {
		return 0, err
	}
	if err := r.carePlan.RecordCareExitRemovals(ctx, deletedBookingRemovals(deleted)); err != nil {
		return 0, &modelBase.DatabaseError{Op: "record deleted future bookings after care end", Err: err}
	}
	capped, err := listCappedBookings(ctx, db, tenantID, studentIDs, validUntil)
	if err != nil {
		return 0, err
	}
	if err := r.carePlan.RecordCareExitRemovals(ctx, cappedBookingRemovals(capped)); err != nil {
		return 0, &modelBase.DatabaseError{Op: "record capped bookings after care end", Err: err}
	}
	cappedRows, err := capBookings(ctx, db, tenantID, studentIDs, validUntil)
	return int64(len(deleted)) + cappedRows, err
}

func deleteFutureBookings(
	ctx context.Context, db bun.IDB, tenantID int64, studentIDs []int64, validUntil timezone.Date,
) ([]bookingRemoval, error) {
	removed := make([]bookingRemoval, 0)
	err := db.NewRaw(`
		DELETE FROM activities.student_enrollments
		WHERE tenant_id = ? AND student_id IN (?) AND valid_from >= ?
		  AND (valid_until IS NULL OR valid_until > ?)
		RETURNING id, tenant_id, student_id, activity_group_id, valid_from,
		          valid_until, calendar_period_id, enrollment_request_child_id,
		          selected_weekdays, attendance_status, weekday
	`, tenantID, bun.List(studentIDs), validUntil, validUntil).Scan(ctx, &removed)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "delete future bookings after care end", Err: base.TranslateNotFound(err)}
	}
	return removed, nil
}

const capBookingPredicate = `
	e.tenant_id = ? AND e.student_id IN (?) AND e.valid_from < ?
	  AND (e.valid_until IS NULL OR e.valid_until > ?)`

func listCappedBookings(
	ctx context.Context, db bun.IDB, tenantID int64, studentIDs []int64, validUntil timezone.Date,
) ([]bookingCapSnapshot, error) {
	rows := make([]bookingCapSnapshot, 0)
	err := db.NewRaw(`SELECT e.student_id, e.id, e.valid_until
		FROM activities.student_enrollments AS e WHERE `+capBookingPredicate,
		tenantID, bun.List(studentIDs), validUntil, validUntil).Scan(ctx, &rows)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "record capped bookings after care end", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

func capBookings(
	ctx context.Context, db bun.IDB, tenantID int64, studentIDs []int64, validUntil timezone.Date,
) (int64, error) {
	result, err := db.ExecContext(ctx, `
		UPDATE activities.student_enrollments AS e
		SET valid_until = ?, updated_at = NOW()
		WHERE `+capBookingPredicate, validUntil, tenantID, bun.List(studentIDs), validUntil, validUntil)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "cap bookings after care end", Err: base.TranslateNotFound(err)}
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

func deletedBookingRemovals(rows []bookingRemoval) []CareExitRemoval {
	result := make([]CareExitRemoval, 0, len(rows))
	for _, row := range rows {
		enrollmentID, groupID := row.ID, row.ActivityGroupID
		from := timezone.Date(row.ValidFrom.String())
		result = append(result, CareExitRemoval{
			StudentID: row.StudentID, Kind: CareExitRemovalBooking, EnrollmentID: &enrollmentID,
			WasDeleted: true, PreviousValidUntil: carePlanDate(row.ValidUntil), ActivityGroupID: &groupID,
			ValidFrom: &from, CalendarPeriodID: row.CalendarPeriodID,
			EnrollmentRequestChildID: row.EnrollmentRequestChildID,
			SelectedWeekdays:         row.SelectedWeekdays, AttendanceStatus: row.AttendanceStatus, Weekday: row.Weekday,
		})
	}
	return result
}

func cappedBookingRemovals(rows []bookingCapSnapshot) []CareExitRemoval {
	result := make([]CareExitRemoval, 0, len(rows))
	for _, row := range rows {
		enrollmentID := row.ID
		result = append(result, CareExitRemoval{
			StudentID: row.StudentID, Kind: CareExitRemovalBooking, EnrollmentID: &enrollmentID,
			PreviousValidUntil: carePlanDate(row.ValidUntil),
		})
	}
	return result
}

func carePlanDate(value *timezone.Date) *timezone.Date {
	if value == nil {
		return nil
	}
	date := timezone.Date(value.String())
	return &date
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
		plans, err := r.endCarePlanRows(txCtx, studentIDs, validUntil, tenant.FromContext(txCtx))
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

func (r *CareExitCleanupRepository) endCarePlanRows(ctx context.Context, studentIDs []int64, validUntil timezone.Date, tenantID int64) (int64, error) {
	db := base.GetDB(ctx, r.db)
	var total int64
	for _, item := range careExitPlanTables {
		datePredicate := ""
		args := []any{tenantID, bun.List(studentIDs)}
		if item.DateColumn != "" {
			datePredicate = " AND " + item.DateColumn + " >= ?"
			args = append(args, validUntil)
		}
		removals := make([]CareExitSourceRemoval, 0)
		snapshotSQL := `SELECT tenant_id, student_id, '` + item.Kind + `'::text AS kind,
			id AS source_row_id, TRUE AS was_deleted, to_jsonb(plan) AS snapshot
			FROM ` + item.Table + ` AS plan
			WHERE tenant_id = ? AND student_id IN (?)` + datePredicate
		if err := db.NewRaw(snapshotSQL, args...).Scan(ctx, &removals); err != nil {
			return 0, &modelBase.DatabaseError{Op: "snapshot " + item.Kind + " before care exit", Err: base.TranslateNotFound(err)}
		}
		if err := r.carePlan.RecordCareExitSourceRemovals(ctx, removals); err != nil {
			return 0, &modelBase.DatabaseError{Op: "record " + item.Kind + " before care exit", Err: err}
		}
		deleteSQL := `DELETE FROM ` + item.Table + ` WHERE tenant_id = ? AND student_id IN (?)` + datePredicate
		deleted, err := db.ExecContext(ctx, deleteSQL, args...)
		if err != nil {
			return 0, &modelBase.DatabaseError{Op: "delete " + item.Kind + " after care exit", Err: base.TranslateNotFound(err)}
		}
		rows, _ := deleted.RowsAffected()
		total += rows
	}
	return total, nil
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
	rosterResult, err := db.ExecContext(ctx, `
		INSERT INTO schedule.instance_students (
			tenant_id, instance_id, student_id, room_id, status, substatus, note,
			is_unplanned, not_scheduled, manual_status_at, student_status_day_id,
			pickup_exception_id
		)
		SELECT rm.tenant_id, rm.instance_id, rm.student_id,
		       CASE WHEN rm.room_id = ANY(?) THEN rm.room_id END,
		       rm.status, rm.substatus, rm.note,
		       COALESCE(rm.is_unplanned, FALSE), COALESCE(rm.not_scheduled, FALSE),
		       rm.manual_status_at,
		       (SELECT sd.id FROM active.student_status_days AS sd
		         WHERE sd.tenant_id = rm.tenant_id AND sd.id = rm.student_status_day_id),
		       (SELECT pe.id FROM schedule.student_pickup_exceptions AS pe
		         WHERE pe.tenant_id = rm.tenant_id AND pe.id = rm.pickup_exception_id)
		FROM `+careExitRemovalRecordset+`
		JOIN schedule.activity_instances AS ai
		  ON ai.tenant_id = rm.tenant_id AND ai.id = rm.instance_id
		WHERE rm.kind = 'roster'
		  AND rm.tenant_id = ?
		  AND rm.student_id IN (?)
		  AND ai.status NOT IN ('completed', 'cancelled')
		ON CONFLICT DO NOTHING
	`, pgdialect.Array(roomIDs), removals, tenantID, bun.List(studentIDs))
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore roster rows after care exit change", Err: base.TranslateNotFound(err)}
	}
	rosterRows, _ := rosterResult.RowsAffected() // nil-driver-safe: fall through with 0
	restored += int(rosterRows)

	// Bookings that were only capped: the previous upper bound goes back on.
	cappedResult, err := db.ExecContext(ctx, `
		UPDATE activities.student_enrollments AS e
		SET valid_until = rm.previous_valid_until, updated_at = NOW()
		FROM `+careExitRemovalRecordset+`
		WHERE rm.kind = 'booking'
		  AND rm.was_deleted = FALSE
		  AND rm.tenant_id = e.tenant_id
		  AND rm.enrollment_id = e.id
		  AND rm.tenant_id = ?
		  AND rm.student_id IN (?)
	`, removals, tenantID, bun.List(studentIDs))
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore capped bookings after care exit change", Err: base.TranslateNotFound(err)}
	}
	cappedRows, _ := cappedResult.RowsAffected()
	restored += int(cappedRows)

	// Bookings that were deleted: written back under their ORIGINAL id, so
	// anything that referenced the booking still finds it. ON CONFLICT DO
	// NOTHING carries no target on purpose — besides the primary key there is a
	// partial unique index over the open-ended bookings, and a booking somebody
	// re-created in the meantime must be left alone, not error out the restore.
	// The calendar period reference is re-validated against the School
	// Calendar owner's surviving ids (#2666): a snapshot that points at a
	// deleted period restores as NULL, exactly like the former subquery.
	if r.periods == nil {
		return 0, errCalendarPeriodDirectoryRequired
	}
	periodIDs, err := r.periods.ListCalendarPeriodIDs(ctx)
	if err != nil {
		return 0, err
	}
	deletedResult, err := db.ExecContext(ctx, `
		INSERT INTO activities.student_enrollments (
			id, tenant_id, student_id, activity_group_id, valid_from, valid_until,
			calendar_period_id, enrollment_request_child_id, selected_weekdays,
			attendance_status, weekday
		)
		SELECT rm.enrollment_id, rm.tenant_id, rm.student_id, rm.activity_group_id,
		       rm.valid_from, rm.previous_valid_until,
		       CASE WHEN rm.calendar_period_id = ANY(?::BIGINT[]) THEN rm.calendar_period_id END,
		       (SELECT rc.id FROM enrollment.request_children AS rc
		         WHERE rc.tenant_id = rm.tenant_id AND rc.id = rm.enrollment_request_child_id),
		       rm.selected_weekdays, rm.attendance_status, rm.weekday
		FROM `+careExitRemovalRecordset+`
		JOIN activities.groups AS g
		  ON g.tenant_id = rm.tenant_id AND g.id = rm.activity_group_id
		WHERE rm.kind = 'booking'
		  AND rm.was_deleted = TRUE
		  AND rm.tenant_id = ?
		  AND rm.student_id IN (?)
		ON CONFLICT DO NOTHING
	`, pgdialect.Array(periodIDs), removals, tenantID, bun.List(studentIDs))
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore deleted bookings after care exit change", Err: base.TranslateNotFound(err)}
	}
	deletedRows, _ := deletedResult.RowsAffected()
	restored += int(deletedRows)

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

	// Deleted source and weekly rows retain their original ids. This preserves
	// references such as roster pickup_exception_id when cancellation restores
	// both ledgers.
	restores := append([]careExitPlanTable{{Kind: "source_booking", Table: "enrollment.request_child_offerings"}}, careExitPlanTables...)
	for _, item := range restores {
		sql := `INSERT INTO ` + item.Table + `
			SELECT (jsonb_populate_record(NULL::` + item.Table + `, rm.snapshot)).*
			FROM ` + careExitSourceRemovalRecordset + `
			WHERE rm.kind = ? AND rm.was_deleted = TRUE
			  AND rm.tenant_id = ? AND rm.student_id IN (?)
			ON CONFLICT DO NOTHING`
		result, err := db.ExecContext(ctx, sql, sourceRemovals, item.Kind, tenantID, bun.List(studentIDs))
		if err != nil {
			return 0, &modelBase.DatabaseError{Op: "restore " + item.Kind + " after care exit", Err: base.TranslateNotFound(err)}
		}
		rows, _ := result.RowsAffected()
		restored += int(rows)
	}

	// Rosters are restored before their pickup exceptions because the original
	// roster ledger is shared with older exits. Reconnect the FK now that the
	// exception snapshots are back; otherwise cancellation would silently turn
	// an exception-bound roster row into an ordinary row.
	if _, err := db.ExecContext(ctx, `
		UPDATE schedule.instance_students AS live
		SET pickup_exception_id = rm.pickup_exception_id
		FROM `+careExitRemovalRecordset+`
		JOIN schedule.student_pickup_exceptions AS pe
		  ON pe.tenant_id = rm.tenant_id AND pe.id = rm.pickup_exception_id
		WHERE rm.kind = 'roster'
		  AND rm.tenant_id = ? AND rm.student_id IN (?)
		  AND live.tenant_id = rm.tenant_id
		  AND live.instance_id = rm.instance_id
		  AND live.student_id = rm.student_id
	`, removals, tenantID, bun.List(studentIDs)); err != nil {
		return 0, &modelBase.DatabaseError{Op: "reconnect restored roster pickup exception", Err: base.TranslateNotFound(err)}
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
