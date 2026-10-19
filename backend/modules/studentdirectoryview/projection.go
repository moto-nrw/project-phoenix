// Package studentdirectoryview joins the three student owners in one statement.
// The inner joins and live-membership predicate preserve the rollback view's
// row selection, without reading that view or its retired contact copies.
package studentdirectoryview

import (
	"errors"

	"github.com/uptrace/bun"
)

const studentSource = `(SELECT p.id, p.tenant_id, p.person_id, p.created_at,
	GREATEST(p.updated_at, m.updated_at, c.updated_at) AS updated_at,
	p.address_street, p.address_city, p.address_postal_code, p.extra_info,
	p.photo_path, p.photo_consent_given_at, p.photo_consent_given_by,
	p.agb_accepted_at, p.data_processing_accepted_at, p.email_contact_accepted_at,
	m.id AS membership_id, m.school_class, m.group_id, m.status, m.enrolled_from, m.enrolled_until,
	c.supervisor_notes, c.health_info, c.pickup_status, c.departure_days,
	c.allowed_departure_modes, c.departure_companion_note, c.pickup_days, c.bus_days,
	c.sick, c.sick_since, c.excused, c.excused_since
	FROM users.student_profiles AS p
	JOIN users.student_school_memberships AS m
		ON m.tenant_id = p.tenant_id AND m.student_profile_id = p.id AND m.deleted_at IS NULL
	JOIN users.student_care_profiles AS c
		ON c.tenant_id = m.tenant_id AND c.membership_id = m.id) AS "student"`

// Query embeds the rows of one required school in the caller's statement.
func Query(db bun.IDB, tenantID int64) *bun.SelectQuery {
	query := AllSchoolsQuery(db)
	if tenantID <= 0 {
		return query.Err(errors.New("studentdirectoryview: tenant is required"))
	}
	return query.Where(`"student".tenant_id = ?`, tenantID)
}

// AllSchoolsQuery embeds an explicitly cross-school source. The caller owns
// administrative/audience or outer school-correlation predicates; this executes no SQL.
func AllSchoolsQuery(db bun.IDB) *bun.SelectQuery { return db.NewSelect().TableExpr(studentSource) }

// ModelQuery preserves model column selection for one required school.
func ModelQuery(db bun.IDB, tenantID int64, model any) *bun.SelectQuery {
	query := AllSchoolsModelQuery(db, model)
	if tenantID <= 0 {
		return query.Err(errors.New("studentdirectoryview: tenant is required"))
	}
	return query.Where(`"student".tenant_id = ?`, tenantID)
}

// AllSchoolsModelQuery is the explicit cross-school model row source.
func AllSchoolsModelQuery(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(studentSource)
}
