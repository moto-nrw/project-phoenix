package postgres

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/uptrace/bun"
)

// whereInList narrows a listing to the rows whose column is in ids. A nil list
// leaves the query unchanged; an empty list matches nothing, which the false
// result reports so the caller can answer without a statement.
func whereInList(query *bun.SelectQuery, condition string, ids []int64) (*bun.SelectQuery, bool) {
	if ids == nil {
		return query, true
	}
	if len(ids) == 0 {
		return query, false
	}
	return query.Where(condition, bun.List(ids)), true
}

// teacherListQuery applies a teacher filter; false means it matches nothing.
func teacherListQuery(query *bun.SelectQuery, filter domain.TeacherFilter) (*bun.SelectQuery, bool) {
	if !filter.IncludeDeleted {
		query = query.Where(`"teacher".deleted_at IS NULL`)
	}
	query, ok := whereInList(query, `"teacher".id IN (?)`, filter.IDs)
	if !ok {
		return query, false
	}
	query, ok = whereInList(query, `"teacher".staff_id IN (?)`, filter.StaffIDs)
	if !ok {
		return query, false
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

// guestListQuery applies a guest filter; false means it matches nothing.
func guestListQuery(query *bun.SelectQuery, filter domain.GuestFilter) (*bun.SelectQuery, bool, error) {
	query, ok := whereInList(query, `"guest".id IN (?)`, filter.IDs)
	if !ok {
		return query, false, nil
	}
	query, ok = whereInList(query, `"guest".staff_id IN (?)`, filter.StaffIDs)
	if !ok {
		return query, false, nil
	}
	if filter.ActiveOn != "" {
		activeOn, err := optionalDate(filter.ActiveOn, "active-on date")
		if err != nil {
			return query, false, err
		}
		query = query.Where(`("guest".start_date IS NULL OR "guest".start_date <= ?)`, activeOn).
			Where(`("guest".end_date IS NULL OR "guest".end_date >= ?)`, activeOn)
	}
	if filter.OrganizationContains != "" {
		query = query.Where(`"guest".organization ILIKE ?`, "%"+escapeLike(filter.OrganizationContains)+"%")
	}
	if filter.ExpertiseContains != "" {
		query = query.Where(`"guest".activity_expertise ILIKE ?`, "%"+escapeLike(filter.ExpertiseContains)+"%")
	}
	if filter.HasOrganization != nil {
		if *filter.HasOrganization {
			query = query.Where(`"guest".organization IS NOT NULL`)
		} else {
			query = query.Where(`"guest".organization IS NULL`)
		}
	}
	return query, true, nil
}

// groupAssignmentListQuery applies a group assignment filter; false means it
// matches nothing.
func groupAssignmentListQuery(query *bun.SelectQuery, filter domain.GroupAssignmentFilter) (*bun.SelectQuery, bool) {
	query, ok := whereInList(query, `"group_assignment".id IN (?)`, filter.IDs)
	if !ok {
		return query, false
	}
	query, ok = whereInList(query, `"group_assignment".group_id IN (?)`, filter.GroupIDs)
	if !ok {
		return query, false
	}
	query, ok = whereInList(query, `"group_assignment".teacher_id IN (?)`, filter.TeacherIDs)
	if !ok {
		return query, false
	}
	if filter.TeacherStaffIDs != nil {
		if len(filter.TeacherStaffIDs) == 0 {
			return query, false
		}
		query = query.
			Join(`INNER JOIN users.teachers AS "teacher" ON "teacher".id = "group_assignment".teacher_id`).
			Where(`"teacher".staff_id IN (?)`, bun.List(filter.TeacherStaffIDs))
	}
	query = query.OrderExpr(`"group_assignment".id ASC`)
	if filter.ForUpdate {
		query = query.For("UPDATE OF group_assignment")
	}
	return query, true
}
