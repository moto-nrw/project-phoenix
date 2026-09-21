package studentdirectoryview

import (
	"context"
	"errors"
	"fmt"
	"strings"

	calendar "github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/uptrace/bun"
)

// The care-exit reads of Care Plan (#3427): the archive of ended care, the
// person rows the binding preview freezes, and the children the booking
// evaluation interprets. They join directory profiles, live school
// memberships and persons; the care row only filters existence (ADR 0025).
// Every read takes the caller's transaction and an explicit tenant.

const careExitMembershipJoins = `
 JOIN users.student_school_memberships AS membership ON membership.student_profile_id = student.id AND membership.tenant_id = student.tenant_id AND membership.deleted_at IS NULL
 JOIN users.student_care_profiles AS care ON care.membership_id = membership.id AND care.tenant_id = membership.tenant_id`

// ErrCareExitTenantRequired reports a call that would otherwise read across
// tenants.
var ErrCareExitTenantRequired = errors.New("student directory projection: care exit reads require a tenant")

// EndedCareRow is the directory half of one archived child; Care Plan adds
// the recorded exit reason.
type EndedCareRow struct {
	StudentID   int64         `bun:"student_id"`
	FirstName   string        `bun:"first_name"`
	LastName    string        `bun:"last_name"`
	SchoolClass string        `bun:"school_class"`
	LastCareDay calendar.Date `bun:"last_care_day"`
}

// EndedCareFilter narrows the archive. Search matches first name, last name
// or school class, case-insensitively.
type EndedCareFilter struct {
	Search        string
	SchoolClasses []string
	Page          int
	PageSize      int
}

// ListEndedCare reads the children whose enrolment interval has run out
// before asOf, newest last care day first. It reads the STUDENTS rather than
// the recorded exit reasons on purpose: the archive holds every regularly
// ended care, including one that ended with an enrolment phase and never got
// a manual reason.
func ListEndedCare(ctx context.Context, db bun.IDB, tenantID int64, asOfDay string, filter EndedCareFilter) ([]EndedCareRow, int, error) {
	if tenantID <= 0 {
		return nil, 0, ErrCareExitTenantRequired
	}
	asOf, err := calendar.ParseDate(asOfDay)
	if err != nil {
		return nil, 0, fmt.Errorf("student directory projection: ended care reference day: %w", err)
	}
	build := func() *bun.SelectQuery {
		query := db.NewSelect().
			TableExpr(`users.student_profiles AS "student"`).
			Join(careExitMembershipJoins).
			Join(`JOIN users.persons AS "person" ON "person".id = "student".person_id`).
			Where(`"student".tenant_id = ?`, tenantID).
			Where(`"membership".enrolled_until IS NOT NULL`).
			Where(`"membership".enrolled_until < ?`, asOf).
			Where(`"membership".status <> 'alumnus'`)
		if search := strings.TrimSpace(filter.Search); search != "" {
			pattern := "%" + strings.ToLower(search) + "%"
			query = query.Where(
				`(LOWER("person".first_name) LIKE ? OR LOWER("person".last_name) LIKE ? OR LOWER("membership".school_class) LIKE ?)`,
				pattern, pattern, pattern,
			)
		}
		if len(filter.SchoolClasses) > 0 {
			query = query.Where(`"membership".school_class IN (?)`, bun.List(filter.SchoolClasses))
		}
		return query
	}
	total, err := build().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("student directory projection: count ended care: %w", err)
	}
	rows := make([]EndedCareRow, 0)
	query := build().
		ColumnExpr(`"student".id AS student_id`).
		ColumnExpr(`"person".first_name AS first_name`).
		ColumnExpr(`"person".last_name AS last_name`).
		ColumnExpr(`"membership".school_class AS school_class`).
		ColumnExpr(`"membership".enrolled_until AS last_care_day`).
		OrderExpr(`"membership".enrolled_until DESC, "person".last_name ASC, "person".first_name ASC, "student".id ASC`)
	if filter.PageSize > 0 {
		query = query.Limit(filter.PageSize)
		if filter.Page > 1 {
			query = query.Offset((filter.Page - 1) * filter.PageSize)
		}
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, 0, fmt.Errorf("student directory projection: list ended care: %w", err)
	}
	return rows, total, nil
}

