package users

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// CareExitCleanupRepository owns the two deliberately cross-schema operations
// that ending a child's care needs: closing every open parent request across
// the four queues, and closing whatever presence record the child still has
// open when the exit takes effect (#2487).
//
// Both span schemas no single domain repository owns (users, active,
// enrollment, schedule), which is the documented exception in
// backend-conventions rule 11 — the raw SQL lives here, never in the service.
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
