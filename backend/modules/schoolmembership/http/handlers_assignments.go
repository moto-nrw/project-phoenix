package staff

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// getStaffGroups lists the groups a Betreuungskraft is assigned to. A staff
// member without a teacher profile has none, which is a 200 with an empty
// list — the group screens rely on that.
func (rs *Resource) getStaffGroups(w http.ResponseWriter, r *http.Request) {
	staff, ok := rs.parseAndFindStaff(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	teacher := rs.teacherByStaff(ctx, staff.ID)
	if teacher == nil {
		rs.respond(w, r, http.StatusOK, []GroupResponse{}, "Staff member is not a teacher and has no assigned groups")
		return
	}
	groups, err := rs.runtime.TeacherGroups(ctx, teacher.ID)
	if err != nil {
		rs.internal(w, r, err)
		return
	}
	responses := make([]GroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, GroupResponse(group))
	}
	rs.respond(w, r, http.StatusOK, responses, "Teacher groups retrieved successfully")
}

// getStaffSchoolClasses returns the school classes assigned to a staff member
// (#1772).
func (rs *Resource) getStaffSchoolClasses(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parseStaffID(w, r)
	if !ok {
		return
	}
	classes, err := rs.runtime.SchoolClasses(r.Context(), id)
	if err != nil {
		rs.runtime.SchoolClassFailure(w, r, err)
		return
	}
	rs.respond(w, r, http.StatusOK, SchoolClassesResponse{StaffID: id, SchoolClasses: classes}, "School classes retrieved successfully")
}

// updateStaffSchoolClasses replaces the school class assignments with the
// submitted set.
func (rs *Resource) updateStaffSchoolClasses(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parseStaffID(w, r)
	if !ok {
		return
	}
	req := &SchoolClassesRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.failure(w, r, FailureInvalidRequest, err, "invalid_request")
		return
	}
	ctx := r.Context()

	// The audit actor is the authenticated account (like bank-steuer): a
	// users:manage holder need not have a staff mapping.
	accountID := rs.runtime.CurrentAccountID(ctx)
	if accountID == 0 {
		rs.failure(w, r, FailureUnauthorized, errors.New("invalid token"), "unauthorized")
		return
	}
	if err := rs.runtime.SetSchoolClasses(ctx, id, req.SchoolClasses, accountID); err != nil {
		rs.runtime.SchoolClassFailure(w, r, err)
		return
	}
	classes, err := rs.runtime.SchoolClasses(ctx, id)
	if err != nil {
		rs.runtime.SchoolClassFailure(w, r, err)
		return
	}
	rs.respond(w, r, http.StatusOK, SchoolClassesResponse{StaffID: id, SchoolClasses: classes}, "School classes updated successfully")
}

// getAvailableStaff lists the teachers that can be assigned, with their staff
// and person data.
func (rs *Resource) getAvailableStaff(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	teachers, err := rs.membership.ListTeachers(ctx, schoolmembership.TeacherFilter{})
	if err != nil {
		rs.internal(w, r, err)
		return
	}
	staffIDs := make([]int64, 0, len(teachers))
	for _, teacher := range teachers {
		staffIDs = append(staffIDs, teacher.StaffID)
	}
	responses := make([]TeacherResponse, 0, len(teachers))
	if len(staffIDs) == 0 {
		rs.respond(w, r, http.StatusOK, responses, "Available staff members retrieved successfully")
		return
	}

	members, err := rs.membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: staffIDs})
	if err != nil {
		rs.internal(w, r, err)
		return
	}
	byID := make(map[int64]schoolmembership.Staff, len(members))
	for _, staff := range members {
		byID[staff.ID] = staff
	}
	persons, err := rs.personIndex(ctx, members)
	if err != nil {
		rs.internal(w, r, err)
		return
	}
	access := rs.fieldAccess(ctx)

	for _, teacher := range teachers {
		staff, found := byID[teacher.StaffID]
		if !found {
			continue
		}
		person := persons[staff.PersonID]
		if person == nil {
			continue
		}
		responses = append(responses, buildTeacherResponse(access, staff, person, teacher, enrichment{}))
	}
	rs.respond(w, r, http.StatusOK, responses, "Available staff members retrieved successfully")
}

// getStaffByRole answers GET /by-role?role=user or ?roles=teacher,staff,user.
// The single role "user" or "staff" asks for the active caregiver pool, every
// other role set is a plain account-role lookup.
func (rs *Resource) getStaffByRole(w http.ResponseWriter, r *http.Request) {
	roles := parseRolesQuery(r)
	if len(roles) == 0 {
		rs.failure(w, r, FailureInvalidRequest, errors.New("role or roles parameter is required"), "invalid_parameters")
		return
	}
	ctx := r.Context()

	if requestedCaregiverPool(roles) {
		caregivers, err := rs.runtime.ActiveCaregivers(ctx)
		if err != nil {
			rs.internal(w, r, err)
			return
		}
		if caregivers == nil {
			caregivers = []StaffWithRoleResponse{}
		}
		rs.respond(w, r, http.StatusOK, caregivers, "Active caregivers retrieved successfully")
		return
	}

	rows, err := rs.runtime.StaffByRoles(ctx, roles)
	if err != nil {
		rs.internal(w, r, err)
		return
	}
	rs.respond(w, r, http.StatusOK, dedupeByStaffID(rows), "Staff members with role retrieved successfully")
}

// parseRolesQuery reads the requested roles, supporting both ?role=teacher
// (singular, backward compat) and ?roles=teacher,staff,user (multi).
func parseRolesQuery(r *http.Request) []string {
	if rolesParam := r.URL.Query().Get("roles"); rolesParam != "" {
		var roles []string
		for _, role := range strings.Split(rolesParam, ",") {
			if role = strings.TrimSpace(role); role != "" {
				roles = append(roles, role)
			}
		}
		return roles
	}
	if roleParam := strings.TrimSpace(r.URL.Query().Get("role")); roleParam != "" {
		return []string{roleParam}
	}
	return nil
}

// requestedCaregiverPool reports whether the query asks for the caregiver
// pool: exactly one role, and that role is "user" or "staff".
func requestedCaregiverPool(roles []string) bool {
	if len(roles) != 1 {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(roles[0])) {
	case "user", "staff":
		return true
	default:
		return false
	}
}

// dedupeByStaffID keeps the first row per staff member: one person matching
// several requested roles appears once. The nil result of an empty match is
// deliberate — the endpoint has always answered `"data":null` there.
func dedupeByStaffID(rows []StaffWithRoleResponse) []StaffWithRoleResponse {
	seen := make(map[int64]bool, len(rows))
	var results []StaffWithRoleResponse
	for _, row := range rows {
		if seen[row.ID] {
			continue
		}
		seen[row.ID] = true
		results = append(results, row)
	}
	return results
}
