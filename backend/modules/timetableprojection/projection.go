// Package timetableprojection contains tenant-safe read models that combine
// timetable-owned rows with legacy workflow data during the module migration.
package timetableprojection

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// ErrInvalidTenantID reports a missing or non-positive projection tenant.
var ErrInvalidTenantID = errors.New("timetable projection: tenant ID must be positive")

// courseGroupType is the activities.groups type of a course.
const courseGroupType = "activity"

// CourseGroup is a locked course group in the projection's own vocabulary:
// an active course and its participant limit, nil when the course has none.
type CourseGroup struct {
	ID               int64
	Active           bool
	ParticipantLimit *int
}

// LockCourseGroups serializes capacity checks and the enrollment write for
// every shared course group. IDs are ordered so overlapping approvals cannot
// deadlock.
func (Reader) LockCourseGroups(ctx context.Context, db bun.IDB, tenantID int64, groupIDs []int64) ([]CourseGroup, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	ids := append([]int64(nil), groupIDs...)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if len(ids) == 0 {
		return []CourseGroup{}, nil
	}
	var groups []struct {
		ID              int64 `bun:"id"`
		MaxParticipants int   `bun:"max_participants"`
	}
	err := db.NewSelect().TableExpr(`activities.groups AS "group"`).
		ColumnExpr(`"group".id, COALESCE("group".max_participants, 0) AS max_participants`).
		Where(`"group".tenant_id = ?`, tenantID).
		Where(`"group".id IN (?)`, bun.List(ids)).
		Where(`"group".type = ?`, courseGroupType).
		Where(`"group".archived_at IS NULL`).
		OrderExpr(`"group".id`).
		For("UPDATE").
		Scan(ctx, &groups)
	if err != nil {
		return nil, fmt.Errorf("timetable projection: lock course groups: %w", err)
	}
	locked := make([]CourseGroup, 0, len(groups))
	for _, group := range groups {
		locked = append(locked, CourseGroup{ID: group.ID, Active: true, ParticipantLimit: participantLimit(group.MaxParticipants)})
	}
	return locked, nil
}

// participantLimit maps the stored zero of an unlimited course to nil.
func participantLimit(limit int) *int {
	if limit <= 0 {
		return nil
	}
	return &limit
}

// CountActiveCourseEnrollments returns each course's peak roster occupancy in
// [from, until). A seat is held for the full enrollment window, independently
// of individual days. excludeStudentID omits an existing seat of the child
// currently being checked.
func (Reader) CountActiveCourseEnrollments(ctx context.Context, db bun.IDB, tenantID int64, groupIDs []int64, from, until calendar.Date, excludeStudentID int64) (map[int64]int, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	counts := make(map[int64]int, len(groupIDs))
	if len(groupIDs) == 0 {
		return counts, nil
	}
	if !from.Before(until) {
		return nil, fmt.Errorf("timetable projection: course occupancy range must not be empty")
	}
	var rows []struct {
		GroupID int64 `bun:"activity_group_id"`
		Count   int   `bun:"count"`
	}
	err := db.NewRaw(`WITH intervals AS (
		SELECT activity_group_id, student_id,
		       GREATEST(valid_from, ?) AS starts_at,
		       LEAST(COALESCE(valid_until, ?), ?) AS ends_at
		FROM activities.student_enrollments
		WHERE tenant_id = ? AND activity_group_id IN (?)
		  AND valid_from < ? AND (valid_until IS NULL OR valid_until > ?)
		  AND (? = 0 OR student_id <> ?)
	), boundaries AS (
		SELECT activity_group_id, starts_at AS boundary FROM intervals
		UNION
		SELECT activity_group_id, ends_at AS boundary FROM intervals
	)
	SELECT boundaries.activity_group_id, COALESCE(MAX((
		SELECT COUNT(DISTINCT interval_row.student_id)
		FROM intervals AS interval_row
		WHERE interval_row.activity_group_id = boundaries.activity_group_id
		  AND interval_row.starts_at <= boundaries.boundary
		  AND interval_row.ends_at > boundaries.boundary
	)), 0)::int AS count
	FROM boundaries
	GROUP BY boundaries.activity_group_id`,
		from, until, until, tenantID, bun.List(groupIDs), until, from, excludeStudentID, excludeStudentID,
	).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("timetable projection: count active course enrollments: %w", err)
	}
	for _, row := range rows {
		counts[row.GroupID] = row.Count
	}
	return counts, nil
}

// CountSourceEnrollments counts the timetable enrollments an enrollment
// request (childID nil) or one of its children sourced.
func (Reader) CountSourceEnrollments(ctx context.Context, db bun.IDB, tenantID, requestID int64, childID *int64) (int, error) {
	if childID == nil {
		return CountRequestSourceEnrollments(ctx, db, tenantID, requestID)
	}
	return CountChildSourceEnrollments(ctx, db, tenantID, requestID, *childID)
}

func CountRequestSourceEnrollments(ctx context.Context, db bun.IDB, tenantID, requestID int64) (int, error) {
	if tenantID <= 0 {
		return 0, ErrInvalidTenantID
	}
	var count int
	err := db.NewRaw(`SELECT COUNT(*)::int FROM activities.student_enrollments AS enrollment
		JOIN enrollment.request_children AS child
		  ON child.tenant_id = enrollment.tenant_id AND child.id = enrollment.enrollment_request_child_id
		WHERE enrollment.tenant_id = ? AND child.request_id = ?`, tenantID, requestID).Scan(ctx, &count)
	return count, wrapCountError(err)
}

func CountChildSourceEnrollments(ctx context.Context, db bun.IDB, tenantID, requestID, childID int64) (int, error) {
	if tenantID <= 0 {
		return 0, ErrInvalidTenantID
	}
	var count int
	err := db.NewRaw(`SELECT COUNT(*)::int FROM activities.student_enrollments AS enrollment
		JOIN enrollment.request_children AS child
		  ON child.tenant_id = enrollment.tenant_id AND child.id = enrollment.enrollment_request_child_id
		WHERE enrollment.tenant_id = ? AND child.request_id = ? AND child.id = ?`, tenantID, requestID, childID).Scan(ctx, &count)
	return count, wrapCountError(err)
}

func CountStudentEnrollments(ctx context.Context, db bun.IDB, tenantID, studentID int64) (int, error) {
	if tenantID <= 0 {
		return 0, ErrInvalidTenantID
	}
	var count int
	err := db.NewRaw(`SELECT COUNT(*)::int FROM activities.student_enrollments
		WHERE tenant_id = ? AND student_id = ?`, tenantID, studentID).Scan(ctx, &count)
	return count, wrapCountError(err)
}

func wrapCountError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("timetable projection: count enrollments: %w", err)
}

// StudentEnrollments is the deletion's bounded read capability.
type EnrollmentDatabase func(context.Context) (bun.IDB, int64, error)
type StudentEnrollments struct {
	database EnrollmentDatabase
}

func (Reader) StudentEnrollments(database EnrollmentDatabase) StudentEnrollments {
	return StudentEnrollments{database: database}
}
func (r StudentEnrollments) CountStudentEnrollments(ctx context.Context, id int64) (int, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return 0, err
	}
	return CountStudentEnrollments(ctx, db, tenantID, id)
}

// Reader serves the root-bound course and enrollment projections.
type Reader struct{}
