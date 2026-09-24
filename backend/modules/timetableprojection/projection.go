// Package timetableprojection contains tenant-safe read models that combine
// timetable-owned rows with legacy workflow data during the module migration.
package timetableprojection

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/uptrace/bun"
)

// ErrInvalidTenantID reports a missing or non-positive projection tenant.
var ErrInvalidTenantID = errors.New("timetable projection: tenant ID must be positive")

// LockCourseGroups serializes capacity checks and the enrollment write for
// every shared course group. IDs are ordered so overlapping approvals cannot
// deadlock.
func LockCourseGroups(ctx context.Context, db bun.IDB, tenantID int64, groupIDs []int64) ([]enrollmentModels.CourseGroup, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	ids := append([]int64(nil), groupIDs...)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if len(ids) == 0 {
		return []enrollmentModels.CourseGroup{}, nil
	}
	var groups []*activitiesModels.Group
	err := db.NewSelect().Model(&groups).ModelTableExpr(`activities.groups AS "group"`).
		Where(`"group".tenant_id = ?`, tenantID).
		Where(`"group".id IN (?)`, bun.List(ids)).
		Where(`"group".type = ?`, activitiesModels.GroupTypeActivity).
		Where(`"group".archived_at IS NULL`).
		OrderExpr(`"group".id`).
		For("UPDATE").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("timetable projection: lock course groups: %w", err)
	}
	locked := make([]enrollmentModels.CourseGroup, 0, len(groups))
	for _, group := range groups {
		if group != nil {
			locked = append(locked, enrollmentModels.CourseGroup{
				ID:               group.ID,
				Active:           true,
				ParticipantLimit: group.ParticipantLimit(),
			})
		}
	}
	return locked, nil
}

// CountActiveCourseEnrollments returns each course's peak roster occupancy in
// [from, until). A seat is held for the full enrollment window, independently
// of individual days. excludeStudentID omits an existing seat of the child
// currently being checked.
func CountActiveCourseEnrollments(ctx context.Context, db bun.IDB, tenantID int64, groupIDs []int64, from, until timezone.Date, excludeStudentID int64) (map[int64]int, error) {
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
