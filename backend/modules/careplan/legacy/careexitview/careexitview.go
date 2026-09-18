// Package careexitview is the tenant-safe read projection the care-exit
// workflow needs over the child rows People Directory owns (#3350).
//
// "Betreuung beenden" has to say whose care is ending: the archive lists the
// children whose enrolment interval has run out, the binding preview freezes
// their person rows, and the booking evaluation reads their enrolment bounds.
// None of that is a Care Plan table, and none of it is a write — it is the one
// place where the exit flow looks at the directory. Gathering it here keeps
// that seam named and tenant-scoped instead of spreading `users.students`
// joins through the flow.
//
// Every query takes the tenant id explicitly and filters on it. The
// recordset arguments are owner projections the caller already loaded
// (Enrollment application links, Care Plan offerings, Enrollment offering
// links); they arrive as JSON so this package joins them without reaching
// into either owner's tables.
package careexitview

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

// ErrInvalidTenantID reports a call that would otherwise read across tenants.
var ErrInvalidTenantID = errors.New("care exit view: tenant id is required")

// EndedCareRow is one child of the archive: the directory half of the row,
// which the caller completes with the recorded exit reason.
type EndedCareRow struct {
	StudentID   int64         `bun:"student_id"`
	FirstName   string        `bun:"first_name"`
	LastName    string        `bun:"last_name"`
	SchoolClass string        `bun:"school_class"`
	LastCareDay timezone.Date `bun:"last_care_day"`
}

// EndedCareFilter narrows the archive. Search matches first name, last name
// or school class, case-insensitively.
type EndedCareFilter struct {
	Search        string
	SchoolClasses []string
	Page          int
	PageSize      int
}

// CareStudentRow is the identity and enrolment bound of one child the booking
// evaluation interprets.
type CareStudentRow struct {
	StudentID     int64          `bun:"student_id"`
	FirstName     string         `bun:"first_name"`
	LastName      string         `bun:"last_name"`
	SchoolClass   string         `bun:"school_class"`
	EnrolledUntil *timezone.Date `bun:"enrolled_until"`
}

// SourceOfferingRow is one still-running source booking of a child, named for
// the binding preview.
type SourceOfferingRow struct {
	StudentID int64    `bun:"student_id"`
	Name      string   `bun:"name"`
	Days      []string `bun:"days,type:jsonb"`
}

// BookingExpiryRow is one child whose last care-counting source booking ends,
// with the offerings that end with it.
type BookingExpiryRow struct {
	StudentID            int64         `bun:"student_id"`
	FirstBookinglessDay  timezone.Date `bun:"first_bookingless_day"`
	SourceRequestChildID int64         `bun:"source_request_child_id"`
	SourceOfferings      []byte        `bun:"source_offerings,type:jsonb"`
}

// CareBookingPeriodRow is one care-counting booking window of a child.
type CareBookingPeriodRow struct {
	StudentID            int64          `bun:"student_id"`
	ValidFrom            *timezone.Date `bun:"valid_from"`
	ValidUntil           *timezone.Date `bun:"valid_until"`
	SourceRequestChildID int64          `bun:"source_request_child_id"`
	OfferingName         string         `bun:"offering_name"`
	Days                 []string       `bun:"days,type:jsonb"`
}

// Recordsets are the owner projections a care-exit query joins: the
// Enrollment application links, the Care Plan offerings and the Enrollment
// offering links, each already encoded as JSON by the caller.
type Recordsets struct {
	ApplicationLinks string
	CareOfferings    string
	OfferingLinks    string
}

const applicationLinkRecordset = `application_links AS (
	SELECT * FROM jsonb_to_recordset(?::jsonb) AS application(
		id bigint, tenant_id bigint, created_student_id bigint, matched_student_id bigint, status text
	)
)`

const careOfferingRecordset = `care_offerings AS (
	SELECT * FROM jsonb_to_recordset(?::jsonb) AS offering(
		id bigint, tenant_id bigint, name text, days_of_week_mode text,
		available_days jsonb, counts_as_care boolean, sort_order integer
	)
)`

