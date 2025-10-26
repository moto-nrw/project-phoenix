package students

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// Resource defines the students API resource
type Resource struct {
	PersonService      userService.PersonService
	StudentRepo        users.StudentRepository
	EducationService   educationService.Service
	UserContextService userContextService.UserContextService
	ActiveService      activeService.Service
	IoTService         iotSvc.Service
	PrivacyConsentRepo users.PrivacyConsentRepository
}

// NewResource creates a new students resource
func NewResource(personService userService.PersonService, studentRepo users.StudentRepository, educationService educationService.Service, userContextService userContextService.UserContextService, activeService activeService.Service, iotService iotSvc.Service, privacyConsentRepo users.PrivacyConsentRepository) *Resource {
	return &Resource{
		PersonService:      personService,
		StudentRepo:        studentRepo,
		EducationService:   educationService,
		UserContextService: userContextService,
		ActiveService:      activeService,
		IoTService:         iotService,
		PrivacyConsentRepo: privacyConsentRepo,
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
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}/in-group-room", rs.getStudentInGroupRoom)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}/current-location", rs.getStudentCurrentLocation)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}/current-visit", rs.getStudentCurrentVisit)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}/visit-history", rs.getStudentVisitHistory)

		// Routes requiring users:create permission
		r.With(authorize.RequiresPermission(permissions.UsersCreate)).Post("/", rs.createStudent)

		// Routes requiring users:update permission
		r.With(authorize.RequiresPermission(permissions.UsersUpdate)).Put("/{id}", rs.updateStudent)

		// Routes requiring users:delete permission
		r.With(authorize.RequiresPermission(permissions.UsersDelete)).Delete("/{id}", rs.deleteStudent)

		// Privacy consent routes
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}/privacy-consent", rs.getStudentPrivacyConsent)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate)).Put("/{id}/privacy-consent", rs.updateStudentPrivacyConsent)
	})

	// Device-authenticated routes for RFID devices
	r.Group(func(r chi.Router) {
		r.Use(device.DeviceAuthenticator(rs.IoTService, rs.PersonService))

		// RFID tag assignment endpoint
		r.Post("/{id}/rfid", rs.assignRFIDTag)
		r.Delete("/{id}/rfid", rs.unassignRFIDTag)
	})

	return r
}

