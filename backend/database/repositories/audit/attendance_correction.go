package audit

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/uptrace/bun"
)

const attendanceCorrectionTableExpr = `audit.attendance_corrections AS "attendance_correction"`

type attendanceCorrectionRepository struct {
	runtime Runtime
}

// NewAttendanceCorrectionRepository builds the append-only trail repository
// for attendance corrections (#2898).
func NewAttendanceCorrectionRepository(runtime Runtime) audit.AttendanceCorrectionRepository {
	return &attendanceCorrectionRepository{runtime: requireRuntime(runtime)}
}

// CreateBatch appends every field change of one attendance write, so the trail
// of a correction is written with it. The Appender stamps and verifies the
// tenant against the caller's transaction.
func (r *attendanceCorrectionRepository) CreateBatch(ctx context.Context, corrections []*audit.AttendanceCorrection) error {
	if len(corrections) == 0 {
		return nil
	}
	appender := NewAppender(r.runtime)
	for _, correction := range corrections {
		if correction == nil {
			return errors.New("attendance correction cannot be nil")
		}
		if err := appender.Append(ctx, correction); err != nil {
			return wrapDatabase("create attendance corrections", err)
		}
	}
	return nil
}

// ListByInstanceAndStudent returns one child's correction trail for one
// instance, newest first.
func (r *attendanceCorrectionRepository) ListByInstanceAndStudent(ctx context.Context, instanceID, studentID int64) ([]*audit.AttendanceCorrection, error) {
	if instanceID <= 0 || studentID <= 0 {
		return nil, wrapDatabase("list attendance corrections", errors.New("instance_id and student_id are required"))
	}
	var rows []*audit.AttendanceCorrection
	query := runtimeDB(ctx, r.runtime).NewSelect().
		Model(&rows).
		ModelTableExpr(attendanceCorrectionTableExpr).
		Where(`"attendance_correction".instance_id = ?`, instanceID).
		Where(`"attendance_correction".student_id = ?`, studentID).
		OrderExpr(`"attendance_correction".created_at DESC, "attendance_correction".id DESC`)
	// Defense in depth on top of RLS: this is an audit trail, and a read that
	// silently crosses schools would be worse here than anywhere else.
	if tenantID := runtimeTenantID(ctx, r.runtime); tenantID > 0 {
		query = query.Where(`"attendance_correction".tenant_id = ?`, tenantID)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, wrapDatabase("list attendance corrections", err)
	}
	return rows, nil
}

// CountByInstanceAndStudents reports the number of corrections per child for
// one instance. A list can mark corrected rows from one query instead of one
// per row.
func (r *attendanceCorrectionRepository) CountByInstanceAndStudents(ctx context.Context, instanceID int64, studentIDs []int64) (map[int64]int, error) {
	counts := map[int64]int{}
	if instanceID <= 0 || len(studentIDs) == 0 {
		return counts, nil
	}
	var rows []struct {
		StudentID int64 `bun:"student_id"`
		Total     int   `bun:"total"`
	}
	query := runtimeDB(ctx, r.runtime).NewSelect().
		ModelTableExpr(attendanceCorrectionTableExpr).
		ColumnExpr(`"attendance_correction".student_id AS student_id`).
		ColumnExpr(`COUNT(*) AS total`).
		Where(`"attendance_correction".instance_id = ?`, instanceID).
		Where(`"attendance_correction".student_id IN (?)`, bun.List(studentIDs)).
		GroupExpr(`"attendance_correction".student_id`)
	if tenantID := runtimeTenantID(ctx, r.runtime); tenantID > 0 {
		query = query.Where(`"attendance_correction".tenant_id = ?`, tenantID)
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, wrapDatabase("count attendance corrections", err)
	}
	for _, row := range rows {
		counts[row.StudentID] = row.Total
	}
	return counts, nil
}
