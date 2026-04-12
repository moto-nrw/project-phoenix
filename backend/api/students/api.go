package students

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// renderError writes an error response to the HTTP response writer.
// Delegates to common.RenderError which logs 5xx root causes to slog
// and captures them to Sentry.
func renderError(w http.ResponseWriter, r *http.Request, errorResponse render.Renderer) {
	common.RenderError(w, r, errorResponse)
}

// Resource defines the students API resource
type Resource struct {
	PersonService         userService.PersonService
	StudentRepo           users.StudentRepository
	EducationService      educationService.Service
	UserContextService    userContextService.UserContextService
	ActiveService         activeService.Service
	IoTService            iotSvc.Service
	PrivacyConsentRepo    users.PrivacyConsentRepository
	PickupScheduleService scheduleService.PickupScheduleService
	SchoolRepo            platform.SchoolRepository
	SettingsService       configService.SettingsService
	AttendanceRepo        active.AttendanceRepository
	VisitRepo             active.VisitRepository
	DataAccessLogRepo     auditModels.DataAccessLogRepository
	Logger                *slog.Logger
	db                    *bun.DB
}

// ResourceConfig holds all dependencies for creating a students Resource.
// Using a config struct instead of individual parameters improves maintainability.
type ResourceConfig struct {
	PersonService         userService.PersonService
	StudentRepo           users.StudentRepository
	EducationService      educationService.Service
	UserContextService    userContextService.UserContextService
	ActiveService         activeService.Service
	IoTService            iotSvc.Service
	PrivacyConsentRepo    users.PrivacyConsentRepository
	PickupScheduleService scheduleService.PickupScheduleService
	SchoolRepo            platform.SchoolRepository
	SettingsService       configService.SettingsService
	AttendanceRepo        active.AttendanceRepository
	VisitRepo             active.VisitRepository
	DataAccessLogRepo     auditModels.DataAccessLogRepository
	Logger                *slog.Logger
	DB                    *bun.DB
}

// NewResource creates a new students resource from the provided configuration.
func NewResource(cfg ResourceConfig) *Resource {
	return &Resource{
		PersonService:         cfg.PersonService,
		StudentRepo:           cfg.StudentRepo,
		EducationService:      cfg.EducationService,
		UserContextService:    cfg.UserContextService,
		ActiveService:         cfg.ActiveService,
		IoTService:            cfg.IoTService,
		PrivacyConsentRepo:    cfg.PrivacyConsentRepo,
		PickupScheduleService: cfg.PickupScheduleService,
		SchoolRepo:            cfg.SchoolRepo,
		SettingsService:       cfg.SettingsService,
		AttendanceRepo:        cfg.AttendanceRepo,
		VisitRepo:             cfg.VisitRepo,
		DataAccessLogRepo:     cfg.DataAccessLogRepo,
		Logger:                cfg.Logger,
		db:                    cfg.DB,
	}
}

