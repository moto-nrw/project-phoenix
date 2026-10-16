package users

import "github.com/uptrace/bun"

// StudentDirectoryRows is the retained repositories' reader port onto the
// student directory projection (ADR 0058); the composition root binds it.
// Each row source is aliased "student", runs in the caller's statement and
// transaction, and a zero tenant leaves the rows unscoped for the explicit
// admin reads.
type StudentDirectoryRows interface {
	StudentRows(db bun.IDB, tenantID int64) *bun.SelectQuery
	StudentModelRows(db bun.IDB, tenantID int64, model any) *bun.SelectQuery
}

// GuardianLinkRows is the retained repositories' reader port onto the
// guardian link projection (ADR 0058), aliased "student_guardian", on the
// same terms as StudentDirectoryRows.
type GuardianLinkRows interface {
	GuardianLinkRows(db bun.IDB, tenantID int64) *bun.SelectQuery
	GuardianLinkModelRows(db bun.IDB, tenantID int64, model any) *bun.SelectQuery
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
