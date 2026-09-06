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
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

// ErrInvalidTenantID reports a missing or non-positive projection tenant.
var ErrInvalidTenantID = errors.New("timetable projection: tenant ID must be positive")

func GroupNames(ctx context.Context, db bun.IDB, tenantID int64, ids []int64) (map[int64]string, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	result := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []struct {
		ID   int64  `bun:"id"`
		Name string `bun:"name"`
	}
	query := db.NewSelect().
		TableExpr(`activities.groups AS "group"`).
		ColumnExpr(`"group".id, "group".name`).
		Where(`"group".id IN (?)`, bun.List(ids)).
		Where(`"group".tenant_id = ?`, tenantID)
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("timetable projection: list group names: %w", err)
	}
	for _, row := range rows {
		result[row.ID] = row.Name
	}
	return result, nil
}

func ActivityGroupsByID(ctx context.Context, db bun.IDB, tenantID int64, ids []int64) ([]*activitiesModels.Group, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	if len(ids) == 0 {
		return []*activitiesModels.Group{}, nil
	}
	var groups []*activitiesModels.Group
	err := db.NewSelect().Model(&groups).ModelTableExpr(`activities.groups AS "group"`).
		Where(`"group".tenant_id = ?`, tenantID).Where(`"group".id IN (?)`, bun.List(ids)).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("timetable projection: list activity groups: %w", err)
	}
	return groups, nil
}

// CourseGroupsForOfferings projects active activity templates that a care
// offering reaches, through either its legacy group id or a source-offering
// declaration. Enrollment consumes this named, tenant-safe projection instead
// of timetable repositories.
func CourseGroupsForOfferings(
	ctx context.Context,
	db bun.IDB,
	tenantID int64,
	offerings []enrollmentModels.CourseOfferingReference,
) (map[int64][]enrollmentModels.CourseGroup, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	result := make(map[int64][]enrollmentModels.CourseGroup, len(offerings))
	legacyToOfferings := make(map[int64][]int64, len(offerings))
	offeringIDs := make([]int64, 0, len(offerings))
	legacyIDs := make([]int64, 0, len(offerings))
	for _, offering := range offerings {
		if offering.OfferingID <= 0 {
			continue
		}
		offeringIDs = append(offeringIDs, offering.OfferingID)
		if offering.ActivityGroupID != nil && *offering.ActivityGroupID > 0 {
			legacyToOfferings[*offering.ActivityGroupID] = append(
				legacyToOfferings[*offering.ActivityGroupID], offering.OfferingID,
			)
			legacyIDs = append(legacyIDs, *offering.ActivityGroupID)
		}
	}
	if len(offeringIDs) == 0 {
		return result, nil
	}
	var groups []*activitiesModels.Group
	query := db.NewSelect().Model(&groups).ModelTableExpr(`activities.groups AS "group"`).
		Where(`"group".tenant_id = ?`, tenantID).
		Where(`"group".type = ?`, activitiesModels.GroupTypeActivity).
		WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			if len(legacyIDs) > 0 {
				q = q.WhereOr(`"group".id IN (?)`, bun.List(legacyIDs))
			}
			return q.WhereOr(`"group".is_template = TRUE AND EXISTS (
				SELECT 1 FROM jsonb_array_elements_text(COALESCE("group".source_care_offering_ids, '[]'::jsonb)) AS source(id)
				WHERE source.id::bigint IN (?)
			)`, bun.List(offeringIDs))
		})
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("timetable projection: list course groups: %w", err)
	}
	for _, group := range groups {
		if group == nil {
			continue
		}
		course := enrollmentModels.CourseGroup{
			ID:                  group.ID,
			Active:              group.ArchivedAt == nil,
			ParticipantLimit:    group.ParticipantLimit(),
			SourceGradeLevels:   append([]int(nil), group.SourceGradeLevels...),
			SourceSchoolClasses: append([]string(nil), group.SourceSchoolClasses...),
		}
		for _, offeringID := range legacyToOfferings[group.ID] {
			result[offeringID] = append(result[offeringID], course)
		}
		for _, offeringID := range group.SourceCareOfferingIDs {
			if !slices.Contains(offeringIDs, offeringID) {
				continue
			}
			result[offeringID] = append(result[offeringID], course)
		}
	}
	return result, nil
}

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