// StudentResponse represents a student response
type StudentResponse struct {
	ID                int64                  `json:"id"`
	PersonID          int64                  `json:"person_id"`
	FirstName         string                 `json:"first_name"`
	LastName          string                 `json:"last_name"`
	TagID             string                 `json:"tag_id,omitempty"`
	SchoolClass       string                 `json:"school_class"`
	Location          string                 `json:"current_location"`
	GuardianName      string                 `json:"guardian_name"`
	GuardianContact   string                 `json:"guardian_contact,omitempty"`
	GuardianEmail     string                 `json:"guardian_email,omitempty"`
	GuardianPhone     string                 `json:"guardian_phone,omitempty"`
	GroupID           int64                  `json:"group_id,omitempty"`
	GroupName         string                 `json:"group_name,omitempty"`
	ScheduledCheckout *ScheduledCheckoutInfo `json:"scheduled_checkout,omitempty"`
	ExtraInfo         string                 `json:"extra_info,omitempty"`
	HealthInfo        string                 `json:"health_info,omitempty"`
	SupervisorNotes   string                 `json:"supervisor_notes,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// ScheduledCheckoutInfo represents scheduled checkout information for a student
type ScheduledCheckoutInfo struct {
	ID           int64     `json:"id"`
	ScheduledFor time.Time `json:"scheduled_for"`
	Reason       string    `json:"reason,omitempty"`
	ScheduledBy  string    `json:"scheduled_by"` // Name of the person who scheduled
}

// SupervisorContact represents contact information for a group supervisor
type SupervisorContact struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Role      string `json:"role"` // "teacher" or "staff"
}

// StudentDetailResponse represents a detailed student response with access control
type StudentDetailResponse struct {
	StudentResponse
	HasFullAccess    bool                `json:"has_full_access"`
	GroupSupervisors []SupervisorContact `json:"group_supervisors,omitempty"`
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
	GuardianEmail string  `json:"guardian_email,omitempty"`
	GuardianPhone string  `json:"guardian_phone,omitempty"`
	GroupID       *int64  `json:"group_id,omitempty"`
	ExtraInfo     *string `json:"extra_info,omitempty"` // Extra information visible to supervisors
}

// UpdateStudentRequest represents a student update request
type UpdateStudentRequest struct {
	// Person details (optional for update)
	FirstName *string    `json:"first_name,omitempty"`
	LastName  *string    `json:"last_name,omitempty"`
	Birthday  *time.Time `json:"birthday,omitempty"`
	TagID     *string    `json:"tag_id,omitempty"`

	// Student-specific details (optional for update)
	SchoolClass     *string `json:"school_class,omitempty"`
	GuardianName    *string `json:"guardian_name,omitempty"`
	GuardianContact *string `json:"guardian_contact,omitempty"`
	GuardianEmail   *string `json:"guardian_email,omitempty"`
	GuardianPhone   *string `json:"guardian_phone,omitempty"`
	GroupID         *int64  `json:"group_id,omitempty"`
	HealthInfo      *string `json:"health_info,omitempty"`      // Static health and medical information
	SupervisorNotes *string `json:"supervisor_notes,omitempty"` // Notes from supervisors
	ExtraInfo       *string `json:"extra_info,omitempty"`       // Extra information visible to supervisors
}

// RFIDAssignmentRequest represents an RFID tag assignment request
type RFIDAssignmentRequest struct {
	RFIDTag string `json:"rfid_tag"`
}

// RFIDAssignmentResponse represents an RFID tag assignment response
type RFIDAssignmentResponse struct {
	Success     bool    `json:"success"`
	StudentID   int64   `json:"student_id"`
	StudentName string  `json:"student_name"`
	RFIDTag     string  `json:"rfid_tag"`
	PreviousTag *string `json:"previous_tag,omitempty"`
	Message     string  `json:"message"`
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

// Bind validates the RFID assignment request
func (req *RFIDAssignmentRequest) Bind(r *http.Request) error {
	if req.RFIDTag == "" {
		return errors.New("rfid_tag is required")
	}
	if len(req.RFIDTag) < 8 {
		return errors.New("rfid_tag must be at least 8 characters")
	}
	if len(req.RFIDTag) > 64 {
		return errors.New("rfid_tag must be at most 64 characters")
	}
	return nil
}

// newStudentResponse creates a student response from a student and person model
// hasFullAccess determines whether to include detailed location data and supervisor-only information (like extra info)
func newStudentResponse(ctx context.Context, student *users.Student, person *users.Person, group *education.Group, hasFullAccess bool, activeService activeService.Service, personService userService.PersonService) StudentResponse {
	response := StudentResponse{
		ID:           student.ID,
		PersonID:     student.PersonID,
		SchoolClass:  student.SchoolClass,
		GuardianName: student.GuardianName,
		CreatedAt:    student.CreatedAt,
		UpdatedAt:    student.UpdatedAt,
	}

	// Only include guardian contact info for users with full access
	if hasFullAccess {
		response.GuardianContact = student.GuardianContact
	}

	response.Location = resolveStudentLocation(ctx, student.ID, hasFullAccess, activeService)

	// Check for pending scheduled checkout
	if pendingCheckout, err := activeService.GetPendingScheduledCheckout(ctx, student.ID); err == nil && pendingCheckout != nil {
		// Get the name of the person who scheduled the checkout
		scheduledByName := "Unknown"
		if staff, err := personService.StaffRepository().FindByID(ctx, pendingCheckout.ScheduledBy); err == nil && staff != nil {
			if person, err := personService.Get(ctx, staff.PersonID); err == nil && person != nil {
				scheduledByName = person.FirstName + " " + person.LastName
			}
		}

		response.ScheduledCheckout = &ScheduledCheckoutInfo{
			ID:           pendingCheckout.ID,
			ScheduledFor: pendingCheckout.ScheduledFor,
			Reason:       pendingCheckout.Reason,
			ScheduledBy:  scheduledByName,
		}
	}

	if person != nil {
		response.FirstName = person.FirstName
		response.LastName = person.LastName
		// Only include RFID tag for users with full access
		if hasFullAccess && person.TagID != nil {
			response.TagID = *person.TagID
		}
	}

	// Only include guardian email and phone for users with full access
	if hasFullAccess {
		if student.GuardianEmail != nil {
			response.GuardianEmail = *student.GuardianEmail
		}

		if student.GuardianPhone != nil {
			response.GuardianPhone = *student.GuardianPhone
		}
	}

	if student.GroupID != nil {
		response.GroupID = *student.GroupID
	}

	if group != nil {
		response.GroupName = group.Name
	}

	// Include sensitive fields only for users with full access (supervisors/admins)
	if hasFullAccess {
		if student.ExtraInfo != nil && *student.ExtraInfo != "" {
			response.ExtraInfo = *student.ExtraInfo
		}
		if student.HealthInfo != nil {
			response.HealthInfo = *student.HealthInfo
		}
		if student.SupervisorNotes != nil {
			response.SupervisorNotes = *student.SupervisorNotes
		}
	}

	return response
}

func resolveStudentLocation(ctx context.Context, studentID int64, hasFullAccess bool, activeService activeService.Service) string {
	attendanceStatus, err := activeService.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil || attendanceStatus == nil {
		return "Abwesend"
	}

	if attendanceStatus.Status != "checked_in" {
		return "Abwesend"
	}

	if !hasFullAccess {
		return "Anwesend"
	}

	currentVisit, err := activeService.GetStudentCurrentVisit(ctx, studentID)
	if err != nil || currentVisit == nil {
		return "Anwesend"
	}

	if currentVisit.ActiveGroupID <= 0 {
		return "Anwesend"
	}

	activeGroup, err := activeService.GetActiveGroup(ctx, currentVisit.ActiveGroupID)
	if err != nil || activeGroup == nil {
		return "Anwesend"
	}

	if activeGroup.Room != nil && activeGroup.Room.Name != "" {
		return fmt.Sprintf("Anwesend - %s", activeGroup.Room.Name)
	}

	return "Anwesend"
}

// listStudents handles listing all students with staff-based filtering
func (rs *Resource) listStudents(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	schoolClass := r.URL.Query().Get("school_class")
	guardianName := r.URL.Query().Get("guardian_name")
	firstName := r.URL.Query().Get("first_name")
	lastName := r.URL.Query().Get("last_name")
	location := r.URL.Query().Get("location")
	groupIDStr := r.URL.Query().Get("group_id")
	search := r.URL.Query().Get("search") // New search parameter

	// Get user permissions to check admin status
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	isAdmin := hasAdminPermissions(userPermissions)

	// For search functionality, we show all students regardless of group supervision
	// Permission checking will be done on individual student detail view
	var allowedGroupIDs []int64
	hasFullAccess := isAdmin

	// If a specific group filter is requested, apply it
	if groupIDStr != "" {
		if groupID, err := strconv.ParseInt(groupIDStr, 10, 64); err == nil {
			allowedGroupIDs = []int64{groupID}
		}
	}

	// Create query options
	queryOptions := base.NewQueryOptions()
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

	var students []*users.Student
	var totalCount int

	// Get students - show all for search functionality
	if len(allowedGroupIDs) > 0 {
		// Specific group filter requested
		var err error
		students, err = rs.StudentRepo.FindByGroupIDs(r.Context(), allowedGroupIDs)
		if err != nil {
			if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
			return
		}
		totalCount = len(students)
	} else {
		// No specific group filter - get all students

		// First, count total students matching database filters (without person-based filters)
		// This gives us an approximate count for pagination
		countOptions := base.NewQueryOptions()
		countFilter := base.NewFilter()

		// Apply only database-level filters for counting
		if schoolClass != "" {
			countFilter.ILike("school_class", "%"+schoolClass+"%")
		}
		if guardianName != "" {
			countFilter.ILike("guardian_name", "%"+guardianName+"%")
		}
		countOptions.Filter = countFilter

		// Get the count efficiently from database
		dbCount, err := rs.StudentRepo.CountWithOptions(r.Context(), countOptions)
		if err != nil {
			if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
			return
		}

		// If no search/person filters, use the database count
		if search == "" && firstName == "" && lastName == "" && location == "" {
			totalCount = dbCount
		} else {
			// With search/person filters, we need to count after filtering
			// For now, use the database count as an approximation
			// In a production system, you might want to do this filtering at the database level
			totalCount = dbCount
		}

		// Get the paginated subset
		students, err = rs.StudentRepo.ListWithOptions(r.Context(), queryOptions)
		if err != nil {
			if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
			return
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

		// Apply search filter if provided
		if search != "" {
			// Search in first name, last name, and student ID
			studentIDStr := strconv.FormatInt(student.ID, 10)
			if !containsIgnoreCase(person.FirstName, search) &&
				!containsIgnoreCase(person.LastName, search) &&
				!containsIgnoreCase(studentIDStr, search) &&
				!containsIgnoreCase(person.FirstName+" "+person.LastName, search) {
				continue
			}
		}

		// Filter based on person name if needed (legacy filters)
		if (firstName != "" && !containsIgnoreCase(person.FirstName, firstName)) ||
			(lastName != "" && !containsIgnoreCase(person.LastName, lastName)) {
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

		studentResponse := newStudentResponse(r.Context(), student, person, group, hasFullAccess, rs.ActiveService, rs.PersonService)

		if location != "" && hasFullAccess && location != "Unknown" && studentResponse.Location != location {
			continue
		}

		responses = append(responses, studentResponse)
	}

	common.RespondWithPagination(w, r, http.StatusOK, responses, page, pageSize, totalCount, "Students retrieved successfully")
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

	// Check if user has full access
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	isAdmin := hasAdminPermissions(userPermissions)
	hasFullAccess := isAdmin

	// Check if user supervises the student's group
	if !hasFullAccess && student.GroupID != nil {
		staff, err := rs.UserContextService.GetCurrentStaff(r.Context())
		if err == nil && staff != nil {
			// Check if staff supervises this group
			educationGroups, err := rs.UserContextService.GetMyGroups(r.Context())
			if err == nil {
				for _, supervGroup := range educationGroups {
					if supervGroup.ID == *student.GroupID {
						hasFullAccess = true
						break
					}
				}
			}
		}
	}

	// Prepare response
	response := StudentDetailResponse{
		StudentResponse: newStudentResponse(r.Context(), student, person, group, hasFullAccess, rs.ActiveService, rs.PersonService),
		HasFullAccess:   hasFullAccess,
	}

	// If user doesn't have full access, add supervisor contacts
	if !hasFullAccess && group != nil {
		supervisors := []SupervisorContact{}

		// Get group teachers/supervisors
		teachers, err := rs.EducationService.GetGroupTeachers(r.Context(), group.ID)
		if err == nil {
			for _, teacher := range teachers {
				if teacher != nil && teacher.Staff != nil && teacher.Staff.Person != nil {
					supervisor := SupervisorContact{
						ID:        teacher.ID,
						FirstName: teacher.Staff.Person.FirstName,
						LastName:  teacher.Staff.Person.LastName,
						Role:      "teacher",
					}
					// Get email from person's account if available
					if teacher.Staff.Person.Account != nil {
						supervisor.Email = teacher.Staff.Person.Account.Email
					}
					supervisors = append(supervisors, supervisor)
				}
			}
		}

		response.GroupSupervisors = supervisors

		// Note: Sensitive fields are already handled in newStudentResponse based on hasFullAccess
		// No need to clear them here as they won't be set if user lacks access
		// Location handling is special - we keep basic attendance status as it's public information
		if response.Location != "Anwesend" && response.Location != "Abwesend" {
			response.Location = ""
		}
	}

	common.Respond(w, r, http.StatusOK, response, "Student retrieved successfully")
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

	if req.ExtraInfo != nil {
		student.ExtraInfo = req.ExtraInfo
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

	// Admin users creating students can see full data including detailed location
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	hasFullAccess := hasAdminPermissions(userPermissions)

	// Return the created student with person data
	common.Respond(w, r, http.StatusCreated, newStudentResponse(r.Context(), student, person, group, hasFullAccess, rs.ActiveService, rs.PersonService), "Student created successfully")
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

	// Centralized permission check for updating student data
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	authorized, authErr := canUpdateStudent(r.Context(), userPermissions, student, rs.UserContextService)
	if !authorized {
		if err := render.Render(w, r, ErrorForbidden(authErr)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Track whether the user is admin or group supervisor
	isAdmin := hasAdminPermissions(userPermissions)
	isGroupSupervisor := !isAdmin // If not admin but authorized, must be group supervisor

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
	if req.Birthday != nil {
		person.Birthday = req.Birthday
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
	if req.ExtraInfo != nil {
		student.ExtraInfo = req.ExtraInfo
	}
	if req.HealthInfo != nil {
		student.HealthInfo = req.HealthInfo
	}
	if req.SupervisorNotes != nil {
		student.SupervisorNotes = req.SupervisorNotes
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

	// Admin users and group supervisors can see full data including detailed location
	// Explicitly verify access level based on the checks performed above
	hasFullAccess := isAdmin || isGroupSupervisor // Explicitly check for admin or group supervisor

	// Return the updated student with person data
	common.Respond(w, r, http.StatusOK, newStudentResponse(r.Context(), updatedStudent, person, group, hasFullAccess, rs.ActiveService, rs.PersonService), "Student updated successfully")
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

	// Check if user has permission to delete this student
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	authorized, authErr := canDeleteStudent(r.Context(), userPermissions, student, rs.UserContextService)
	if !authorized {
		if err := render.Render(w, r, ErrorForbidden(authErr)); err != nil {
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

// getStudentCurrentLocation handles getting a student's current location with scheduled checkout info
func (rs *Resource) getStudentCurrentLocation(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid student ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get student with person and group details
	student, err := rs.StudentRepo.FindByID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("student not found"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get person details
	person, err := rs.PersonService.Get(r.Context(), student.PersonID)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get group details if student has a group
	var group *education.Group
	if student.GroupID != nil {
		group, _ = rs.EducationService.GetGroup(r.Context(), *student.GroupID)
	}

	// Check user's permission level
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	isAdmin := hasAdminPermissions(userPermissions)
	staff, _ := rs.UserContextService.GetCurrentStaff(r.Context())

	// Determine if user has full access to student location details
	hasFullAccess := isAdmin
	if !hasFullAccess && staff != nil && student.GroupID != nil {
		educationGroups, _ := rs.UserContextService.GetMyGroups(r.Context())
		for _, eduGroup := range educationGroups {
			if eduGroup.ID == *student.GroupID {
				hasFullAccess = true
				break
			}
		}
	}

	// Build student response
	response := newStudentResponse(r.Context(), student, person, group, hasFullAccess, rs.ActiveService, rs.PersonService)

	// Create location response structure
	locationResponse := struct {
		Location          string                 `json:"current_location"`
		CurrentRoom       string                 `json:"current_room,omitempty"`
		ScheduledCheckout *ScheduledCheckoutInfo `json:"scheduled_checkout,omitempty"`
	}{
		Location:          response.Location,
		ScheduledCheckout: response.ScheduledCheckout,
	}

	// If student is present and user has full access, try to get current room
	if hasFullAccess && response.Location == "Anwesend" {
		if currentVisit, err := rs.ActiveService.GetStudentCurrentVisit(r.Context(), student.ID); err == nil && currentVisit != nil {
			if activeGroup, err := rs.ActiveService.GetActiveGroup(r.Context(), currentVisit.ActiveGroupID); err == nil && activeGroup != nil {
				// The room should be loaded as part of the active group
				if activeGroup.Room != nil {
					locationResponse.CurrentRoom = activeGroup.Room.Name
				}
			}
		}
	}

	common.Respond(w, r, http.StatusOK, locationResponse, "Student location retrieved successfully")
}

// getStudentInGroupRoom checks if a student is in their educational group's room
func (rs *Resource) getStudentInGroupRoom(w http.ResponseWriter, r *http.Request) {
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

	// Check if student has an educational group
	if student.GroupID == nil {
		common.Respond(w, r, http.StatusOK, map[string]interface{}{
			"in_group_room": false,
			"reason":        "no_group",
		}, "Student has no educational group")
		return
	}

	// Get the educational group
	group, err := rs.EducationService.GetGroup(r.Context(), *student.GroupID)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(errors.New("failed to get student's group"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check authorization - only group supervisors can see this information
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	isAdmin := hasAdminPermissions(userPermissions)

	if !isAdmin {
		// Check if user supervises this educational group
		staff, err := rs.UserContextService.GetCurrentStaff(r.Context())
		if err != nil || staff == nil {
			if err := render.Render(w, r, ErrorForbidden(errors.New("unauthorized to view student room status"))); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
			return
		}

		// Check if staff supervises this group
		hasAccess := false
		educationGroups, err := rs.UserContextService.GetMyGroups(r.Context())
		if err == nil {
			for _, supervGroup := range educationGroups {
				if supervGroup.ID == *student.GroupID {
					hasAccess = true
					break
				}
			}
		}

		if !hasAccess {
			if err := render.Render(w, r, ErrorForbidden(errors.New("you do not supervise this student's group"))); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
			return
		}
	}

	// Check if the educational group has a room assigned
	if group.RoomID == nil {
		common.Respond(w, r, http.StatusOK, map[string]interface{}{
			"in_group_room": false,
			"reason":        "group_no_room",
		}, "Educational group has no assigned room")
		return
	}

	// Get the student's current active visit
	visit, err := rs.ActiveService.GetStudentCurrentVisit(r.Context(), student.ID)
	if err != nil {
		// No active visit means student is not in any room
		common.Respond(w, r, http.StatusOK, map[string]interface{}{
			"in_group_room": false,
			"reason":        "no_active_visit",
		}, "Student has no active visit")
		return
	}

	// Get the active group to check its room
	activeGroup, err := rs.ActiveService.GetActiveGroup(r.Context(), visit.ActiveGroupID)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(errors.New("failed to get active group"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if the active group's room matches the educational group's room
	inGroupRoom := activeGroup.RoomID == *group.RoomID

	// Prepare response
	response := map[string]interface{}{
		"in_group_room":   inGroupRoom,
		"group_room_id":   *group.RoomID,
		"current_room_id": activeGroup.RoomID,
	}

	// Add room names if available
	if group.Room != nil {
		response["group_room_name"] = group.Room.Name
	}

	common.Respond(w, r, http.StatusOK, response, "Student room status retrieved successfully")
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

// canModifyStudent centralizes the authorization logic for modifying student data (update/delete)
func canModifyStudent(ctx context.Context, userPermissions []string, student *users.Student, userContextService userContextService.UserContextService, operation string) (bool, error) {
	// Admin users have full access
	if hasAdminPermissions(userPermissions) {
		return true, nil
	}

	// Student must have a group for non-admin operations
	if student.GroupID == nil {
		return false, fmt.Errorf("only administrators can %s students without assigned groups", operation)
	}

	// Check if user is a staff member
	staff, err := userContextService.GetCurrentStaff(ctx)
	if err != nil || staff == nil {
		return false, fmt.Errorf("insufficient permissions to %s this student's data", operation)
	}

	// Check if staff supervises the student's group
	if supervised := isGroupSupervisor(ctx, *student.GroupID, userContextService); supervised {
		return true, nil
	}

	return false, fmt.Errorf("you can only %s students in groups you supervise", operation)
}

// canUpdateStudent is a convenience wrapper for update operations
func canUpdateStudent(ctx context.Context, userPermissions []string, student *users.Student, userContextService userContextService.UserContextService) (bool, error) {
	return canModifyStudent(ctx, userPermissions, student, userContextService, "update")
}

// canDeleteStudent is a convenience wrapper for delete operations
func canDeleteStudent(ctx context.Context, userPermissions []string, student *users.Student, userContextService userContextService.UserContextService) (bool, error) {
	return canModifyStudent(ctx, userPermissions, student, userContextService, "delete")
}

// isGroupSupervisor checks if the current user supervises a specific group
func isGroupSupervisor(ctx context.Context, groupID int64, userContextService userContextService.UserContextService) bool {
	// Check education groups
	educationGroups, err := userContextService.GetMyGroups(ctx)
	if err == nil {
		for _, g := range educationGroups {
			if g.ID == groupID {
				return true
			}
		}
	}

	// Also check active groups
	activeGroups, err := userContextService.GetMyActiveGroups(ctx)
	if err == nil {
		for _, ag := range activeGroups {
			if ag.GroupID == groupID {
				return true
			}
		}
	}

	return false
}

// PrivacyConsentResponse represents a privacy consent response
type PrivacyConsentResponse struct {
	ID                int64                  `json:"id"`
	StudentID         int64                  `json:"student_id"`
	PolicyVersion     string                 `json:"policy_version"`
	Accepted          bool                   `json:"accepted"`
	AcceptedAt        *time.Time             `json:"accepted_at,omitempty"`
	ExpiresAt         *time.Time             `json:"expires_at,omitempty"`
	DurationDays      *int                   `json:"duration_days,omitempty"`
	RenewalRequired   bool                   `json:"renewal_required"`
	DataRetentionDays int                    `json:"data_retention_days"`
	Details           map[string]interface{} `json:"details,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// PrivacyConsentRequest represents a privacy consent update request
type PrivacyConsentRequest struct {
	PolicyVersion     string                 `json:"policy_version"`
	Accepted          bool                   `json:"accepted"`
	DurationDays      *int                   `json:"duration_days,omitempty"`
	DataRetentionDays int                    `json:"data_retention_days"`
	Details           map[string]interface{} `json:"details,omitempty"`
}

// Bind validates the privacy consent request
func (req *PrivacyConsentRequest) Bind(r *http.Request) error {
	if req.PolicyVersion == "" {
		return errors.New("policy version is required")
	}
	if req.DataRetentionDays < 1 || req.DataRetentionDays > 31 {
		return errors.New("data retention days must be between 1 and 31")
	}
	return nil
}

// newPrivacyConsentResponse converts a privacy consent model to a response
func newPrivacyConsentResponse(consent *users.PrivacyConsent) PrivacyConsentResponse {
	return PrivacyConsentResponse{
		ID:                consent.ID,
		StudentID:         consent.StudentID,
		PolicyVersion:     consent.PolicyVersion,
		Accepted:          consent.Accepted,
		AcceptedAt:        consent.AcceptedAt,
		ExpiresAt:         consent.ExpiresAt,
		DurationDays:      consent.DurationDays,
		RenewalRequired:   consent.RenewalRequired,
		DataRetentionDays: consent.DataRetentionDays,
		Details:           consent.Details,
		CreatedAt:         consent.CreatedAt,
		UpdatedAt:         consent.UpdatedAt,
	}
}

// assignRFIDTag handles assigning an RFID tag to a student (device-authenticated endpoint)
func (rs *Resource) assignRFIDTag(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		if err := render.Render(w, r, ErrorUnauthorized(errors.New("device authentication required"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse student ID from URL
	studentID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid student ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	req := &RFIDAssignmentRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get the student to assign tag to
	student, err := rs.StudentRepo.FindByID(r.Context(), studentID)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("student not found"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get person details for the student
	person, err := rs.PersonService.Get(r.Context(), student.PersonID)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(errors.New("failed to get person data for student"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// With global PIN authentication, we trust the device to assign tags to any student
	// No need to check teacher supervision rights

	// Store previous tag for response
	var previousTag *string
	if person.TagID != nil {
		previousTag = person.TagID
	}

	// Assign the RFID tag (this handles unlinking old assignments automatically)
	if err := rs.PersonService.LinkToRFIDCard(r.Context(), person.ID, req.RFIDTag); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Create response
	response := RFIDAssignmentResponse{
		Success:     true,
		StudentID:   student.ID,
		StudentName: person.FirstName + " " + person.LastName,
		RFIDTag:     req.RFIDTag,
		PreviousTag: previousTag,
		Message:     "RFID tag assigned successfully",
	}

	if previousTag != nil {
		response.Message = "RFID tag assigned successfully (previous tag replaced)"
	}

	// Log assignment for audit trail
	log.Printf("RFID tag assignment: device=%s, student=%d, tag=%s, previous_tag=%v",
		deviceCtx.DeviceID, studentID, req.RFIDTag, previousTag)

	common.Respond(w, r, http.StatusOK, response, response.Message)
}

// unassignRFIDTag handles removing an RFID tag from a student (device-authenticated endpoint)
func (rs *Resource) unassignRFIDTag(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		if err := render.Render(w, r, ErrorUnauthorized(errors.New("device authentication required"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse student ID from URL
	studentID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid student ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get the student
	student, err := rs.StudentRepo.FindByID(r.Context(), studentID)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("student not found"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get person details for the student
	person, err := rs.PersonService.Get(r.Context(), student.PersonID)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(errors.New("failed to get person data for student"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if student has an RFID tag assigned
	if person.TagID == nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("student has no RFID tag assigned"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Store removed tag for response
	removedTag := *person.TagID

	// Unlink the RFID tag
	if err := rs.PersonService.UnlinkFromRFIDCard(r.Context(), person.ID); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Create response
	response := RFIDAssignmentResponse{
		Success:     true,
		StudentID:   student.ID,
		StudentName: person.FirstName + " " + person.LastName,
		RFIDTag:     removedTag,
		Message:     "RFID tag unassigned successfully",
	}

	// Log unassignment for audit trail
	log.Printf("RFID tag unassignment: device=%s, student=%d, tag=%s",
		deviceCtx.DeviceID, studentID, removedTag)

	common.Respond(w, r, http.StatusOK, response, response.Message)
}

// getStudentPrivacyConsent handles getting a student's privacy consent
func (rs *Resource) getStudentPrivacyConsent(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid student ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if user has permission to view this student's data
	// Admin users have full access
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	isAdmin := hasAdminPermissions(userPermissions)

	if !isAdmin {
		// Check if user is a staff member who supervises the student's group
		student, err := rs.StudentRepo.FindByID(r.Context(), id)
		if err != nil {
			if err := render.Render(w, r, ErrorNotFound(errors.New("student not found"))); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
			return
		}

		if student.GroupID != nil {
			// Check if current user supervises this group
			staff, err := rs.UserContextService.GetCurrentStaff(r.Context())
			if err != nil || staff == nil {
				if err := render.Render(w, r, ErrorForbidden(errors.New("insufficient permissions to access this student's data"))); err != nil {
					log.Printf("Error rendering error response: %v", err)
				}
				return
			}

			// Check if staff supervises the student's group
			educationGroups, err := rs.UserContextService.GetMyGroups(r.Context())
			if err != nil {
				if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
					log.Printf("Error rendering error response: %v", err)
				}
				return
			}

			hasAccess := false
			for _, group := range educationGroups {
				if group.ID == *student.GroupID {
					hasAccess = true
					break
				}
			}

			if !hasAccess {
				if err := render.Render(w, r, ErrorForbidden(errors.New("you do not supervise this student's group"))); err != nil {
					log.Printf("Error rendering error response: %v", err)
				}
				return
			}
		}
	}

	// Get privacy consents
	consents, err := rs.PrivacyConsentRepo.FindByStudentID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Find the most recent accepted consent
	var consent *users.PrivacyConsent
	for _, c := range consents {
		if c.Accepted && (consent == nil || c.CreatedAt.After(consent.CreatedAt)) {
			consent = c
		}
	}

	// If no consent exists, return a default response
	if consent == nil {
		response := PrivacyConsentResponse{
			StudentID:         id,
			PolicyVersion:     "1.0",
			Accepted:          false,
			RenewalRequired:   true,
			DataRetentionDays: 30, // Default 30 days
		}
		common.Respond(w, r, http.StatusOK, response, "No privacy consent found, returning defaults")
		return
	}

	common.Respond(w, r, http.StatusOK, newPrivacyConsentResponse(consent), "Privacy consent retrieved successfully")
}

// updateStudentPrivacyConsent handles updating a student's privacy consent
func (rs *Resource) updateStudentPrivacyConsent(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid student ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	req := &PrivacyConsentRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Check if student exists
	student, err := rs.StudentRepo.FindByID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if user has permission to update this student's data
	// Admin users have full access
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	isAdmin := hasAdminPermissions(userPermissions)

	if !isAdmin {
		// Check if user is a staff member who supervises the student's group
		if student.GroupID != nil {
			// Check if current user supervises this group
			staff, err := rs.UserContextService.GetCurrentStaff(r.Context())
			if err != nil || staff == nil {
				if err := render.Render(w, r, ErrorForbidden(errors.New("insufficient permissions to update this student's data"))); err != nil {
					log.Printf("Error rendering error response: %v", err)
				}
				return
			}

			// Check if staff supervises the student's group
			educationGroups, err := rs.UserContextService.GetMyGroups(r.Context())
			if err != nil {
				if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
					log.Printf("Error rendering error response: %v", err)
				}
				return
			}

			hasAccess := false
			for _, group := range educationGroups {
				if group.ID == *student.GroupID {
					hasAccess = true
					break
				}
			}

			if !hasAccess {
				if err := render.Render(w, r, ErrorForbidden(errors.New("you do not supervise this student's group"))); err != nil {
					log.Printf("Error rendering error response: %v", err)
				}
				return
			}
		}
	}

	// Get existing consents
	consents, err := rs.PrivacyConsentRepo.FindByStudentID(r.Context(), id)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Find the most recent consent for this policy version
	var consent *users.PrivacyConsent
	for _, c := range consents {
		if c.PolicyVersion == req.PolicyVersion && (consent == nil || c.CreatedAt.After(consent.CreatedAt)) {
			consent = c
		}
	}

	if consent == nil {
		// Create new consent
		consent = &users.PrivacyConsent{
			StudentID: student.ID,
		}
	}

	// Update consent fields
	consent.PolicyVersion = req.PolicyVersion
	consent.Accepted = req.Accepted
	consent.DurationDays = req.DurationDays
	consent.DataRetentionDays = req.DataRetentionDays
	consent.Details = req.Details

	// If accepting, set accepted timestamp
	if req.Accepted && consent.AcceptedAt == nil {
		now := time.Now()
		consent.AcceptedAt = &now
	}

	// Validate consent
	if err := consent.Validate(); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Save consent
	if consent.ID == 0 {
		err = rs.PrivacyConsentRepo.Create(r.Context(), consent)
	} else {
		err = rs.PrivacyConsentRepo.Update(r.Context(), consent)
	}

	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newPrivacyConsentResponse(consent), "Privacy consent updated successfully")
}

// getStudentCurrentVisit handles getting a student's current visit
func (rs *Resource) getStudentCurrentVisit(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid student ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get current visit
	currentVisit, err := rs.ActiveService.GetStudentCurrentVisit(r.Context(), studentID)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	if currentVisit == nil {
		common.Respond(w, r, http.StatusOK, nil, "Student has no current visit")
		return
	}

	common.Respond(w, r, http.StatusOK, currentVisit, "Current visit retrieved successfully")
}

// getStudentVisitHistory handles getting a student's visit history for today
func (rs *Resource) getStudentVisitHistory(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid student ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get all visits for this student
	visits, err := rs.ActiveService.FindVisitsByStudentID(r.Context(), studentID)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Filter to today's visits only
	today := time.Now().Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)

	var todaysVisits []*active.Visit
	for _, visit := range visits {
		if visit.EntryTime.After(today) && visit.EntryTime.Before(tomorrow) {
			todaysVisits = append(todaysVisits, visit)
		}
	}

	common.Respond(w, r, http.StatusOK, todaysVisits, "Visit history retrieved successfully")
}