// offeringLinkRecordset is the Enrollment offering-link projection (#2695)
// these reads join instead of enrollment.request_child_offerings.
const offeringLinkRecordset = `offering_links AS (
	SELECT * FROM jsonb_to_recordset(?::jsonb) AS link(` + enrollment.CareOfferingLinkRecordColumns + `)
)`

const recordsets = `WITH ` + applicationLinkRecordset + `, ` + careOfferingRecordset + `, ` + offeringLinkRecordset

// ListEndedCare reads the children whose enrolment interval has run out
// before asOf, newest last care day first.
//
// It reads the STUDENTS rather than the recorded exit reasons on purpose: the
// archive has to hold every regularly ended care, including the ones that
// ended because an enrolment phase ran out and never got a manual reason.
func ListEndedCare(
	ctx context.Context,
	db bun.IDB,
	tenantID int64,
	asOf timezone.Date,
	filter EndedCareFilter,
) ([]EndedCareRow, int, error) {
	if tenantID <= 0 {
		return nil, 0, ErrInvalidTenantID
	}
	build := func() *bun.SelectQuery {
		query := db.NewSelect().
			TableExpr(`users.students AS "student"`).
			Join(`JOIN users.persons AS "person" ON "person".id = "student".person_id`).
			Where(`"student".tenant_id = ?`, tenantID).
			Where(`"student".enrolled_until IS NOT NULL`).
			Where(`"student".enrolled_until < ?`, asOf).
			Where(`"student".status <> 'alumnus'`)
		if search := strings.TrimSpace(filter.Search); search != "" {
			pattern := "%" + strings.ToLower(search) + "%"
			query = query.Where(
				`(LOWER("person".first_name) LIKE ? OR LOWER("person".last_name) LIKE ? OR LOWER("student".school_class) LIKE ?)`,
				pattern, pattern, pattern,
			)
		}
		if len(filter.SchoolClasses) > 0 {
			query = query.Where(`"student".school_class IN (?)`, bun.List(filter.SchoolClasses))
		}
		return query
	}

	total, err := build().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("care exit view: count ended care: %w", err)
	}

	rows := make([]EndedCareRow, 0)
	query := build().
		ColumnExpr(`"student".id AS student_id`).
		ColumnExpr(`"person".first_name AS first_name`).
		ColumnExpr(`"person".last_name AS last_name`).
		ColumnExpr(`"student".school_class AS school_class`).
		ColumnExpr(`"student".enrolled_until AS last_care_day`).
		OrderExpr(`"student".enrolled_until DESC, "person".last_name ASC, "person".first_name ASC, "student".id ASC`)
	if filter.PageSize > 0 {
		query = query.Limit(filter.PageSize)
		if filter.Page > 1 {
			query = query.Offset((filter.Page - 1) * filter.PageSize)
		}
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, 0, fmt.Errorf("care exit view: list ended care: %w", err)
	}
	return rows, total, nil
}

// LockImpactPeople freezes the person rows whose names and RFID assignment the
// binding preview quotes back. The confirmation compares its token against
// them, so they must not move between the preview and the write.
func LockImpactPeople(ctx context.Context, db bun.IDB, tenantID int64, studentIDs []int64) error {
	if tenantID <= 0 {
		return ErrInvalidTenantID
	}
	if len(studentIDs) == 0 {
		return nil
	}
	_, err := db.ExecContext(ctx,
		`SELECT person.id FROM users.persons AS person
		 JOIN users.students AS student
		   ON student.person_id = person.id AND student.tenant_id = person.tenant_id
		 WHERE student.tenant_id = ? AND student.id IN (?) FOR UPDATE OF person`,
		tenantID, bun.List(studentIDs))
	if err != nil {
		return fmt.Errorf("care exit view: lock people for care exit: %w", err)
	}
	return nil
}

