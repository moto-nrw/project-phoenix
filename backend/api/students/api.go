package students

import (
	"context"
	"database/sql"
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
	"github.com/moto-nrw/project-phoenix/realtime"
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
	PersonService          userService.PersonService
	StudentRepo            users.StudentRepository
	EducationService       educationService.Service
	UserContextService     userContextService.UserContextService
	ActiveService          activeService.Service
	IoTService             iotSvc.Service
	PrivacyConsentRepo     users.PrivacyConsentRepository
	PickupScheduleService  scheduleService.PickupScheduleService
	ArrivalScheduleService scheduleService.ArrivalScheduleService
	SchoolRepo             platform.SchoolRepository
	SettingsService        configService.SettingsService
	AttendanceRepo         active.AttendanceRepository
	StudentStatusDayRepo   active.StudentStatusDayRepository
	VisitRepo              active.VisitRepository
	DataAccessLogRepo      auditModels.DataAccessLogRepository
	Broadcaster            realtime.Broadcaster
	Logger                 *slog.Logger
	db                     *bun.DB
}

// ResourceConfig holds all dependencies for creating a students Resource.
// Using a config struct instead of individual parameters improves maintainability.
type ResourceConfig struct {
	PersonService          userService.PersonService
	StudentRepo            users.StudentRepository
	EducationService       educationService.Service
	UserContextService     userContextService.UserContextService
	ActiveService          activeService.Service
	IoTService             iotSvc.Service
	PrivacyConsentRepo     users.PrivacyConsentRepository
	PickupScheduleService  scheduleService.PickupScheduleService
	ArrivalScheduleService scheduleService.ArrivalScheduleService
	SchoolRepo             platform.SchoolRepository
	SettingsService        configService.SettingsService
	AttendanceRepo         active.AttendanceRepository
	StudentStatusDayRepo   active.StudentStatusDayRepository
	VisitRepo              active.VisitRepository
	DataAccessLogRepo      auditModels.DataAccessLogRepository
	Broadcaster            realtime.Broadcaster
	Logger                 *slog.Logger
	DB                     *bun.DB
}

