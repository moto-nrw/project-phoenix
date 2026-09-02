package active

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// CrossTenantRepository provides queries for cross-tenant student data.
type CrossTenantRepository struct {
	db       *bun.DB
	students StudentDirectory
}

// NewCrossTenantRepository creates a new CrossTenantRepository.
func NewCrossTenantRepository(db *bun.DB) *CrossTenantRepository {
	return &CrossTenantRepository{db: db}
}

// BindStudentDirectory installs the People Directory the visitor lookup
// resolves students through (#2662).
func (r *CrossTenantRepository) BindStudentDirectory(students StudentDirectory) {
	r.students = students
}

// FindCrossTenantStudents returns minimal student data for students
// visiting from other tenants (Ferienbetreuung/holiday care).
// It selects students with active visits in the hosting tenant
// (hostingTenantID) and keeps those whose home tenant differs. The visit
// rows are read here; the students behind them belong to the People
// Directory, which reads them across tenants because the hosting tenant's
// row-level security would hide the visitors' home-school rows.
func (r *CrossTenantRepository) FindCrossTenantStudents(ctx context.Context, hostingTenantID int64) ([]active.CrossTenantStudent, error) {
	if r.students == nil {
		return nil, errStudentDirectoryRequired
	}
	var studentIDs []int64
	err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`active.visits AS "v"`).
		ColumnExpr(`DISTINCT "v".student_id`).
		Where(`"v".exit_time IS NULL`).
		Where(`"v".tenant_id = ?`, hostingTenantID).
		Scan(ctx, &studentIDs)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find cross-tenant students",
			Err: base.TranslateNotFound(err),
		}
	}

	results := []active.CrossTenantStudent{}
	if len(studentIDs) == 0 {
		return results, nil
	}
	students, err := r.students.ListStudentsAcrossTenantsByID(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	for _, student := range students {
		if student.TenantID == hostingTenantID {
			continue
		}
		results = append(results, active.CrossTenantStudent{
			StudentID:    student.ID,
			PersonID:     student.PersonID,
			HomeTenantID: student.TenantID,
			GroupID:      student.GroupID,
		})
	}
	return results, nil
}
