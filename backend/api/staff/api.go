package staff

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// Resource defines the staff API resource
type Resource struct {
	PersonService    usersSvc.PersonService
	StaffRepo        users.StaffRepository
	TeacherRepo      users.TeacherRepository
	EducationService educationSvc.Service
	AuthService      authSvc.AuthService
}

// NewResource creates a new staff resource
func NewResource(
	personService usersSvc.PersonService,
	educationService educationSvc.Service,
	authService authSvc.AuthService,
) *Resource {
	return &Resource{
		PersonService:    personService,
		StaffRepo:        personService.StaffRepository(),
		TeacherRepo:      personService.TeacherRepository(),
		EducationService: educationService,
		AuthService:      authService,
	}
}

// Router returns a configured router for staff endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Read operations only require users:read permission
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/", rs.listStaff)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}", rs.getStaff)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}/groups", rs.getStaffGroups)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}/substitutions", rs.getStaffSubstitutions)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/available", rs.getAvailableStaff)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/available-for-substitution", rs.getAvailableForSubstitution)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/by-role", rs.getStaffByRole)

		// Write operations require users:create, users:update, or users:delete permission
		r.With(authorize.RequiresPermission(permissions.UsersCreate)).Post("/", rs.createStaff)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate)).Put("/{id}", rs.updateStaff)
		r.With(authorize.RequiresPermission(permissions.UsersDelete)).Delete("/{id}", rs.deleteStaff)

		// PIN management endpoints - staff can manage their own PIN
		r.Get("/pin", rs.getPINStatus)
		r.Put("/pin", rs.updatePIN)
	})

	return r
}

// PersonResponse represents a simplified person response
type PersonResponse struct {
	ID        int64     `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email,omitempty"`
	TagID     string    `json:"tag_id,omitempty"`
	AccountID *int64    `json:"account_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StaffResponse represents a staff response
type StaffResponse struct {
	ID         int64           `json:"id"`
	PersonID   int64           `json:"person_id"`
	StaffNotes string          `json:"staff_notes,omitempty"`
	Person     *PersonResponse `json:"person,omitempty"`
	IsTeacher  bool            `json:"is_teacher"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// TeacherResponse represents a teacher response (extends staff)
type TeacherResponse struct {
	StaffResponse
	TeacherID      int64  `json:"teacher_id"`
	Specialization string `json:"specialization,omitempty"`
	Role           string `json:"role,omitempty"`
	Qualifications string `json:"qualifications,omitempty"`
}

// GroupResponse represents a simplified group response
type GroupResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// StaffRequest represents a staff creation/update request
type StaffRequest struct {
	PersonID   int64  `json:"person_id"`
	StaffNotes string `json:"staff_notes,omitempty"`
	// Teacher-specific fields for creating a teacher
	IsTeacher      bool   `json:"is_teacher,omitempty"`
	Specialization string `json:"specialization,omitempty"`
	Role           string `json:"role,omitempty"`
	Qualifications string `json:"qualifications,omitempty"`
}

// PINStatusResponse represents the PIN status response
type PINStatusResponse struct {
	HasPIN      bool       `json:"has_pin"`
	LastChanged *time.Time `json:"last_changed,omitempty"`
}

// PINUpdateRequest represents a PIN update request
type PINUpdateRequest struct {
	CurrentPIN *string `json:"current_pin,omitempty"` // null for first-time setup
	NewPIN     string  `json:"new_pin"`
}

// Bind validates the staff request
func (req *StaffRequest) Bind(r *http.Request) error {
	if req.PersonID <= 0 {
		return errors.New("person ID is required")
	}

	req.Specialization = strings.TrimSpace(req.Specialization)
	req.Role = strings.TrimSpace(req.Role)
	req.Qualifications = strings.TrimSpace(req.Qualifications)

	return nil
}

// Bind validates the PIN update request
func (req *PINUpdateRequest) Bind(r *http.Request) error {
	if req.NewPIN == "" {
		return errors.New("new PIN is required")
	}

	// Validate PIN format (4 digits)
	if len(req.NewPIN) != 4 {
		return errors.New("PIN must be exactly 4 digits")
	}

	// Check if PIN contains only digits
	for _, char := range req.NewPIN {
		if char < '0' || char > '9' {
			return errors.New("PIN must contain only digits")
		}
	}

	return nil
}

// newPersonResponse creates a simplified person response
func newPersonResponse(person *users.Person) *PersonResponse {
	if person == nil {
		return nil
	}

	response := &PersonResponse{
		ID:        person.ID,
		FirstName: person.FirstName,
		LastName:  person.LastName,
		AccountID: person.AccountID,
		CreatedAt: person.CreatedAt,
		UpdatedAt: person.UpdatedAt,
	}

	if person.TagID != nil {
		response.TagID = *person.TagID
	}

	if person.Account != nil {
		response.Email = person.Account.Email
	}

	return response
}

// =============================================================================
// HELPER METHODS - Reduce code duplication for common parsing/validation
// =============================================================================

// parseAndGetStaff parses staff ID from URL and returns the staff if it exists.
// Returns nil and false if parsing fails or staff doesn't exist (error already rendered).
func (rs *Resource) parseAndGetStaff(w http.ResponseWriter, r *http.Request) (*users.Staff, bool) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidStaffID)))
		return nil, false
	}

	staff, err := rs.StaffRepo.FindByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New(common.MsgStaffNotFound)))
		return nil, false
	}

	return staff, true
}

