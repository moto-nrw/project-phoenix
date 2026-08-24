package users

import (
	"context"
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
	db *bun.DB
}

// NewCareExitCleanupRepository builds the repository.
func NewCareExitCleanupRepository(db *bun.DB) userModels.CareExitCleanupRepository {
	return &CareExitCleanupRepository{db: db}
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
	{"enrollment.offering_change_requests", enrollmentModels.OfferingChangeStatusPending},
	{"schedule.care_schedule_change_requests", scheduleModels.CareRequestStatusPending},
}

type careExitPlanTable struct {
	Kind, Table, DateColumn string
}

var careExitPlanTables = []careExitPlanTable{
	{Kind: "pickup_schedule", Table: "schedule.student_pickup_schedules"},
	{Kind: "arrival_schedule", Table: "schedule.student_arrival_schedules"},
	{Kind: "pickup_exception", Table: "schedule.student_pickup_exceptions", DateColumn: "exception_date"},
	{Kind: "arrival_exception", Table: "schedule.student_arrival_exceptions", DateColumn: "exception_date"},
}

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
			return nil, &modelBase.DatabaseError{Op: "count open parent requests", Err: err}
		}
		for _, row := range rows {
			counts[row.StudentID] += row.Total
		}
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
			return &modelBase.DatabaseError{Op: "lock open parent requests for care exit", Err: err}
		}
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
			`UPDATE enrollment.offering_change_requests
			 SET status = ?, decision_reason = ?, reviewed_by = ?, reviewed_at = ?, updated_at = ?
			 WHERE tenant_id = ? AND student_id IN (?) AND status = ?`,
			[]any{enrollmentModels.OfferingChangeStatusCareEnded, userModels.CareEndedDecisionReason, reviewedBy, at, at,
				tenantID, bun.List(studentIDs), enrollmentModels.OfferingChangeStatusPending},
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
			return 0, &modelBase.DatabaseError{Op: "close open parent requests", Err: err}
		}
		affected, _ := result.RowsAffected() // nil-driver-safe: fall through with 0
		total += int(affected)
	}
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
		return nil, &modelBase.DatabaseError{Op: "find open presence", Err: err}
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
	statements := []struct {
		op  string
		sql string
	}{
		{"lock people for care exit", `SELECT person.id FROM users.persons AS person JOIN users.students AS student ON student.person_id = person.id AND student.tenant_id = person.tenant_id WHERE student.tenant_id = ? AND student.id IN (?) FOR UPDATE OF person`},
		{"lock attendance for care exit", `SELECT id FROM active.attendance WHERE tenant_id = ? AND student_id IN (?) AND check_out_time IS NULL FOR UPDATE`},
		{"lock visits for care exit", `SELECT id FROM active.visits WHERE tenant_id = ? AND student_id IN (?) AND exit_time IS NULL FOR UPDATE`},
		{"lock roster presence for care exit", `SELECT id FROM schedule.instance_students WHERE tenant_id = ? AND student_id IN (?) AND checked_in_at IS NOT NULL AND checked_out_at IS NULL FOR UPDATE`},
		{"lock source offering names for care exit", `SELECT offering.id FROM enrollment.care_offerings AS offering JOIN enrollment.request_child_offerings AS link ON link.care_offering_id = offering.id AND link.tenant_id = offering.tenant_id JOIN enrollment.request_children AS child ON child.id = link.request_child_id AND child.tenant_id = link.tenant_id WHERE offering.tenant_id = ? AND child.created_student_id IN (?) FOR UPDATE OF offering`},
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement.sql, tenantID, bun.List(studentIDs)); err != nil {
			return &modelBase.DatabaseError{Op: statement.op, Err: err}
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
		return nil, &modelBase.DatabaseError{Op: "find latest attendance before care exit", Err: err}
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
			return 0, &modelBase.DatabaseError{Op: "close open presence", Err: err}
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
			FROM users.student_care_exit_removals AS rm
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
		tenantID, bun.List(studentIDs), after,
	).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "count planned roster rows after care end", Err: err}
	}
	for _, row := range rows {
		counts[row.StudentID] = row.Total
	}
	return counts, nil
}

