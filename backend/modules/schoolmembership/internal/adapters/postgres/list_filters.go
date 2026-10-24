package postgres

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/uptrace/bun"
)

// matchesNothing reports an ID filter that is set but empty: a nil list leaves
// the listing unfiltered, an empty one matches no row, so the caller can
// answer without a statement.
func matchesNothing(ids ...[]int64) bool {
	for _, list := range ids {
		if list != nil && len(list) == 0 {
			return true
		}
	}
	return false
}

// teacherListQuery applies a teacher filter; false means it matches nothing.
func teacherListQuery(query *bun.SelectQuery, filter schoolmembership.TeacherFilter) (*bun.SelectQuery, bool) {
	if !filter.IncludeDeleted {
		query = query.Where(`"teacher".deleted_at IS NULL`)
	}
	if matchesNothing(filter.IDs, filter.StaffIDs) {
		return query, false
	}
	if filter.IDs != nil {
		query = query.Where(`"teacher".id IN (?)`, bun.List(filter.IDs))
	}
	if filter.StaffIDs != nil {
		query = query.Where(`"teacher".staff_id IN (?)`, bun.List(filter.StaffIDs))
	}
	if filter.Specialization != "" {
		query = query.Where(`LOWER("teacher".specialization) = LOWER(?)`, filter.Specialization)
	}
	if filter.SpecializationContains != "" {
		query = query.Where(`"teacher".specialization ILIKE ?`, "%"+escapeLike(filter.SpecializationContains)+"%")
	}
	if filter.RoleContains != "" {
		query = query.Where(`"teacher".role ILIKE ?`, "%"+escapeLike(filter.RoleContains)+"%")
	}
	if filter.HasQualifications != nil {
		if *filter.HasQualifications {
			query = query.Where(`"teacher".qualifications IS NOT NULL AND "teacher".qualifications <> ''`)
		} else {
			query = query.Where(`("teacher".qualifications IS NULL OR "teacher".qualifications = '')`)
		}
	}
	return query, true
}

// externalCaregiverRecordListQuery applies an external caregiver record filter; false means it matches nothing.
func externalCaregiverRecordListQuery(query *bun.SelectQuery, filter schoolmembership.ExternalCaregiverRecordFilter) (*bun.SelectQuery, bool, error) {
	if matchesNothing(filter.IDs, filter.StaffIDs) {
		return query, false, nil
	}
	if filter.IDs != nil {
		query = query.Where(`"external_caregiver".id IN (?)`, bun.List(filter.IDs))
	}
	if filter.StaffIDs != nil {
		query = query.Where(`"external_caregiver".staff_id IN (?)`, bun.List(filter.StaffIDs))
	}
	if filter.ActiveOn != "" {
		activeOn, err := optionalDate(filter.ActiveOn, "active-on date")
		if err != nil {
			return query, false, err
		}
		query = query.Where(`("external_caregiver".start_date IS NULL OR "external_caregiver".start_date <= ?)`, activeOn).
			Where(`("external_caregiver".end_date IS NULL OR "external_caregiver".end_date >= ?)`, activeOn)
	}
	if filter.OrganizationContains != "" {
		query = query.Where(`"external_caregiver".organization ILIKE ?`, "%"+escapeLike(filter.OrganizationContains)+"%")
	}
	if filter.ExpertiseContains != "" {
		query = query.Where(`"external_caregiver".activity_expertise ILIKE ?`, "%"+escapeLike(filter.ExpertiseContains)+"%")
	}
	if filter.HasOrganization != nil {
		if *filter.HasOrganization {
			query = query.Where(`"external_caregiver".organization IS NOT NULL`)
		} else {
			query = query.Where(`"external_caregiver".organization IS NULL`)
		}
	}
	return query, true, nil
}

// groupAssignmentListQuery applies a group assignment filter; false means it
// matches nothing.
func groupAssignmentListQuery(query *bun.SelectQuery, filter schoolmembership.GroupAssignmentFilter, forUpdate bool) (*bun.SelectQuery, bool) {
	if matchesNothing(filter.IDs, filter.GroupIDs, filter.TeacherIDs, filter.TeacherStaffIDs) {
		return query, false
	}
	if filter.IDs != nil {
		query = query.Where(`"group_assignment".id IN (?)`, bun.List(filter.IDs))
	}
	if filter.GroupIDs != nil {
		query = query.Where(`"group_assignment".group_id IN (?)`, bun.List(filter.GroupIDs))
	}
	if filter.TeacherIDs != nil {
		query = query.Where(`"group_assignment".teacher_id IN (?)`, bun.List(filter.TeacherIDs))
	}
	if filter.TeacherStaffIDs != nil {
		query = query.
			Join(`INNER JOIN users.teachers AS "teacher" ON "teacher".id = "group_assignment".teacher_id`).
			Where(`"teacher".staff_id IN (?)`, bun.List(filter.TeacherStaffIDs))
	}
	query = query.OrderExpr(`"group_assignment".id ASC`)
	if forUpdate {
		query = query.For("UPDATE OF group_assignment")
	}
	return query, true
}
