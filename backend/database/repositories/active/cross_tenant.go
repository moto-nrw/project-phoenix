package active

import (
	"context"
	"database/sql"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// CrossTenantRepository provides queries for cross-tenant student data.
type CrossTenantRepository struct {
	db *bun.DB
}

// NewCrossTenantRepository creates a new CrossTenantRepository.
func NewCrossTenantRepository(db *bun.DB) *CrossTenantRepository {
	return &CrossTenantRepository{db: db}
}

// FindCrossTenantStudents returns minimal student data for students
// visiting from other tenants (Ferienbetreuung/holiday care).
// It selects students with active visits whose home tenant differs from
// the hosting tenant (hostingTenantID).
func (r *CrossTenantRepository) FindCrossTenantStudents(ctx context.Context, hostingTenantID int64) ([]active.CrossTenantStudent, error) {
	var results []active.CrossTenantStudent

	err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`active.visits AS "v"`).
		ColumnExpr(`"s".id AS "student_id"`).
		ColumnExpr(`"p".first_name AS "first_name"`).
		ColumnExpr(`"p".last_name AS "last_name"`).
		ColumnExpr(`COALESCE("eg".name, '') AS "group_name"`).
		ColumnExpr(`"s".tenant_id AS "home_tenant_id"`).
		Join(`INNER JOIN users.students AS "s" ON "s".id = "v".student_id`).
		Join(`INNER JOIN users.persons AS "p" ON "p".id = "s".person_id`).
		Join(`LEFT JOIN education.groups AS "eg" ON "eg".id = "s".group_id`).
		Where(`"v".exit_time IS NULL`).
		Where(`"v".tenant_id = ?`, hostingTenantID).
		Where(`"s".tenant_id != ?`, hostingTenantID).
		Scan(ctx, &results)
	if err != nil {
		if err == sql.ErrNoRows {
			return []active.CrossTenantStudent{}, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find cross-tenant students",
			Err: base.TranslateNotFound(err),
		}
	}

	if results == nil {
		results = []active.CrossTenantStudent{}
	}

	return results, nil
}
