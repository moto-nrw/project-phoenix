package users

import (
	"context"
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
		if err := base.GetDB(ctx, r.db).NewRaw(sql, tenantID, bun.In(studentIDs), queue.Pending).Scan(ctx, &rows); err != nil {
			return nil, &modelBase.DatabaseError{Op: "count open parent requests", Err: err}
		}
		for _, row := range rows {
			counts[row.StudentID] += row.Total
		}
	}
	return counts, nil
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
				tenantID, bun.In(studentIDs), userModels.DataChangeStatusPending},
		},
		{
			`UPDATE active.excused_absence_requests
			 SET status = ?, decision_reason = ?, reviewed_by = ?, reviewed_at = ?, updated_at = ?
			 WHERE tenant_id = ? AND student_id IN (?) AND status = ?`,
			[]any{activeModels.ExcusedRequestStatusCareEnded, userModels.CareEndedDecisionReason, reviewedBy, at, at,
				tenantID, bun.In(studentIDs), activeModels.ExcusedRequestStatusPending},
		},
		{
			`UPDATE enrollment.offering_change_requests
			 SET status = ?, decision_reason = ?, reviewed_by = ?, reviewed_at = ?, updated_at = ?
			 WHERE tenant_id = ? AND student_id IN (?) AND status = ?`,
			[]any{enrollmentModels.OfferingChangeStatusCareEnded, userModels.CareEndedDecisionReason, reviewedBy, at, at,
				tenantID, bun.In(studentIDs), enrollmentModels.OfferingChangeStatusPending},
		},
		{
			`UPDATE schedule.care_schedule_change_requests
			 SET status = ?, decision_reason = ?, reviewed_by = ?, reviewed_at = ?, updated_at = ?
			 WHERE tenant_id = ? AND student_id IN (?) AND status = ?`,
			[]any{scheduleModels.CareRequestStatusCareEnded, userModels.CareEndedDecisionReason, reviewedBy, at, at,
				tenantID, bun.In(studentIDs), scheduleModels.CareRequestStatusPending},
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
	`, tenant.FromContext(ctx), bun.In(studentIDs),
		tenant.FromContext(ctx), bun.In(studentIDs)).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find open presence", Err: err}
	}
	for _, row := range rows {
		present[row.StudentID] = true
	}
	return present, nil
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
		result, err := base.GetDB(ctx, r.db).ExecContext(ctx, statement, at, at, tenantID, bun.In(studentIDs))
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

// CountPlannedByStudentIDsAfter counts the rows DeletePlannedByStudentIDsAfter
// would remove, per student, for the "Betreuung beenden" preview.
func (r *CareExitCleanupRepository) CountPlannedByStudentIDsAfter(
	ctx context.Context, studentIDs []int64, after timezone.Date,
) (map[int64]int, error) {
	counts := make(map[int64]int, len(studentIDs))
	if len(studentIDs) == 0 {
		return counts, nil
	}
	var rows []struct {
		StudentID int64 `bun:"student_id"`
		Total     int   `bun:"total"`
	}
	sql := `
		SELECT s.student_id AS student_id, COUNT(*)::int AS total
		FROM schedule.instance_students AS s
		JOIN schedule.activity_instances AS ai ON ai.id = s.instance_id
		WHERE s.student_id IN (?)` + carePlannedRosterPredicate + `
		GROUP BY s.student_id`
	if err := base.GetDB(ctx, r.db).NewRaw(sql,
		bun.In(studentIDs), after, tenant.FromContext(ctx),
	).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "count planned roster rows after care end", Err: err}
	}
	for _, row := range rows {
		counts[row.StudentID] = row.Total
	}
	return counts, nil
}

// DeletePlannedByStudentIDsAfter drops the children from every roster dated
// after their last care day.
//
// Unlike the graduation path (ArchivePlannedByStudentIDsFrom) nothing is
// archived: ending care is not reverted, a resumed child is re-planned
// deliberately, and the acceptance criteria require that nothing is switched
// back on automatically.
func (r *CareExitCleanupRepository) DeletePlannedByStudentIDsAfter(
	ctx context.Context, studentIDs []int64, after timezone.Date,
) (int, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	sql := `
		DELETE FROM schedule.instance_students AS s
		USING schedule.activity_instances AS ai
		WHERE s.instance_id = ai.id
		  AND s.student_id IN (?)` + carePlannedRosterPredicate
	result, err := base.GetDB(ctx, r.db).ExecContext(ctx, sql,
		bun.List(studentIDs), after, tenant.FromContext(ctx),
	)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete planned roster rows after care end", Err: err}
	}
	affected, _ := result.RowsAffected() // nil-driver-safe: fall through with 0
	return int(affected), nil
}

// CountRunningByStudentIDsAfter counts the bookings CapByStudentIDs would
// touch, per student, for the preview. valid_until is an EXCLUSIVE upper
// bound, so the caller passes the day AFTER the last care day.
func (r *CareExitCleanupRepository) CountRunningByStudentIDsAfter(
	ctx context.Context, studentIDs []int64, validUntil timezone.Date,
) (map[int64]int, error) {
	counts := make(map[int64]int, len(studentIDs))
	if len(studentIDs) == 0 {
		return counts, nil
	}
	var rows []struct {
		StudentID int64 `bun:"student_id"`
		Total     int   `bun:"total"`
	}
	if err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT student_id, COUNT(*)::int AS total
		FROM activities.student_enrollments
		WHERE tenant_id = ? AND student_id IN (?)
		  AND (valid_until IS NULL OR valid_until > ?)
		GROUP BY student_id
	`, tenant.FromContext(ctx), bun.In(studentIDs), validUntil).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "count running bookings after care end", Err: err}
	}
	for _, row := range rows {
		counts[row.StudentID] = row.Total
	}
	return counts, nil
}

// CapByStudentIDs ends every offering and activity booking of the given
// children at validUntil (exclusive), deleting the ones that would be left
// with no interval at all.
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
	deleted, err := base.GetDB(ctx, r.db).ExecContext(ctx, `
		DELETE FROM activities.student_enrollments
		WHERE tenant_id = ? AND student_id IN (?) AND valid_from >= ?
	`, tenantID, bun.In(studentIDs), validUntil)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete future bookings after care end", Err: err}
	}
	deletedRows, _ := deleted.RowsAffected() // nil-driver-safe: fall through with 0

	capped, err := base.GetDB(ctx, r.db).ExecContext(ctx, `
		UPDATE activities.student_enrollments
		SET valid_until = ?, updated_at = NOW()
		WHERE tenant_id = ? AND student_id IN (?) AND valid_from < ?
		  AND (valid_until IS NULL OR valid_until > ?)
	`, validUntil, tenantID, bun.In(studentIDs), validUntil, validUntil)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "cap bookings after care end", Err: err}
	}
	cappedRows, _ := capped.RowsAffected()
	return deletedRows + cappedRows, nil
}
