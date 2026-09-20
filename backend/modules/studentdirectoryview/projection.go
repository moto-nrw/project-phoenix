// Package studentdirectoryview joins the three student owners in one statement.
// The inner joins and live-membership predicate preserve the rollback view's
// row selection, without reading that view or its retired contact copies.
package studentdirectoryview

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// Database supplies the caller's existing RLS transaction and school ID.
// A zero school is reserved for the existing explicit admin-read transaction.
type Database func(context.Context) (bun.IDB, int64, error)

var ErrStudentLockBusy = directory.ErrStudentLockBusy
var ErrStudentNotFound = directory.ErrStudentNotFound

type OperationStats struct {
	Queries           int64
	Rows              int64
	StatementDuration time.Duration
}

type Filter struct {
	IDs           []int64
	SchoolClasses []string
	GradeLevels   []string
	KeepAlumni    []int64
	CareStatus    string
	CareStatusOn  string
	Page          int
	PageSize      int
}

func studentDateParam(value string) *timezone.Date {
	if value == "" {
		return nil
	}
	day, err := timezone.ParseDate(value)
	if err != nil {
		return nil
	}
	return &day
}

func isLockNotAvailable(err error) bool {
	var postgresError pgdriver.Error
	return errors.As(err, &postgresError) && postgresError.Field('C') == "55P03"
}

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

// Query is the directory projection embedded by existing owner-specific joins.
// It executes no statement: the outer query keeps its ambient transaction,
// result shape and budget. Zero tenant is reserved for operator/admin reads.
func Query(db bun.IDB, tenantID int64) *bun.SelectQuery {
	query := db.NewSelect().TableExpr(studentSource)
	if tenantID > 0 {
		query = query.Where(`"student".tenant_id = ?`, tenantID)
	}
	return query
}

// ModelQuery preserves BUN's model column selection without adding a second
// FROM item for the retained Student DTO's rollback-table annotation.
func ModelQuery(db bun.IDB, tenantID int64, model any) *bun.SelectQuery {
	query := db.NewSelect().Model(model).ModelTableExpr(studentSource)
	if tenantID > 0 {
		query = query.Where(`"student".tenant_id = ?`, tenantID)
	}
	return query
}