// Router returns a configured router for student endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustNewTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)
		r.Use(jwt.TenantMiddleware)
		withTx := tenant.TenantTxMiddleware(rs.db)

		// Routes requiring users:read permission
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/", rs.listStudents)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}", rs.getStudent)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/in-group-room", rs.getStudentInGroupRoom)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/current-location", rs.getStudentCurrentLocation)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/current-visit", rs.getStudentCurrentVisit)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/visit-history", rs.getStudentVisitHistory)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/attendance-history", rs.getStudentAttendanceHistory)

		// Routes requiring users:create permission
		r.With(authorize.RequiresPermission(permissions.UsersCreate), withTx).Post("/", rs.createStudent)

		// Routes requiring users:update permission
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}", rs.updateStudent)

		// Routes requiring users:delete permission
		r.With(authorize.RequiresPermission(permissions.UsersDelete), withTx).Delete("/{id}", rs.deleteStudent)

		// Privacy consent routes
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/privacy-consent", rs.getStudentPrivacyConsent)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/privacy-consent", rs.updateStudentPrivacyConsent)

		// Pickup schedule routes (full access required - checked in handlers)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/pickup-schedules", rs.getStudentPickupSchedules)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/pickup-schedules", rs.updateStudentPickupSchedules)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/pickup-exceptions", rs.createStudentPickupException)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/pickup-exceptions/{exceptionId}", rs.updateStudentPickupException)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/pickup-exceptions/{exceptionId}", rs.deleteStudentPickupException)

		// Pickup note routes (full access required - checked in handlers)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/pickup-notes", rs.createStudentPickupNote)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/pickup-notes/{noteId}", rs.updateStudentPickupNote)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/pickup-notes/{noteId}", rs.deleteStudentPickupNote)

		// Bulk pickup times endpoint (returns pickup times for multiple students)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Post("/pickup-times/bulk", rs.getBulkPickupTimes)
	})

	// Device-authenticated routes for RFID devices.
	// DeviceAuthenticator validates API key + PIN and sets tenant context,
	// then TenantTxMiddleware wraps each handler in a tenant-scoped transaction
	// (SET LOCAL ROLE phoenix_tenant + set_config) so RLS is enforced.
	r.Group(func(r chi.Router) {
		r.Use(device.DeviceAuthenticator(rs.IoTService, rs.PersonService, rs.SchoolRepo, nil))
		r.Use(tenant.TenantTxMiddleware(rs.db))

		// RFID tag assignment endpoint
		r.Post("/{id}/rfid", rs.assignRFIDTag)
		r.Delete("/{id}/rfid", rs.unassignRFIDTag)
	})

	return r
}