// DeletePlannedByStudentIDsAfter drops the children from every roster dated
// after their last care day, writing each removed row into the care-exit
// ledger first.
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
	tenantID := tenant.FromContext(ctx)
	sql := `
		WITH removed AS (
			DELETE FROM schedule.instance_students AS s
			USING schedule.activity_instances AS ai
			WHERE s.instance_id = ai.id
			  AND s.tenant_id = ai.tenant_id
			  AND s.student_id IN (?)` + carePlannedRosterPredicate + `
			RETURNING s.tenant_id, s.student_id, s.instance_id, s.room_id, s.status,
			          s.substatus, s.note, s.is_unplanned, s.not_scheduled,
			          s.manual_status_at, s.student_status_day_id, s.pickup_exception_id
		)
		INSERT INTO users.student_care_exit_removals (
			tenant_id, student_id, kind, instance_id, room_id, status, substatus,
			note, is_unplanned, not_scheduled, manual_status_at,
			student_status_day_id, pickup_exception_id
		)
		SELECT tenant_id, student_id, 'roster', instance_id, room_id, status, substatus,
		       note, is_unplanned, not_scheduled, manual_status_at,
		       student_status_day_id, pickup_exception_id
		FROM removed
		ON CONFLICT DO NOTHING`
	result, err := base.GetDB(ctx, r.db).ExecContext(ctx, sql,
		bun.List(studentIDs), after, tenantID,
	)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete planned roster rows after care end", Err: err}
	}
	affected, _ := result.RowsAffected() // nil-driver-safe: fall through with 0
	return int(affected), nil
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
		return &modelBase.DatabaseError{Op: "lock planned roster rows for care exit", Err: err}
	}
	if _, err := db.ExecContext(ctx, `
		SELECT e.id
		FROM activities.student_enrollments AS e
		WHERE e.tenant_id = ? AND e.student_id IN (?)
		  AND (e.valid_until IS NULL OR e.valid_until > ?)
		FOR UPDATE OF e`, tenantID, bun.List(studentIDs), after.AddDays(1)); err != nil {
		return &modelBase.DatabaseError{Op: "lock bookings for care exit", Err: err}
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
		return &modelBase.DatabaseError{Op: "lock source bookings for care exit", Err: err}
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
			return &modelBase.DatabaseError{Op: "lock weekly plan for care exit", Err: err}
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
			LEFT JOIN users.student_care_exit_removals AS rm
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
			FROM users.student_care_exit_removals AS rm
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
	`, tenantID, bun.List(studentIDs), validUntil, validUntil,
		tenantID, bun.List(studentIDs), validUntil).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "count running bookings after care end", Err: err}
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
	var rows []struct {
		StudentID int64    `bun:"student_id"`
		Name      string   `bun:"name"`
		Days      []string `bun:"days,type:jsonb"`
	}
	if err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT rc.created_student_id AS student_id, co.name,
		       COALESCE(NULLIF(rco.selected_days, '[]'::jsonb), co.available_days, '[]'::jsonb) AS days
		FROM enrollment.request_child_offerings AS rco
		JOIN enrollment.request_children AS rc
		  ON rc.id = rco.request_child_id AND rc.tenant_id = rco.tenant_id
		JOIN enrollment.care_offerings AS co
		  ON co.id = rco.care_offering_id AND co.tenant_id = rco.tenant_id
		WHERE rco.tenant_id = ? AND rc.created_student_id IN (?)
		  AND (rco.valid_until IS NULL OR rco.valid_until > ?)
		ORDER BY rc.created_student_id, co.sort_order, co.id
	`, tenant.FromContext(ctx), bun.List(studentIDs), validUntil).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list source offerings for care exit preview", Err: err}
	}
	for _, row := range rows {
		result[row.StudentID] = append(result[row.StudentID], userModels.CareExitSourceOffering{Name: row.Name, Days: row.Days})
	}
	return result, nil
}

