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
	"github.com/moto-nrw/project-phoenix/models/users"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// Resource defines the students API resource
type Resource struct {
	PersonService userService.PersonService
	StudentRepo   users.StudentRepository
}

// NewResource creates a new students resource
func NewResource(personService userService.PersonService, studentRepo users.StudentRepository) *Resource {
	return &Resource{
		PersonService: personService,
		StudentRepo:   studentRepo,
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
		r.With(authorize.RequiresPermission(permissions.UsersCreate)).Post("/with-user", rs.createStudentWithUser)
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
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// StudentRequest represents a student creation request
type StudentRequest struct {
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	TagID           string `json:"tag_id,omitempty"`
	SchoolClass     string `json:"school_class"`
	GuardianName    string `json:"guardian_name"`
	GuardianContact string `json:"guardian_contact"`
	GuardianEmail   string `json:"guardian_email,omitempty"`
	GuardianPhone   string `json:"guardian_phone,omitempty"`
	GroupID         *int64 `json:"group_id,omitempty"`
}

// StudentWithUserRequest represents a student creation request with user account
type StudentWithUserRequest struct {
	Student  StudentRequest `json:"student"`
	Email    string         `json:"email"`
	Password string         `json:"password"`
}

// Bind validates the student request
func (req *StudentRequest) Bind(r *http.Request) error {
	// Basic validation
	if req.FirstName == "" {
		return errors.New("first name is required")
	}
	if req.LastName == "" {
		return errors.New("last name is required")
	}
	if req.SchoolClass == "" {
		return errors.New("school class is required")
	}
	if req.GuardianName == "" {
		return errors.New("guardian name is required")
	}
	if req.GuardianContact == "" {
		return errors.New("guardian contact is required")
	}
	return nil
}

// Bind validates the student with user request
func (req *StudentWithUserRequest) Bind(r *http.Request) error {
	// Validate student data
	if err := req.Student.Bind(r); err != nil {
		return err
	}

	// Validate user data
	if req.Email == "" {
		return errors.New("email is required")
	}
	if req.Password == "" {
		return errors.New("password is required")
	}
	if len(req.Password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}

// newStudentResponse creates a student response from a student and person model
func newStudentResponse(student *users.Student, person *users.Person) StudentResponse {
	response := StudentResponse{
		ID:              student.ID,
		PersonID:        student.PersonID,
		SchoolClass:     student.SchoolClass,
		Location:        student.GetLocation(),
		GuardianName:    student.GuardianName,
		GuardianContact: student.GuardianContact,
		CreatedAt:       student.CreatedAt,
		UpdatedAt:       student.UpdatedAt,
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

	// Create filter
	filter := base.NewFilter()

	// Apply filters
	if schoolClass != "" {
		filter.ILike("school_class", "%"+schoolClass+"%")
	}
	if guardianName != "" {
		filter.ILike("guardian_name", "%"+guardianName+"%")
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

	// Get all students using the new ListWithOptions method
	students, err := rs.StudentRepo.ListWithOptions(r.Context(), queryOptions)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
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

		// Filter based on location
		if location != "" && student.GetLocation() != location && location != "Unknown" {
			continue
		}

		responses = append(responses, newStudentResponse(student, person))
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

	common.Respond(w, r, http.StatusOK, newStudentResponse(student, person), "Student retrieved successfully")
}

// createStudentWithUser handles creating a new student with a user account
func (rs *Resource) createStudentWithUser(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &StudentWithUserRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// TODO: Create user account via auth service
	// This would require access to the auth service to create accounts
	// For now, we'll just create the person and student

	// Create person
	person := &users.Person{
		FirstName: req.Student.FirstName,
		LastName:  req.Student.LastName,
	}

	// Set TagID if provided
	if req.Student.TagID != "" {
		tagID := req.Student.TagID
		person.TagID = &tagID
	}

	// Create person
	if err := rs.PersonService.Create(r.Context(), person); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Create student
	student := &users.Student{
		PersonID:        person.ID,
		SchoolClass:     req.Student.SchoolClass,
		GuardianName:    req.Student.GuardianName,
		GuardianContact: req.Student.GuardianContact,
	}

	// Set optional fields
	if req.Student.GuardianEmail != "" {
		email := req.Student.GuardianEmail
		student.GuardianEmail = &email
	}

	if req.Student.GuardianPhone != "" {
		phone := req.Student.GuardianPhone
		student.GuardianPhone = &phone
	}

	if req.Student.GroupID != nil {
		student.GroupID = req.Student.GroupID
	}

	// Create student
	if err := rs.StudentRepo.Create(r.Context(), student); err != nil {
		// Attempt to clean up person if student creation fails
		_ = rs.PersonService.Delete(r.Context(), person.ID)
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Return the created student
	common.Respond(w, r, http.StatusCreated, newStudentResponse(student, person), "Student created successfully")
}

// createStudent handles creating a new student without requiring user account or RFID
func (rs *Resource) createStudent(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &StudentRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Create person with minimal requirements
	person := &users.Person{
		FirstName: req.FirstName,
		LastName:  req.LastName,
	}

	// Set TagID only if provided and not empty
	if req.TagID != "" {
		tagID := req.TagID
		person.TagID = &tagID
	}

	// Create person using the service
	// No longer requires TagID or AccountID
	if err := rs.PersonService.Create(r.Context(), person); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Create student
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
		// Attempt to clean up person if student creation fails
		_ = rs.PersonService.Delete(r.Context(), person.ID)
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Return the created student
	common.Respond(w, r, http.StatusCreated, newStudentResponse(student, person), "Student created successfully")
}

// Helper function to check if a string contains another string, ignoring case
func containsIgnoreCase(s, substr string) bool {
	s, substr = strings.ToLower(s), strings.ToLower(substr)
	return strings.Contains(s, substr)
}