// containsIgnoreCase checks if a string contains another string, ignoring case
func containsIgnoreCase(s, substr string) bool {
	s, substr = strings.ToLower(s), strings.ToLower(substr)
	return strings.Contains(s, substr)
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

// checkStudentFullAccess determines if the current user has full access to
// a student's data for write operations (update, delete, privacy consent, etc.).
// Returns true if the user is an admin or supervises the student's group.
//
// The gdpr.student_data_scope setting intentionally does NOT apply here —
// write operations remain restricted to group supervisors regardless of scope.
// For read access checks, use checkStudentReadAccess instead.
func (rs *Resource) checkStudentFullAccess(r *http.Request, student *users.Student) bool {
	return rs.isGroupSupervisorOrAdmin(r, student)
}

// checkStudentReadAccess determines if the current user has full read access
// to a student's data (profile, location, visit info, privacy details, pickup
// schedules). Returns true if the user is an admin, a verified staff member
// when the tenant's student_data_scope is set to all_staff, or a supervisor
// of the student's education group.
//
// This function MUST only be used on read paths. Write operations must use
// checkStudentFullAccess which ignores the scope setting.
func (rs *Resource) checkStudentReadAccess(r *http.Request, student *users.Student) bool {
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	if hasAdminPermissions(userPermissions) {
		return true
	}

	// Tenant-configurable: when student_data_scope is set to all_staff, any
	// authenticated staff member gets full read access to any student.
	// Verify the caller is actually a staff member — other roles (guest,
	// guardian) with users:read must NOT get unredacted access.
	scope := configService.ResolveStringOrDefault(
		r.Context(),
		rs.SettingsService,
		configModel.KeyStudentDataScope,
		configModel.StudentDataScopeGroupSupervisorsOnly,
		rs.Logger,
	)
	if scope == configModel.StudentDataScopeAllStaff {
		if staff, err := rs.UserContextService.GetCurrentStaff(r.Context()); err == nil && staff != nil {
			return true
		}
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

// isGroupSupervisorOrAdmin checks if the caller is an admin or supervises the
// student's education group. This is the core authorization logic shared by
// both read and write access paths (before scope overrides are applied).
func (rs *Resource) isGroupSupervisorOrAdmin(r *http.Request, student *users.Student) bool {
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
		slog.Default().Error("failed to load student data snapshot", slog.String("error", err.Error()))
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build and filter responses
	responses := rs.buildStudentResponses(r.Context(), students, params, accessCtx, dataSnapshot)

	// Apply in-memory pagination if person-based filters were used
	if params.hasPersonFilters() {
		responses, totalCount = applyInMemoryPagination(responses, params.page, params.pageSize)
	}

	common.RespondPaginated(w, r, http.StatusOK, responses, common.PaginationParams{Page: params.page, PageSize: params.pageSize, Total: totalCount}, "Students retrieved successfully")
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
	countOptions := params.buildCountOptions()
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
	hasFullAccess := rs.checkStudentReadAccess(r, student)

	attendanceLogEnabled := configService.ResolveBoolOrDefault(r.Context(), rs.SettingsService, configModel.KeyAttendanceLogEnabled, false, rs.Logger)
	feedbackEnabled := configService.ResolveBoolOrDefault(r.Context(), rs.SettingsService, configModel.KeyFeedbackEnabled, false, rs.Logger)

	response := StudentDetailResponse{
		StudentResponse: newStudentResponseWithOpts(r.Context(), StudentResponseOpts{
			Student:       student,
			Person:        person,
			Group:         group,
			HasFullAccess: hasFullAccess,
		}, StudentResponseServices{
			ActiveService: rs.ActiveService,
			PersonService: rs.PersonService,
		}),
		HasFullAccess:        hasFullAccess,
		AttendanceLogEnabled: attendanceLogEnabled,
		FeedbackEnabled:      feedbackEnabled,
	}

	// Add supervisor contacts for users without full access
	if !hasFullAccess && group != nil {
		response.GroupSupervisors = rs.buildSupervisorContacts(r.Context(), group.ID)
	}

	common.Respond(w, r, http.StatusOK, response, "Student retrieved successfully")
}

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

	// Create person and student in tenant transaction
	student := createStudentFromRequest(req, 0) // personID set after create

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		// Create person - validation occurs at the model layer
		if err := rs.PersonService.Create(ctx, person); err != nil {
			return err
		}

		// Create student with the person ID
		student.PersonID = person.ID
		if err := rs.StudentRepo.Create(ctx, student); err != nil {
			rs.cleanupPersonAfterStudentFailure(ctx, person.ID)
			return err
		}
		return nil
	}); err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	// Get group data if student has a group
	group := rs.fetchStudentGroup(r.Context(), student.GroupID)

	// Admin users creating students can see full data including detailed location
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	hasFullAccess := hasAdminPermissions(userPermissions)

	// Return the created student with person data
	common.Respond(w, r, http.StatusCreated, newStudentResponseWithOpts(r.Context(), StudentResponseOpts{
		Student:       student,
		Person:        person,
		Group:         group,
		HasFullAccess: hasFullAccess,
	}, StudentResponseServices{
		ActiveService: rs.ActiveService,
		PersonService: rs.PersonService,
	}), "Student created successfully")
}

// cleanupPersonAfterStudentFailure removes the person record if student creation fails
func (rs *Resource) cleanupPersonAfterStudentFailure(ctx context.Context, personID int64) {
	if err := rs.PersonService.Delete(ctx, personID); err != nil {
		slog.Default().Error("failed to cleanup person after failed student creation",
			slog.Int64("person_id", personID),
			slog.String("error", err.Error()))
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

	// Update student fields using helper function
	applyStudentFieldUpdates(req, student)

	// Persist person and student updates in tenant transaction
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if personResult.updated {
			if err := rs.PersonService.Update(ctx, person); err != nil {
				return err
			}
		}
		return rs.StudentRepo.Update(ctx, student)
	}); err != nil {
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
	common.Respond(w, r, http.StatusOK, newStudentResponseWithOpts(r.Context(), StudentResponseOpts{
		Student:       updatedStudent,
		Person:        person,
		Group:         group,
		HasFullAccess: hasFullAccess,
	}, StudentResponseServices{
		ActiveService: rs.ActiveService,
		PersonService: rs.PersonService,
	}), "Student updated successfully")
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

	// Delete the student and associated person record in tenant transaction
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		// Delete the student first
		if err := rs.StudentRepo.Delete(ctx, student.ID); err != nil {
			return err
		}

		// Then delete the associated person record
		if err := rs.PersonService.Delete(ctx, student.PersonID); err != nil {
			// Log the error but don't fail the request since student is already deleted
			slog.Default().Error("failed to delete associated person record",
				slog.Int64("person_id", student.PersonID),
				slog.String("error", err.Error()))
		}
		return nil
	}); err != nil {
		if common.IsConstraintViolation(err) {
			renderError(w, r, common.ErrorConflictMessage("Schüler/in kann nicht gelöscht werden: Schüler/in hat aktive Besuche, Einschreibungen oder andere verknüpfte Daten"))
			return
		}
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Student deleted successfully")
}