// NewResource creates a new students resource from the provided configuration.
func NewResource(cfg ResourceConfig) *Resource {
	return &Resource{
		PersonService:          cfg.PersonService,
		StudentRepo:            cfg.StudentRepo,
		EducationService:       cfg.EducationService,
		UserContextService:     cfg.UserContextService,
		ActiveService:          cfg.ActiveService,
		IoTService:             cfg.IoTService,
		PrivacyConsentRepo:     cfg.PrivacyConsentRepo,
		PickupScheduleService:  cfg.PickupScheduleService,
		ArrivalScheduleService: cfg.ArrivalScheduleService,
		SchoolRepo:             cfg.SchoolRepo,
		SettingsService:        cfg.SettingsService,
		AttendanceRepo:         cfg.AttendanceRepo,
		StudentStatusDayRepo:   cfg.StudentStatusDayRepo,
		VisitRepo:              cfg.VisitRepo,
		DataAccessLogRepo:      cfg.DataAccessLogRepo,
		Broadcaster:            cfg.Broadcaster,
		Logger:                 cfg.Logger,
		db:                     cfg.DB,
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

		// Arrival schedule routes (full access required - checked in handlers)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/arrival-schedules", rs.getStudentArrivalSchedules)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/arrival-schedules", rs.updateStudentArrivalSchedules)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/arrival-exceptions", rs.createStudentArrivalException)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/arrival-exceptions/{exceptionId}", rs.updateStudentArrivalException)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/arrival-exceptions/{exceptionId}", rs.deleteStudentArrivalException)

		// Arrival note routes (full access required - checked in handlers)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/arrival-notes", rs.createStudentArrivalNote)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/arrival-notes/{noteId}", rs.updateStudentArrivalNote)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/arrival-notes/{noteId}", rs.deleteStudentArrivalNote)

		// Bulk arrival schedule and time endpoints
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Post("/arrival-schedules/bulk", rs.bulkUpsertArrivalSchedules)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Post("/arrival-times/bulk", rs.getBulkArrivalTimes)

		// Web-based school check-in/out. Mode-agnostic (writes attendance only).
		// The users:checkin permission is the coarse gate; the
		// attendance.web_checkin_access setting is the fine gate enforced inside
		// the handler (group_supervisors vs all_staff).
		r.With(authorize.RequiresPermission(permissions.UsersCheckin), withTx).Post("/{id}/school-checkin", rs.schoolCheckinHandler)

		// Student photo (Datenverwaltung). Upload + delete gate on
		// users:update (same permission as Stammdaten edit). The serve route
		// uses users:read so any staff member who can see the student's
		// profile can render the avatar. Setting + consent are enforced
		// inside the handlers — see api/students/photo.go.
		// uploadStudentPhoto deliberately does NOT use `withTx`: the
		// outer middleware tx would start before the handler reads the
		// multipart body, pinning a bun pool connection for the full
		// duration of the parse + disk save (slow clients on 5 MiB
		// bodies can drag this out for tens of seconds). The handler
		// runs ParseImage + SaveImage outside any tx, then opens its
		// own short WithTenantTx for the auth + DB writes — same
		// pattern serveStudentPhoto uses for the same reason.
		r.With(authorize.RequiresPermission(permissions.UsersUpdate)).Post("/{id}/photo", rs.uploadStudentPhoto)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/photo", rs.deleteStudentPhoto)
		// serveStudentPhoto deliberately does NOT use `withTx`: TenantTxMiddleware
		// would keep the bun pool connection bound for the full lifetime of the
		// response, including the file-stream Write loop. Pages rendering many
		// avatars or slow clients downloading large photos would each pin a
		// connection — under load this starves unrelated requests. Instead the
		// handler opens its own short WithTenantTx for the auth + setting checks
		// and resolves the on-disk path inside it; the actual bytes stream
		// AFTER that tx commits and releases the connection. See photo.go.
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}/photo/{filename}", rs.serveStudentPhoto)
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
		renderError(w, r, common.ErrorInternalServerWrap("failed to get person data for student", err))
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
//
// Delegates to authorize.CanReadStudent so the same predicate is reusable
// from other handlers (timetable, per-student day view) without duplicating
// the scope/admin/supervisor logic.
func (rs *Resource) checkStudentReadAccess(r *http.Request, student *users.Student) bool {
	return authorize.CanReadStudent(
		r.Context(),
		jwt.PermissionsFromCtx(r.Context()),
		student,
		rs.UserContextService,
		rs.SettingsService,
		rs.Logger,
	)
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

	// Resolve the photo feature flag once per request — populatePhotoFields
	// runs per student, and the settings cache lookup is cheap but not free.
	photosEnabled := configService.ResolveBoolOrDefault(r.Context(), rs.SettingsService, configModel.KeyStudentPhotosEnabled, false, rs.Logger)

	// Build and filter responses
	responses := rs.buildStudentResponses(r.Context(), students, params, accessCtx, dataSnapshot, photosEnabled)

	// Apply in-memory pagination if person-based filters were used
	if params.hasPersonFilters() {
		responses, totalCount = applyInMemoryPagination(responses, params.page, params.pageSize)
	}

	for i := range responses {
		if !responses[i].HasFullAccess {
			continue
		}
		applyActualTimesFromSnapshot(&responses[i], dataSnapshot)
	}

	// Optionally enrich the paginated slice with today's effective pickup times (single bulk query).
	// Only query for students the caller has full access to (GDPR — skip redacted students).
	if params.includePickupTimes || params.includeArrivalTimes {
		fullAccessIDs := collectFullAccessStudentIDs(responses)
		if params.includePickupTimes {
			rs.enrichWithPickupTimes(r.Context(), responses, fullAccessIDs, time.Now())
		}
		if params.includeArrivalTimes {
			rs.enrichWithArrivalTimes(r.Context(), responses, fullAccessIDs, time.Now())
		}
	}

	common.RespondPaginated(w, r, http.StatusOK, responses, common.PaginationParams{Page: params.page, PageSize: params.pageSize, Total: totalCount}, "Students retrieved successfully")
}

// fetchStudentsForList fetches students based on the provided parameters
func (rs *Resource) fetchStudentsForList(r *http.Request, params *studentListParams) ([]*users.Student, int, error) {
	ctx := r.Context()

	// room_id pre-filter (#1323) — resolve students currently checked-in to
	// any active group in the room, then push the IDs through the standard
	// query path so school_class / guardian_name / pagination still apply.
	// The visit join lives in the active service (rule 11: services own
	// queries, not handlers).
	if params.roomID > 0 {
		ids, err := rs.ActiveService.ListStudentsPresentInRoom(ctx, params.roomID)
		if err != nil {
			return nil, 0, err
		}
		if len(ids) == 0 {
			return []*users.Student{}, 0, nil
		}
		// When both room_id and group_id are supplied, intersect with the
		// student's group_id so the response stays consistent with the
		// active-group chip in the search UI. group_id lives on the Student
		// row, so a single bulk lookup is enough.
		if params.groupID > 0 {
			studentMap, err := rs.StudentRepo.FindByIDs(ctx, ids)
			if err != nil {
				return nil, 0, err
			}
			filtered := make([]int64, 0, len(studentMap))
			for _, sid := range ids {
				s, ok := studentMap[sid]
				if !ok || s.GroupID == nil || *s.GroupID != params.groupID {
					continue
				}
				filtered = append(filtered, sid)
			}
			if len(filtered) == 0 {
				return []*users.Student{}, 0, nil
			}
			ids = filtered
		}
		params.studentIDs = ids
		// fall through to the standard ListWithOptions path. params.groupID is
		// intentionally NOT cleared here even though buildBaseFilter ignores it
		// — the room∩group intersection has already been computed via FindByIDs
		// above, so re-applying group_id downstream would be redundant.
	} else if params.groupID > 0 {
		// group-only branch keeps existing behavior
		students, err := rs.StudentRepo.FindByGroupIDs(ctx, []int64{params.groupID})
		if err != nil {
			return nil, 0, err
		}
		return students, len(students), nil
	}

	// Standard path — buildBaseFilter picks up params.studentIDs (if set by
	// the room_id branch above) and combines it with school_class /
	// guardian_name and pagination.
	queryOptions := params.buildQueryOptions()
	countOptions := params.buildCountOptions()

	totalCount, err := rs.StudentRepo.CountWithOptions(ctx, countOptions)
	if err != nil {
		return nil, 0, err
	}

	students, err := rs.StudentRepo.ListWithOptions(ctx, queryOptions)
	if err != nil {
		return nil, 0, err
	}

	return students, totalCount, nil
}

// buildStudentResponses builds filtered student responses
func (rs *Resource) buildStudentResponses(ctx context.Context, students []*users.Student, params *studentListParams, accessCtx *studentAccessContext, dataSnapshot *common.StudentDataSnapshot, photosEnabled bool) []StudentResponse {
	responses := make([]StudentResponse, 0, len(students))

	for _, student := range students {
		response := rs.buildSingleStudentResponse(ctx, student, params, accessCtx, dataSnapshot, photosEnabled)
		if response != nil {
			responses = append(responses, *response)
		}
	}

	return responses
}

// buildSingleStudentResponse builds a response for a single student, returning nil if filtered out
func (rs *Resource) buildSingleStudentResponse(ctx context.Context, student *users.Student, params *studentListParams, accessCtx *studentAccessContext, dataSnapshot *common.StudentDataSnapshot, photosEnabled bool) *StudentResponse {
	hasFullAccess := accessCtx.HasFullAccessToStudent(student)

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
	studentResponse := newStudentResponseFromSnapshot(ctx, student, person, group, hasFullAccess, dataSnapshot, photosEnabled)

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
	hasWriteAccess := rs.checkStudentFullAccess(r, student)

	attendanceLogEnabled := configService.ResolveBoolOrDefault(r.Context(), rs.SettingsService, configModel.KeyAttendanceLogEnabled, false, rs.Logger)
	feedbackEnabled := configService.ResolveBoolOrDefault(r.Context(), rs.SettingsService, configModel.KeyFeedbackEnabled, false, rs.Logger)
	photosEnabled := configService.ResolveBoolOrDefault(r.Context(), rs.SettingsService, configModel.KeyStudentPhotosEnabled, false, rs.Logger)

	response := StudentDetailResponse{
		StudentResponse: newStudentResponseWithOpts(r.Context(), StudentResponseOpts{
			Student:       student,
			Person:        person,
			Group:         group,
			HasFullAccess: hasFullAccess,
			PhotosEnabled: photosEnabled,
		}, StudentResponseServices{
			ActiveService: rs.ActiveService,
			PersonService: rs.PersonService,
		}),
		HasFullAccess:        hasFullAccess,
		HasWriteAccess:       hasWriteAccess,
		AttendanceLogEnabled: attendanceLogEnabled,
		FeedbackEnabled:      feedbackEnabled,
	}

	if hasFullAccess {
		attendanceStatus, err := rs.ActiveService.GetStudentAttendanceStatus(r.Context(), student.ID)
		if err != nil {
			rs.Logger.Warn("failed to resolve actual student arrival/pickup times",
				"student_id", student.ID,
				"error", err.Error(),
			)
		} else {
			applyActualTimesFromAttendance(&response.StudentResponse, attendanceStatus)
		}
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
	photosEnabled := configService.ResolveBoolOrDefault(r.Context(), rs.SettingsService, configModel.KeyStudentPhotosEnabled, false, rs.Logger)
	common.Respond(w, r, http.StatusCreated, newStudentResponseWithOpts(r.Context(), StudentResponseOpts{
		Student:       student,
		Person:        person,
		Group:         group,
		HasFullAccess: hasFullAccess,
		PhotosEnabled: photosEnabled,
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
	applyExcusedStatus(req, student)
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

// photoConsentTransition describes a consent toggle on update. The handler
// uses RemovedPhotoPath to delete the file *after* the DB transaction commits,
// keeping consent withdrawal atomic with the file unlink.
type photoConsentTransition struct {
	GrantedNow       bool   // false→true: stamp _at/_by below
	WithdrawnNow     bool   // true→false: nil out _at/_by/path; file removed by caller
	RemovedPhotoPath string // filesystem URL to RemoveImage on the way out
}

// applyPhotoConsent reconciles req.PhotoConsentGiven with the student's
// existing consent state. Returns the transition so the handler can apply
// audit fields (acting account, timestamp) and remove the photo file when
// consent is withdrawn — the model can't do either by itself because both
// require request context.
func applyPhotoConsent(req *UpdateStudentRequest, student *users.Student) photoConsentTransition {
	if req.PhotoConsentGiven == nil {
		return photoConsentTransition{}
	}
	hadConsent := student.PhotoConsentGivenAt != nil
	wantConsent := *req.PhotoConsentGiven

	if !hadConsent && wantConsent {
		return photoConsentTransition{GrantedNow: true}
	}
	if hadConsent && !wantConsent {
		// Capture the path *before* nilling so the handler can clean up the
		// file post-commit. Nil out all three columns so the model write
		// reflects the withdrawn state in a single update.
		removed := ""
		if student.PhotoPath != nil {
			removed = *student.PhotoPath
		}
		student.PhotoPath = nil
		student.PhotoConsentGivenAt = nil
		student.PhotoConsentGivenBy = nil
		return photoConsentTransition{WithdrawnNow: true, RemovedPhotoPath: removed}
	}
	return photoConsentTransition{}
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

// applyExcusedStatus handles excused status updates with ExcusedSince timestamp logic
func applyExcusedStatus(req *UpdateStudentRequest, student *users.Student) {
	if req.Excused == nil {
		return
	}
	student.Excused = req.Excused
	if *req.Excused {
		if student.ExcusedSince == nil {
			now := time.Now()
			student.ExcusedSince = &now
		}
	} else {
		student.ExcusedSince = nil
	}
}

// checkSickExcusedConflict returns an error if the incoming update would
// result in both sick and excused being true simultaneously. Callers with a
// conflict should prompt the user to switch states rather than hold both.
func checkSickExcusedConflict(req *UpdateStudentRequest, student *users.Student) error {
	sickFinal := student.Sick != nil && *student.Sick
	if req.Sick != nil {
		sickFinal = *req.Sick
	}
	excusedFinal := student.Excused != nil && *student.Excused
	if req.Excused != nil {
		excusedFinal = *req.Excused
	}
	if sickFinal && excusedFinal {
		return errors.New("a student cannot be both sick and excused at the same time")
	}
	return nil
}

// broadcastStudentUpdated emits an SSE student_updated event to the
// AFFECTED tenant's connected clients only. use-global-sse.ts treats
// that event as "invalidate every student / room / dashboard cache in
// this tab", so a global fan-out (BroadcastToAll) would force tabs in
// schools B…N to refetch unrelated data whenever school A edits one
// student or one avatar. Routing via BroadcastToTenant — the helper
// already added in this branch for tenant_settings_changed — keeps the
// invalidation scoped to the school that actually changed.
//
// Callers MUST pass tenantID from request context (tenant.FromContext)
// or from a captured value inside an after-commit hook. Passing zero
// no-ops the broadcast rather than fanning out, since
// BroadcastToTenant rejects zero tenant IDs by definition (no Client
// has TenantID == 0).
func (rs *Resource) broadcastStudentUpdated(tenantID, studentID int64) {
	if rs.Broadcaster == nil {
		return
	}
	if tenantID <= 0 {
		// Defensive: a missing tenant context means we don't know which
		// school's clients should invalidate. Logging the case so a
		// future caller that forgets to thread tenantID through gets a
		// breadcrumb instead of silent loss.
		if rs.Logger != nil {
			rs.Logger.Warn(
				"skipping student_updated broadcast — no tenant context",
				"student_id", studentID,
			)
		}
		return
	}

	source := "manual"
	event := realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{
		Source: &source,
	})

	if err := rs.Broadcaster.BroadcastToTenant(tenantID, event); err != nil && rs.Logger != nil {
		rs.Logger.Warn(
			"failed to broadcast student update",
			"tenant_id", tenantID,
			"student_id", studentID,
			"error", err.Error(),
		)
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

	// Reject updates that would leave the student in both sick and excused
	// states simultaneously. The frontend uses the SICK_EXCUSED_CONFLICT code
	// to prompt the user to switch states rather than hold both.
	if err := checkSickExcusedConflict(req, student); err != nil {
		renderError(w, r, ErrorConflictWithCode(err, ErrCodeSickExcusedConflict))
		return
	}

	statusHistoryNow := time.Now()

	// In-tx sentinel: a concurrent partial update committed between our
	// pre-tx conflict check and the locked-row re-check below produced
	// the forbidden sick && excused state. Mapped to 409 +
	// SICK_EXCUSED_CONFLICT in the outer error switch so the frontend
	// can prompt the user the same way it does for a synchronous
	// conflict. errors.Is keeps the dispatch resilient to wrapping.
	errSickExcusedConflict := errors.New("sick and excused conflict on locked row")

	// In-tx sentinel: a concurrent delete removed the student row
	// between parseAndGetStudent above and the locked re-read below.
	// The non-racy path returns 404; bare sql.ErrNoRows would otherwise
	// fall through to a 500 for a legitimate concurrent delete.
	errStudentNotFoundUnderLock := errors.New("student deleted between snapshot and lock")

	// In-tx sentinel: the pre-tx canUpdateStudent check ran on
	// student.GroupID from the snapshot. canUpdateStudent decides off
	// group membership, so a concurrent admin moving the student into
	// a different group between snapshot and lock can leave the caller
	// without write authority on the locked row. Re-checking against
	// fresh closes that window. Mapped to 403 in the outer switch so
	// the response status matches what the pre-tx gate would emit.
	errStudentReassigned := errors.New("student reassigned out of caller's scope mid-update")

	// Persist person and student updates in tenant transaction. The patch
	// (field updates + photo-consent transition) is applied INSIDE the tx
	// against a freshly locked row read via FindByIDForUpdate, not against
	// the pre-tx student snapshot. Without this, a concurrent photo upload
	// that commits between parseAndGetStudent and this tx would have its
	// photo_path silently clobbered when StudentRepo.Update writes the
	// stale snapshot back, and a consent withdrawal would unlink the OLD
	// file while leaving the freshly-uploaded file orphaned on disk. The
	// upload/delete handlers in photo.go already use this same pattern.
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		// Order matters: lock + re-check the sick/excused invariant
		// BEFORE any other writes in this tx. The handler maps the
		// SICK_EXCUSED_CONFLICT sentinel to a 4xx, and TenantTxMiddleware
		// only rolls back on 5xx — so a 409 path with PersonService.Update
		// already executed would commit the person change while the
		// client is told the save failed. Running the check first keeps
		// the conflict response atomic: either the whole patch lands or
		// nothing does.

		// LockPhotoFeature only when consent is actually being toggled —
		// the per-tenant advisory lock serialises against feature
		// disable/purge, and we don't want every name/notes edit to
		// queue behind those operations. Field-only updates are still
		// safe against concurrent photo writes because we re-read the
		// row FOR UPDATE below and apply the patch on top of fresh.
		consentChanging := req.PhotoConsentGiven != nil &&
			(*req.PhotoConsentGiven) != (student.PhotoConsentGivenAt != nil)
		if consentChanging {
			if err := rs.StudentRepo.LockPhotoFeature(ctx); err != nil {
				return err
			}
		}

		fresh, err := rs.StudentRepo.FindByIDForUpdate(ctx, student.ID)
		if err != nil {
			// Concurrent delete between parseAndGetStudent and this
			// locked re-read — surface as 404 instead of letting the
			// raw sql.ErrNoRows fall through to 500.
			if errors.Is(err, sql.ErrNoRows) {
				return errStudentNotFoundUnderLock
			}
			return err
		}

		// Re-run the per-student write gate against the LOCKED row.
		// The pre-tx check at line ~1003 ran on student.GroupID from
		// the snapshot; canUpdateStudent decides off group membership,
		// so a concurrent admin moving the student into a different
		// group between snapshot and lock can leave the caller without
		// authority on the row we're about to mutate. Re-checking on
		// fresh closes that window — the 403-mapped sentinel rolls
		// back the entire tx (including the still-pending person
		// update further down), keeping the response atomic.
		//
		// Running the check BEFORE PersonService.Update is mandatory
		// for the same reason the sick/excused re-check is — see the
		// order rationale at the top of this closure.
		if ok, _ := canUpdateStudent(ctx, userPermissions, fresh, rs.UserContextService); !ok {
			return errStudentReassigned
		}

		// Re-validate the sick/excused invariant against the LOCKED row.
		// The pre-tx check at line ~986 ran on a stale snapshot; with
		// the patch-on-fresh model, two concurrent partial updates can
		// otherwise merge into the forbidden sick && excused state:
		//   T1 commits {sick:true} on a row with excused=false
		//   T2 reads pre-tx snapshot (sick=false, excused=false), passes
		//     the early check with {excused:true}, opens its tx, locks
		//     fresh (now sick=true), applyStudentFieldUpdates only sets
		//     excused — sick stays at the now-true fresh value, and we
		//     write the invariant-violating row.
		// Re-running the check against fresh closes that window. Older
		// last-write-wins behaviour didn't have this hazard because
		// Update overwrote every column with the snapshot's value, so
		// only one of the two flags could end up true.
		//
		// Running the check BEFORE PersonService.Update is mandatory:
		// see the order rationale at the top of this closure.
		if err := checkSickExcusedConflict(req, fresh); err != nil {
			return errSickExcusedConflict
		}

		// Person update runs only AFTER the conflict check passes — a
		// 409 returned past this point would otherwise be committed by
		// TenantTxMiddleware (it only rolls back on 5xx).
		if personResult.updated {
			if err := rs.PersonService.Update(ctx, person); err != nil {
				return err
			}
		}

		// Pre-update flags read from the LOCKED row, not the pre-tx
		// snapshot, so status history reflects the actual transition the
		// commit performs (e.g. a concurrent excused-toggle that won the
		// race becomes the wasExcused baseline for ours).
		wasSick := boolPtrValue(fresh.Sick)
		wasExcused := boolPtrValue(fresh.Excused)

		// Re-apply the request patch on top of the locked row. This is
		// the SAME helper trio used pre-tx in the previous version of
		// this handler — moved here so all writes flow through `fresh`
		// instead of the stale `student` snapshot.
		applyStudentFieldUpdates(req, fresh)
		photoTransition := applyPhotoConsent(req, fresh)
		if photoTransition.GrantedNow {
			stampPhotoConsent(ctx, fresh)
		}

		if err := rs.persistStudentStatusHistory(ctx, fresh, wasSick, wasExcused, statusHistoryNow); err != nil {
			rs.logStatusHistoryError(student.ID, err)
			return err
		}
		if err := rs.StudentRepo.Update(ctx, fresh); err != nil {
			return err
		}

		// Consent withdrawn — schedule the orphaned-file unlink for
		// AFTER the OUTERMOST tenant tx commits. Students sit behind
		// TenantTxMiddleware, so this WithTenantTx is reusing the
		// middleware's transaction; running the unlink before that
		// commit would leave the row pointing at a deleted file if the
		// outer commit later fails.
		if photoTransition.WithdrawnNow && photoTransition.RemovedPhotoPath != "" {
			removed := photoTransition.RemovedPhotoPath
			tenant.RegisterAfterCommit(ctx, func() {
				removeStudentPhotoFile(removed, rs.Logger)
			})
		}

		// Defer the SSE broadcast to AFTER the outer tx commits — same
		// reasoning as the file unlink above. Subscribers refetch the
		// student row when they receive the event, and on a route wrapped
		// by TenantTxMiddleware this WithTenantTx has only reused the
		// middleware's transaction at this point. Broadcasting now would
		// race other tabs into refetching the OLD row (the change is not
		// yet visible) and they would never receive a second invalidation
		// — the avatar / consent / status would stay stale until manual
		// refresh. Photo consent withdrawal is the user-visible failure
		// mode flagged in review.
		// Capture both IDs into the after-commit closure so the broadcast
		// stays scoped to THIS tenant even if anything reads tenantID off
		// context after the tx releases.
		studentID := student.ID
		capturedTenantID := tenantID
		tenant.RegisterAfterCommit(ctx, func() {
			rs.broadcastStudentUpdated(capturedTenantID, studentID)
		})
		return nil
	}); err != nil {
		// 409 the same way the pre-tx check does — frontend uses the
		// SICK_EXCUSED_CONFLICT code to prompt the user to switch states.
		// Other tx errors stay as 500.
		if errors.Is(err, errSickExcusedConflict) {
			renderError(w, r, ErrorConflictWithCode(
				errors.New("a student cannot be both sick and excused at the same time"),
				ErrCodeSickExcusedConflict,
			))
			return
		}
		// 403 mirrors what the pre-tx canUpdateStudent gate emits —
		// the locked row showed the student is no longer in a group
		// the caller supervises.
		if errors.Is(err, errStudentReassigned) {
			renderError(w, r, ErrorForbidden(errors.New("you can only update students in groups you supervise")))
			return
		}
		// 404 when a concurrent delete won the race for the row.
		if errors.Is(err, errStudentNotFoundUnderLock) {
			renderError(w, r, ErrorNotFound(errors.New("student not found")))
			return
		}
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
	photosEnabled := configService.ResolveBoolOrDefault(r.Context(), rs.SettingsService, configModel.KeyStudentPhotosEnabled, false, rs.Logger)
	common.Respond(w, r, http.StatusOK, newStudentResponseWithOpts(r.Context(), StudentResponseOpts{
		Student:       updatedStudent,
		Person:        person,
		Group:         group,
		HasFullAccess: hasFullAccess,
		PhotosEnabled: photosEnabled,
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

	// Delete the student and associated person record in tenant transaction.
	// The photo URL is captured INSIDE the tx from the locked row (not from
	// the pre-tx student snapshot) so a concurrent upload can't leave a new
	// file orphaned on disk. The unlink itself runs via RegisterAfterCommit
	// — student routes are wrapped in TenantTxMiddleware, so this
	// WithTenantTx merely reuses the middleware's transaction; running the
	// unlink before the OUTER commit would leave the row pointing at a
	// deleted file if the outer commit later rolls back.
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		// Re-read FOR UPDATE to capture whichever photo_path is current at
		// the moment of delete (a concurrent upload may have replaced it
		// since parseAndGetStudent ran). FindByIDForUpdate also row-locks
		// against any in-flight upload tx, so we either see its committed
		// photo_path here or it sees our deleted row and aborts.
		fresh, err := rs.StudentRepo.FindByIDForUpdate(ctx, student.ID)
		if err != nil {
			return err
		}
		var photoToRemove string
		if fresh.PhotoPath != nil {
			photoToRemove = *fresh.PhotoPath
		}

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

		if photoToRemove != "" {
			removed := photoToRemove
			tenant.RegisterAfterCommit(ctx, func() {
				removeStudentPhotoFile(removed, rs.Logger)
			})
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

// SchoolCheckinHandler returns the handler for POST /api/students/{id}/school-checkin.
// Exposed for integration tests that bypass the router's middleware chain.
func (rs *Resource) SchoolCheckinHandler() http.HandlerFunc { return rs.schoolCheckinHandler }

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
