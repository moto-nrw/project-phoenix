package users

import (
	"context"

	"github.com/uptrace/bun"
)

// StudentDirectoryRows is the retained repositories' reader port onto the
// student directory projection (ADR 0058); the composition root binds it.
// Each row source is aliased "student", runs in the caller's statement and
// transaction. School row sources require a positive ID; separately named
// all-school sources serve explicit administrative or correlated queries.
type StudentDirectoryRows interface {
	StudentRows(db bun.IDB, tenantID int64) *bun.SelectQuery
	AllSchoolsStudentRows(db bun.IDB) *bun.SelectQuery
	StudentModelRows(db bun.IDB, tenantID int64, model any) *bun.SelectQuery
	AllSchoolsStudentModelRows(db bun.IDB, model any) *bun.SelectQuery
}

// GuardianLinkRows is the retained repositories' reader port onto the
// guardian link projection (ADR 0058), aliased "student_guardian", on the
// same terms as StudentDirectoryRows.
type GuardianLinkRows interface {
	GuardianLinkRows(db bun.IDB, tenantID int64) *bun.SelectQuery
	AllSchoolsGuardianLinkRows(db bun.IDB) *bun.SelectQuery
	GuardianLinkModelRows(db bun.IDB, tenantID int64, model any) *bun.SelectQuery
	AllSchoolsGuardianLinkModelRows(db bun.IDB, model any) *bun.SelectQuery
}

func requireStudentRows(rows StudentDirectoryRows) StudentDirectoryRows {
	if rows == nil {
		panic("people directory repository: student directory rows are required")
	}
	return rows
}

func requireGuardianLinkRows(rows GuardianLinkRows) GuardianLinkRows {
	if rows == nil {
		panic("people directory repository: guardian link rows are required")
	}
	return rows
}

func studentRows(ctx context.Context, runtime Runtime, rows StudentDirectoryRows, db bun.IDB) *bun.SelectQuery {
	if runtime.AcrossSchools(ctx) {
		return rows.AllSchoolsStudentRows(db)
	}
	id, err := runtime.RequireTenantID(ctx)
	if err != nil {
		return rows.AllSchoolsStudentRows(db).Err(err)
	}
	return rows.StudentRows(db, id)
}
func guardianLinkRows(ctx context.Context, runtime Runtime, rows GuardianLinkRows, db bun.IDB) *bun.SelectQuery {
	if runtime.AcrossSchools(ctx) {
		return rows.AllSchoolsGuardianLinkRows(db)
	}
	id, err := runtime.RequireTenantID(ctx)
	if err != nil {
		return rows.AllSchoolsGuardianLinkRows(db).Err(err)
	}
	return rows.GuardianLinkRows(db, id)
}
func guardianLinkModelRows(ctx context.Context, runtime Runtime, rows GuardianLinkRows, db bun.IDB, model any) *bun.SelectQuery {
	if runtime.AcrossSchools(ctx) {
		return rows.AllSchoolsGuardianLinkModelRows(db, model)
	}
	id, err := runtime.RequireTenantID(ctx)
	if err != nil {
		return rows.AllSchoolsGuardianLinkModelRows(db, model).Err(err)
	}
	return rows.GuardianLinkModelRows(db, id, model)
}