// =============================================================================
// RESPONSE HELPERS
// =============================================================================

// newStaffResponse creates a staff response
func newStaffResponse(staff *users.Staff, isTeacher bool) StaffResponse {
	response := StaffResponse{
		ID:         staff.ID,
		PersonID:   staff.PersonID,
		StaffNotes: staff.StaffNotes,
		IsTeacher:  isTeacher,
		CreatedAt:  staff.CreatedAt,
		UpdatedAt:  staff.UpdatedAt,
	}

	if staff.Person != nil {
		response.Person = newPersonResponse(staff.Person)
	}

	return response
}

// newTeacherResponse creates a teacher response
func newTeacherResponse(staff *users.Staff, teacher *users.Teacher) TeacherResponse {
	staffResponse := newStaffResponse(staff, true)

	response := TeacherResponse{
		StaffResponse:  staffResponse,
		TeacherID:      teacher.ID,
		Specialization: strings.TrimSpace(teacher.Specialization),
		Role:           teacher.Role,
		Qualifications: teacher.Qualifications,
	}

	return response
}

// listStaff handles listing all staff members with optional filtering
func (rs *Resource) listStaff(w http.ResponseWriter, r *http.Request) {
	// Get query parameters for filtering
	firstName := r.URL.Query().Get("first_name")
	lastName := r.URL.Query().Get("last_name")
	teachersOnly := r.URL.Query().Get("teachers_only") == "true"
	filterByRole := r.URL.Query().Get("role") // Optional role filter (e.g., "user")

	// Create filter options
	filters := make(map[string]interface{})

	// Get all staff members
	staffMembers, err := rs.StaffRepo.List(r.Context(), filters)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response objects
	responses := make([]interface{}, 0, len(staffMembers))

	for _, staff := range staffMembers {
		// Get associated person
		person, err := rs.PersonService.Get(r.Context(), staff.PersonID)
		if err != nil {
			// Skip this staff member if person not found
			continue
		}

		// Apply role filter if specified (e.g., ?role=user)
		if filterByRole != "" && person.AccountID != nil {
			account, err := rs.AuthService.GetAccountByID(r.Context(), int(*person.AccountID))
			if err != nil {
				// Skip if account not found
				continue
			}

			// Check if account has the specified role
			hasRole := false
			roles, err := rs.AuthService.GetAccountRoles(r.Context(), int(account.ID))
			if err == nil {
				for _, role := range roles {
					if role.Name == filterByRole {
						hasRole = true
						break
					}
				}
			}

			// Skip if account doesn't have the specified role
			if !hasRole {
				continue
			}
		} else if filterByRole != "" {
			// Skip if filtering by role but person has no account
			continue
		}

		// Apply name filters if provided
		if (firstName != "" && !containsIgnoreCase(person.FirstName, firstName)) ||
			(lastName != "" && !containsIgnoreCase(person.LastName, lastName)) {
			continue
		}

		// Set person data
		staff.Person = person

		// Check if this staff member is also a teacher
		isTeacher := false
		var teacher *users.Teacher

		teacher, err = rs.TeacherRepo.FindByStaffID(r.Context(), staff.ID)
		if err == nil && teacher != nil {
			isTeacher = true
		}

		// Skip non-teachers if teachersOnly filter is applied
		if teachersOnly && !isTeacher {
			continue
		}

		// Create appropriate response based on teacher status
		if isTeacher {
			responses = append(responses, newTeacherResponse(staff, teacher))
		} else {
			responses = append(responses, newStaffResponse(staff, false))
		}
	}

	common.Respond(w, r, http.StatusOK, responses, "Staff members retrieved successfully")
}

