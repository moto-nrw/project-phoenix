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
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidStaffID))); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return nil, false
	}

	staff, err := rs.StaffRepo.FindByID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New(common.MsgStaffNotFound))); err != nil {
			log.Printf(common.LogRenderError, err)
		}
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
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(common.LogRenderError, err)
		}
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
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidStaffID))); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Get staff member with person data using FindWithPerson method
	staff, err := rs.StaffRepo.FindWithPerson(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New(common.MsgStaffNotFound))); err != nil {
			log.Printf(common.LogRenderError, err)
		}
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
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Verify person exists
	person, err := rs.PersonService.Get(r.Context(), req.PersonID)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("person not found"))); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Create staff
	staff := &users.Staff{
		PersonID:   req.PersonID,
		StaffNotes: req.StaffNotes,
	}

	// Create staff record
	if err := rs.StaffRepo.Create(r.Context(), staff); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(common.LogRenderError, err)
		}
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
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidStaffID))); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Parse request
	req := &StaffRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Get existing staff member
	staff, err := rs.StaffRepo.FindByID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New(common.MsgStaffNotFound))); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Update fields
	staff.StaffNotes = req.StaffNotes

	// If person ID is changing, verify new person exists
	if staff.PersonID != req.PersonID {
		person, err := rs.PersonService.Get(r.Context(), req.PersonID)
		if err != nil {
			if err := render.Render(w, r, ErrorNotFound(errors.New("person not found"))); err != nil {
				log.Printf(common.LogRenderError, err)
			}
			return
		}
		staff.PersonID = req.PersonID
		staff.Person = person
	}

	// Update staff record
	if err := rs.StaffRepo.Update(r.Context(), staff); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(common.LogRenderError, err)
		}
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
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidStaffID))); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Check if this staff member is also a teacher
	teacher, err := rs.TeacherRepo.FindByStaffID(r.Context(), id)
	if err == nil && teacher != nil {
		// Delete teacher record first
		if err := rs.TeacherRepo.Delete(r.Context(), teacher.ID); err != nil {
			if err := render.Render(w, r, ErrorInternalServer(errors.New("failed to delete teacher record"))); err != nil {
				log.Printf(common.LogRenderError, err)
			}
			return
		}
	}

	// Delete staff member
	if err := rs.StaffRepo.Delete(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(common.LogRenderError, err)
		}
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
		if err := render.Render(w, r, ErrorInternalServer(errors.New("education service not available"))); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Get groups for this teacher
	groups, err := rs.EducationService.GetTeacherGroups(r.Context(), teacher.ID)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(common.LogRenderError, err)
		}
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
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(common.LogRenderError, err)
		}
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
		if err := render.Render(w, r, ErrorInternalServer(errors.New("education service not available"))); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Get substitutions where this staff member is the substitute
	substitutions, err := rs.EducationService.GetStaffSubstitutions(r.Context(), staff.ID, false)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, substitutions, "Staff substitutions retrieved successfully")
}

