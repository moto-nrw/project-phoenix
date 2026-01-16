package staff

import (
	"context"
	"net/http"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
)

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
