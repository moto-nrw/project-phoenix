package students

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
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
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// Constants for date formats (using shared error message from common package)
const (
	dateFormatYYYYMMDD = "2006-01-02"
)

// Use shared constant from common package
var errRenderingErrorResponse = common.LogRenderError

// renderError writes an error response to the HTTP response writer
// Logs rendering errors but doesn't propagate them (already in error state)
func renderError(w http.ResponseWriter, r *http.Request, errorResponse render.Renderer) {
	if err := render.Render(w, r, errorResponse); err != nil {
		log.Printf(errRenderingErrorResponse, err)
	}
}

// populatePersonAndGuardianData fills the response with person and guardian information
// based on access level permissions
func populatePersonAndGuardianData(response *StudentResponse, person *users.Person, student *users.Student, group *education.Group, hasFullAccess bool) {
	if person != nil {
		response.FirstName = person.FirstName
		response.LastName = person.LastName
		// Format birthday as YYYY-MM-DD string if available
		if person.Birthday != nil {
			response.Birthday = person.Birthday.Format(dateFormatYYYYMMDD)
		}
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
}

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
	Birthday          string                 `json:"birthday,omitempty"` // Date in YYYY-MM-DD format
	SchoolClass       string                 `json:"school_class"`
	Location          string                 `json:"current_location"`
	LocationSince     *time.Time             `json:"location_since,omitempty"` // When student entered current location
	GuardianName      string                 `json:"guardian_name,omitempty"`
	GuardianContact   string                 `json:"guardian_contact,omitempty"`
	GuardianEmail     string                 `json:"guardian_email,omitempty"`
	GuardianPhone     string                 `json:"guardian_phone,omitempty"`
	GroupID           int64                  `json:"group_id,omitempty"`
	GroupName         string                 `json:"group_name,omitempty"`
	ScheduledCheckout *ScheduledCheckoutInfo `json:"scheduled_checkout,omitempty"`
	ExtraInfo         string                 `json:"extra_info,omitempty"`
	HealthInfo        string                 `json:"health_info,omitempty"`
	SupervisorNotes   string                 `json:"supervisor_notes,omitempty"`
	PickupStatus      string                 `json:"pickup_status,omitempty"`
	Bus               bool                   `json:"bus"`
	Sick              bool                   `json:"sick"`
	SickSince         *time.Time             `json:"sick_since,omitempty"`
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
	TagID     string `json:"tag_id,omitempty"`   // RFID tag ID (optional)
	Birthday  string `json:"birthday,omitempty"` // Date in YYYY-MM-DD format

	// Student-specific details (required)
	SchoolClass string `json:"school_class"`

	// Legacy guardian fields (optional - use guardian_profiles system instead)
	GuardianName    string `json:"guardian_name,omitempty"`
	GuardianContact string `json:"guardian_contact,omitempty"`
	GuardianEmail   string `json:"guardian_email,omitempty"`
	GuardianPhone   string `json:"guardian_phone,omitempty"`

	// Optional fields
	GroupID         *int64  `json:"group_id,omitempty"`
	ExtraInfo       *string `json:"extra_info,omitempty"`       // Extra information visible to supervisors
	HealthInfo      *string `json:"health_info,omitempty"`      // Static health and medical information
	SupervisorNotes *string `json:"supervisor_notes,omitempty"` // Notes from supervisors
	PickupStatus    *string `json:"pickup_status,omitempty"`    // How the child gets home
	Bus             *bool   `json:"bus,omitempty"`              // Administrative permission flag (Buskind)
}