// getStaff handles getting a staff member by ID
func (rs *Resource) getStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}

	// Get staff member with person data using FindWithPerson method
	staff, err := rs.StaffRepo.FindWithPerson(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New(common.MsgStaffNotFound)))
		return
	}

	// If person data was not loaded by FindWithPerson, try to fetch it separately
	if staff.Person == nil && staff.PersonID > 0 {
		person, err := rs.PersonService.Get(r.Context(), staff.PersonID)
		if err != nil {
			log.Printf("Warning: failed to get person data for staff member %d: %v", id, err)
			// Don't fail the request, just log the warning
		} else {
			staff.Person = person
		}
	}

	// Check if this staff member is also a teacher
	isTeacher := false
	var teacher *users.Teacher

	teacher, err = rs.TeacherRepo.FindByStaffID(r.Context(), staff.ID)
	if err == nil && teacher != nil {
		// Create teacher response
		response := newTeacherResponse(staff, teacher)
		common.Respond(w, r, http.StatusOK, response, "Teacher retrieved successfully")
		return
	}

	// Create staff response
	response := newStaffResponse(staff, isTeacher)
	common.Respond(w, r, http.StatusOK, response, "Staff member retrieved successfully")
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
			log.Printf("Failed to grant groups:read permission to %s account %d: %v", role, accountID, err)
		}
	}
}

// createStaff handles creating a new staff member
func (rs *Resource) createStaff(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &StaffRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Verify person exists
	person, err := rs.PersonService.Get(r.Context(), req.PersonID)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New("person not found")))
		return
	}

	// Create staff
	staff := &users.Staff{
		PersonID:   req.PersonID,
		StaffNotes: req.StaffNotes,
	}

	// Create staff record
	if err := rs.StaffRepo.Create(r.Context(), staff); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Set person data for response
	staff.Person = person

	// If request indicates this is a teacher, create teacher record as well
	isTeacher := req.IsTeacher
	var teacher *users.Teacher

	if isTeacher {
		teacher = &users.Teacher{
			StaffID:        staff.ID,
			Specialization: strings.TrimSpace(req.Specialization),
			Role:           req.Role,
			Qualifications: req.Qualifications,
		}

		if err := rs.TeacherRepo.Create(r.Context(), teacher); err != nil {
			// Still return staff member even if teacher creation fails
			isTeacher = false
			response := newStaffResponse(staff, isTeacher)
			common.Respond(w, r, http.StatusCreated, response, "Staff member created successfully, but failed to create teacher record")
			return
		}

		// Grant groups:read permission to teacher if they have an account
		if person.AccountID != nil {
			rs.grantDefaultPermissions(r.Context(), *person.AccountID, "teacher")
		}

		// Return teacher response
		response := newTeacherResponse(staff, teacher)
		common.Respond(w, r, http.StatusCreated, response, "Teacher created successfully")
		return
	}

	// Grant groups:read permission to staff if they have an account
	if person.AccountID != nil {
		rs.grantDefaultPermissions(r.Context(), *person.AccountID, "staff")
	}

	// Return staff response
	response := newStaffResponse(staff, isTeacher)
	common.Respond(w, r, http.StatusCreated, response, "Staff member created successfully")
}

