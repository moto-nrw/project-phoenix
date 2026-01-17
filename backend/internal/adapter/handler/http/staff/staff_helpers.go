package staff

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	usersSvc "github.com/moto-nrw/project-phoenix/internal/core/service/users"
)

// =============================================================================
// STAFF LOOKUP HELPERS
// =============================================================================

// parseAndGetStaff parses staff ID from URL and returns the staff if it exists.
// Returns nil and false if parsing fails or staff doesn't exist (error already rendered).
func (rs *Resource) parseAndGetStaff(w http.ResponseWriter, r *http.Request) (*users.Staff, bool) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidStaffID)))
		return nil, false
	}

	staff, err := rs.PersonService.GetStaffByID(r.Context(), id)
	if err != nil {
		if isNotFoundErr(err) {
			common.RenderError(w, r, ErrorNotFound(errors.New(common.MsgStaffNotFound)))
		} else {
			common.RenderError(w, r, ErrorInternalServer(err))
		}
		return nil, false
	}

	return staff, true
}

func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, sql.ErrNoRows) ||
		errors.Is(err, usersSvc.ErrPersonNotFound) ||
		errors.Is(err, usersSvc.ErrStaffNotFound) ||
		errors.Is(err, usersSvc.ErrTeacherNotFound)
}

// grantDefaultPermissions grants default permissions to a newly created account
func (rs *Resource) grantDefaultPermissions(ctx context.Context, accountID int64, role string) {
	if rs.AuthService == nil {
		return
	}

	// Get the groups:read permission
	perm, err := rs.AuthService.GetPermissionByName(ctx, permissions.GroupsRead)
	if err == nil && perm != nil {
		// Grant the permission to the account
		if err := rs.AuthService.GrantPermissionToAccount(ctx, int(accountID), int(perm.ID)); err != nil {
			if logger.Logger != nil {
				logger.Logger.WithFields(map[string]interface{}{
					"role":       role,
					"account_id": accountID,
				}).WithError(err).Warn("failed to grant groups:read permission")
			}
		}
	}
}

// =============================================================================
// LIST STAFF HELPERS - Reduce complexity of listStaff handler (S3776)
// =============================================================================

// listStaffFilters holds parsed query parameters for listing staff
type listStaffFilters struct {
	firstName    string
	lastName     string
	teachersOnly bool
	filterByRole string
}

// parseListStaffFilters extracts filter parameters from the request
func parseListStaffFilters(r *http.Request) listStaffFilters {
	return listStaffFilters{
		firstName:    r.URL.Query().Get("first_name"),
		lastName:     r.URL.Query().Get("last_name"),
		teachersOnly: r.URL.Query().Get("teachers_only") == "true",
		filterByRole: r.URL.Query().Get("role"),
	}
}

// checkStaffRoleFilter checks if a staff member passes the role filter
// Returns true if the staff should be included, false if it should be skipped
func (rs *Resource) checkStaffRoleFilter(ctx context.Context, person *users.Person, filterByRole string) bool {
	if filterByRole == "" {
		return true
	}

	if person.AccountID == nil {
		return false
	}

	account, err := rs.AuthService.GetAccountByID(ctx, int(*person.AccountID))
	if err != nil {
		return false
	}

	return rs.accountHasRole(ctx, account.ID, filterByRole)
}

// matchesNameFilter checks if a person matches the name filters
func matchesNameFilter(person *users.Person, firstName, lastName string) bool {
	if firstName != "" && !containsIgnoreCase(person.FirstName, firstName) {
		return false
	}
	if lastName != "" && !containsIgnoreCase(person.LastName, lastName) {
		return false
	}
	return true
}

// staffResponseBuilder builds the appropriate response for a staff member
type staffResponseBuilder struct {
	staff     *users.Staff
	teacher   *users.Teacher
	isTeacher bool
}

// buildResponse returns the appropriate response type based on teacher status
func (b *staffResponseBuilder) buildResponse() interface{} {
	if b.isTeacher && b.teacher != nil {
		return newTeacherResponse(b.staff, b.teacher)
	}
	return newStaffResponse(b.staff, false)
}

// processStaffForListOptimized processes a single staff member using pre-loaded data
// This avoids N+1 queries by using batch-loaded Person (via ListAllWithPerson) and Teacher data
// Returns the response object and true if staff should be included, nil and false otherwise
func (rs *Resource) processStaffForListOptimized(
	ctx context.Context,
	staff *users.Staff,
	teacherMap map[int64]*users.Teacher,
	filters listStaffFilters,
) (interface{}, bool) {
	// Person is already loaded via ListAllWithPerson
	if staff.Person == nil {
		return nil, false
	}

	// Apply role filter (still requires DB call if role filter is set)
	if !rs.checkStaffRoleFilter(ctx, staff.Person, filters.filterByRole) {
		return nil, false
	}

	// Apply name filter using pre-loaded person data
	if !matchesNameFilter(staff.Person, filters.firstName, filters.lastName) {
		return nil, false
	}

	// Look up teacher from pre-loaded map (O(1) lookup instead of DB query)
	teacher, isTeacher := teacherMap[staff.ID]

	if filters.teachersOnly && !isTeacher {
		return nil, false
	}

	builder := &staffResponseBuilder{
		staff:     staff,
		teacher:   teacher,
		isTeacher: isTeacher,
	}

	return builder.buildResponse(), true
}

