package staff

import (
	"context"
	"net/http"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// staffFieldAccess splits the staff directory into the two tiers a caller can
// be entitled to beyond the minimal colleague view.
//
// Everyone with users:read may read the directory itself — the group,
// substitution and supervision screens need names. Before #2906 the same read
// also carried everything else, because the response was built
// unconditionally.
type staffFieldAccess struct {
	// notes covers the private staff notes an OGS-Leitung keeps on a
	// colleague. They are the most sensitive part of the staff record, so
	// they stay with the permission that writes them (staff:manage) instead
	// of following everybody who maintains the personnel file.
	notes bool
	// qualifications covers the free-text qualifications on the teacher
	// record. Those are personnel data proper, so maintaining the personnel
	// file (staff:stammdaten) reads them too.
	qualifications bool
	// personnel covers the personnel-file data: employment type, today's
	// absence reason including the school's own wording, and the NFC tag.
	// Being allowed to change a staff record (staff:manage) does not entitle
	// anybody to read those; maintaining the personnel file
	// (staff:stammdaten) and the time-management view (time_tracking:manage)
	// do.
	personnel bool
}

// staffFieldAccessFromCtx reads the caller's tiers off the JWT permissions.
// authorize.HasPermission is wildcard aware, so admin:* matches every tier.
func staffFieldAccessFromCtx(ctx context.Context) staffFieldAccess {
	return staffFieldAccessFor(jwt.PermissionsFromCtx(ctx))
}

// staffFieldAccessFor is staffFieldAccessFromCtx on a plain permission list,
// so the tier mapping can be tested without importing auth/jwt — which the
// architecture ratchet forbids in api/staff internal tests.
func staffFieldAccessFor(granted []string) staffFieldAccess {
	has := func(required string) bool { return authorize.HasPermission(required, granted) }

	return staffFieldAccess{
		notes:          has(permissions.StaffManage),
		qualifications: has(permissions.StaffManage) || has(permissions.StaffStammdaten),
		personnel:      has(permissions.StaffStammdaten) || has(permissions.TimeTrackingManage),
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

// buildResponse returns the appropriate response type based on teacher status.
// The absence-type label is applied after construction, so it repeats the
// personnel check the constructors make (#2906) — the school's own wording for
// today's absence names the reason just as AbsenceType does.
func (b *staffResponseBuilder) buildResponse(ctx context.Context) interface{} {
	label := b.absenceTypeLabel
	if !staffFieldAccessFromCtx(ctx).personnel {
		label = ""
	}
	if b.isTeacher && b.teacher != nil {
		response := newTeacherResponse(ctx, b.staff, b.teacher, b.wasPresentToday, b.workStatus, b.absenceType, b.accountRole, b.email, b.avatar)
		response.AbsenceTypeLabel = label
		return response
	}
	response := newStaffResponse(ctx, b.staff, false, b.wasPresentToday, b.workStatus, b.absenceType, b.accountRole, b.email, b.avatar)
	response.AbsenceTypeLabel = label
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

	return builder.buildResponse(ctx), true
}

// =============================================================================
// UPDATE STAFF HELPERS - Reduce complexity of updateStaff handler (S3776)
// =============================================================================
