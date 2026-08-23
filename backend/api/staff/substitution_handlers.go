package staff

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

func (rs *Resource) listActiveCaregivers(ctx context.Context) ([]*users.ActiveCaregiver, error) {
	directory, err := usersSvc.CaregiverDirectoryFromPersonService(rs.PersonService)
	if err != nil {
		return nil, fmt.Errorf("resolve caregiver directory: %w", err)
	}

	return directory.ListActiveCaregivers(ctx)
}

func caregiverStaffIDSet(caregivers []*users.ActiveCaregiver) map[int64]struct{} {
	result := make(map[int64]struct{}, len(caregivers))
	for _, caregiver := range caregivers {
		result[caregiver.StaffID] = struct{}{}
	}
	return result
}

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

// loadWorkStatusMap loads work session status map (non-critical, returns empty map on error)
func (rs *Resource) loadWorkStatusMap(ctx context.Context) map[int64]string {
	if rs.WorkSessionService == nil {
		return make(map[int64]string)
	}
	wsm, err := rs.WorkSessionService.GetTodayPresenceMap(ctx)
	if err != nil {
		rs.getLogger().Warn("failed to fetch work status map", slog.String("error", err.Error()))
		return make(map[int64]string)
	}
	return wsm
}

// loadAbsenceMap loads absence status map (non-critical, returns empty map on error)
func (rs *Resource) loadAbsenceMap(ctx context.Context) map[int64]string {
	if rs.StaffAbsenceService == nil {
		return make(map[int64]string)
	}
	am, err := rs.StaffAbsenceService.GetTodayAbsenceMap(ctx)
	if err != nil {
		rs.getLogger().Warn("failed to fetch absence map", slog.String("error", err.Error()))
		return make(map[int64]string)
	}
	return am
}

// loadAbsenceLabelMap loads the school's own Abwesenheitsart wording per staff
// member for today (#2403). Non-critical like loadAbsenceMap: an error yields
// an empty map, and every staff member then falls back to the standard label.
func (rs *Resource) loadAbsenceLabelMap(ctx context.Context) map[int64]string {
	if rs.StaffAbsenceService == nil {
		return make(map[int64]string)
	}
	labels, err := rs.StaffAbsenceService.GetTodayAbsenceLabelMap(ctx)
	if err != nil {
		rs.getLogger().Warn("failed to fetch absence label map", slog.String("error", err.Error()))
		return make(map[int64]string)
	}
	return labels
}

// loadAccountMap batch-loads one per-account string attribute (role name,
// email, avatar path) for all staff members. Non-critical: returns an empty
// map when AuthService is missing, no staff member has an account, or the
// fetch fails (logged as a warning).
func (rs *Resource) loadAccountMap(ctx context.Context, staffMembers []*users.Staff, label string, fetch func(context.Context, []int64) (map[int64]string, error)) map[int64]string {
	if rs.AuthService == nil {
		return make(map[int64]string)
	}

	// Collect account IDs from staff members
	accountIDs := make([]int64, 0, len(staffMembers))
	for _, s := range staffMembers {
		if s.Person != nil && s.Person.AccountID != nil {
			accountIDs = append(accountIDs, *s.Person.AccountID)
		}
	}

	if len(accountIDs) == 0 {
		return make(map[int64]string)
	}

	m, err := fetch(ctx, accountIDs)
	if err != nil {
		rs.getLogger().Warn("failed to fetch account "+label+" map", slog.String("error", err.Error()))
		return make(map[int64]string)
	}
	return m
}

// getStaffGroups handles getting groups for a staff member
func (rs *Resource) getStaffGroups(w http.ResponseWriter, r *http.Request) {
	// Parse and get staff
	staff, ok := rs.parseAndGetStaff(w, r)
	if !ok {
		return
	}

	// Check if this staff member is a teacher
	teacher, err := rs.PersonService.GetTeacherByStaffID(r.Context(), staff.ID)
	if err != nil || teacher == nil {
		// If not a teacher, return empty groups list
		common.Respond(w, r, http.StatusOK, []GroupResponse{}, "Staff member is not a teacher and has no assigned groups")
		return
	}

	// Check if we have a reference to the Education service
	if rs.EducationService == nil {
		// If not, return an error
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("education service not available")))
		return
	}

	// Get groups for this teacher
	groups, err := rs.EducationService.GetTeacherGroups(r.Context(), teacher.ID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Build response
	responses := make([]GroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, GroupResponse{
			ID:   group.ID,
			Name: group.Name,
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Teacher groups retrieved successfully")
}

// getAvailableStaff handles getting available staff members (teachers) for assignments
// Optimized to avoid N+1 queries by using ListAllWithStaffAndPerson
func (rs *Resource) getAvailableStaff(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get all teachers with staff and person data in a single query (avoids N+1)
	teachers, err := rs.PersonService.ListTeachersWithStaffAndPerson(ctx)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Build response objects - all returned items are teachers by definition
	responses := make([]TeacherResponse, 0, len(teachers))

	for _, teacher := range teachers {
		// Skip if staff or person data is missing
		if teacher.Staff == nil || teacher.Staff.Person == nil {
			continue
		}

		// Create teacher response using pre-loaded data (false for wasPresentToday - not needed here)
		responses = append(responses, newTeacherResponse(teacher.Staff, teacher, false, "", "", "", "", ""))
	}

	common.Respond(w, r, http.StatusOK, responses, "Available staff members retrieved successfully")
}

// getStaffSubstitutions handles getting substitutions for a staff member
func (rs *Resource) getStaffSubstitutions(w http.ResponseWriter, r *http.Request) {
	// Parse and get staff
	staff, ok := rs.parseAndGetStaff(w, r)
	if !ok {
		return
	}

	// Check if we have a reference to the Education service
	if rs.EducationService == nil {
		// If not, return an error
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("education service not available")))
		return
	}

	// Get substitutions where this staff member is the substitute
	substitutions, err := rs.EducationService.GetStaffSubstitutions(r.Context(), staff.ID, false)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, substitutions, "Staff substitutions retrieved successfully")
}