// CapByStudentIDs ends every offering and activity booking of the given
// children at validUntil (exclusive), deleting the ones that would be left
// with no interval at all. Both halves write the ledger first so a cancelled
// or re-dated exit can put the bookings back (#2487).
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
	tenantID := tenant.FromContext(ctx)

	// Deleted outright: the whole row goes into the ledger, keeping its id so a
	// restore can write it back under the id anything else may reference.
	deleted, err := base.GetDB(ctx, r.db).ExecContext(ctx, `
		WITH removed AS (
			DELETE FROM activities.student_enrollments
			WHERE tenant_id = ? AND student_id IN (?) AND valid_from >= ?
			  AND (valid_until IS NULL OR valid_until > ?)
			RETURNING id, tenant_id, student_id, activity_group_id, valid_from,
			          valid_until, calendar_period_id, enrollment_request_child_id,
			          selected_weekdays, attendance_status, weekday
		)
		INSERT INTO users.student_care_exit_removals (
			tenant_id, student_id, kind, enrollment_id, was_deleted,
			previous_valid_until, activity_group_id, valid_from, calendar_period_id,
			enrollment_request_child_id, selected_weekdays, attendance_status, weekday
		)
		SELECT tenant_id, student_id, 'booking', id, TRUE,
		       valid_until, activity_group_id, valid_from, calendar_period_id,
		       enrollment_request_child_id, selected_weekdays, attendance_status, weekday
		FROM removed
		ON CONFLICT DO NOTHING
	`, tenantID, bun.List(studentIDs), validUntil, validUntil)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete future bookings after care end", Err: err}
	}
	deletedRows, _ := deleted.RowsAffected() // nil-driver-safe: fall through with 0

	// Capped: only the previous upper bound has to survive, NULL included —
	// which is why the ledger column is nullable and the row's EXISTENCE, not
	// its value, says "this booking was capped".
	//
	// Written before the UPDATE, not from its RETURNING clause: RETURNING hands
	// back the new row, so the bound we need would already be overwritten. The
	// two statements share one predicate, spelled once below.
	const capPredicate = `
		e.tenant_id = ? AND e.student_id IN (?) AND e.valid_from < ?
		  AND (e.valid_until IS NULL OR e.valid_until > ?)`
	capArgs := []any{tenantID, bun.List(studentIDs), validUntil, validUntil}

	if _, err := base.GetDB(ctx, r.db).ExecContext(ctx, `
		INSERT INTO users.student_care_exit_removals (
			tenant_id, student_id, kind, enrollment_id, was_deleted, previous_valid_until
		)
		SELECT e.tenant_id, e.student_id, 'booking', e.id, FALSE, e.valid_until
		FROM activities.student_enrollments AS e
		WHERE `+capPredicate+`
		ON CONFLICT DO NOTHING`, capArgs...); err != nil {
		return 0, &modelBase.DatabaseError{Op: "record capped bookings after care end", Err: err}
	}

	capped, err := base.GetDB(ctx, r.db).ExecContext(ctx, `
		UPDATE activities.student_enrollments AS e
		SET valid_until = ?, updated_at = NOW()
		WHERE `+capPredicate, append([]any{validUntil}, capArgs...)...)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "cap bookings after care end", Err: err}
	}
	cappedRows, _ := capped.RowsAffected()
	return deletedRows + cappedRows, nil
}

func (r *CareExitCleanupRepository) EndSourceBookingsAndSchedules(
	ctx context.Context,
	studentIDs []int64,
	validUntil timezone.Date,
) (int64, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	tenantID := tenant.FromContext(ctx)
	bookings, err := r.EndSourceBookings(ctx, studentIDs, validUntil)
	if err != nil {
		return 0, err
	}
	plans, err := r.endCarePlanRows(ctx, studentIDs, validUntil, tenantID)
	return bookings + plans, err
}

func (r *CareExitCleanupRepository) EndSourceBookings(
	ctx context.Context, studentIDs []int64, validUntil timezone.Date,
) (int64, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	tenantID := tenant.FromContext(ctx)
	if err := r.snapshotSourceBookings(ctx, studentIDs, validUntil, tenantID); err != nil {
		return 0, err
	}
	return r.endSourceBookingRows(ctx, studentIDs, validUntil, tenantID)
}