// updateStaff handles updating a staff member
func (rs *Resource) updateStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}

	// Parse request
	req := &StaffRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get existing staff member
	staff, err := rs.StaffRepo.FindByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New(common.MsgStaffNotFound)))
		return
	}

	// Update fields
	staff.StaffNotes = req.StaffNotes

	// If person ID is changing, verify new person exists
	if staff.PersonID != req.PersonID {
		person, err := rs.PersonService.Get(r.Context(), req.PersonID)
		if err != nil {
			common.RenderError(w, r, ErrorNotFound(errors.New("person not found")))
			return
		}
		staff.PersonID = req.PersonID
		staff.Person = person
	}

	// Update staff record
	if err := rs.StaffRepo.Update(r.Context(), staff); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Reload staff with person data to ensure we have the latest information
	staff, err = rs.StaffRepo.FindWithPerson(r.Context(), id)
	if err != nil {
		// If we can't reload, try to at least get the person data
		if staff.Person == nil && staff.PersonID > 0 {
			person, _ := rs.PersonService.Get(r.Context(), staff.PersonID)
			staff.Person = person
		}
	}

	// Check if this staff member is also a teacher
	isTeacher := false
	var teacher *users.Teacher

	teacher, _ = rs.TeacherRepo.FindByStaffID(r.Context(), staff.ID)

	// Handle teacher record modifications
	if req.IsTeacher {
		if teacher != nil {
			// Update existing teacher record
			teacher.Specialization = strings.TrimSpace(req.Specialization)
			teacher.Role = req.Role
			teacher.Qualifications = req.Qualifications

			if err := rs.TeacherRepo.Update(r.Context(), teacher); err != nil {
				// Still return updated staff member even if teacher update fails
				response := newStaffResponse(staff, false)
				common.Respond(w, r, http.StatusOK, response, "Staff member updated successfully, but failed to update teacher record")
				return
			}
		} else {
			// Create new teacher record
			teacher = &users.Teacher{
				StaffID:        staff.ID,
				Specialization: strings.TrimSpace(req.Specialization),
				Role:           req.Role,
				Qualifications: req.Qualifications,
			}

			if err := rs.TeacherRepo.Create(r.Context(), teacher); err != nil {
				// Still return updated staff member even if teacher creation fails
				response := newStaffResponse(staff, false)
				common.Respond(w, r, http.StatusOK, response, "Staff member updated successfully, but failed to create teacher record")
				return
			}
		}

		// Return teacher response
		response := newTeacherResponse(staff, teacher)
		common.Respond(w, r, http.StatusOK, response, "Teacher updated successfully")
		return
	} else if teacher != nil {
		// User no longer wants this to be a teacher - we should keep the teacher record
		// but note that it's no longer considered active
		// In a real implementation, you might want to delete the teacher record or mark it as inactive

		// Return teacher response
		response := newTeacherResponse(staff, teacher)
		common.Respond(w, r, http.StatusOK, response, "Teacher updated successfully")
		return
	}

	// Return staff response
	response := newStaffResponse(staff, isTeacher)
	common.Respond(w, r, http.StatusOK, response, "Staff member updated successfully")
}

