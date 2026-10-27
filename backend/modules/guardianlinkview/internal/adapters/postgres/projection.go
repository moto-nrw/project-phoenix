// Package guardianlinkview joins the three owners of a student-guardian
// relationship in one statement (#2756): People Directory's relationship,
// Care Plan's pickup permission and Identity & Access's portal access. It
// serves the retained reads that consumed users.students_guardians as one row,
// in that row's column shape, without reading the rollback mirror.
//
// The projection only reads. Every write goes to the owner of the column.
package guardianlinkview

import (
	"errors"

	"github.com/uptrace/bun"
)

// linkSource is one row per relationship. Every relationship carries exactly
// one pickup permission and one access row, created in the same unit of work,
// so the joins are inner: a relationship whose halves are missing is not a
// complete link and stays invisible, exactly as the old table never held half
// a row. updated_at is the latest change of any of the three halves.
// access_account_id is Identity's account binding, which the old shape did
// not carry.
const linkSource = `(SELECT r.id, r.tenant_id, r.student_id, r.guardian_profile_id, r.relationship_type,
	r.guardian_role, r.is_primary, r.is_emergency_contact, p.can_pickup, p.pickup_notes, p.pickup_weekdays,
	r.emergency_priority, r.is_payer, a.permissions, a.account_id AS access_account_id, r.created_at,
	GREATEST(r.updated_at, p.updated_at, a.updated_at) AS updated_at
	FROM users.student_guardian_relationships AS r
	JOIN users.student_guardian_pickup_permissions AS p
		ON p.tenant_id = r.tenant_id AND p.relationship_id = r.id
	JOIN auth.guardian_student_access AS a
		ON a.tenant_id = r.tenant_id AND a.relationship_id = r.id) AS "student_guardian"`

// Query embeds the rows of one required school in the caller's statement.
func Query(db bun.IDB, tenantID int64) *bun.SelectQuery {
	query := AllSchoolsQuery(db)
	if tenantID <= 0 {
		return query.Err(errors.New("guardianlinkview: tenant is required"))
	}
	return query.Where(`"student_guardian".tenant_id = ?`, tenantID)
}

// AllSchoolsQuery embeds an explicitly cross-school source. The caller owns
// administrative/audience or outer school-correlation predicates; this executes no SQL.
func AllSchoolsQuery(db bun.IDB) *bun.SelectQuery { return db.NewSelect().TableExpr(linkSource) }

// ModelQuery preserves model column selection for one required school.
func ModelQuery(db bun.IDB, tenantID int64, model any) *bun.SelectQuery {
	query := AllSchoolsModelQuery(db, model)
	if tenantID <= 0 {
		return query.Err(errors.New("guardianlinkview: tenant is required"))
	}
	return query.Where(`"student_guardian".tenant_id = ?`, tenantID)
}

// AllSchoolsModelQuery is the explicit cross-school model row source.
func AllSchoolsModelQuery(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(linkSource)
}
