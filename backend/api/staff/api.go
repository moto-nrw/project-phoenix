package staff

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/users"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// Resource defines the staff API resource
type Resource struct {
	PersonService    usersSvc.PersonService
	StaffRepo        users.StaffRepository
	TeacherRepo      users.TeacherRepository
	EducationService educationSvc.Service
}

// NewResource creates a new staff resource
func NewResource(
	personService usersSvc.PersonService,
	educationService educationSvc.Service,
) *Resource {
	return &Resource{
		PersonService:    personService,
		StaffRepo:        personService.StaffRepository(),
		TeacherRepo:      personService.TeacherRepository(),
		EducationService: educationService,
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
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/available", rs.getAvailableStaff)

		// Write operations require users:create, users:update, or users:delete permission
		r.With(authorize.RequiresPermission(permissions.UsersCreate)).Post("/", rs.createStaff)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate)).Put("/{id}", rs.updateStaff)
		r.With(authorize.RequiresPermission(permissions.UsersDelete)).Delete("/{id}", rs.deleteStaff)
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
	Specialization string `json:"specialization"`
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

// Bind validates the staff request
func (req *StaffRequest) Bind(r *http.Request) error {
	if req.PersonID <= 0 {
		return errors.New("person ID is required")
	}

	// If creating a teacher, specialization is required
	if req.IsTeacher && req.Specialization == "" {
		return errors.New("specialization is required for teachers")
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
		Specialization: teacher.Specialization,
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

	// Create filter options
	filters := make(map[string]interface{})

	// Get all staff members
	staffMembers, err := rs.StaffRepo.List(r.Context(), filters)
	if err != nil {
		render.Render(w, r, ErrorInternalServer(err))
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		render.Render(w, r, ErrorInvalidRequest(errors.New("invalid staff ID")))
		return
	}

	// Get staff member
	staff, err := rs.StaffRepo.FindByID(r.Context(), id)
	if err != nil {
		render.Render(w, r, ErrorNotFound(errors.New("staff member not found")))
		return
	}

	// Get associated person
	person, err := rs.PersonService.Get(r.Context(), staff.PersonID)
	if err != nil {
		render.Render(w, r, ErrorInternalServer(errors.New("failed to get person data for staff member")))
		return
	}

	// Set person data
	staff.Person = person

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

// createStaff handles creating a new staff member
func (rs *Resource) createStaff(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &StaffRequest{}
	if err := render.Bind(r, req); err != nil {
		render.Render(w, r, ErrorInvalidRequest(err))
		return
	}

	// Verify person exists
	person, err := rs.PersonService.Get(r.Context(), req.PersonID)
	if err != nil {
		render.Render(w, r, ErrorNotFound(errors.New("person not found")))
		return
	}

	// Create staff
	staff := &users.Staff{
		PersonID:   req.PersonID,
		StaffNotes: req.StaffNotes,
	}

	// Create staff record
	if err := rs.StaffRepo.Create(r.Context(), staff); err != nil {
		render.Render(w, r, ErrorInternalServer(err))
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
			Specialization: req.Specialization,
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

		// Return teacher response
		response := newTeacherResponse(staff, teacher)
		common.Respond(w, r, http.StatusCreated, response, "Teacher created successfully")
		return
	}

	// Return staff response
	response := newStaffResponse(staff, isTeacher)
	common.Respond(w, r, http.StatusCreated, response, "Staff member created successfully")
}

// updateStaff handles updating a staff member
func (rs *Resource) updateStaff(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		render.Render(w, r, ErrorInvalidRequest(errors.New("invalid staff ID")))
		return
	}

	// Parse request
	req := &StaffRequest{}
	if err := render.Bind(r, req); err != nil {
		render.Render(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get existing staff member
	staff, err := rs.StaffRepo.FindByID(r.Context(), id)
	if err != nil {
		render.Render(w, r, ErrorNotFound(errors.New("staff member not found")))
		return
	}

	// Update fields
	staff.StaffNotes = req.StaffNotes

	// If person ID is changing, verify new person exists
	if staff.PersonID != req.PersonID {
		person, err := rs.PersonService.Get(r.Context(), req.PersonID)
		if err != nil {
			render.Render(w, r, ErrorNotFound(errors.New("person not found")))
			return
		}
		staff.PersonID = req.PersonID
		staff.Person = person
	} else {
		// Get associated person for response
		person, err := rs.PersonService.Get(r.Context(), staff.PersonID)
		if err != nil {
			render.Render(w, r, ErrorInternalServer(errors.New("failed to get person data for staff member")))
			return
		}
		staff.Person = person
	}

	// Update staff record
	if err := rs.StaffRepo.Update(r.Context(), staff); err != nil {
		render.Render(w, r, ErrorInternalServer(err))
		return
	}

	// Check if this staff member is also a teacher
	isTeacher := false
	var teacher *users.Teacher

	teacher, _ = rs.TeacherRepo.FindByStaffID(r.Context(), staff.ID)

	// Handle teacher record modifications
	if req.IsTeacher {
		if teacher != nil {
			// Update existing teacher record
			teacher.Specialization = req.Specialization
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
				Specialization: req.Specialization,
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		render.Render(w, r, ErrorInvalidRequest(errors.New("invalid staff ID")))
		return
	}

	// Check if this staff member is also a teacher
	teacher, err := rs.TeacherRepo.FindByStaffID(r.Context(), id)
	if err == nil && teacher != nil {
		// Delete teacher record first
		if err := rs.TeacherRepo.Delete(r.Context(), teacher.ID); err != nil {
			render.Render(w, r, ErrorInternalServer(errors.New("failed to delete teacher record")))
			return
		}
	}

	// Delete staff member
	if err := rs.StaffRepo.Delete(r.Context(), id); err != nil {
		render.Render(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Staff member deleted successfully")
}

// getStaffGroups handles getting groups for a staff member
func (rs *Resource) getStaffGroups(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		render.Render(w, r, ErrorInvalidRequest(errors.New("invalid staff ID")))
		return
	}

	// Check if staff exists
	staff, err := rs.StaffRepo.FindByID(r.Context(), id)
	if err != nil {
		render.Render(w, r, ErrorNotFound(errors.New("staff member not found")))
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
		render.Render(w, r, ErrorInternalServer(errors.New("education service not available")))
		return
	}

	// Get groups for this teacher
	groups, err := rs.EducationService.GetTeacherGroups(r.Context(), teacher.ID)
	if err != nil {
		render.Render(w, r, ErrorInternalServer(err))
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
		render.Render(w, r, ErrorInternalServer(err))
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

// Helper function to check if a string contains another string, ignoring case
func containsIgnoreCase(s, substr string) bool {
	s, substr = strings.ToLower(s), strings.ToLower(substr)
	return strings.Contains(s, substr)
}