// deleteStaff handles deleting a staff member
func (rs *Resource) deleteStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}

	// Check if this staff member is also a teacher
	teacher, err := rs.TeacherRepo.FindByStaffID(r.Context(), id)
	if err == nil && teacher != nil {
		// Delete teacher record first
		if rs.TeacherRepo.Delete(r.Context(), teacher.ID) != nil {
			common.RenderError(w, r, ErrorInternalServer(errors.New("failed to delete teacher record")))
			return
		}
	}

	// Delete staff member
	if err := rs.StaffRepo.Delete(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Staff member deleted successfully")
}

// getStaffGroups handles getting groups for a staff member
func (rs *Resource) getStaffGroups(w http.ResponseWriter, r *http.Request) {
	// Parse and get staff
	staff, ok := rs.parseAndGetStaff(w, r)
	if !ok {
		return
	}

	// Check if this staff member is a teacher
	teacher, err := rs.TeacherRepo.FindByStaffID(r.Context(), staff.ID)
	if err != nil || teacher == nil {
		// If not a teacher, return empty groups list
		common.Respond(w, r, http.StatusOK, []GroupResponse{}, "Staff member is not a teacher and has no assigned groups")
		return
	}

	// Check if we have a reference to the Education service
	if rs.EducationService == nil {
		// If not, return an error
		common.RenderError(w, r, ErrorInternalServer(errors.New("education service not available")))
		return
	}

	// Get groups for this teacher
	groups, err := rs.EducationService.GetTeacherGroups(r.Context(), teacher.ID)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
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
func (rs *Resource) getAvailableStaff(w http.ResponseWriter, r *http.Request) {
	// Get all staff members
	staffMembers, err := rs.StaffRepo.List(r.Context(), nil)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response objects - only include staff who are teachers
	responses := make([]TeacherResponse, 0)

	for _, staff := range staffMembers {
		// Check if this staff member is a teacher
		teacher, err := rs.TeacherRepo.FindByStaffID(r.Context(), staff.ID)
		if err != nil || teacher == nil {
			continue
		}

		// Get associated person
		person, err := rs.PersonService.Get(r.Context(), staff.PersonID)
		if err != nil {
			// Skip this staff member if person not found
			continue
		}

		// Set person data
		staff.Person = person

		// Create teacher response
		responses = append(responses, newTeacherResponse(staff, teacher))
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
		common.RenderError(w, r, ErrorInternalServer(errors.New("education service not available")))
		return
	}

	// Get substitutions where this staff member is the substitute
	substitutions, err := rs.EducationService.GetStaffSubstitutions(r.Context(), staff.ID, false)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, substitutions, "Staff substitutions retrieved successfully")
}

// SubstitutionInfo represents a single substitution with transfer indicator
type SubstitutionInfo struct {
	ID         int64            `json:"id"`
	GroupID    int64            `json:"group_id"`
	GroupName  string           `json:"group_name,omitempty"`
	IsTransfer bool             `json:"is_transfer"`
	StartDate  string           `json:"start_date"`
	EndDate    string           `json:"end_date"`
	Group      *education.Group `json:"group,omitempty"`
}

// StaffWithSubstitutionStatus represents a staff member with their substitution status
type StaffWithSubstitutionStatus struct {
	*StaffResponse
	IsSubstituting    bool               `json:"is_substituting"`
	SubstitutionCount int                `json:"substitution_count"`
	Substitutions     []SubstitutionInfo `json:"substitutions,omitempty"`
	CurrentGroup      *education.Group   `json:"current_group,omitempty"`
	RegularGroup      *education.Group   `json:"regular_group,omitempty"`
	TeacherID         int64              `json:"teacher_id,omitempty"`
	Specialization    string             `json:"specialization,omitempty"`
	Role              string             `json:"role,omitempty"`
	Qualifications    string             `json:"qualifications,omitempty"`
}

