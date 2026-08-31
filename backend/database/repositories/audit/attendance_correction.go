package audit

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const (
	tableAttendanceCorrections        = `audit.attendance_corrections`
	tableAttendanceCorrectionsAliased = `audit.attendance_corrections AS "attendance_correction"`
)

type attendanceCorrectionRepository struct {
	*base.Repository[*audit.AttendanceCorrection]
	db *bun.DB
}

// NewAttendanceCorrectionRepository builds the append-only trail repository
// for attendance corrections (#2898).
func NewAttendanceCorrectionRepository(db *bun.DB) audit.AttendanceCorrectionRepository {
	repo := base.NewRepository[*audit.AttendanceCorrection](db, tableAttendanceCorrections, "AttendanceCorrection")
	repo.TenantScoped = true
	return &attendanceCorrectionRepository{Repository: repo, db: db}
}

// CreateBatch appends every field change of one attendance write in a single
// insert, so the trail of a correction is written atomically with it.
func (r *attendanceCorrectionRepository) CreateBatch(ctx context.Context, corrections []*audit.AttendanceCorrection) error {
	if len(corrections) == 0 {
		return nil
	}
	for _, correction := range corrections {
		if correction == nil {
			return &modelBase.DatabaseError{Op: "create attendance corrections", Err: errors.New("correction cannot be nil")}
		}
		if err := correction.Validate(); err != nil {
			return &modelBase.DatabaseError{Op: "validate attendance correction", Err: err}
		}
		base.EnsureTenantID(ctx, correction)
	}

	if _, err := base.GetDB(ctx, r.db).NewInsert().
		Model(&corrections).
		ModelTableExpr(tableAttendanceCorrections).
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "create attendance corrections", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// ListByInstanceAndStudent returns one child's correction trail for one
// instance, newest first.
func (r *attendanceCorrectionRepository) ListByInstanceAndStudent(ctx context.Context, instanceID, studentID int64) ([]*audit.AttendanceCorrection, error) {
	if instanceID <= 0 || studentID <= 0 {
		return nil, &modelBase.DatabaseError{Op: "list attendance corrections", Err: errors.New("instance_id and student_id are required")}
	}
	var rows []*audit.AttendanceCorrection
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(tableAttendanceCorrectionsAliased).
		Where(`"attendance_correction".instance_id = ?`, instanceID).
		Where(`"attendance_correction".student_id = ?`, studentID).
		OrderExpr(`"attendance_correction".created_at DESC, "attendance_correction".id DESC`)
	// Defense in depth on top of RLS: this is an audit trail, and a read that
	// silently crosses schools would be worse here than anywhere else.
	err := base.WithTenantFilter(ctx, query, "attendance_correction").Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list attendance corrections", Err: base.TranslateNotFound(err)}
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
	query := base.GetDB(ctx, r.db).NewSelect().
		ModelTableExpr(tableAttendanceCorrectionsAliased).
		ColumnExpr(`"attendance_correction".student_id AS student_id`).
		ColumnExpr(`COUNT(*) AS total`).
		Where(`"attendance_correction".instance_id = ?`, instanceID).
		Where(`"attendance_correction".student_id IN (?)`, bun.List(studentIDs)).
		GroupExpr(`"attendance_correction".student_id`)
	err := base.WithTenantFilter(ctx, query, "attendance_correction").Scan(ctx, &rows)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "count attendance corrections", Err: base.TranslateNotFound(err)}
	}
	for _, row := range rows {
		counts[row.StudentID] = row.Total
	}
	return counts, nil
}