// buildSubstitutionInfoList creates substitution info from active substitutions
func buildSubstitutionInfoList(subs []*education.GroupSubstitution) []SubstitutionInfo {
	result := make([]SubstitutionInfo, 0, len(subs))
	for _, sub := range subs {
		info := SubstitutionInfo{
			ID:         sub.ID,
			GroupID:    sub.GroupID,
			IsTransfer: sub.Duration() == 1,
			StartDate:  sub.StartDate.String(),
			EndDate:    sub.EndDate.String(),
		}
		if sub.Group != nil {
			info.GroupName = sub.Group.Name
			info.Group = sub.Group
		}
		result = append(result, info)
	}
	return result
}

// buildStaffSubstitutionStatus creates a staff status entry with substitution data
func (rs *Resource) buildStaffSubstitutionStatus(
	ctx context.Context,
	staff *users.Staff,
	teacher *users.Teacher,
	subs []*education.GroupSubstitution,
) StaffWithSubstitutionStatus {
	staffResp := newStaffResponse(staff, false, false, "", "", "", "", "")
	result := StaffWithSubstitutionStatus{
		StaffResponse:     &staffResp,
		IsSubstituting:    len(subs) > 0,
		SubstitutionCount: len(subs),
		Substitutions:     []SubstitutionInfo{},
		TeacherID:         teacher.ID,
		Specialization:    teacher.Specialization,
		Role:              teacher.Role,
		Qualifications:    teacher.Qualifications,
	}

	if len(subs) > 0 {
		result.Substitutions = buildSubstitutionInfoList(subs)
		if subs[0].Group != nil {
			result.CurrentGroup = subs[0].Group
		}
	}

	// Find regular group for this teacher
	if rs.EducationService != nil {
		groups, err := rs.EducationService.GetTeacherGroups(ctx, teacher.ID)
		if err == nil && len(groups) > 0 {
			result.RegularGroup = groups[0]
		}
	}

	return result
}

// matchesSearchTerm checks if staff member matches the search filter
func matchesSearchTerm(person *users.Person, searchTerm string) bool {
	if searchTerm == "" {
		return true
	}
	return strutil.ContainsFold(person.FirstName, searchTerm) ||
		strutil.ContainsFold(person.LastName, searchTerm)
}

// getAvailableForSubstitution handles getting staff available for substitution with their current status
// Optimized to avoid N+1 queries by using ListAllWithStaffAndPerson
func (rs *Resource) getAvailableForSubstitution(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	searchTerm := r.URL.Query().Get("search")
	ctx := r.Context()

	date := timezone.TodayDate()
	if dateStr != "" {
		if parsedDate, err := timezone.ParseDate(dateStr); err == nil {
			date = parsedDate
		}
	}

	caregivers, err := rs.listActiveCaregivers(ctx)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Keep the richer teacher payload, but only for canonical caregivers.
	teachers, err := rs.PersonService.ListTeachersWithStaffAndPerson(ctx)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	activeCaregiverStaffIDs := caregiverStaffIDSet(caregivers)
	substitutingStaffMap := rs.buildSubstitutionMap(ctx, date)
	results := rs.filterAndBuildTeacherResults(ctx, teachers, activeCaregiverStaffIDs, substitutingStaffMap, searchTerm)

	common.Respond(w, r, http.StatusOK, results, "Available staff for substitution retrieved successfully")
}

// buildSubstitutionMap creates a map of staff IDs to their active substitutions
func (rs *Resource) buildSubstitutionMap(ctx context.Context, date timezone.Date) map[int64][]*education.GroupSubstitution {
	result := make(map[int64][]*education.GroupSubstitution)
	if rs.EducationService == nil {
		return result
	}

	activeSubstitutions, _ := rs.EducationService.GetActiveSubstitutions(ctx, date)
	for _, sub := range activeSubstitutions {
		result[sub.SubstituteStaffID] = append(result[sub.SubstituteStaffID], sub)
	}
	return result
}