func ListManualPlanningOccurrences(ctx context.Context, db bun.IDB, tenantID, studentID int64, from, to timezone.Date) ([]ManualPlanningOccurrence, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	var rows []struct {
		ActivityGroupID   int64
		ActivityGroupName string
		InstanceID        int64
		Date              timezone.Date
	}
	err := db.NewRaw(`
		SELECT activity_group.id AS activity_group_id, activity_group.name AS activity_group_name,
		       activity_instance.id AS instance_id, activity_instance.date
		FROM schedule.instance_students AS instance_student
		JOIN schedule.activity_instances AS activity_instance
		  ON activity_instance.id = instance_student.instance_id AND activity_instance.tenant_id = instance_student.tenant_id
		JOIN activities.groups AS activity_group
		  ON activity_group.id = activity_instance.activity_group_id AND activity_group.tenant_id = activity_instance.tenant_id
		WHERE instance_student.tenant_id = ? AND instance_student.student_id = ?
		  AND instance_student.is_unplanned = FALSE AND instance_student.not_scheduled = FALSE
		  AND activity_instance.date BETWEEN ? AND ? AND activity_instance.status = ?
		  AND activity_instance.calendar_period_id IS NOT NULL AND activity_instance.is_spontaneous = FALSE
		  AND activity_group.is_template = TRUE AND activity_group.type = ?
		  AND COALESCE(jsonb_array_length(activity_group.source_care_offering_ids), 0) = 0
		ORDER BY activity_group.name, activity_group.id, activity_instance.date, activity_instance.id`,
		tenantID, studentID, from, to, scheduleModels.InstanceStatusPlanned, activitiesModels.GroupTypeCare).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("timetable projection: list manual planning occurrences: %w", err)
	}
	result := make([]ManualPlanningOccurrence, 0, len(rows))
	for _, row := range rows {
		result = append(result, ManualPlanningOccurrence{ActivityGroupID: row.ActivityGroupID, ActivityGroupName: row.ActivityGroupName, InstanceID: row.InstanceID, Date: row.Date.String()})
	}
	return result, nil
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

func CountRunningEnrollmentsAfter(ctx context.Context, db bun.IDB, tenantID int64, studentIDs []int64, validUntil timezone.Date, removals string) (map[int64]int, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	counts := make(map[int64]int, len(studentIDs))
	if len(studentIDs) == 0 {
		return counts, nil
	}
	var rows []struct {
		StudentID int64 `bun:"student_id"`
		Total     int   `bun:"total"`
	}
	err := db.NewRaw(runningEnrollmentsQuery, removals, tenantID, bun.List(studentIDs), validUntil, validUntil,
		removals, tenantID, bun.List(studentIDs), validUntil).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("timetable projection: count running enrollments: %w", err)
	}
	for _, row := range rows {
		counts[row.StudentID] = row.Total
	}
	return counts, nil
}

func wrapCountError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("timetable projection: count enrollments: %w", err)
}

const runningEnrollmentsQuery = `SELECT student_id, COUNT(*)::int AS total FROM (
	SELECT enrollment.student_id
	FROM activities.student_enrollments AS enrollment
	LEFT JOIN jsonb_to_recordset(?::jsonb) AS removal(
		tenant_id bigint, student_id bigint, kind text, enrollment_id bigint,
		was_deleted boolean, previous_valid_until date)
	  ON removal.kind = 'booking' AND removal.tenant_id = enrollment.tenant_id
	 AND removal.enrollment_id = enrollment.id AND removal.was_deleted = FALSE
	WHERE enrollment.tenant_id = ? AND enrollment.student_id IN (?)
	  AND ((removal.enrollment_id IS NULL AND (enrollment.valid_until IS NULL OR enrollment.valid_until > ?))
	    OR (removal.enrollment_id IS NOT NULL AND (removal.previous_valid_until IS NULL OR removal.previous_valid_until > ?)))
	UNION ALL
	SELECT removal.student_id
	FROM jsonb_to_recordset(?::jsonb) AS removal(
		tenant_id bigint, student_id bigint, kind text, enrollment_id bigint,
		was_deleted boolean, previous_valid_until date)
	WHERE removal.kind = 'booking' AND removal.was_deleted = TRUE
	  AND removal.tenant_id = ? AND removal.student_id IN (?)
	  AND (removal.previous_valid_until IS NULL OR removal.previous_valid_until > ?)
	  AND NOT EXISTS (SELECT 1 FROM activities.student_enrollments AS live
		WHERE live.id = removal.enrollment_id AND live.tenant_id = removal.tenant_id)
) AS baseline GROUP BY student_id`

type ManualPlanningOccurrence struct {
	ActivityGroupID   int64
	ActivityGroupName string
	InstanceID        int64
	Date              string
}
