package students

import (
	"errors"
	"log"
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
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// Resource defines the students API resource
type Resource struct {
	PersonService      userService.PersonService
	StudentRepo        users.StudentRepository
	EducationService   educationService.Service
	UserContextService userContextService.UserContextService
}

// NewResource creates a new students resource
func NewResource(personService userService.PersonService, studentRepo users.StudentRepository, educationService educationService.Service, userContextService userContextService.UserContextService) *Resource {
	return &Resource{
		PersonService:      personService,
		StudentRepo:        studentRepo,
		EducationService:   educationService,
		UserContextService: userContextService,
	}
}

// Router returns a configured router for student endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Routes requiring users:read permission
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/", rs.listStudents)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}", rs.getStudent)

		// Routes requiring users:create permission
		r.With(authorize.RequiresPermission(permissions.UsersCreate)).Post("/", rs.createStudent)

		// Routes requiring users:update permission
		r.With(authorize.RequiresPermission(permissions.UsersUpdate)).Put("/{id}", rs.updateStudent)

		// Routes requiring users:delete permission
		r.With(authorize.RequiresPermission(permissions.UsersDelete)).Delete("/{id}", rs.deleteStudent)
	})

	return r
}

// StudentResponse represents a student response
type StudentResponse struct {
	ID              int64     `json:"id"`
	PersonID        int64     `json:"person_id"`
	FirstName       string    `json:"first_name"`
	LastName        string    `json:"last_name"`
	TagID           string    `json:"tag_id,omitempty"`
	SchoolClass     string    `json:"school_class"`
	Location        string    `json:"location"`
	GuardianName    string    `json:"guardian_name"`
	GuardianContact string    `json:"guardian_contact"`
	GuardianEmail   string    `json:"guardian_email,omitempty"`
	GuardianPhone   string    `json:"guardian_phone,omitempty"`
	GroupID         int64     `json:"group_id,omitempty"`
	GroupName       string    `json:"group_name,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// StudentRequest represents a student creation request with person details
type StudentRequest struct {
	// Person details (required)
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	TagID     string `json:"tag_id,omitempty"` // RFID tag ID (optional)

	// Student-specific details (required)
	SchoolClass     string `json:"school_class"`
	GuardianName    string `json:"guardian_name"`
	GuardianContact string `json:"guardian_contact"`

	// Optional fields
	GuardianEmail string `json:"guardian_email,omitempty"`
	GuardianPhone string `json:"guardian_phone,omitempty"`
	GroupID       *int64 `json:"group_id,omitempty"`
}

// UpdateStudentRequest represents a student update request
type UpdateStudentRequest struct {
	// Person details (optional for update)
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	TagID     *string `json:"tag_id,omitempty"`

	// Student-specific details (optional for update)
	SchoolClass     *string `json:"school_class,omitempty"`
	GuardianName    *string `json:"guardian_name,omitempty"`
	GuardianContact *string `json:"guardian_contact,omitempty"`
	GuardianEmail   *string `json:"guardian_email,omitempty"`
	GuardianPhone   *string `json:"guardian_phone,omitempty"`
	GroupID         *int64  `json:"group_id,omitempty"`
}

// Bind validates the student request
func (req *StudentRequest) Bind(r *http.Request) error {
	// Basic validation for person fields
	if req.FirstName == "" {
		return errors.New("first name is required")
	}
	if req.LastName == "" {
		return errors.New("last name is required")
	}

	// Basic validation for student fields
	if req.SchoolClass == "" {
		return errors.New("school class is required")
	}
	if req.GuardianName == "" {
		return errors.New("guardian name is required")
	}
	if req.GuardianContact == "" {
		return errors.New("guardian contact is required")
	}

	// Optional fields are not validated here - they will be validated in the model layer
	return nil
}

// Bind validates the update student request
func (req *UpdateStudentRequest) Bind(r *http.Request) error {
	// All fields are optional for updates, but validate if provided
	if req.FirstName != nil && *req.FirstName == "" {
		return errors.New("first name cannot be empty")
	}
	if req.LastName != nil && *req.LastName == "" {
		return errors.New("last name cannot be empty")
	}
	if req.SchoolClass != nil && *req.SchoolClass == "" {
		return errors.New("school class cannot be empty")
	}
	if req.GuardianName != nil && *req.GuardianName == "" {
		return errors.New("guardian name cannot be empty")
	}
	if req.GuardianContact != nil && *req.GuardianContact == "" {
		return errors.New("guardian contact cannot be empty")
	}
	return nil
}

// newStudentResponse creates a student response from a student and person model
// includeLocation determines whether to include sensitive location data
func newStudentResponse(student *users.Student, person *users.Person, group *education.Group, includeLocation bool) StudentResponse {
	response := StudentResponse{
		ID:              student.ID,
		PersonID:        student.PersonID,
		SchoolClass:     student.SchoolClass,
		GuardianName:    student.GuardianName,
		GuardianContact: student.GuardianContact,
		CreatedAt:       student.CreatedAt,
		UpdatedAt:       student.UpdatedAt,
	}

	// Include location only if authorized
	if includeLocation {
		response.Location = student.GetLocation()
	}

	if person != nil {
		response.FirstName = person.FirstName
		response.LastName = person.LastName
		if person.TagID != nil {
			response.TagID = *person.TagID
		}
	}

	if student.GuardianEmail != nil {
		response.GuardianEmail = *student.GuardianEmail
	}

	if student.GuardianPhone != nil {
		response.GuardianPhone = *student.GuardianPhone
	}

	if student.GroupID != nil {
		response.GroupID = *student.GroupID
	}

	if group != nil {
		response.GroupName = group.Name
	}

	return response
}

// listStudents handles listing all students
func (rs *Resource) listStudents(w http.ResponseWriter, r *http.Request) {
	// Create query options
	queryOptions := base.NewQueryOptions()

	// Add filters if provided
	schoolClass := r.URL.Query().Get("school_class")
	guardianName := r.URL.Query().Get("guardian_name")
	firstName := r.URL.Query().Get("first_name")
	lastName := r.URL.Query().Get("last_name")
	location := r.URL.Query().Get("location")
	groupIDStr := r.URL.Query().Get("group_id")

	// Create filter
	filter := base.NewFilter()

	// Apply filters
	if schoolClass != "" {
		filter.ILike("school_class", "%"+schoolClass+"%")
	}
	if guardianName != "" {
		filter.ILike("guardian_name", "%"+guardianName+"%")
	}
	if groupIDStr != "" {
		if groupID, err := strconv.ParseInt(groupIDStr, 10, 64); err == nil {
			filter.Equal("group_id", groupID)
		}
	}

	// Add pagination
	page := 1
	pageSize := 50

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	queryOptions.WithPagination(page, pageSize)
	queryOptions.Filter = filter

	var students []*users.Student
	var err error

	// Get user permissions to check admin status
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	isAdmin := hasAdminPermissions(userPermissions)

	// Track if user can access location data (admins or supervisors)
	canAccessLocation := isAdmin

	// If group_id is specified, check authorization and get students from that group
	if groupIDStr != "" {
		if groupID, parseErr := strconv.ParseInt(groupIDStr, 10, 64); parseErr == nil {
			// Check if user supervises this group (for location access)
			if !isAdmin {
				myGroups, err := rs.UserContextService.GetMyGroups(r.Context())
				if err != nil {
					if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
						log.Printf("Error rendering error response: %v", err)
					}
					return
				}

				// Check if the requested group_id is among the user's supervised groups
				supervises := false
				for _, group := range myGroups {
					if group.ID == groupID {
						supervises = true
						canAccessLocation = true
						break
					}
				}

				// Non-admin users can only access groups they supervise
				if !supervises {
					if err := render.Render(w, r, ErrorUnauthorized(errors.New("access denied: you don't supervise this group"))); err != nil {
						log.Printf("Error rendering error response: %v", err)
					}
					return
				}
			}

			// Get students from this educational group
			students, err = rs.StudentRepo.FindByGroupID(r.Context(), groupID)
			if err != nil {
				if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
					log.Printf("Error rendering error response: %v", err)
				}
				return
			}
		} else {
			// Invalid group_id format
			if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid group_id format"))); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
			return
		}
	} else {
		// Get all students using the new ListWithOptions method (when no group_id specified)
		// Admins can access all students, non-admins get empty list for general queries
		if !isAdmin {
			students = []*users.Student{} // Non-admins cannot list all students
		} else {
			students, err = rs.StudentRepo.ListWithOptions(r.Context(), queryOptions)
			if err != nil {
				if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
					log.Printf("Error rendering error response: %v", err)
				}
				return
			}
		}
	}

	// Build response with person data for each student
	responses := make([]StudentResponse, 0, len(students))
	for _, student := range students {
		// Get person data
		person, err := rs.PersonService.Get(r.Context(), student.PersonID)
		if err != nil {
			// Skip this student if person not found
			continue
		}

		// Filter based on person name if needed
		if (firstName != "" && !containsIgnoreCase(person.FirstName, firstName)) ||
			(lastName != "" && !containsIgnoreCase(person.LastName, lastName)) {
			continue
		}

		// Filter based on location (only if user can access location data)
		if location != "" && canAccessLocation && student.GetLocation() != location && location != "Unknown" {
			continue
		}

		// Get group data if student has a group
		var group *education.Group
		if student.GroupID != nil {
			groupData, err := rs.EducationService.GetGroup(r.Context(), *student.GroupID)
			if err == nil {
				group = groupData
			}
		}

		responses = append(responses, newStudentResponse(student, person, group, canAccessLocation))
	}

	common.RespondWithPagination(w, r, http.StatusOK, responses, page, pageSize, len(responses), "Students retrieved successfully")
}

// getStudent handles getting a student by ID
func (rs *Resource) getStudent(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid student ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get student
	student, err := rs.StudentRepo.FindByID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("student not found"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get person data
	person, err := rs.PersonService.Get(r.Context(), student.PersonID)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(errors.New("failed to get person data for student"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get group data if student has a group
	var group *education.Group
	if student.GroupID != nil {
		groupData, err := rs.EducationService.GetGroup(r.Context(), *student.GroupID)
		if err == nil {
			group = groupData
		}
	}

	// Check if user can access location data (admins can always access)
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	canAccessLocation := hasAdminPermissions(userPermissions)

	// TODO: For individual student access, could also check if user supervises the student's group
	// This would require additional logic to verify supervision

	common.Respond(w, r, http.StatusOK, newStudentResponse(student, person, group, canAccessLocation), "Student retrieved successfully")
}

// createStudent handles creating a new student with their person record
func (rs *Resource) createStudent(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &StudentRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Create person from request
	person := &users.Person{
		FirstName: req.FirstName,
		LastName:  req.LastName,
	}

	// Set optional TagID if provided
	if req.TagID != "" {
		tagID := req.TagID
		person.TagID = &tagID
	}

	// Create person - validation occurs at the model layer
	if err := rs.PersonService.Create(r.Context(), person); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Create student with the person ID
	student := &users.Student{
		PersonID:        person.ID,
		SchoolClass:     req.SchoolClass,
		GuardianName:    req.GuardianName,
		GuardianContact: req.GuardianContact,
	}

	// Set optional fields
	if req.GuardianEmail != "" {
		email := req.GuardianEmail
		student.GuardianEmail = &email
	}

	if req.GuardianPhone != "" {
		phone := req.GuardianPhone
		student.GuardianPhone = &phone
	}

	if req.GroupID != nil {
		student.GroupID = req.GroupID
	}

	// Create student
	if err := rs.StudentRepo.Create(r.Context(), student); err != nil {
		// Clean up person if student creation fails
		if deleteErr := rs.PersonService.Delete(r.Context(), person.ID); deleteErr != nil {
			log.Printf("Error cleaning up person after failed student creation: %v", deleteErr)
		}
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get group data if student has a group
	var group *education.Group
	if student.GroupID != nil {
		groupData, err := rs.EducationService.GetGroup(r.Context(), *student.GroupID)
		if err == nil {
			group = groupData
		}
	}

	// Admin users creating students can see full data including location
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	canAccessLocation := hasAdminPermissions(userPermissions)

	// Return the created student with person data
	common.Respond(w, r, http.StatusCreated, newStudentResponse(student, person, group, canAccessLocation), "Student created successfully")
}

// updateStudent handles updating an existing student
func (rs *Resource) updateStudent(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid student ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	req := &UpdateStudentRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Get existing student
	student, err := rs.StudentRepo.FindByID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("student not found"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get existing person
	person, err := rs.PersonService.Get(r.Context(), student.PersonID)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(errors.New("failed to get person data for student"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Update person fields if provided
	updatePerson := false
	if req.FirstName != nil {
		person.FirstName = *req.FirstName
		updatePerson = true
	}
	if req.LastName != nil {
		person.LastName = *req.LastName
		updatePerson = true
	}
	if req.TagID != nil {
		// Only update TagID if a value is provided
		if *req.TagID != "" {
			person.TagID = req.TagID
		} else {
			// Empty string means clear the TagID
			person.TagID = nil
		}
		updatePerson = true
	}

	// Update person if any fields changed
	if updatePerson {
		if err := rs.PersonService.Update(r.Context(), person); err != nil {
			if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
				log.Printf("Render error: %v", err)
			}
			return
		}
	}

	// Update student fields if provided
	if req.SchoolClass != nil {
		student.SchoolClass = *req.SchoolClass
	}
	if req.GuardianName != nil {
		student.GuardianName = *req.GuardianName
	}
	if req.GuardianContact != nil {
		student.GuardianContact = *req.GuardianContact
	}
	if req.GuardianEmail != nil {
		student.GuardianEmail = req.GuardianEmail
	}
	if req.GuardianPhone != nil {
		student.GuardianPhone = req.GuardianPhone
	}
	if req.GroupID != nil {
		student.GroupID = req.GroupID
	}

	// Update student
	if err := rs.StudentRepo.Update(r.Context(), student); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get updated student with person data
	updatedStudent, err := rs.StudentRepo.FindByID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get group data if student has a group
	var group *education.Group
	if updatedStudent.GroupID != nil {
		groupData, err := rs.EducationService.GetGroup(r.Context(), *updatedStudent.GroupID)
		if err == nil {
			group = groupData
		}
	}

	// Admin users updating students can see full data including location
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	canAccessLocation := hasAdminPermissions(userPermissions)

	// Return the updated student with person data
	common.Respond(w, r, http.StatusOK, newStudentResponse(updatedStudent, person, group, canAccessLocation), "Student updated successfully")
}

// deleteStudent handles deleting a student and their associated person record
func (rs *Resource) deleteStudent(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid student ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get the student to find the person ID
	student, err := rs.StudentRepo.FindByID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("student not found"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Delete the student first
	if err := rs.StudentRepo.Delete(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Then delete the associated person record
	if err := rs.PersonService.Delete(r.Context(), student.PersonID); err != nil {
		// Log the error but don't fail the request since student is already deleted
		log.Printf("Error deleting associated person record: %v", err)
	}

	common.Respond(w, r, http.StatusOK, nil, "Student deleted successfully")
}

// Helper function to check if a string contains another string, ignoring case
func containsIgnoreCase(s, substr string) bool {
	s, substr = strings.ToLower(s), strings.ToLower(substr)
	return strings.Contains(s, substr)
}

// Helper function to check if user has admin permissions
func hasAdminPermissions(permissions []string) bool {
	for _, perm := range permissions {
		if perm == "admin:*" || perm == "*:*" {
			return true
		}
	}
	return false
}