// ListSourceOfferingsAfter names the source bookings of the given children
// that still run on or after validUntil, in the offerings' display order.
func ListSourceOfferingsAfter(
	ctx context.Context,
	db bun.IDB,
	tenantID int64,
	studentIDs []int64,
	validUntil timezone.Date,
	sets Recordsets,
) ([]SourceOfferingRow, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	rows := make([]SourceOfferingRow, 0)
	if len(studentIDs) == 0 {
		return rows, nil
	}
	const query = recordsets + `
		SELECT rc.created_student_id AS student_id, co.name,
		       CASE WHEN co.days_of_week_mode = 'fixed' THEN co.available_days ELSE rco.selected_days END AS days
		FROM offering_links AS rco
		JOIN application_links AS rc
		  ON rc.id = rco.request_child_id AND rc.tenant_id = rco.tenant_id
		JOIN care_offerings AS co
		  ON co.id = rco.care_offering_id AND co.tenant_id = rco.tenant_id
		WHERE rco.tenant_id = ? AND rc.created_student_id IN (?)
		  AND (rco.valid_until IS NULL OR rco.valid_until > ?)
		ORDER BY rc.created_student_id, co.sort_order, co.id`
	if err := db.NewRaw(query,
		sets.ApplicationLinks, sets.CareOfferings, sets.OfferingLinks,
		tenantID, bun.List(studentIDs), validUntil,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care exit view: list source offerings for care exit preview: %w", err)
	}
	return rows, nil
}

// ListBookingExpiries reports the children whose last care-counting source
// booking ends while their enrolment still runs, with the day it ends on.
func ListBookingExpiries(
	ctx context.Context,
	db bun.IDB,
	tenantID int64,
	sets Recordsets,
) ([]BookingExpiryRow, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	rows := make([]BookingExpiryRow, 0)
	const query = recordsets + `
	SELECT rc.created_student_id AS student_id,
	       rco.valid_until AS first_bookingless_day,
	       rco.request_child_id AS source_request_child_id,
	       jsonb_agg(jsonb_build_object(
			'name', co.name,
			'days', CASE WHEN co.days_of_week_mode = 'fixed' THEN co.available_days ELSE rco.selected_days END
		)) AS source_offerings
	FROM offering_links AS rco
	JOIN application_links AS rc
	  ON rc.id = rco.request_child_id AND rc.tenant_id = rco.tenant_id
	JOIN care_offerings AS co
	  ON co.id = rco.care_offering_id AND co.tenant_id = rco.tenant_id
	JOIN users.students AS student
	  ON student.id = rc.created_student_id AND student.tenant_id = rc.tenant_id
	WHERE rco.tenant_id = ? AND rco.valid_until IS NOT NULL AND co.counts_as_care
	  AND ((co.days_of_week_mode = 'fixed' AND jsonb_array_length(co.available_days) > 0)
	    OR (co.days_of_week_mode <> 'fixed' AND jsonb_array_length(rco.selected_days) > 0))
	  AND student.status = 'active'
	  AND (student.enrolled_until IS NULL OR student.enrolled_until >= rco.valid_until)
	  AND NOT EXISTS (
		SELECT 1 FROM offering_links AS later
		JOIN care_offerings AS later_offering
		  ON later_offering.id = later.care_offering_id AND later_offering.tenant_id = later.tenant_id
		JOIN application_links AS later_child
		  ON later_child.id = later.request_child_id AND later_child.tenant_id = later.tenant_id
		WHERE later.tenant_id = rco.tenant_id AND later_child.created_student_id = rc.created_student_id
		  AND later_offering.counts_as_care
		  AND ((later_offering.days_of_week_mode = 'fixed' AND jsonb_array_length(later_offering.available_days) > 0)
		    OR (later_offering.days_of_week_mode <> 'fixed' AND jsonb_array_length(later.selected_days) > 0))
		  AND COALESCE(later.valid_from, '-infinity'::date) <= rco.valid_until
		  AND (later.valid_until IS NULL OR later.valid_until > rco.valid_until)
	  )
	GROUP BY rc.created_student_id, rco.valid_until, rco.request_child_id`
	if err := db.NewRaw(query,
		sets.ApplicationLinks, sets.CareOfferings, sets.OfferingLinks, tenantID,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care exit view: find expired final care bookings: %w", err)
	}
	return rows, nil
}