// filterAndBuildTeacherResults filters teachers and builds response entries
// Optimized version that uses pre-loaded Teacher/Staff/Person data
func (rs *Resource) filterAndBuildTeacherResults(
	ctx context.Context,
	teachers []*users.Teacher,
	activeCaregiverStaffIDs map[int64]struct{},
	subsMap map[int64][]*education.GroupSubstitution,
	searchTerm string,
) []StaffWithSubstitutionStatus {
	results := make([]StaffWithSubstitutionStatus, 0, len(teachers))

	for _, teacher := range teachers {
		result := rs.processTeacherForSubstitution(ctx, teacher, activeCaregiverStaffIDs, subsMap, searchTerm)
		if result != nil {
			results = append(results, *result)
		}
	}
	return results
}

// processTeacherForSubstitution processes a teacher with pre-loaded data for the substitution list
// Uses pre-loaded Staff and Person data to avoid N+1 queries
func (rs *Resource) processTeacherForSubstitution(
	ctx context.Context,
	teacher *users.Teacher,
	activeCaregiverStaffIDs map[int64]struct{},
	subsMap map[int64][]*education.GroupSubstitution,
	searchTerm string,
) *StaffWithSubstitutionStatus {
	// Skip if staff or person data is missing
	if teacher.Staff == nil || teacher.Staff.Person == nil {
		return nil
	}
	if _, ok := activeCaregiverStaffIDs[teacher.Staff.ID]; !ok {
		return nil
	}

	// Apply search filter using pre-loaded person data
	if !matchesSearchTerm(teacher.Staff.Person, searchTerm) {
		return nil
	}

	subs := subsMap[teacher.Staff.ID]
	result := rs.buildStaffSubstitutionStatus(ctx, teacher.Staff, teacher, subs)
	return &result
}

// getStaffByRole handles GET /api/staff/by-role?role=user or ?roles=teacher,staff,user
// Returns staff members filtered by account role (useful for group transfer dropdowns)
func (rs *Resource) getStaffByRole(w http.ResponseWriter, r *http.Request) {
	roles := parseRolesQuery(r)
	if len(roles) == 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("role or roles parameter is required")))
		return
	}

	ctx := r.Context()

	if requestedCaregiverPool(roles) {
		caregivers, err := rs.listActiveCaregivers(ctx)
		if err != nil {
			common.RenderError(w, r, common.ErrorInternalServer(err))
			return
		}
		common.Respond(w, r, http.StatusOK, caregiversToRoleResponses(caregivers), "Active caregivers retrieved successfully")
		return
	}

	staffByRoles, err := rs.PersonService.ListStaffByRoles(ctx, roles)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, staffByRolesToRoleResponses(staffByRoles), "Staff members with role retrieved successfully")
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

// caregiversToRoleResponses maps active caregivers to the role-response wire shape.
func caregiversToRoleResponses(caregivers []*users.ActiveCaregiver) []StaffWithRoleResponse {
	results := make([]StaffWithRoleResponse, 0, len(caregivers))
	for _, caregiver := range caregivers {
		results = append(results, StaffWithRoleResponse{
			ID:                caregiver.StaffID,
			PersonID:          caregiver.PersonID,
			TeacherID:         caregiver.TeacherID,
			FirstName:         caregiver.FirstName,
			LastName:          caregiver.LastName,
			FullName:          caregiver.FullName(),
			AccountID:         caregiver.AccountID,
			Email:             caregiver.Email,
			IsActiveCaregiver: true,
			CreatedAt:         caregiver.CreatedAt,
			UpdatedAt:         caregiver.UpdatedAt,
		})
	}
	return results
}

// staffByRolesToRoleResponses maps staff-by-role rows to the wire shape,
// deduplicating by staff ID (a staff member matching multiple roles appears once).
func staffByRolesToRoleResponses(staffByRoles []*users.StaffWithRoleInfo) []StaffWithRoleResponse {
	seen := make(map[int64]bool)
	var results []StaffWithRoleResponse
	for _, s := range staffByRoles {
		if seen[s.StaffID] {
			continue
		}
		seen[s.StaffID] = true
		results = append(results, StaffWithRoleResponse{
			ID:        s.StaffID,
			PersonID:  s.PersonID,
			FirstName: s.FirstName,
			LastName:  s.LastName,
			FullName:  s.FirstName + " " + s.LastName,
			AccountID: s.AccountID,
			Email:     s.Email,
			CreatedAt: s.CreatedAt,
			UpdatedAt: s.UpdatedAt,
		})
	}
	return results
}

// accountHasRole checks if an account has a specific role
func (rs *Resource) accountHasRole(ctx context.Context, accountID int64, roleName string) bool {
	roles, err := rs.AuthService.GetAccountRoles(ctx, int(accountID))
	if err != nil {
		return false
	}

	for _, role := range roles {
		if role.Name == roleName {
			return true
		}
	}
	return false
}