// =============================================================================
// Exported Handler Methods for Testing
// =============================================================================
// These methods expose the underlying handlers for test access without going
// through the router's middleware chain.

// ListStudentsHandler returns the handler for listing students.
func (rs *Resource) ListStudentsHandler() http.HandlerFunc { return rs.listStudents }

// GetStudentHandler returns the handler for getting a single student.
func (rs *Resource) GetStudentHandler() http.HandlerFunc { return rs.getStudent }

// CreateStudentHandler returns the handler for creating a student.
func (rs *Resource) CreateStudentHandler() http.HandlerFunc { return rs.createStudent }

// UpdateStudentHandler returns the handler for updating a student.
func (rs *Resource) UpdateStudentHandler() http.HandlerFunc { return rs.updateStudent }

// DeleteStudentHandler returns the handler for deleting a student.
func (rs *Resource) DeleteStudentHandler() http.HandlerFunc { return rs.deleteStudent }

// GetStudentCurrentLocationHandler returns the handler for getting a student's current location.
func (rs *Resource) GetStudentCurrentLocationHandler() http.HandlerFunc {
	return rs.getStudentCurrentLocation
}

// GetStudentInGroupRoomHandler returns the handler for checking if a student is in their group room.
func (rs *Resource) GetStudentInGroupRoomHandler() http.HandlerFunc { return rs.getStudentInGroupRoom }

// GetStudentCurrentVisitHandler returns the handler for getting a student's current visit.
func (rs *Resource) GetStudentCurrentVisitHandler() http.HandlerFunc {
	return rs.getStudentCurrentVisit
}

// GetStudentVisitHistoryHandler returns the handler for getting a student's visit history.
func (rs *Resource) GetStudentVisitHistoryHandler() http.HandlerFunc {
	return rs.getStudentVisitHistory
}

// GetStudentAttendanceHistoryHandler returns the handler for getting a student's
// attendance history (daily presence + per-day room movement).
func (rs *Resource) GetStudentAttendanceHistoryHandler() http.HandlerFunc {
	return rs.getStudentAttendanceHistory
}

// GetStudentPrivacyConsentHandler returns the handler for getting a student's privacy consent.
func (rs *Resource) GetStudentPrivacyConsentHandler() http.HandlerFunc {
	return rs.getStudentPrivacyConsent
}

// UpdateStudentPrivacyConsentHandler returns the handler for updating a student's privacy consent.
func (rs *Resource) UpdateStudentPrivacyConsentHandler() http.HandlerFunc {
	return rs.updateStudentPrivacyConsent
}

// AssignRFIDTagHandler returns the handler for assigning an RFID tag to a student.
func (rs *Resource) AssignRFIDTagHandler() http.HandlerFunc { return rs.assignRFIDTag }

// UnassignRFIDTagHandler returns the handler for unassigning an RFID tag from a student.
func (rs *Resource) UnassignRFIDTagHandler() http.HandlerFunc { return rs.unassignRFIDTag }
