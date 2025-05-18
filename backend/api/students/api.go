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

	// Return the created student with person data
	common.Respond(w, r, http.StatusCreated, newStudentResponse(student, person), "Student created successfully")
}

// Helper function to check if a string contains another string, ignoring case
func containsIgnoreCase(s, substr string) bool {
	s, substr = strings.ToLower(s), strings.ToLower(substr)
	return strings.Contains(s, substr)
}