// =============================================================================
// UPDATE STAFF HELPERS - Reduce complexity of updateStaff handler (S3776)
// =============================================================================

// handleTeacherRecordUpdate handles creating or updating a teacher record during staff update
// Returns the response to send and whether to exit early
func (rs *Resource) handleTeacherRecordUpdate(
	ctx context.Context,
	staff *users.Staff,
	req *StaffRequest,
	existingTeacher *users.Teacher,
) (interface{}, string, bool) {
	if existingTeacher != nil {
		existingTeacher.Specialization = req.Specialization
		existingTeacher.Role = req.Role
		existingTeacher.Qualifications = req.Qualifications

		// Update teacher record
		if rs.PersonService.UpdateTeacher(ctx, existingTeacher) != nil {
			return newStaffResponse(staff, false), "Staff member updated successfully, but failed to update teacher record", true
		}

		return newTeacherResponse(staff, existingTeacher), "Teacher updated successfully", false
	}

	teacher := &users.Teacher{
		StaffID:        staff.ID,
		Specialization: req.Specialization,
		Role:           req.Role,
		Qualifications: req.Qualifications,
	}

	// Create teacher record
	if rs.PersonService.CreateTeacher(ctx, teacher) != nil {
		return newStaffResponse(staff, false), "Staff member updated successfully, but failed to create teacher record", true
	}

	return newTeacherResponse(staff, teacher), "Teacher updated successfully", false
}

// updateStaffPerson validates and updates the person ID for a staff member
func (rs *Resource) updateStaffPerson(ctx context.Context, staff *users.Staff, personID int64) error {
	person, err := rs.PersonService.Get(ctx, personID)
	if err != nil {
		return err
	}
	staff.PersonID = personID
	staff.Person = person
	return nil
}

// reloadStaffWithPerson attempts to reload staff with person data
func (rs *Resource) reloadStaffWithPerson(ctx context.Context, staff *users.Staff, id int64) {
	reloaded, err := rs.PersonService.GetStaffWithPerson(ctx, id)
	if err == nil {
		*staff = *reloaded
		return
	}
	// Fallback: load person separately
	if staff.Person == nil && staff.PersonID > 0 {
		if person, err := rs.PersonService.Get(ctx, staff.PersonID); err == nil {
			staff.Person = person
		}
	}
}

// buildUpdateStaffResponse builds the appropriate response for staff update
func (rs *Resource) buildUpdateStaffResponse(
	ctx context.Context,
	staff *users.Staff,
	req *StaffRequest,
	existingTeacher *users.Teacher,
) (interface{}, string) {
	// Handle teacher record creation/update
	if req.IsTeacher {
		response, message, _ := rs.handleTeacherRecordUpdate(ctx, staff, req, existingTeacher)
		return response, message
	}

	// Return existing teacher response if they have a teacher record
	if existingTeacher != nil {
		return newTeacherResponse(staff, existingTeacher), "Teacher updated successfully"
	}

	return newStaffResponse(staff, false), "Staff member updated successfully"
}

// =============================================================================
// STAFF ROLE FILTER HELPERS
// =============================================================================

// filterStaffByRole filters staff members that have the specified role
func (rs *Resource) filterStaffByRole(ctx context.Context, staff []*users.Staff, roleName string) []StaffWithRoleResponse {
	var results []StaffWithRoleResponse

	for _, s := range staff {
		entry := rs.buildStaffRoleEntry(ctx, s, roleName)
		if entry != nil {
			results = append(results, *entry)
		}
	}
	return results
}

// buildStaffRoleEntry creates a role response entry if staff has the requested role
func (rs *Resource) buildStaffRoleEntry(ctx context.Context, s *users.Staff, roleName string) *StaffWithRoleResponse {
	person, err := rs.PersonService.Get(ctx, s.PersonID)
	if err != nil || person == nil || person.AccountID == nil {
		return nil
	}

	account, err := rs.AuthService.GetAccountByID(ctx, int(*person.AccountID))
	if err != nil || account == nil {
		return nil
	}

	if !rs.accountHasRole(ctx, account.ID, roleName) {
		return nil
	}

	return &StaffWithRoleResponse{
		ID:        s.ID,
		PersonID:  person.ID,
		FirstName: person.FirstName,
		LastName:  person.LastName,
		FullName:  person.FirstName + " " + person.LastName,
		AccountID: *person.AccountID,
		Email:     account.Email,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
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