// buildSubstitutionInfoList creates substitution info from active substitutions
func buildSubstitutionInfoList(subs []*education.GroupSubstitution) []SubstitutionInfo {
	result := make([]SubstitutionInfo, 0, len(subs))
	for _, sub := range subs {
		info := SubstitutionInfo{
			ID:         sub.ID,
			GroupID:    sub.GroupID,
			IsTransfer: sub.Duration() == 1,
			StartDate:  sub.StartDate.Format(common.DateFormatISO),
			EndDate:    sub.EndDate.Format(common.DateFormatISO),
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
	staffResp := newStaffResponse(staff, false)
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
	return containsIgnoreCase(person.FirstName, searchTerm) ||
		containsIgnoreCase(person.LastName, searchTerm)
}

// getAvailableForSubstitution handles getting staff available for substitution with their current status
func (rs *Resource) getAvailableForSubstitution(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	searchTerm := r.URL.Query().Get("search")

	date := time.Now()
	if dateStr != "" {
		if parsedDate, err := time.Parse(common.DateFormatISO, dateStr); err == nil {
			date = parsedDate
		}
	}

	staff, err := rs.StaffRepo.List(r.Context(), nil)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	substitutingStaffMap := rs.buildSubstitutionMap(r.Context(), date)
	results := rs.filterAndBuildStaffResults(r.Context(), staff, substitutingStaffMap, searchTerm)

	common.Respond(w, r, http.StatusOK, results, "Available staff for substitution retrieved successfully")
}

// buildSubstitutionMap creates a map of staff IDs to their active substitutions
func (rs *Resource) buildSubstitutionMap(ctx context.Context, date time.Time) map[int64][]*education.GroupSubstitution {
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

// filterAndBuildStaffResults filters staff and builds response entries
func (rs *Resource) filterAndBuildStaffResults(
	ctx context.Context,
	staff []*users.Staff,
	subsMap map[int64][]*education.GroupSubstitution,
	searchTerm string,
) []StaffWithSubstitutionStatus {
	var results []StaffWithSubstitutionStatus

	for _, s := range staff {
		result := rs.processStaffForSubstitution(ctx, s, subsMap, searchTerm)
		if result != nil {
			results = append(results, *result)
		}
	}
	return results
}

// processStaffForSubstitution processes a single staff member for the substitution list
func (rs *Resource) processStaffForSubstitution(
	ctx context.Context,
	s *users.Staff,
	subsMap map[int64][]*education.GroupSubstitution,
	searchTerm string,
) *StaffWithSubstitutionStatus {
	teacher, err := rs.TeacherRepo.FindByStaffID(ctx, s.ID)
	if err != nil || teacher == nil {
		return nil
	}

	rs.ensurePersonLoaded(ctx, s)

	if s.Person != nil && !matchesSearchTerm(s.Person, searchTerm) {
		return nil
	}

	subs := subsMap[s.ID]
	result := rs.buildStaffSubstitutionStatus(ctx, s, teacher, subs)
	return &result
}

// ensurePersonLoaded loads person data if not already loaded
func (rs *Resource) ensurePersonLoaded(ctx context.Context, s *users.Staff) {
	if s.Person == nil && s.PersonID > 0 {
		if person, err := rs.PersonService.Get(ctx, s.PersonID); err == nil {
			s.Person = person
		}
	}
}

// Helper function to check if a string contains another string, ignoring case
func containsIgnoreCase(s, substr string) bool {
	s, substr = strings.ToLower(s), strings.ToLower(substr)
	return strings.Contains(s, substr)
}

// getPINStatus handles getting the current user's PIN status
func (rs *Resource) getPINStatus(w http.ResponseWriter, r *http.Request) {
	// Get user from JWT context
	userClaims := jwt.ClaimsFromCtx(r.Context())
	if userClaims.ID == 0 {
		common.RenderError(w, r, ErrorUnauthorized(errors.New("invalid token")))
		return
	}

	// Get account directly
	account, err := rs.AuthService.GetAccountByID(r.Context(), userClaims.ID)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New("account not found")))
		return
	}

	// Ensure the account belongs to a staff member (admins without person records are allowed)
	person, err := rs.PersonService.FindByAccountID(r.Context(), int64(account.ID))
	if err == nil && person != nil {
		if _, err := rs.StaffRepo.FindByPersonID(r.Context(), person.ID); err != nil {
			common.RenderError(w, r, ErrorForbidden(errors.New("only staff members can access PIN settings")))
			return
		}
	}

	// Build response using account PIN data
	response := PINStatusResponse{
		HasPIN: account.HasPIN(),
	}

	// Include last changed timestamp if available (use UpdatedAt as proxy)
	if account.HasPIN() {
		response.LastChanged = &account.UpdatedAt
	}

	common.Respond(w, r, http.StatusOK, response, "PIN status retrieved successfully")
}

