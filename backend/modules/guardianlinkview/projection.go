// Package guardianlinkview joins the three owners of a student-guardian
// relationship in one statement (#2756): People Directory's relationship,
// Care Plan's pickup permission and Identity & Access's portal access. It
// serves the retained reads that consumed users.students_guardians as one row,
// in that row's column shape, without reading the rollback mirror.
//
// The projection only reads. Every write goes to the owner of the column.
package guardianlinkview

import "github.com/uptrace/bun"

// linkSource is one row per relationship. Every relationship carries exactly
// one pickup permission and one access row, created in the same unit of work,
// so the joins are inner: a relationship whose halves are missing is not a
// complete link and stays invisible, exactly as the old table never held half
// a row. updated_at is the latest change of any of the three halves.
// access_account_id is Identity's account binding, which the old shape did
// not carry.
const linkSource = `(SELECT r.id, r.tenant_id, r.student_id, r.guardian_profile_id, r.relationship_type,
	r.guardian_role, r.is_primary, r.is_emergency_contact, p.can_pickup, p.pickup_notes,
	r.emergency_priority, r.is_payer, a.permissions, a.account_id AS access_account_id, r.created_at,
	GREATEST(r.updated_at, p.updated_at, a.updated_at) AS updated_at
	FROM users.student_guardian_relationships AS r
	JOIN users.student_guardian_pickup_permissions AS p
		ON p.tenant_id = r.tenant_id AND p.relationship_id = r.id
	JOIN auth.guardian_student_access AS a
		ON a.tenant_id = r.tenant_id AND a.relationship_id = r.id) AS "student_guardian"`

// Query is the relationship projection embedded by the callers' own joins,
// typically as `JOIN (?) AS sg`. It executes no statement: the outer query
// keeps its ambient transaction, result shape and statement budget. A
// positive tenant scopes the rows explicitly; zero is reserved for the
// explicit admin transactions that read across schools and filter tenants
// themselves.
func Query(db bun.IDB, tenantID int64) *bun.SelectQuery {
	query := db.NewSelect().TableExpr(linkSource)
	if tenantID > 0 {
		query = query.Where(`"student_guardian".tenant_id = ?`, tenantID)
	}
	return query
}

// ModelQuery preserves BUN's model column selection over the projection for
// the retained users.StudentGuardian row.
func ModelQuery(db bun.IDB, tenantID int64, model any) *bun.SelectQuery {
	query := db.NewSelect().Model(model).ModelTableExpr(linkSource)
	if tenantID > 0 {
		query = query.Where(`"student_guardian".tenant_id = ?`, tenantID)
	}
	return query
}