// ListCareStudents reads the children the booking evaluation covers: the given
// ids, or every child whose enrolment interval contains on.
func ListCareStudents(
	ctx context.Context,
	db bun.IDB,
	tenantID int64,
	on timezone.Date,
	studentIDs []int64,
	inactiveStatus, activeStatus string,
) ([]CareStudentRow, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	rows := make([]CareStudentRow, 0)
	query := db.NewSelect().
		TableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".id AS student_id`).
		ColumnExpr(`"person".first_name, "person".last_name`).
		ColumnExpr(`"student".school_class, "student".enrolled_until`).
		Join(`JOIN users.persons AS "person" ON "person".id = "student".person_id AND "person".tenant_id = "student".tenant_id`).
		Where(`"student".tenant_id = ?`, tenantID).
		Where(`"student".status <> 'alumnus'`).
		OrderExpr(`"student".id`)
	if len(studentIDs) > 0 {
		query = query.Where(`"student".id IN (?)`, bun.List(studentIDs))
	} else {
		query = query.
			Where(`NOT ("student".enrolled_from IS NULL AND "student".enrolled_until IS NULL AND "student".status = ?)`, inactiveStatus).
			Where(`("student".enrolled_from IS NULL OR "student".enrolled_from <= ? OR "student".status = ?)`, on, activeStatus).
			Where(`("student".enrolled_until IS NULL OR "student".enrolled_until >= ?)`, on)
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care exit view: list current care students for booking evaluation: %w", err)
	}
	return rows, nil
}

// ListCareBookingPeriods reads every care-counting booking window of the given
// children, oldest first, so the evaluator can merge them.
func ListCareBookingPeriods(
	ctx context.Context,
	db bun.IDB,
	tenantID int64,
	studentIDs []int64,
	approvedChildStatus string,
	sets Recordsets,
) ([]CareBookingPeriodRow, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	rows := make([]CareBookingPeriodRow, 0)
	if len(studentIDs) == 0 {
		return rows, nil
	}
	const query = recordsets + `
		SELECT COALESCE(rc.created_student_id, rc.matched_student_id) AS student_id, rco.valid_from, rco.valid_until,
		       rco.request_child_id AS source_request_child_id, co.name AS offering_name,
		       CASE WHEN co.days_of_week_mode = 'fixed' THEN co.available_days ELSE rco.selected_days END AS days
		FROM offering_links AS rco
		JOIN application_links AS rc
		  ON rc.id = rco.request_child_id AND rc.tenant_id = rco.tenant_id
		JOIN care_offerings AS co
		  ON co.id = rco.care_offering_id AND co.tenant_id = rco.tenant_id
		WHERE rco.tenant_id = ? AND COALESCE(rc.created_student_id, rc.matched_student_id) IN (?)
		  AND rc.status = ? AND co.counts_as_care
		  AND ((co.days_of_week_mode = 'fixed' AND jsonb_array_length(co.available_days) > 0)
		    OR (co.days_of_week_mode <> 'fixed' AND jsonb_array_length(rco.selected_days) > 0))
		ORDER BY COALESCE(rc.created_student_id, rc.matched_student_id), rco.valid_from NULLS FIRST, rco.valid_until NULLS LAST, rco.id`
	if err := db.NewRaw(query,
		sets.ApplicationLinks, sets.CareOfferings, sets.OfferingLinks,
		tenantID, bun.List(studentIDs), approvedChildStatus,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care exit view: list care booking periods for evaluation: %w", err)
	}
	return rows, nil
}