func (r *CareExitCleanupRepository) FindCareWithdrawalBookingExpiries(
	ctx context.Context, asOf timezone.Date,
) ([]userModels.CareWithdrawalBookingChange, error) {
	rows := make([]userModels.CareWithdrawalBookingChange, 0)
	err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT rc.created_student_id AS student_id,
		       ? AS first_bookingless_day,
		       jsonb_agg(jsonb_build_object(
				'name', co.name,
				'days', COALESCE(NULLIF(rco.selected_days, '[]'::jsonb), co.available_days, '[]'::jsonb)
			)) AS source_offerings
		FROM enrollment.request_child_offerings AS rco
		JOIN enrollment.request_children AS rc
		  ON rc.id = rco.request_child_id AND rc.tenant_id = rco.tenant_id
		JOIN enrollment.care_offerings AS co
		  ON co.id = rco.care_offering_id AND co.tenant_id = rco.tenant_id
		JOIN users.students AS student
		  ON student.id = rc.created_student_id AND student.tenant_id = rc.tenant_id
		WHERE rco.tenant_id = ? AND rco.valid_until = ? AND co.counts_as_care
		  AND student.status = 'active'
		  AND (student.enrolled_until IS NULL OR student.enrolled_until >= ?)
		  AND NOT EXISTS (
			SELECT 1 FROM enrollment.request_child_offerings AS later
			JOIN enrollment.care_offerings AS later_offering
			  ON later_offering.id = later.care_offering_id AND later_offering.tenant_id = later.tenant_id
			WHERE later.tenant_id = rco.tenant_id AND later.request_child_id = rco.request_child_id
			  AND later_offering.counts_as_care
			  AND COALESCE(later.valid_from, '-infinity'::date) <= ?
			  AND (later.valid_until IS NULL OR later.valid_until > ?)
		  )
		  AND NOT EXISTS (
			SELECT 1 FROM users.care_withdrawal_completions AS completion
			WHERE completion.tenant_id = rco.tenant_id
			  AND completion.student_id = rc.created_student_id
			  AND completion.first_bookingless_day = ?
			  AND completion.state = 'pending'
		  )
		GROUP BY rc.created_student_id
	`, asOf, tenant.FromContext(ctx), asOf, asOf, asOf, asOf, asOf).Scan(ctx, &rows)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find expired final care bookings", Err: err}
	}
	for index := range rows {
		rows[index].WasCompleteWithdrawal = true
	}
	return rows, nil
}

func (r *CareExitCleanupRepository) snapshotSourceBookings(ctx context.Context, studentIDs []int64, validUntil timezone.Date, tenantID int64) error {
	if _, err := base.GetDB(ctx, r.db).ExecContext(ctx, `
		INSERT INTO users.student_care_exit_source_removals
			(tenant_id, student_id, kind, source_row_id, was_deleted, snapshot)
		SELECT rco.tenant_id, rc.created_student_id, 'source_booking', rco.id,
		       COALESCE(rco.valid_from, '-infinity'::date) >= ?, to_jsonb(rco)
		FROM enrollment.request_child_offerings AS rco
		JOIN enrollment.request_children AS rc
		  ON rc.id = rco.request_child_id AND rc.tenant_id = rco.tenant_id
		WHERE rco.tenant_id = ? AND rc.created_student_id IN (?)
		  AND (rco.valid_until IS NULL OR rco.valid_until > ?)
		ON CONFLICT DO NOTHING
	`, validUntil, tenantID, bun.List(studentIDs), validUntil); err != nil {
		return &modelBase.DatabaseError{Op: "snapshot source bookings before care exit", Err: err}
	}
	return nil
}

func (r *CareExitCleanupRepository) endSourceBookingRows(ctx context.Context, studentIDs []int64, validUntil timezone.Date, tenantID int64) (int64, error) {
	db := base.GetDB(ctx, r.db)
	deleted, err := db.ExecContext(ctx, `
		DELETE FROM enrollment.request_child_offerings AS rco
		USING enrollment.request_children AS rc
		WHERE rc.id = rco.request_child_id AND rc.tenant_id = rco.tenant_id
		  AND rco.tenant_id = ? AND rc.created_student_id IN (?)
		  AND COALESCE(rco.valid_from, '-infinity'::date) >= ?
		  AND (rco.valid_until IS NULL OR rco.valid_until > ?)
	`, tenantID, bun.List(studentIDs), validUntil, validUntil)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete future source bookings after care exit", Err: err}
	}
	deletedRows, _ := deleted.RowsAffected()
	capped, err := db.ExecContext(ctx, `
		UPDATE enrollment.request_child_offerings AS rco
		SET valid_until = ?, updated_at = NOW()
		FROM enrollment.request_children AS rc
		WHERE rc.id = rco.request_child_id AND rc.tenant_id = rco.tenant_id
		  AND rco.tenant_id = ? AND rc.created_student_id IN (?)
		  AND COALESCE(rco.valid_from, '-infinity'::date) < ?
		  AND (rco.valid_until IS NULL OR rco.valid_until > ?)
	`, validUntil, tenantID, bun.List(studentIDs), validUntil, validUntil)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "cap source bookings after care exit", Err: err}
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
		snapshotSQL := `INSERT INTO users.student_care_exit_source_removals
			(tenant_id, student_id, kind, source_row_id, was_deleted, snapshot)
			SELECT tenant_id, student_id, '` + item.Kind + `', id, TRUE, to_jsonb(plan)
			FROM ` + item.Table + ` AS plan
			WHERE tenant_id = ? AND student_id IN (?)` + datePredicate + ` ON CONFLICT DO NOTHING`
		if _, err := db.ExecContext(ctx, snapshotSQL, args...); err != nil {
			return 0, &modelBase.DatabaseError{Op: "snapshot " + item.Kind + " before care exit", Err: err}
		}
		deleteSQL := `DELETE FROM ` + item.Table + ` WHERE tenant_id = ? AND student_id IN (?)` + datePredicate
		deleted, err := db.ExecContext(ctx, deleteSQL, args...)
		if err != nil {
			return 0, &modelBase.DatabaseError{Op: "delete " + item.Kind + " after care exit", Err: err}
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
	tenantID := tenant.FromContext(ctx)
	db := base.GetDB(ctx, r.db)
	restored := 0

	// Rosters. room_id / student_status_day_id / pickup_exception_id are
	// re-validated instead of trusted: all three are ON DELETE SET NULL on the
	// live table, so a snapshot may point at something that is gone, and a bare
	// insert would fail the whole restore over a deleted room.
	rosterResult, err := db.ExecContext(ctx, `
		INSERT INTO schedule.instance_students (
			tenant_id, instance_id, student_id, room_id, status, substatus, note,
			is_unplanned, not_scheduled, manual_status_at, student_status_day_id,
			pickup_exception_id
		)
		SELECT rm.tenant_id, rm.instance_id, rm.student_id,
		       (SELECT ro.id FROM facilities.rooms AS ro
		         WHERE ro.tenant_id = rm.tenant_id AND ro.id = rm.room_id),
		       rm.status, rm.substatus, rm.note,
		       COALESCE(rm.is_unplanned, FALSE), COALESCE(rm.not_scheduled, FALSE),
		       rm.manual_status_at,
		       (SELECT sd.id FROM active.student_status_days AS sd
		         WHERE sd.tenant_id = rm.tenant_id AND sd.id = rm.student_status_day_id),
		       (SELECT pe.id FROM schedule.student_pickup_exceptions AS pe
		         WHERE pe.tenant_id = rm.tenant_id AND pe.id = rm.pickup_exception_id)
		FROM users.student_care_exit_removals AS rm
		JOIN schedule.activity_instances AS ai
		  ON ai.tenant_id = rm.tenant_id AND ai.id = rm.instance_id
		WHERE rm.kind = 'roster'
		  AND rm.tenant_id = ?
		  AND rm.student_id IN (?)
		  AND ai.status NOT IN ('completed', 'cancelled')
		ON CONFLICT DO NOTHING
	`, tenantID, bun.List(studentIDs))
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore roster rows after care exit change", Err: err}
	}
	rosterRows, _ := rosterResult.RowsAffected() // nil-driver-safe: fall through with 0
	restored += int(rosterRows)

	// Bookings that were only capped: the previous upper bound goes back on.
	cappedResult, err := db.ExecContext(ctx, `
		UPDATE activities.student_enrollments AS e
		SET valid_until = rm.previous_valid_until, updated_at = NOW()
		FROM users.student_care_exit_removals AS rm
		WHERE rm.kind = 'booking'
		  AND rm.was_deleted = FALSE
		  AND rm.tenant_id = e.tenant_id
		  AND rm.enrollment_id = e.id
		  AND rm.tenant_id = ?
		  AND rm.student_id IN (?)
	`, tenantID, bun.List(studentIDs))
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore capped bookings after care exit change", Err: err}
	}
	cappedRows, _ := cappedResult.RowsAffected()
	restored += int(cappedRows)

	// Bookings that were deleted: written back under their ORIGINAL id, so
	// anything that referenced the booking still finds it. ON CONFLICT DO
	// NOTHING carries no target on purpose — besides the primary key there is a
	// partial unique index over the open-ended bookings, and a booking somebody
	// re-created in the meantime must be left alone, not error out the restore.
	deletedResult, err := db.ExecContext(ctx, `
		INSERT INTO activities.student_enrollments (
			id, tenant_id, student_id, activity_group_id, valid_from, valid_until,
			calendar_period_id, enrollment_request_child_id, selected_weekdays,
			attendance_status, weekday
		)
		SELECT rm.enrollment_id, rm.tenant_id, rm.student_id, rm.activity_group_id,
		       rm.valid_from, rm.previous_valid_until,
		       (SELECT cp.id FROM schedule.calendar_periods AS cp
		         WHERE cp.tenant_id = rm.tenant_id AND cp.id = rm.calendar_period_id),
		       (SELECT rc.id FROM enrollment.request_children AS rc
		         WHERE rc.tenant_id = rm.tenant_id AND rc.id = rm.enrollment_request_child_id),
		       rm.selected_weekdays, rm.attendance_status, rm.weekday
		FROM users.student_care_exit_removals AS rm
		JOIN activities.groups AS g
		  ON g.tenant_id = rm.tenant_id AND g.id = rm.activity_group_id
		WHERE rm.kind = 'booking'
		  AND rm.was_deleted = TRUE
		  AND rm.tenant_id = ?
		  AND rm.student_id IN (?)
		ON CONFLICT DO NOTHING
	`, tenantID, bun.List(studentIDs))
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore deleted bookings after care exit change", Err: err}
	}
	deletedRows, _ := deletedResult.RowsAffected()
	restored += int(deletedRows)

	// Source bookings capped by the exit recover their original exclusive end.
	if _, err := db.ExecContext(ctx, `
		UPDATE enrollment.request_child_offerings AS rco
		SET valid_until = NULLIF(rm.snapshot->>'valid_until', '')::date,
		    updated_at = NOW()
		FROM users.student_care_exit_source_removals AS rm
		WHERE rm.kind = 'source_booking' AND rm.was_deleted = FALSE
		  AND rm.tenant_id = ? AND rm.student_id IN (?)
		  AND rco.tenant_id = rm.tenant_id AND rco.id = rm.source_row_id
	`, tenantID, bun.List(studentIDs)); err != nil {
		return 0, &modelBase.DatabaseError{Op: "restore capped source bookings after care exit", Err: err}
	}

	// Deleted source and weekly rows retain their original ids. This preserves
	// references such as roster pickup_exception_id when cancellation restores
	// both ledgers.
	restores := append([]careExitPlanTable{{Kind: "source_booking", Table: "enrollment.request_child_offerings"}}, careExitPlanTables...)
	for _, item := range restores {
		sql := `INSERT INTO ` + item.Table + `
			SELECT (jsonb_populate_record(NULL::` + item.Table + `, rm.snapshot)).*
			FROM users.student_care_exit_source_removals AS rm
			WHERE rm.kind = ? AND rm.was_deleted = TRUE
			  AND rm.tenant_id = ? AND rm.student_id IN (?)
			ON CONFLICT DO NOTHING`
		result, err := db.ExecContext(ctx, sql, item.Kind, tenantID, bun.List(studentIDs))
		if err != nil {
			return 0, &modelBase.DatabaseError{Op: "restore " + item.Kind + " after care exit", Err: err}
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
		FROM users.student_care_exit_removals AS rm
		JOIN schedule.student_pickup_exceptions AS pe
		  ON pe.tenant_id = rm.tenant_id AND pe.id = rm.pickup_exception_id
		WHERE rm.kind = 'roster'
		  AND rm.tenant_id = ? AND rm.student_id IN (?)
		  AND live.tenant_id = rm.tenant_id
		  AND live.instance_id = rm.instance_id
		  AND live.student_id = rm.student_id
	`, tenantID, bun.List(studentIDs)); err != nil {
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
	if _, err := base.GetDB(ctx, r.db).ExecContext(ctx, `
		DELETE FROM users.student_care_exit_removals
		WHERE tenant_id = ? AND student_id IN (?);
		DELETE FROM users.student_care_exit_source_removals
		WHERE tenant_id = ? AND student_id IN (?)
	`, tenant.FromContext(ctx), bun.List(studentIDs), tenant.FromContext(ctx), bun.List(studentIDs)); err != nil {
		return &modelBase.DatabaseError{Op: "discard care exit removals", Err: err}
	}
	return nil
}
