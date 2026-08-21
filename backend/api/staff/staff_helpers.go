package staff

import (
	"context"
	"net/http"

	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/models/users"
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
	if firstName != "" && !strutil.ContainsFold(person.FirstName, firstName) {
		return false
	}
	if lastName != "" && !strutil.ContainsFold(person.LastName, lastName) {
		return false
	}
	return true
}

// staffResponseBuilder builds the appropriate response for a staff member
type staffResponseBuilder struct {
	staff           *users.Staff
	teacher         *users.Teacher
	isTeacher       bool
	wasPresentToday bool
	workStatus      string
	absenceType     string
	// absenceTypeLabel is the school's own wording, set after construction so
	// the response constructors keep their signature (#2403).
	absenceTypeLabel string
	accountRole      string
	email            string
	avatar           string
}

// buildResponse returns the appropriate response type based on teacher status
func (b *staffResponseBuilder) buildResponse() interface{} {
	if b.isTeacher && b.teacher != nil {
		response := newTeacherResponse(b.staff, b.teacher, b.wasPresentToday, b.workStatus, b.absenceType, b.accountRole, b.email, b.avatar)
		response.AbsenceTypeLabel = b.absenceTypeLabel
		return response
	}
	response := newStaffResponse(b.staff, false, b.wasPresentToday, b.workStatus, b.absenceType, b.accountRole, b.email, b.avatar)
	response.AbsenceTypeLabel = b.absenceTypeLabel
	return response
}

// processStaffForListOptimized processes a single staff member using pre-loaded data
// This avoids N+1 queries by using batch-loaded Person (via ListAllWithPerson) and Teacher data
// Returns the response object and true if staff should be included, nil and false otherwise
func (rs *Resource) processStaffForListOptimized(
	ctx context.Context,
	staff *users.Staff,
	teacherMap map[int64]*users.Teacher,
	presentMap map[int64]bool,
	workStatusMap map[int64]string,
	absenceMap map[int64]string,
	absenceLabelMap map[int64]string,
	accountRoleMap map[int64]string,
	accountEmailMap map[int64]string,
	accountAvatarMap map[int64]string,
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

	if filters.teachersOnly {
		if !isTeacher {
			return nil, false
		}
	}

	// Look up presence from pre-loaded map (O(1) lookup)
	wasPresentToday := presentMap[staff.ID]

	// Look up account role, email, and avatar from pre-loaded maps (O(1) lookup)
	var accountRole string
	var email string
	var avatar string
	if staff.Person != nil && staff.Person.AccountID != nil {
		accountRole = accountRoleMap[*staff.Person.AccountID]
		email = accountEmailMap[*staff.Person.AccountID]
		avatar = accountAvatarMap[*staff.Person.AccountID]
	}

	builder := &staffResponseBuilder{
		staff:            staff,
		teacher:          teacher,
		isTeacher:        isTeacher,
		wasPresentToday:  wasPresentToday,
		workStatus:       workStatusMap[staff.ID],
		absenceType:      absenceMap[staff.ID],
		absenceTypeLabel: absenceLabelMap[staff.ID],
		accountRole:      accountRole,
		email:            email,
		avatar:           avatar,
	}

	return builder.buildResponse(), true
}

// =============================================================================
// UPDATE STAFF HELPERS - Reduce complexity of updateStaff handler (S3776)
// =============================================================================