// UpdateStudentRequest represents a student update request
type UpdateStudentRequest struct {
	// Person details (optional for update)
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Birthday  *string `json:"birthday,omitempty"` // Date in YYYY-MM-DD format
	TagID     *string `json:"tag_id,omitempty"`

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
	PickupStatus    *string `json:"pickup_status,omitempty"`    // How the child gets home
	Bus             *bool   `json:"bus,omitempty"`              // Administrative permission flag (Buskind)
	Sick            *bool   `json:"sick,omitempty"`             // true = currently sick
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
func (req *StudentRequest) Bind(_ *http.Request) error {
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

	// Guardian fields are now optional (legacy fields - use guardian_profiles system instead)
	// No validation required for guardian fields

	return nil
}

// Bind validates the update student request
func (req *UpdateStudentRequest) Bind(_ *http.Request) error {
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
	// Guardian fields are deprecated - allow empty strings for clearing
	// Empty strings will be converted to nil in the update handler
	return nil
}

// Bind validates the RFID assignment request
func (req *RFIDAssignmentRequest) Bind(_ *http.Request) error {
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

// resolveScheduledCheckout looks up scheduled checkout info and scheduler name
func resolveScheduledCheckout(ctx context.Context, studentID int64, activeService activeService.Service, personService userService.PersonService) *ScheduledCheckoutInfo {
	pendingCheckout, err := activeService.GetPendingScheduledCheckout(ctx, studentID)
	if err != nil || pendingCheckout == nil {
		return nil
	}

	scheduledByName := resolveSchedulerName(ctx, pendingCheckout.ScheduledBy, personService)

	return &ScheduledCheckoutInfo{
		ID:           pendingCheckout.ID,
		ScheduledFor: pendingCheckout.ScheduledFor,
		Reason:       pendingCheckout.Reason,
		ScheduledBy:  scheduledByName,
	}
}

// resolveSchedulerName gets the name of the staff member who scheduled a checkout
func resolveSchedulerName(ctx context.Context, staffID int64, personService userService.PersonService) string {
	staff, err := personService.StaffRepository().FindByID(ctx, staffID)
	if err != nil || staff == nil {
		return "Unknown"
	}
	person, err := personService.Get(ctx, staff.PersonID)
	if err != nil || person == nil {
		return "Unknown"
	}
	return person.FirstName + " " + person.LastName
}

// populatePublicStudentFields sets fields visible to all authenticated staff
func populatePublicStudentFields(response *StudentResponse, student *users.Student) {
	if student.HealthInfo != nil {
		response.HealthInfo = *student.HealthInfo
	}
	if student.Bus != nil {
		response.Bus = *student.Bus
	}
	if student.PickupStatus != nil {
		response.PickupStatus = *student.PickupStatus
	}
}

// populateSensitiveStudentFields sets fields visible only to supervisors/admins
func populateSensitiveStudentFields(response *StudentResponse, student *users.Student) {
	if student.ExtraInfo != nil && *student.ExtraInfo != "" {
		response.ExtraInfo = *student.ExtraInfo
	}
	if student.SupervisorNotes != nil {
		response.SupervisorNotes = *student.SupervisorNotes
	}
	if student.Sick != nil {
		response.Sick = *student.Sick
	}
	if student.SickSince != nil {
		response.SickSince = student.SickSince
	}
}

// StudentResponseOpts groups parameters for creating a student response to reduce function parameter count
type StudentResponseOpts struct {
	Student          *users.Student
	Person           *users.Person
	Group            *education.Group
	HasFullAccess    bool
	LocationOverride *string
}

// StudentResponseServices groups service dependencies for student response creation
type StudentResponseServices struct {
	ActiveService activeService.Service
	PersonService userService.PersonService
}

// newStudentResponse creates a student response from a student and person model
// hasFullAccess determines whether to include detailed location data and supervisor-only information (like extra info)
// Deprecated: Use newStudentResponseWithOpts instead for cleaner API
func newStudentResponse(ctx context.Context, student *users.Student, person *users.Person, group *education.Group, hasFullAccess bool, activeSvc activeService.Service, personSvc userService.PersonService, locationOverride *string) StudentResponse {
	return newStudentResponseWithOpts(ctx, StudentResponseOpts{
		Student:          student,
		Person:           person,
		Group:            group,
		HasFullAccess:    hasFullAccess,
		LocationOverride: locationOverride,
	}, StudentResponseServices{
		ActiveService: activeSvc,
		PersonService: personSvc,
	})
}

// newStudentResponseWithOpts creates a student response using options structs
func newStudentResponseWithOpts(ctx context.Context, opts StudentResponseOpts, services StudentResponseServices) StudentResponse {
	student := opts.Student
	person := opts.Person
	group := opts.Group
	hasFullAccess := opts.HasFullAccess
	locationOverride := opts.LocationOverride
	response := StudentResponse{
		ID:          student.ID,
		PersonID:    student.PersonID,
		SchoolClass: student.SchoolClass,
		CreatedAt:   student.CreatedAt,
		UpdatedAt:   student.UpdatedAt,
	}

	// Include legacy guardian name if available
	if student.GuardianName != nil {
		response.GuardianName = *student.GuardianName
	}

	// Only include guardian contact info for users with full access
	if hasFullAccess && student.GuardianContact != nil {
		response.GuardianContact = *student.GuardianContact
	}

	// Resolve location
	if locationOverride != nil {
		response.Location = *locationOverride
	} else {
		locationInfo := resolveStudentLocationWithTime(ctx, student.ID, hasFullAccess, services.ActiveService)
		response.Location = locationInfo.Location
		response.LocationSince = locationInfo.Since
	}

	// Check for pending scheduled checkout
	response.ScheduledCheckout = resolveScheduledCheckout(ctx, student.ID, services.ActiveService, services.PersonService)

	populatePersonAndGuardianData(&response, person, student, group, hasFullAccess)
	populatePublicStudentFields(&response, student)

	if hasFullAccess {
		populateSensitiveStudentFields(&response, student)
	}

	return response
}

// populateSnapshotSensitiveFields sets sensitive fields for the snapshot version
// Note: This differs from populateSensitiveStudentFields by including HealthInfo
func populateSnapshotSensitiveFields(response *StudentResponse, student *users.Student) {
	if student.ExtraInfo != nil && *student.ExtraInfo != "" {
		response.ExtraInfo = *student.ExtraInfo
	}
	if student.HealthInfo != nil {
		response.HealthInfo = *student.HealthInfo
	}
	if student.SupervisorNotes != nil {
		response.SupervisorNotes = *student.SupervisorNotes
	}
	if student.Sick != nil {
		response.Sick = *student.Sick
	}
	if student.SickSince != nil {
		response.SickSince = student.SickSince
	}
}

// populateSnapshotPublicFields sets fields visible to all staff in snapshot version
func populateSnapshotPublicFields(response *StudentResponse, student *users.Student) {
	if student.Bus != nil {
		response.Bus = *student.Bus
	}
	if student.PickupStatus != nil {
		response.PickupStatus = *student.PickupStatus
	}
}

// resolveScheduledCheckoutFromSnapshot converts snapshot checkout to info struct
func resolveScheduledCheckoutFromSnapshot(snapshot *common.StudentDataSnapshot, studentID int64) *ScheduledCheckoutInfo {
	pendingCheckout := snapshot.GetScheduledCheckout(studentID)
	if pendingCheckout == nil {
		return nil
	}

	return &ScheduledCheckoutInfo{
		ID:           pendingCheckout.ID,
		ScheduledFor: pendingCheckout.ScheduledFor,
		Reason:       pendingCheckout.Reason,
		ScheduledBy:  "System",
	}
}

// newStudentResponseFromSnapshot creates a student response using pre-loaded snapshot data
// This eliminates N+1 queries by using cached person, group, and scheduled checkout data
func newStudentResponseFromSnapshot(_ context.Context, student *users.Student, person *users.Person, group *education.Group, hasFullAccess bool, snapshot *common.StudentDataSnapshot) StudentResponse {
	response := StudentResponse{
		ID:          student.ID,
		PersonID:    student.PersonID,
		SchoolClass: student.SchoolClass,
		CreatedAt:   student.CreatedAt,
		UpdatedAt:   student.UpdatedAt,
	}

	if student.GuardianName != nil {
		response.GuardianName = *student.GuardianName
	}

	if hasFullAccess && student.GuardianContact != nil {
		response.GuardianContact = *student.GuardianContact
	}

	locationInfo := snapshot.ResolveLocationWithTime(student.ID, hasFullAccess)
	response.Location = locationInfo.Location
	response.LocationSince = locationInfo.Since

	response.ScheduledCheckout = resolveScheduledCheckoutFromSnapshot(snapshot, student.ID)

	populatePersonAndGuardianData(&response, person, student, group, hasFullAccess)
	populateSnapshotPublicFields(&response, student)

	if hasFullAccess {
		populateSnapshotSensitiveFields(&response, student)
	}

	return response
}

// presentOrTransit returns the appropriate location for a checked-in student
// without a specific room assignment, based on access level.
func presentOrTransit(hasFullAccess bool) common.StudentLocationInfo {
	if hasFullAccess {
		return common.StudentLocationInfo{Location: "Unterwegs"}
	}
	return common.StudentLocationInfo{Location: "Anwesend"}
}

// absentInfo returns the "Abwesend" location, optionally with checkout time for full access users.
func absentInfo(hasFullAccess bool, checkOutTime *time.Time) common.StudentLocationInfo {
	if hasFullAccess && checkOutTime != nil {
		return common.StudentLocationInfo{Location: "Abwesend", Since: checkOutTime}
	}
	return common.StudentLocationInfo{Location: "Abwesend"}
}

func resolveStudentLocationWithTime(ctx context.Context, studentID int64, hasFullAccess bool, activeService activeService.Service) common.StudentLocationInfo {
	attendanceStatus, err := activeService.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil || attendanceStatus == nil {
		return common.StudentLocationInfo{Location: "Abwesend"}
	}

	// Handle non-checked-in states (checked_out or other)
	if attendanceStatus.Status != "checked_in" {
		return absentInfo(hasFullAccess, attendanceStatus.CheckOutTime)
	}

	// Student is checked in - get current visit to check room assignment
	currentVisit, err := activeService.GetStudentCurrentVisit(ctx, studentID)
	if err != nil || currentVisit == nil || currentVisit.ActiveGroupID <= 0 {
		return presentOrTransit(hasFullAccess)
	}

	activeGroup, err := activeService.GetActiveGroup(ctx, currentVisit.ActiveGroupID)
	if err != nil || activeGroup == nil {
		return presentOrTransit(hasFullAccess)
	}

	// Include room name for all authenticated staff (needed for supervised room checkout)
	if activeGroup.Room != nil && activeGroup.Room.Name != "" {
		return common.StudentLocationInfo{
			Location: fmt.Sprintf("Anwesend - %s", activeGroup.Room.Name),
			Since:    &currentVisit.EntryTime,
		}
	}

	return presentOrTransit(hasFullAccess)
}

// listStudents handles listing all students with staff-based filtering
func (rs *Resource) listStudents(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters and determine access
	params := parseStudentListParams(r)
	accessCtx := rs.determineStudentAccess(r)

	// Fetch students based on parameters
	students, totalCount, err := rs.fetchStudentsForList(r, params)
	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	// Bulk load all related data
	studentIDs, personIDs, groupIDs := collectIDsFromStudents(students)
	dataSnapshot, err := common.LoadStudentDataSnapshot(
		r.Context(),
		rs.PersonService,
		rs.EducationService,
		rs.ActiveService,
		studentIDs,
		personIDs,
		groupIDs,
	)
	if err != nil {
		log.Printf("Failed to load student data snapshot: %v", err)
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build and filter responses
	responses := rs.buildStudentResponses(r.Context(), students, params, accessCtx, dataSnapshot)

	// Apply in-memory pagination if person-based filters were used
	if params.hasPersonFilters() {
		responses, totalCount = applyInMemoryPagination(responses, params.page, params.pageSize)
	}

	common.RespondWithPagination(w, r, http.StatusOK, responses, params.page, params.pageSize, totalCount, "Students retrieved successfully")
}

// fetchStudentsForList fetches students based on the provided parameters
func (rs *Resource) fetchStudentsForList(r *http.Request, params *studentListParams) ([]*users.Student, int, error) {
	ctx := r.Context()

	// If specific group filter requested
	if params.groupID > 0 {
		students, err := rs.StudentRepo.FindByGroupIDs(ctx, []int64{params.groupID})
		if err != nil {
			return nil, 0, err
		}
		return students, len(students), nil
	}

	// No specific group filter - get all students
	queryOptions := params.buildQueryOptions()

	// Get count for pagination
	countOptions := base.NewQueryOptions()
	countOptions.Filter = params.buildCountFilter()
	totalCount, err := rs.StudentRepo.CountWithOptions(ctx, countOptions)
	if err != nil {
		return nil, 0, err
	}

	// Get students
	students, err := rs.StudentRepo.ListWithOptions(ctx, queryOptions)
	if err != nil {
		return nil, 0, err
	}

	return students, totalCount, nil
}

// buildStudentResponses builds filtered student responses
func (rs *Resource) buildStudentResponses(ctx context.Context, students []*users.Student, params *studentListParams, accessCtx *studentAccessContext, dataSnapshot *common.StudentDataSnapshot) []StudentResponse {
	responses := make([]StudentResponse, 0, len(students))

	for _, student := range students {
		response := rs.buildSingleStudentResponse(ctx, student, params, accessCtx, dataSnapshot)
		if response != nil {
			responses = append(responses, *response)
		}
	}

	return responses
}

// buildSingleStudentResponse builds a response for a single student, returning nil if filtered out
func (rs *Resource) buildSingleStudentResponse(ctx context.Context, student *users.Student, params *studentListParams, accessCtx *studentAccessContext, dataSnapshot *common.StudentDataSnapshot) *StudentResponse {
	hasFullAccess := accessCtx.hasFullAccessToStudent(student)

	// Get person data from snapshot
	person := dataSnapshot.GetPerson(student.PersonID)
	if person == nil {
		return nil
	}

	// Apply filters
	if !matchesSearchFilter(person, student.ID, params.search) {
		return nil
	}
	if !matchesNameFilters(person, params.firstName, params.lastName) {
		return nil
	}

	// Get group data from snapshot
	var group *education.Group
	if student.GroupID != nil {
		group = dataSnapshot.GetGroup(*student.GroupID)
	}

	// Build response
	studentResponse := newStudentResponseFromSnapshot(ctx, student, person, group, hasFullAccess, dataSnapshot)

	// Apply location filter
	if !matchesLocationFilter(params.location, studentResponse.Location, hasFullAccess) {
		return nil
	}

	return &studentResponse
}

// teacherToSupervisorContact converts a teacher to a supervisor contact if valid
func teacherToSupervisorContact(teacher *users.Teacher) *SupervisorContact {
	if teacher == nil || teacher.Staff == nil || teacher.Staff.Person == nil {
		return nil
	}

	supervisor := &SupervisorContact{
		ID:        teacher.ID,
		FirstName: teacher.Staff.Person.FirstName,
		LastName:  teacher.Staff.Person.LastName,
		Role:      "teacher",
	}

	if teacher.Staff.Person.Account != nil {
		supervisor.Email = teacher.Staff.Person.Account.Email
	}

	return supervisor
}

// buildSupervisorContacts creates supervisor contact list from group teachers
func (rs *Resource) buildSupervisorContacts(ctx context.Context, groupID int64) []SupervisorContact {
	teachers, err := rs.EducationService.GetGroupTeachers(ctx, groupID)
	if err != nil {
		return nil
	}

	supervisors := make([]SupervisorContact, 0, len(teachers))
	for _, teacher := range teachers {
		if supervisor := teacherToSupervisorContact(teacher); supervisor != nil {
			supervisors = append(supervisors, *supervisor)
		}
	}
	return supervisors
}

// getStudent handles getting a student by ID
func (rs *Resource) getStudent(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	person, ok := rs.getPersonForStudent(w, r, student)
	if !ok {
		return
	}

	group := rs.getStudentGroup(r.Context(), student)
	hasFullAccess := rs.checkStudentFullAccess(r, student)

	response := StudentDetailResponse{
		StudentResponse: newStudentResponse(r.Context(), student, person, group, hasFullAccess, rs.ActiveService, rs.PersonService, nil),
		HasFullAccess:   hasFullAccess,
	}

	// Add supervisor contacts for users without full access
	if !hasFullAccess && group != nil {
		response.GroupSupervisors = rs.buildSupervisorContacts(r.Context(), group.ID)
	}

	common.Respond(w, r, http.StatusOK, response, "Student retrieved successfully")
}

// Helper functions for createStudent to reduce cognitive complexity

// createPersonFromStudentRequest creates a Person object from a StudentRequest
func createPersonFromStudentRequest(req *StudentRequest) (*users.Person, error) {
	person := &users.Person{
		FirstName: req.FirstName,
		LastName:  req.LastName,
	}

	// Set optional TagID if provided
	if req.TagID != "" {
		tagID := req.TagID
		person.TagID = &tagID
	}

	// Set optional Birthday if provided
	if req.Birthday != "" {
		parsedBirthday, err := time.Parse(dateFormatYYYYMMDD, req.Birthday)
		if err != nil {
			return nil, fmt.Errorf("invalid birthday format, expected YYYY-MM-DD: %w", err)
		}
		person.Birthday = &parsedBirthday
	}

	return person, nil
}

// createStudentFromRequest creates a Student object from a StudentRequest and personID
func createStudentFromRequest(req *StudentRequest, personID int64) *users.Student {
	student := &users.Student{
		PersonID:    personID,
		SchoolClass: req.SchoolClass,
	}

	// Set optional legacy guardian fields if provided
	if req.GuardianName != "" {
		name := req.GuardianName
		student.GuardianName = &name
	}
	if req.GuardianContact != "" {
		contact := req.GuardianContact
		student.GuardianContact = &contact
	}
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
	if req.HealthInfo != nil {
		student.HealthInfo = req.HealthInfo
	}
	if req.SupervisorNotes != nil {
		student.SupervisorNotes = req.SupervisorNotes
	}
	if req.PickupStatus != nil {
		student.PickupStatus = req.PickupStatus
	}
	if req.Bus != nil {
		student.Bus = req.Bus
	}

	return student
}

// createStudent handles creating a new student with their person record
func (rs *Resource) createStudent(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &StudentRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Create person from request
	person, err := createPersonFromStudentRequest(req)
	if err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Create person - validation occurs at the model layer
	if err := rs.PersonService.Create(r.Context(), person); err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	// Create student with the person ID
	student := createStudentFromRequest(req, person.ID)

	// Create student
	if err := rs.StudentRepo.Create(r.Context(), student); err != nil {
		rs.cleanupPersonAfterStudentFailure(r.Context(), person.ID)
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	// Get group data if student has a group
	group := rs.fetchStudentGroup(r.Context(), student.GroupID)

	// Admin users creating students can see full data including detailed location
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	hasFullAccess := hasAdminPermissions(userPermissions)

	// Return the created student with person data
	common.Respond(w, r, http.StatusCreated, newStudentResponse(r.Context(), student, person, group, hasFullAccess, rs.ActiveService, rs.PersonService, nil), "Student created successfully")
}

// cleanupPersonAfterStudentFailure removes the person record if student creation fails
func (rs *Resource) cleanupPersonAfterStudentFailure(ctx context.Context, personID int64) {
	if err := rs.PersonService.Delete(ctx, personID); err != nil {
		log.Printf("Error cleaning up person after failed student creation: %v", err)
	}
}

// fetchStudentGroup retrieves group data if the student has an assigned group
func (rs *Resource) fetchStudentGroup(ctx context.Context, groupID *int64) *education.Group {
	if groupID == nil {
		return nil
	}
	group, err := rs.EducationService.GetGroup(ctx, *groupID)
	if err != nil {
		return nil
	}
	return group
}

// personUpdateResult contains the result of updating person fields
type personUpdateResult struct {
	updated bool
	err     error
}

// applyPersonUpdates applies person field changes from the request
// Returns whether any fields were updated and any error encountered
func applyPersonUpdates(req *UpdateStudentRequest, person *users.Person) personUpdateResult {
	result := personUpdateResult{}

	if req.FirstName != nil {
		person.FirstName = *req.FirstName
		result.updated = true
	}
	if req.LastName != nil {
		person.LastName = *req.LastName
		result.updated = true
	}
	if req.Birthday != nil {
		if *req.Birthday != "" {
			parsedBirthday, err := time.Parse(dateFormatYYYYMMDD, *req.Birthday)
			if err != nil {
				result.err = fmt.Errorf("invalid birthday format, expected YYYY-MM-DD: %w", err)
				return result
			}
			person.Birthday = &parsedBirthday
		} else {
			person.Birthday = nil
		}
		result.updated = true
	}
	if req.TagID != nil {
		if *req.TagID != "" {
			person.TagID = req.TagID
		} else {
			person.TagID = nil
		}
		result.updated = true
	}

	return result
}

// applyStudentFieldUpdates applies student field changes from the request
func applyStudentFieldUpdates(req *UpdateStudentRequest, student *users.Student) {
	if req.SchoolClass != nil {
		student.SchoolClass = *req.SchoolClass
	}
	applyGuardianUpdates(req, student)
	applyOptionalStudentFields(req, student)
	applySickStatus(req, student)
}

// applyGuardianUpdates handles legacy guardian field updates
func applyGuardianUpdates(req *UpdateStudentRequest, student *users.Student) {
	if req.GuardianName != nil {
		trimmed := strings.TrimSpace(*req.GuardianName)
		if trimmed == "" {
			student.GuardianName = nil
		} else {
			student.GuardianName = &trimmed
		}
	}
	if req.GuardianContact != nil {
		trimmed := strings.TrimSpace(*req.GuardianContact)
		if trimmed == "" {
			student.GuardianContact = nil
		} else {
			student.GuardianContact = &trimmed
		}
	}
	if req.GuardianEmail != nil {
		student.GuardianEmail = req.GuardianEmail
	}
	if req.GuardianPhone != nil {
		student.GuardianPhone = req.GuardianPhone
	}
}

// applyOptionalStudentFields applies optional fields like GroupID, ExtraInfo, etc.
func applyOptionalStudentFields(req *UpdateStudentRequest, student *users.Student) {
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
	if req.PickupStatus != nil {
		student.PickupStatus = req.PickupStatus
	}
	if req.Bus != nil {
		student.Bus = req.Bus
	}
}

// applySickStatus handles sick status updates with SickSince timestamp logic
func applySickStatus(req *UpdateStudentRequest, student *users.Student) {
	if req.Sick == nil {
		return
	}
	student.Sick = req.Sick
	if *req.Sick {
		if student.SickSince == nil {
			now := time.Now()
			student.SickSince = &now
		}
	} else {
		student.SickSince = nil
	}
}

// updateStudent handles updating an existing student
func (rs *Resource) updateStudent(w http.ResponseWriter, r *http.Request) {
	// Parse ID and get student
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	// Parse request
	req := &UpdateStudentRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get existing person
	person, ok := rs.getPersonForStudent(w, r, student)
	if !ok {
		return
	}

	// Centralized permission check for updating student data
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	authorized, authErr := canUpdateStudent(r.Context(), userPermissions, student, rs.UserContextService)
	if !authorized {
		renderError(w, r, ErrorForbidden(authErr))
		return
	}

	// Track whether the user is admin or group supervisor
	isAdmin := hasAdminPermissions(userPermissions)
	isGroupSupervisor := !isAdmin // If not admin but authorized, must be group supervisor

	// Update person fields using helper function
	personResult := applyPersonUpdates(req, person)
	if personResult.err != nil {
		renderError(w, r, ErrorInvalidRequest(personResult.err))
		return
	}

	// Persist person updates if any fields changed
	if personResult.updated {
		if err := rs.PersonService.Update(r.Context(), person); err != nil {
			renderError(w, r, ErrorInternalServer(err))
			return
		}
	}

	// Update student fields using helper function
	applyStudentFieldUpdates(req, student)

	// Update student
	if err := rs.StudentRepo.Update(r.Context(), student); err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	// Get updated student with person data
	updatedStudent, err := rs.StudentRepo.FindByID(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	// Get group data if student has a group
	group := rs.getStudentGroup(r.Context(), updatedStudent)

	// Admin users and group supervisors can see full data including detailed location
	// Explicitly verify access level based on the checks performed above
	hasFullAccess := isAdmin || isGroupSupervisor // Explicitly check for admin or group supervisor

	// Return the updated student with person data
	common.Respond(w, r, http.StatusOK, newStudentResponse(r.Context(), updatedStudent, person, group, hasFullAccess, rs.ActiveService, rs.PersonService, nil), "Student updated successfully")
}

// deleteStudent handles deleting a student and their associated person record
func (rs *Resource) deleteStudent(w http.ResponseWriter, r *http.Request) {
	// Parse ID and get student
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	// Check if user has permission to delete this student
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	authorized, authErr := canDeleteStudent(r.Context(), userPermissions, student, rs.UserContextService)
	if !authorized {
		renderError(w, r, ErrorForbidden(authErr))
		return
	}

	// Delete the student first
	if err := rs.StudentRepo.Delete(r.Context(), student.ID); err != nil {
		renderError(w, r, ErrorInternalServer(err))
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
	// Parse ID and get student
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	// Get person details
	person, ok := rs.getPersonForStudent(w, r, student)
	if !ok {
		return
	}

	// Get group details if student has a group
	group := rs.getStudentGroup(r.Context(), student)

	// Determine if user has full access to student location details
	hasFullAccess := rs.checkStudentFullAccess(r, student)

	// Build student response
	response := newStudentResponse(r.Context(), student, person, group, hasFullAccess, rs.ActiveService, rs.PersonService, nil)

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
// checkGroupRoomAccessAuthorization verifies if the user can view student room status
// Returns an error if unauthorized, nil if authorized
func (rs *Resource) checkGroupRoomAccessAuthorization(r *http.Request, studentGroupID int64) error {
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	if hasAdminPermissions(userPermissions) {
		return nil
	}

	staff, err := rs.UserContextService.GetCurrentStaff(r.Context())
	if err != nil || staff == nil {
		return errors.New("unauthorized to view student room status")
	}

	educationGroups, err := rs.UserContextService.GetMyGroups(r.Context())
	if err != nil {
		return errors.New("you do not supervise this student's group")
	}

	for _, supervGroup := range educationGroups {
		if supervGroup.ID == studentGroupID {
			return nil
		}
	}

	return errors.New("you do not supervise this student's group")
}

// buildGroupRoomResponse constructs the response for in-group-room check
func buildGroupRoomResponse(activeGroup *active.Group, group *education.Group) map[string]interface{} {
	inGroupRoom := activeGroup.RoomID == *group.RoomID
	response := map[string]interface{}{
		"in_group_room":   inGroupRoom,
		"group_room_id":   *group.RoomID,
		"current_room_id": activeGroup.RoomID,
	}
	if group.Room != nil {
		response["group_room_name"] = group.Room.Name
	}
	return response
}

func (rs *Resource) getStudentInGroupRoom(w http.ResponseWriter, r *http.Request) {
	// Parse ID and get student
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
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
		renderError(w, r, ErrorInternalServer(errors.New("failed to get student's group")))
		return
	}

	// Check authorization - only group supervisors can see this information
	if authErr := rs.checkGroupRoomAccessAuthorization(r, *student.GroupID); authErr != nil {
		renderError(w, r, ErrorForbidden(authErr))
		return
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
		common.Respond(w, r, http.StatusOK, map[string]interface{}{
			"in_group_room": false,
			"reason":        "no_active_visit",
		}, "Student has no active visit")
		return
	}

	// Get the active group to check its room
	activeGroup, err := rs.ActiveService.GetActiveGroup(r.Context(), visit.ActiveGroupID)
	if err != nil {
		renderError(w, r, ErrorInternalServer(errors.New("failed to get active group")))
		return
	}

	// Build and return the response
	response := buildGroupRoomResponse(activeGroup, group)
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

// parseAndGetStudent parses the student ID from the URL and fetches the student
// Returns the student and true if successful, or renders an error and returns nil, false
func (rs *Resource) parseAndGetStudent(w http.ResponseWriter, r *http.Request) (*users.Student, bool) {
	id, err := common.ParseID(r)
	if err != nil {
		renderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return nil, false
	}

	student, err := rs.StudentRepo.FindByID(r.Context(), id)
	if err != nil {
		renderError(w, r, ErrorNotFound(errors.New("student not found")))
		return nil, false
	}

	return student, true
}

// getPersonForStudent fetches the person data for a student
// Returns the person and true if successful, or renders an error and returns nil, false
func (rs *Resource) getPersonForStudent(w http.ResponseWriter, r *http.Request, student *users.Student) (*users.Person, bool) {
	person, err := rs.PersonService.Get(r.Context(), student.PersonID)
	if err != nil {
		renderError(w, r, ErrorInternalServer(errors.New("failed to get person data for student")))
		return nil, false
	}
	return person, true
}

// getStudentGroup fetches the group for a student if they have one assigned
func (rs *Resource) getStudentGroup(ctx context.Context, student *users.Student) *education.Group {
	if student.GroupID == nil {
		return nil
	}
	group, err := rs.EducationService.GetGroup(ctx, *student.GroupID)
	if err != nil {
		return nil
	}
	return group
}

// checkStudentFullAccess determines if the current user has full access to a student's data
// Returns true if user is admin or supervises the student's group
func (rs *Resource) checkStudentFullAccess(r *http.Request, student *users.Student) bool {
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	if hasAdminPermissions(userPermissions) {
		return true
	}

	if student.GroupID == nil {
		return false
	}

	educationGroups, err := rs.UserContextService.GetMyGroups(r.Context())
	if err != nil {
		return false
	}

	for _, group := range educationGroups {
		if group.ID == *student.GroupID {
			return true
		}
	}

	return false
}

// checkDeviceAuth verifies device authentication and returns the device
// Returns the device and true if successful, or renders an error and returns nil, false
func (rs *Resource) checkDeviceAuth(w http.ResponseWriter, r *http.Request) (*iot.Device, bool) {
	deviceCtx := device.DeviceFromCtx(r.Context())
	if deviceCtx == nil {
		renderError(w, r, ErrorUnauthorized(errors.New("device authentication required")))
		return nil, false
	}
	return deviceCtx, true
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
	if isGroupSupervisor(ctx, *student.GroupID, userContextService) {
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
func (req *PrivacyConsentRequest) Bind(_ *http.Request) error {
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
	deviceCtx, ok := rs.checkDeviceAuth(w, r)
	if !ok {
		return
	}

	// Parse ID and get student
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	// Parse request
	req := &RFIDAssignmentRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get person details for the student
	person, ok := rs.getPersonForStudent(w, r, student)
	if !ok {
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
		renderError(w, r, ErrorInternalServer(err))
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
		deviceCtx.DeviceID, student.ID, req.RFIDTag, previousTag)

	common.Respond(w, r, http.StatusOK, response, response.Message)
}

// unassignRFIDTag handles removing an RFID tag from a student (device-authenticated endpoint)
func (rs *Resource) unassignRFIDTag(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	deviceCtx, ok := rs.checkDeviceAuth(w, r)
	if !ok {
		return
	}

	// Parse ID and get student
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	// Get person details for the student
	person, ok := rs.getPersonForStudent(w, r, student)
	if !ok {
		return
	}

	// Check if student has an RFID tag assigned
	if person.TagID == nil {
		renderError(w, r, ErrorNotFound(errors.New("student has no RFID tag assigned")))
		return
	}

	// Store removed tag for response
	removedTag := *person.TagID

	// Unlink the RFID tag
	if err := rs.PersonService.UnlinkFromRFIDCard(r.Context(), person.ID); err != nil {
		renderError(w, r, ErrorInternalServer(err))
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
		deviceCtx.DeviceID, student.ID, removedTag)

	common.Respond(w, r, http.StatusOK, response, response.Message)
}

// getStudentPrivacyConsent handles getting a student's privacy consent
func (rs *Resource) getStudentPrivacyConsent(w http.ResponseWriter, r *http.Request) {
	// Parse ID and get student
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	// Check if user has permission to view this student's data
	hasFullAccess := rs.checkStudentFullAccess(r, student)
	if !hasFullAccess {
		renderError(w, r, ErrorForbidden(errors.New("insufficient permissions to access this student's data")))
		return
	}

	// Get privacy consents
	consents, err := rs.PrivacyConsentRepo.FindByStudentID(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
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
			StudentID:         student.ID,
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

// findOrCreateConsent finds existing consent for a policy version or creates a new one
func findOrCreateConsent(consents []*users.PrivacyConsent, studentID int64, policyVersion string) *users.PrivacyConsent {
	var consent *users.PrivacyConsent
	for _, c := range consents {
		if c.PolicyVersion == policyVersion && (consent == nil || c.CreatedAt.After(consent.CreatedAt)) {
			consent = c
		}
	}

	if consent == nil {
		return &users.PrivacyConsent{StudentID: studentID}
	}
	return consent
}

// applyConsentUpdates updates consent fields from the request
func applyConsentUpdates(consent *users.PrivacyConsent, req *PrivacyConsentRequest) {
	consent.PolicyVersion = req.PolicyVersion
	consent.Accepted = req.Accepted
	consent.DurationDays = req.DurationDays
	consent.DataRetentionDays = req.DataRetentionDays
	consent.Details = req.Details

	if req.Accepted && consent.AcceptedAt == nil {
		now := time.Now()
		consent.AcceptedAt = &now
	}
}

// updateStudentPrivacyConsent handles updating a student's privacy consent
func (rs *Resource) updateStudentPrivacyConsent(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	req := &PrivacyConsentRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}

	if !rs.checkStudentFullAccess(r, student) {
		renderError(w, r, ErrorForbidden(errors.New("insufficient permissions to update this student's data")))
		return
	}

	consents, err := rs.PrivacyConsentRepo.FindByStudentID(r.Context(), student.ID)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	consent := findOrCreateConsent(consents, student.ID, req.PolicyVersion)
	applyConsentUpdates(consent, req)

	if err := consent.Validate(); err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}

	if consent.ID == 0 {
		err = rs.PrivacyConsentRepo.Create(r.Context(), consent)
	} else {
		err = rs.PrivacyConsentRepo.Update(r.Context(), consent)
	}

	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newPrivacyConsentResponse(consent), "Privacy consent updated successfully")
}

// getStudentCurrentVisit handles getting a student's current visit
func (rs *Resource) getStudentCurrentVisit(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL (we only need the ID, not the full student)
	studentID, err := common.ParseID(r)
	if err != nil {
		renderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return
	}

	// Get current visit
	currentVisit, err := rs.ActiveService.GetStudentCurrentVisit(r.Context(), studentID)
	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
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
	studentID, err := common.ParseID(r)
	if err != nil {
		renderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return
	}

	// Get all visits for this student
	visits, err := rs.ActiveService.FindVisitsByStudentID(r.Context(), studentID)
	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
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