// getAvailableForSubstitution handles getting staff available for substitution with their current status
func (rs *Resource) getAvailableForSubstitution(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	dateStr := r.URL.Query().Get("date")
	searchTerm := r.URL.Query().Get("search")

	date := time.Now()
	if dateStr != "" {
		parsedDate, err := time.Parse(common.DateFormatISO, dateStr)
		if err == nil {
			date = parsedDate
		}
	}

	// Get all staff members
	staff, err := rs.StaffRepo.List(r.Context(), nil)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Get active substitutions for the date
	var activeSubstitutions []*education.GroupSubstitution
	if rs.EducationService != nil {
		activeSubstitutions, _ = rs.EducationService.GetActiveSubstitutions(r.Context(), date)
	}

	// Create a map of staff IDs to ALL their active substitutions (supports multiple)
	substitutingStaffMap := make(map[int64][]*education.GroupSubstitution)
	for _, sub := range activeSubstitutions {
		substitutingStaffMap[sub.SubstituteStaffID] = append(substitutingStaffMap[sub.SubstituteStaffID], sub)
	}

	// SubstitutionInfo represents a single substitution with transfer indicator
	type SubstitutionInfo struct {
		ID         int64            `json:"id"`
		GroupID    int64            `json:"group_id"`
		GroupName  string           `json:"group_name,omitempty"`
		IsTransfer bool             `json:"is_transfer"` // true if duration is 1 day (day transfer)
		StartDate  string           `json:"start_date"`
		EndDate    string           `json:"end_date"`
		Group      *education.Group `json:"group,omitempty"`
	}

	// Get groups for teachers to find their regular group
	type StaffWithSubstitutionStatus struct {
		*StaffResponse
		IsSubstituting   bool               `json:"is_substituting"`
		SubstitutionCount int               `json:"substitution_count"`
		Substitutions    []SubstitutionInfo `json:"substitutions,omitempty"`
		CurrentGroup     *education.Group   `json:"current_group,omitempty"`
		RegularGroup     *education.Group   `json:"regular_group,omitempty"`
		// Teacher-specific fields
		TeacherID      int64  `json:"teacher_id,omitempty"`
		Specialization string `json:"specialization,omitempty"`
		Role           string `json:"role,omitempty"`
		Qualifications string `json:"qualifications,omitempty"`
	}

	var results []StaffWithSubstitutionStatus

	for _, s := range staff {
		// Check if this staff member is a teacher first
		teacher, err := rs.TeacherRepo.FindByStaffID(r.Context(), s.ID)
		if err != nil || teacher == nil {
			// Skip non-teachers
			continue
		}

		// Load person data if not already loaded
		if s.Person == nil && s.PersonID > 0 {
			person, err := rs.PersonService.Get(r.Context(), s.PersonID)
			if err == nil {
				s.Person = person
			}
		}

		// Apply search filter if provided
		if searchTerm != "" && s.Person != nil {
			// Check if search term matches first name or last name
			if !containsIgnoreCase(s.Person.FirstName, searchTerm) &&
				!containsIgnoreCase(s.Person.LastName, searchTerm) {
				continue // Skip this staff member
			}
		}

		// Create staff response
		staffResp := newStaffResponse(s, false)
		result := StaffWithSubstitutionStatus{
			StaffResponse:     &staffResp,
			IsSubstituting:    false,
			SubstitutionCount: 0,
			Substitutions:     []SubstitutionInfo{},
		}

		// Check if this staff member has any substitutions (supports multiple)
		if subs, ok := substitutingStaffMap[s.ID]; ok && len(subs) > 0 {
			result.IsSubstituting = true
			result.SubstitutionCount = len(subs)

			// Build substitution info list
			for _, sub := range subs {
				subInfo := SubstitutionInfo{
					ID:         sub.ID,
					GroupID:    sub.GroupID,
					IsTransfer: sub.Duration() == 1, // Transfer if duration is 1 day (Tagesübergabe)
					StartDate:  sub.StartDate.Format(common.DateFormatISO),
					EndDate:    sub.EndDate.Format(common.DateFormatISO),
				}
				if sub.Group != nil {
					subInfo.GroupName = sub.Group.Name
					subInfo.Group = sub.Group
				}
				result.Substitutions = append(result.Substitutions, subInfo)
			}

			// Set CurrentGroup to first substitution's group (for backward compatibility)
			if subs[0].Group != nil {
				result.CurrentGroup = subs[0].Group
			}
		}

		// Populate teacher info (we already have the teacher record from above)
		result.TeacherID = teacher.ID
		result.Specialization = teacher.Specialization
		result.Role = teacher.Role
		result.Qualifications = teacher.Qualifications

		// Find regular group for this teacher
		if rs.EducationService != nil {
			// Get groups for this teacher
			groups, err := rs.EducationService.GetTeacherGroups(r.Context(), teacher.ID)
			if err == nil && len(groups) > 0 {
				// Assume first group is their regular group
				result.RegularGroup = groups[0]
			}
		}

		results = append(results, result)
	}

	common.Respond(w, r, http.StatusOK, results, "Available staff for substitution retrieved successfully")
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
		if err := render.Render(w, r, ErrorUnauthorized(errors.New("invalid token"))); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Get account directly
	account, err := rs.AuthService.GetAccountByID(r.Context(), userClaims.ID)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("account not found"))); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Ensure the account belongs to a staff member (admins without person records are allowed)
	person, err := rs.PersonService.FindByAccountID(r.Context(), int64(account.ID))
	if err == nil && person != nil {
		if _, err := rs.StaffRepo.FindByPersonID(r.Context(), person.ID); err != nil {
			if err := render.Render(w, r, ErrorForbidden(errors.New("only staff members can access PIN settings"))); err != nil {
				log.Printf(common.LogRenderError, err)
			}
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
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Get user from JWT context
	userClaims := jwt.ClaimsFromCtx(r.Context())
	if userClaims.ID == 0 {
		if err := render.Render(w, r, ErrorUnauthorized(errors.New("invalid token"))); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Get account directly
	account, err := rs.AuthService.GetAccountByID(r.Context(), userClaims.ID)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("account not found"))); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Ensure the account belongs to a staff member (admins without person records are allowed)
	person, err := rs.PersonService.FindByAccountID(r.Context(), int64(account.ID))
	if err == nil && person != nil {
		if _, err := rs.StaffRepo.FindByPersonID(r.Context(), person.ID); err != nil {
			if err := render.Render(w, r, ErrorForbidden(errors.New("only staff members can manage PIN settings"))); err != nil {
				log.Printf(common.LogRenderError, err)
			}
			return
		}
	}

	// Check if account is locked
	if account.IsPINLocked() {
		if err := render.Render(w, r, ErrorForbidden(errors.New("account is temporarily locked due to failed PIN attempts"))); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// If account has existing PIN, validate current PIN
	if account.HasPIN() {
		if req.CurrentPIN == nil || *req.CurrentPIN == "" {
			if err := render.Render(w, r, ErrorInvalidRequest(errors.New("current PIN is required when updating existing PIN"))); err != nil {
				log.Printf(common.LogRenderError, err)
			}
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

			if err := render.Render(w, r, ErrorUnauthorized(errors.New("current PIN is incorrect"))); err != nil {
				log.Printf(common.LogRenderError, err)
			}
			return
		}
	}

	// Hash and set the new PIN
	if err := account.HashPIN(req.NewPIN); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(errors.New("failed to hash PIN"))); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Reset PIN attempts on successful PIN change
	account.ResetPINAttempts()

	if err := rs.AuthService.UpdateAccount(r.Context(), account); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "PIN updated successfully",
	}, "PIN updated successfully")
}

