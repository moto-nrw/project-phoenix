package studentdirectoryview

import "github.com/uptrace/bun"

// StudentDisplayQuery is the display row of every student, alumni included:
// id, tenant_id, school_class, and first_name and last_name while a live
// person row exists (NULL otherwise). Like Query it executes no statement: an
// owner joins it as a subquery, so its search, count and page window stay in
// the owner's one statement. Zero tenant is reserved for operator/admin reads.
func StudentDisplayQuery(db bun.IDB, tenantID int64) *bun.SelectQuery {
	query := db.NewSelect().TableExpr(studentSource).
		ColumnExpr(`"student".id, "student".tenant_id, "student".school_class, "person".first_name, "person".last_name`).
		Join(`LEFT JOIN users.persons AS "person" ON "person".tenant_id = "student".tenant_id AND "person".id = "student".person_id AND "person".deleted_at IS NULL`)
	return withStudentTenant(query, tenantID)
}