// LockCareExitPeople freezes the person rows whose names and RFID assignment
// the binding preview quotes back, so they cannot move between the preview
// and the confirmation that compares its token against them.
func LockCareExitPeople(ctx context.Context, db bun.IDB, tenantID int64, studentIDs []int64) error {
	if tenantID <= 0 {
		return ErrCareExitTenantRequired
	}
	if len(studentIDs) == 0 {
		return nil
	}
	_, err := db.ExecContext(ctx,
		`SELECT person.id FROM users.persons AS person
		 JOIN users.student_profiles AS student
		   ON student.person_id = person.id AND student.tenant_id = person.tenant_id`+careExitMembershipJoins+`
		 WHERE student.tenant_id = ? AND student.id IN (?) FOR UPDATE OF person`,
		tenantID, bun.List(studentIDs))
	if err != nil {
		return fmt.Errorf("student directory projection: lock people for care exit: %w", err)
	}
	return nil
}

// CareStudentRow is the identity and enrolment bound of one child the
// booking evaluation interprets.
type CareStudentRow struct {
	StudentID     int64          `bun:"student_id"`
	FirstName     string         `bun:"first_name"`
	LastName      string         `bun:"last_name"`
	SchoolClass   string         `bun:"school_class"`
	EnrolledUntil *calendar.Date `bun:"enrolled_until"`
}

// CareStudentStatuses names the two lifecycle values the candidate filter
// compares against. They arrive as one value because the caller owns the
// status vocabulary and swapping two adjacent strings would silently widen
// the candidate set.
type CareStudentStatuses struct {
	// Inactive is the status of a child left with no enrolment interval at
	// all; such a child is not a candidate.
	Inactive string
	// Active keeps a child a candidate even while its enrolment start is
	// still in the future (immediate activation).
	Active string
}

// ListCareStudents reads the children the booking evaluation covers: the
// given ids, or every child whose enrolment interval contains on.
func ListCareStudents(ctx context.Context, db bun.IDB, tenantID int64, onDay string, studentIDs []int64, statuses CareStudentStatuses) ([]CareStudentRow, error) {
	if tenantID <= 0 {
		return nil, ErrCareExitTenantRequired
	}
	on, err := calendar.ParseDate(onDay)
	if err != nil {
		return nil, fmt.Errorf("student directory projection: care student reference day: %w", err)
	}
	rows := make([]CareStudentRow, 0)
	query := db.NewSelect().
		TableExpr(`users.student_profiles AS "student"`).
		Join(careExitMembershipJoins).
		ColumnExpr(`"student".id AS student_id`).
		ColumnExpr(`"person".first_name, "person".last_name`).
		ColumnExpr(`"membership".school_class, "membership".enrolled_until`).
		Join(`JOIN users.persons AS "person" ON "person".id = "student".person_id AND "person".tenant_id = "student".tenant_id`).
		Where(`"student".tenant_id = ?`, tenantID).
		Where(`"membership".status <> 'alumnus'`).
		OrderExpr(`"student".id`)
	if len(studentIDs) > 0 {
		query = query.Where(`"student".id IN (?)`, bun.List(studentIDs))
	} else {
		query = query.
			Where(`NOT ("membership".enrolled_from IS NULL AND "membership".enrolled_until IS NULL AND "membership".status = ?)`, statuses.Inactive).
			Where(`("membership".enrolled_from IS NULL OR "membership".enrolled_from <= ? OR "membership".status = ?)`, on, statuses.Active).
			Where(`("membership".enrolled_until IS NULL OR "membership".enrolled_until >= ?)`, on)
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("student directory projection: list current care students for booking evaluation: %w", err)
	}
	return rows, nil
}