// updatePIN handles updating the current user's PIN
func (rs *Resource) updatePIN(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &PINUpdateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get user from JWT context
	userClaims := jwt.ClaimsFromCtx(r.Context())
	if userClaims.ID == 0 {
		common.RenderError(w, r, ErrorUnauthorized(errors.New("invalid token")))
		return
	}

	// Get account directly
	account, err := rs.AuthService.GetAccountByID(r.Context(), userClaims.ID)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New("account not found")))
		return
	}

	// Ensure the account belongs to a staff member (admins without person records are allowed)
	person, err := rs.PersonService.FindByAccountID(r.Context(), int64(account.ID))
	if err == nil && person != nil {
		if _, err := rs.StaffRepo.FindByPersonID(r.Context(), person.ID); err != nil {
			common.RenderError(w, r, ErrorForbidden(errors.New("only staff members can manage PIN settings")))
			return
		}
	}

	// Check if account is locked
	if account.IsPINLocked() {
		common.RenderError(w, r, ErrorForbidden(errors.New("account is temporarily locked due to failed PIN attempts")))
		return
	}

	// If account has existing PIN, validate current PIN
	if account.HasPIN() {
		if req.CurrentPIN == nil || *req.CurrentPIN == "" {
			common.RenderError(w, r, ErrorInvalidRequest(errors.New("current PIN is required when updating existing PIN")))
			return
		}

		// Verify current PIN
		if !account.VerifyPIN(*req.CurrentPIN) {
			// Increment failed attempts
			account.IncrementPINAttempts()

			// Update account record with incremented attempts
			if updateErr := rs.AuthService.UpdateAccount(r.Context(), account); updateErr != nil {
				log.Printf("Failed to update account PIN attempts: %v", updateErr)
			}

			common.RenderError(w, r, ErrorUnauthorized(errors.New("current PIN is incorrect")))
			return
		}
	}

	// Hash and set the new PIN
	if account.HashPIN(req.NewPIN) != nil {
		common.RenderError(w, r, ErrorInternalServer(errors.New("failed to hash PIN")))
		return
	}

	// Reset PIN attempts on successful PIN change
	account.ResetPINAttempts()

	if err := rs.AuthService.UpdateAccount(r.Context(), account); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "PIN updated successfully",
	}, "PIN updated successfully")
}

// StaffWithRoleResponse represents a staff member with role information
type StaffWithRoleResponse struct {
	ID        int64     `json:"id"`
	PersonID  int64     `json:"person_id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	FullName  string    `json:"full_name"`
	AccountID int64     `json:"account_id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// getStaffByRole handles GET /api/staff/by-role?role=user
// Returns staff members filtered by account role (useful for group transfer dropdowns)
func (rs *Resource) getStaffByRole(w http.ResponseWriter, r *http.Request) {
	roleName := r.URL.Query().Get("role")
	if roleName == "" {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("role parameter is required")))
		return
	}

	staff, err := rs.StaffRepo.List(r.Context(), nil)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	results := rs.filterStaffByRole(r.Context(), staff, roleName)
	common.Respond(w, r, http.StatusOK, results, "Staff members with role retrieved successfully")
}

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