// getStaffByRole handles GET /api/staff/by-role?role=user
// Returns staff members filtered by account role (useful for group transfer dropdowns)
func (rs *Resource) getStaffByRole(w http.ResponseWriter, r *http.Request) {
	// Get role from query parameter
	roleName := r.URL.Query().Get("role")
	if roleName == "" {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("role parameter is required"))); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Get all staff members
	staff, err := rs.StaffRepo.List(r.Context(), nil)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Filter staff by account role
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

	var results []StaffWithRoleResponse

	for _, s := range staff {
		// Load person data
		person, err := rs.PersonService.Get(r.Context(), s.PersonID)
		if err != nil || person == nil {
			continue
		}

		// Skip if person has no account
		if person.AccountID == nil {
			continue
		}

		// Get account
		account, err := rs.AuthService.GetAccountByID(r.Context(), int(*person.AccountID))
		if err != nil || account == nil {
			continue
		}

		// Get account roles
		roles, err := rs.AuthService.GetAccountRoles(r.Context(), int(account.ID))
		if err != nil {
			continue
		}

		// Check if account has the requested role
		hasRole := false
		for _, role := range roles {
			if role.Name == roleName {
				hasRole = true
				break
			}
		}

		if !hasRole {
			continue
		}

		// Build response
		fullName := person.FirstName + " " + person.LastName
		results = append(results, StaffWithRoleResponse{
			ID:        s.ID,
			PersonID:  person.ID,
			FirstName: person.FirstName,
			LastName:  person.LastName,
			FullName:  fullName,
			AccountID: *person.AccountID,
			Email:     account.Email,
			CreatedAt: s.CreatedAt,
			UpdatedAt: s.UpdatedAt,
		})
	}

	common.Respond(w, r, http.StatusOK, results, "Staff members with role retrieved successfully")
}
