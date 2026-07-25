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
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
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
	PersonService           userService.PersonService
	GuardianService         userService.GuardianService
	EducationService        educationService.Service
	UserContextService      userContextService.UserContextService
	ActiveService           activeService.Service
	IoTService              iotSvc.Service
	PickupScheduleService   scheduleService.PickupScheduleService
	ArrivalScheduleService  scheduleService.ArrivalScheduleService
	InstanceService         scheduleService.InstanceService
	SchoolService           platformSvc.SchoolService
	SettingsService         configService.SettingsService
	StudentService          userService.StudentService
	StudentAuditService     userService.StudentAuditService
	StudentStatusDayService activeService.StudentStatusDayService
	StudentHistoryService   activeService.StudentHistoryService
	Broadcaster             realtime.Broadcaster
	StudentPhotos           userService.StudentPhotoService
	ListExportService       listexport.Service
	Logger                  *slog.Logger
	Now                     func() time.Time
	db                      *bun.DB
}

// ResourceConfig holds all dependencies for creating a students Resource.
// Using a config struct instead of individual parameters improves maintainability.
type ResourceConfig struct {
	PersonService           userService.PersonService
	GuardianService         userService.GuardianService
	EducationService        educationService.Service
	UserContextService      userContextService.UserContextService
	ActiveService           activeService.Service
	IoTService              iotSvc.Service
	PickupScheduleService   scheduleService.PickupScheduleService
	ArrivalScheduleService  scheduleService.ArrivalScheduleService
	InstanceService         scheduleService.InstanceService
	SchoolService           platformSvc.SchoolService
	SettingsService         configService.SettingsService
	StudentService          userService.StudentService
	StudentAuditService     userService.StudentAuditService
	StudentStatusDayService activeService.StudentStatusDayService
	StudentHistoryService   activeService.StudentHistoryService
	Broadcaster             realtime.Broadcaster
	StudentPhotos           userService.StudentPhotoService
	ListExportService       listexport.Service
	Logger                  *slog.Logger
	Now                     func() time.Time
	DB                      *bun.DB
}

// NewResource creates a new students resource from the provided configuration.
func NewResource(cfg ResourceConfig) *Resource {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	return &Resource{
		PersonService:           cfg.PersonService,
		GuardianService:         cfg.GuardianService,
		EducationService:        cfg.EducationService,
		UserContextService:      cfg.UserContextService,
		ActiveService:           cfg.ActiveService,
		IoTService:              cfg.IoTService,
		PickupScheduleService:   cfg.PickupScheduleService,
		ArrivalScheduleService:  cfg.ArrivalScheduleService,
		InstanceService:         cfg.InstanceService,
		SchoolService:           cfg.SchoolService,
		SettingsService:         cfg.SettingsService,
		StudentService:          cfg.StudentService,
		StudentAuditService:     cfg.StudentAuditService,
		StudentStatusDayService: cfg.StudentStatusDayService,
		StudentHistoryService:   cfg.StudentHistoryService,
		Broadcaster:             cfg.Broadcaster,
		StudentPhotos:           cfg.StudentPhotos,
		ListExportService:       cfg.ListExportService,
		Logger:                  cfg.Logger,
		Now:                     now,
		db:                      cfg.DB,
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
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Post("/export", rs.exportStudents)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}", rs.getStudent)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/in-group-room", rs.getStudentInGroupRoom)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/current-location", rs.getStudentCurrentLocation)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/current-visit", rs.getStudentCurrentVisit)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/visit-history", rs.getStudentVisitHistory)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/attendance-history", rs.getStudentAttendanceHistory)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/status-days", rs.getStudentStatusDays)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/parent-notes", rs.getStudentParentNotes)
		// Per-child change history (issue #1455). Full access (admin / group
		// supervisor) enforced inside the handler so it reads as a support tool,
		// not general staff surveillance.
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/change-history", rs.getStudentChangeHistory)

		// Routes requiring users:create permission
		r.With(authorize.RequiresPermission(permissions.UsersCreate), withTx).Post("/", rs.createStudent)

		// Routes requiring users:update permission
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}", rs.updateStudent)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Post("/status-days/bulk", rs.bulkCreateStudentStatusDays)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/status-days", rs.createStudentStatusDays)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/status-days/{statusDayId}", rs.deleteStudentStatusDay)

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

		// Student photo (Datenverwaltung). upload + delete: users:update;
		// serve: users:read. Feature gate + consent enforced in photo.go.
		// upload + serve skip withTx so a slow body / file stream doesn't
		// pin a bun pool connection. The handlers open their own short tx.
		r.With(authorize.RequiresPermission(permissions.UsersUpdate)).Post("/{id}/photo", rs.uploadStudentPhoto)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/photo", rs.deleteStudentPhoto)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}/photo/{filename}", rs.serveStudentPhoto)
	})

	// Device-authenticated routes for RFID devices.
	// DeviceAuthenticator validates API key + PIN and sets tenant context,
	// then TenantTxMiddleware wraps each handler in a tenant-scoped transaction
	// (SET LOCAL ROLE phoenix_tenant + set_config) so RLS is enforced.
	r.Group(func(r chi.Router) {
		r.Use(device.DeviceAuthenticator(rs.IoTService, rs.PersonService, rs.SchoolService, nil))
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

	student, err := rs.PersonService.GetStudentByID(r.Context(), id)
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
// The gdpr.student_data_scope setting intentionally does NOT apply here.
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
	if common.HasAdminPermissions(userPermissions) {
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
		if errors.Is(err, ErrInvalidRequest) {
			renderError(w, r, ErrorInvalidRequest(err))
			return
		}
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

	// Resolve once per request. populatePhotoFields runs per student.
	photosEnabled := configService.ResolveBoolOrDefault(r.Context(), rs.SettingsService, configModel.KeyStudentPhotosEnabled, false, rs.Logger)

	// Build and filter responses
	responses := rs.buildStudentResponses(r.Context(), students, params, accessCtx, dataSnapshot, photosEnabled)

	now := rs.Now()
	rs.applyStatusDaysForDate(r.Context(), responses, now)
	if err := rs.enrichWithDayPlanning(r.Context(), responses, now, attendanceMapFromSnapshot(dataSnapshot)); err != nil {
		slog.Default().Error("failed to enrich student day planning", slog.String("error", err.Error()))
		renderError(w, r, ErrorInternalServer(err))
		return
	}
	responses = applyDayPlanningFilter(responses, params.dayStatus)
	// Administrative filters (#1492): bus / photo consent / pickup rule.
	// Applied here, before in-memory pagination, so server-side counts and
	// page boundaries reflect the filtered set (no client-side full-page
	// fetch needed).
	responses = applyAdministrativeFilters(responses, params.bus, params.photoConsent, params.pickupStatus)

	// Apply in-memory pagination if response-derived filters were used.
	if params.hasInMemoryFilters() {
		responses, totalCount = applyInMemoryPagination(responses, params.page, params.pageSize)
	}

	for i := range responses {
		if !responses[i].HasFullAccess {
			continue
		}
		applyActualTimesFromSnapshot(&responses[i], dataSnapshot)
	}

	// Optionally enrich the paginated slice with today's effective pickup times (single bulk query).
	// Only query for students the caller has full access to. GDPR: skip redacted students.
	if params.includePickupTimes || params.includeArrivalTimes {
		fullAccessIDs := collectFullAccessStudentIDs(responses)
		if params.includePickupTimes {
			rs.enrichWithPickupTimes(r.Context(), responses, fullAccessIDs, now)
		}
		if params.includeArrivalTimes {
			rs.enrichWithArrivalTimes(r.Context(), responses, fullAccessIDs, now)
		}
	}

	common.RespondPaginated(w, r, http.StatusOK, responses, common.PaginationParams{Page: params.page, PageSize: params.pageSize, Total: totalCount}, "Students retrieved successfully")
}

// fetchStudentsForList fetches students based on the provided parameters
func (rs *Resource) fetchStudentsForList(r *http.Request, params *studentListParams) ([]*users.Student, int, error) {
	ctx := r.Context()

	if params.locationState != "" {
		if params.locationState != "transit" {
			return nil, 0, ErrInvalidRequest
		}
		if params.roomID > 0 {
			return nil, 0, ErrInvalidRequest
		}
		ids, err := rs.ActiveService.ListStudentsInTransit(ctx)
		if err != nil {
			return nil, 0, err
		}
		if len(ids) == 0 {
			return []*users.Student{}, 0, nil
		}
		if params.groupID > 0 {
			ids, err = rs.filterStudentIDsByGroup(ctx, ids, params.groupID)
			if err != nil {
				return nil, 0, err
			}
			if len(ids) == 0 {
				return []*users.Student{}, 0, nil
			}
		}
		params.studentIDs = ids
	}

	// room_id pre-filter (#1323): resolve students currently checked-in to
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
			filtered, err := rs.filterStudentIDsByGroup(ctx, ids, params.groupID)
			if err != nil {
				return nil, 0, err
			}
			if len(filtered) == 0 {
				return []*users.Student{}, 0, nil
			}
			ids = filtered
		}
		params.studentIDs = ids
		// fall through to the standard ListWithOptions path. params.groupID is
		// intentionally NOT cleared here even though buildBaseFilter ignores it
		// because the room and group intersection was already computed via FindByIDs
		// above, so re-applying group_id downstream would be redundant.
	} else if params.groupID > 0 && params.locationState == "" {
		// group-only branch keeps existing behavior
		students, err := rs.PersonService.GetStudentsByGroupIDs(ctx, []int64{params.groupID})
		if err != nil {
			return nil, 0, err
		}
		return students, len(students), nil
	}

	// Standard path. buildBaseFilter picks up params.studentIDs (if set by
	// the room_id branch above) and combines it with school_class /
	// guardian_name and pagination.
	queryOptions := params.buildQueryOptions()
	countOptions := params.buildCountOptions()

	totalCount, err := rs.StudentService.CountWithOptions(ctx, countOptions)
	if err != nil {
		return nil, 0, err
	}

	students, err := rs.StudentService.ListWithOptions(ctx, queryOptions)
	if err != nil {
		return nil, 0, err
	}

	return students, totalCount, nil
}

func (rs *Resource) filterStudentIDsByGroup(ctx context.Context, studentIDs []int64, groupID int64) ([]int64, error) {
	studentMap, err := rs.PersonService.GetStudentsByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	filtered := make([]int64, 0, len(studentMap))
	for _, sid := range studentIDs {
		student, ok := studentMap[sid]
		if !ok || student.GroupID == nil || *student.GroupID != groupID {
			continue
		}
		filtered = append(filtered, sid)
	}
	return filtered, nil
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
	now := rs.Now()
	rs.applyStatusDaysForDateToResponse(r.Context(), &response.StudentResponse, now)

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

		single := []StudentResponse{response.StudentResponse}
		if err := rs.enrichWithDayPlanning(r.Context(), single, now, map[int64]*activeService.AttendanceStatus{
			student.ID: attendanceStatus,
		}); err != nil {
			renderError(w, r, ErrorInternalServer(err))
			return
		}
		response.StudentResponse = single[0]
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
		parsedBirthday, err := timezone.ParseDate(req.Birthday)
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
	applyDeparturePlan(req.DepartureDays, req.PickupStatus, req.PickupDays, req.Bus, req.BusDays, student)

	return student
}

// applyDeparturePlan sets how a child leaves each weekday from a create/update
// request. A unified DepartureDays is authoritative when present and is
// decomposed (full replacement) onto the legacy per-day maps; otherwise the
// legacy pickup_status/pickup_days/bus/bus_days inputs are applied. Either way
// the repository folds bus_days + pickup_days into departure_days, the single
// source of truth, on persist (#1610).
func applyDeparturePlan(departure *users.DepartureDays, status *string, pickupDays *users.PickupDays, legacyBus *bool, busDays *users.BusDays, student *users.Student) {
	if departure != nil {
		dd := departure.Normalize()
		student.DepartureDays = dd
		student.BusDays = dd.BusDays()
		student.PickupDays = dd.PickupDays()
		return
	}
	reconcilePickupFields(student, status, pickupDays)
	applyBusDays(legacyBus, busDays, student)
}

// applyBusDays sets the student's bus_days from a create/update request.
// bus_days is the single source of truth (#1582); the legacy bus boolean is
// accepted only as an alias (true => Mon–Fri, false => no days) and is ignored
// when bus_days is also supplied. The derived bus flag is no longer stored.
func applyBusDays(legacyBus *bool, days *users.BusDays, student *users.Student) {
	if days != nil {
		student.BusDays = *days
		return
	}
	if legacyBus == nil {
		return
	}
	switch {
	case !*legacyBus:
		// Explicitly off: clear all bus days.
		student.BusDays = users.BusDays{}
	case !student.BusDays.HasAny():
		// On with no existing per-day selection: default to all weekdays.
		student.BusDays = users.BusDaysFromLegacyFlag(true)
		// On with an existing per-day selection: preserve it (a legacy bus=true
		// must not flatten Mo/Fr into all weekdays).
	}
}

// reconcilePickupFields keeps student.PickupDays (the authoritative per-weekday
// map) and the legacy student.PickupStatus string in sync from a request that
// may carry either, both, or neither. When both are present the weekday map
// wins, since it is the granular source of truth.
//
// Contract for legacy (status-only) callers: pickup_status is treated as
// authoritative. A non-"Wird abgeholt" status therefore CLEARS any existing
// weekday map — a partial update that sends pickup_status without pickup_days
// will wipe previously stored pickup days. This is intentional (the legacy
// string has no per-day information to preserve), but it means new clients must
// always send pickup_days, never pickup_status alone, to mutate the map.
func reconcilePickupFields(student *users.Student, status *string, days *users.PickupDays) {
	if status != nil {
		student.PickupStatus = status
		if *status != users.PickupStatusPickedUp {
			student.PickupDays = users.PickupDays{}
		} else if !student.PickupDays.HasAny() {
			student.PickupDays = users.PickupDaysFromLegacyStatus(*status)
		}
	}
	if days != nil {
		student.PickupDays = *days
		s := student.PickupDays.LegacyPickupStatus()
		student.PickupStatus = &s
	}
}

// optionalString returns a pointer to the trimmed string, or nil when empty,
// so optional JSON fields map cleanly onto nullable model columns.
func optionalString(s string) *string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// toNewStudentGuardians maps the request guardian DTOs onto the service input
// used by GuardianService.AddGuardiansToStudent.
func toNewStudentGuardians(inputs []GuardianInput) []userService.NewStudentGuardian {
	if len(inputs) == 0 {
		return nil
	}

	out := make([]userService.NewStudentGuardian, 0, len(inputs))
	for i := range inputs {
		in := inputs[i]
		out = append(out, userService.NewStudentGuardian{
			Profile: userService.GuardianCreateRequest{
				FirstName:              strings.TrimSpace(in.FirstName),
				LastName:               strings.TrimSpace(in.LastName),
				Email:                  optionalString(in.Email),
				AddressStreet:          optionalString(in.AddressStreet),
				AddressCity:            optionalString(in.AddressCity),
				AddressPostalCode:      optionalString(in.AddressPostalCode),
				PreferredContactMethod: in.PreferredContactMethod,
				LanguagePreference:     in.LanguagePreference,
				Notes:                  optionalString(in.Notes),
			},
			Relationship: userService.StudentGuardianRelationship{
				RelationshipType:   in.RelationshipType,
				IsPrimary:          in.IsPrimary,
				IsEmergencyContact: in.IsEmergencyContact,
				CanPickup:          in.CanPickup,
				PickupNotes:        optionalString(in.PickupNotes),
				EmergencyPriority:  in.EmergencyPriority,
			},
			PhoneNumbers:      toPhoneRequests(in.PhoneNumbers),
			ExistingProfileID: in.GuardianProfileID,
		})
	}
	return out
}

// toPhoneRequests maps phone DTOs onto the service phone-number requests.
func toPhoneRequests(phones []GuardianPhoneInput) []userService.PhoneNumberCreateRequest {
	if len(phones) == 0 {
		return nil
	}

	out := make([]userService.PhoneNumberCreateRequest, 0, len(phones))
	for i := range phones {
		p := phones[i]
		out = append(out, userService.PhoneNumberCreateRequest{
			PhoneNumber: strings.TrimSpace(p.PhoneNumber),
			PhoneType:   p.PhoneType,
			Label:       optionalString(p.Label),
			IsPrimary:   p.IsPrimary,
		})
	}
	return out
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

	guardians := toNewStudentGuardians(req.Guardians)

	// Resolve the acting staff once — weekly schedules are stamped with
	// CreatedBy. Only required when schedules are supplied so plain student
	// creation (no schedules) is unaffected.
	//
	// Creating a student is governed by users:create, but attached weekly
	// schedules are writes to the same Betreuungszeiten records edited by the
	// standalone PUT endpoints. Keep that schedule write contract aligned:
	// callers need users:update and must resolve to a staff record so schedule
	// rows always carry a valid author.
	var staffID int64
	if len(req.ArrivalSchedules) > 0 || len(req.PickupSchedules) > 0 {
		if !authorize.HasPermission(permissions.UsersUpdate, jwt.PermissionsFromCtx(r.Context())) {
			renderError(w, r, ErrorForbidden(errors.New("users:update permission required to create student schedules")))
			return
		}
		staffID, err = rs.getStaffIDFromJWT(r)
		if err != nil {
			renderError(w, r, ErrorForbidden(err))
			return
		}
	}

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		// Validate guardians BEFORE writing the student. This route runs inside
		// TenantTxMiddleware, which only rolls back on 5xx; a guardian
		// ValidationError renders 400, so the middleware would otherwise commit
		// an already-created student. Validating first means a 400 commits an
		// empty transaction — no orphaned student/person rows.
		if len(guardians) > 0 {
			if err := rs.GuardianService.ValidateNewGuardians(ctx, guardians); err != nil {
				return err
			}
		}

		// Create person - validation occurs at the model layer
		if err := rs.PersonService.Create(ctx, person); err != nil {
			return err
		}

		// Create student with the person ID
		student.PersonID = person.ID
		if err := rs.StudentService.Create(ctx, student); err != nil {
			rs.cleanupPersonAfterStudentFailure(ctx, person.ID)
			return err
		}

		// Create any guardians supplied with the request inside the same
		// transaction so the student and its guardians are persisted
		// atomically — a guardian failure rolls back the whole student.
		if len(guardians) > 0 {
			if err := rs.GuardianService.AddGuardiansToStudent(ctx, student.ID, guardians); err != nil {
				return err
			}
		}

		// Persist weekly arrival/pickup schedules in the same transaction so the
		// student and its recurring care times are created atomically (mirrors
		// the guardian handling above). The schedule tables FK to the student,
		// which now exists within this transaction.
		if len(req.ArrivalSchedules) > 0 {
			arrivals := toArrivalScheduleModels(req.ArrivalSchedules, student.ID, staffID)
			if err := rs.ArrivalScheduleService.UpsertBulkStudentArrivalSchedules(ctx, student.ID, arrivals); err != nil {
				return err
			}
		}
		if len(req.PickupSchedules) > 0 {
			pickups := toPickupScheduleModels(req.PickupSchedules, student.ID, staffID)
			if err := rs.PickupScheduleService.UpsertBulkStudentPickupSchedules(ctx, student.ID, pickups); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		// Bad guardian input (e.g. invalid email) is a client error: the
		// transaction has already rolled back, so no partial data survives.
		var validationErr *userService.ValidationError
		if errors.As(err, &validationErr) {
			renderError(w, r, ErrorInvalidRequest(validationErr))
			return
		}
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	// Get group data if student has a group
	group := rs.fetchStudentGroup(r.Context(), student.GroupID)

	// Admin users creating students can see full data including detailed location
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	hasFullAccess := common.HasAdminPermissions(userPermissions)

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
			parsedBirthday, err := timezone.ParseDate(*req.Birthday)
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

func reconcilePhotoConsentRequest(requested *bool, snapshot, fresh *users.Student) *bool {
	if requested == nil {
		return nil
	}
	snapshotHadConsent := snapshot != nil && snapshot.PhotoConsentGivenAt != nil
	freshHasConsent := fresh != nil && fresh.PhotoConsentGivenAt != nil

	// Treat values that merely echo the pre-transaction snapshot as no-ops.
	// Old clients used to serialize photo_consent_given on every PUT; if another
	// tab changed consent between the snapshot read and this row lock, replaying
	// that stale unchanged boolean would re-grant withdrawn consent or withdraw a
	// newly granted consent/photo.
	if *requested == snapshotHadConsent {
		return nil
	}

	// If a concurrent request already completed the intended transition, do not
	// re-stamp audit metadata or schedule duplicate photo cleanup.
	if *requested == freshHasConsent {
		return nil
	}

	return requested
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
	applyDeparturePlan(req.DepartureDays, req.PickupStatus, req.PickupDays, req.Bus, req.BusDays, student)
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
// schools B to N to refetch unrelated data whenever school A edits one
// student or one avatar. Routing via BroadcastToTenant, the helper
// already added in this branch for tenant_settings_changed, keeps the
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
				"skipping student_updated broadcast, no tenant context",
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
	isAdmin := common.HasAdminPermissions(userPermissions)
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

	// Patch is applied to a freshly FOR-UPDATE-locked row inside the tx so a
	// concurrent photo upload can't have its photo_path clobbered by a stale
	// snapshot write.
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		// Acquire the photo-feature advisory lock only when consent is
		// actually toggling. Name/notes edits must not queue behind
		// feature disable/purge.
		consentChanging := req.PhotoConsentGiven != nil &&
			(*req.PhotoConsentGiven) != (student.PhotoConsentGivenAt != nil)
		if consentChanging {
			if err := rs.StudentService.LockPhotoFeature(ctx); err != nil {
				return err
			}
		}

		fresh, err := rs.StudentService.GetByIDForUpdate(ctx, student.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errStudentNotFoundUnderLock
			}
			return err
		}

		// Re-check authorisation against the LOCKED row. A concurrent admin
		// reassignment could otherwise let a non-supervisor mutate the row.
		if ok, _ := canUpdateStudent(ctx, userPermissions, fresh, rs.UserContextService); !ok {
			return errStudentReassigned
		}

		// Re-validate sick/excused on fresh: two concurrent partial updates
		// against a stale snapshot can otherwise merge into the forbidden
		// sick && excused state. TenantTxMiddleware only rolls back on 5xx,
		// so this 409-mapped sentinel must fire before any write below.
		if err := checkSickExcusedConflict(req, fresh); err != nil {
			return errSickExcusedConflict
		}

		if personResult.updated {
			if err := rs.PersonService.Update(ctx, person); err != nil {
				return err
			}
		}

		// Read pre-update flags off fresh, not the snapshot, so status
		// history reflects the actual transition the commit will perform.
		wasSick := boolPtrValue(fresh.Sick)
		wasExcused := boolPtrValue(fresh.Excused)

		// Snapshot the tracked profile fields before applying the patch so the
		// audit diff (#1455) compares pre- vs post-update. applyStudentFieldUpdates
		// replaces (never mutates in place) the pointer/map fields we track, so a
		// shallow struct copy is a safe before-image.
		before := *fresh

		applyStudentFieldUpdates(req, fresh)
		effectiveConsent := reconcilePhotoConsentRequest(req.PhotoConsentGiven, student, fresh)
		rs.StudentPhotos.ApplyConsentTransition(ctx, effectiveConsent, fresh)

		if err := rs.persistStudentStatusHistory(ctx, fresh, wasSick, wasExcused, statusHistoryNow, normalizeSickReason(req.SickReason)); err != nil {
			rs.logStatusHistoryError(student.ID, err)
			return err
		}
		if err := rs.StudentService.Update(ctx, fresh); err != nil {
			return err
		}

		if req.DepartureDays != nil || req.PickupStatus != nil ||
			req.PickupDays != nil || req.Bus != nil || req.BusDays != nil {
			// Audit the effective unified plan even when a legacy client changed
			// only bus_days/pickup_days. The concrete repository currently writes
			// its resolved plan back into fresh, but the service contract does not
			// promise that mutation.
			normalizeDeparturePlanForAudit(fresh)
		}

		// Record the per-child change history (#1455). The audit write runs on
		// this same tx, so a failure must propagate: swallowing it would still
		// poison the tx and fail the COMMIT, losing the edit behind a 500. A
		// recorded edit must always carry its audit row, so roll back together.
		if err := rs.recordStudentChanges(ctx, &before, fresh); err != nil {
			return err
		}

		// Broadcast after the OUTER tx commits. Broadcasting now would race
		// subscribers into refetching the still-pre-commit row.
		studentID := student.ID
		capturedTenantID := tenantID
		tenant.RegisterAfterCommit(ctx, func() {
			rs.broadcastStudentUpdated(capturedTenantID, studentID)
		})
		return nil
	}); err != nil {
		if errors.Is(err, errSickExcusedConflict) {
			renderError(w, r, ErrorConflictWithCode(
				errors.New("a student cannot be both sick and excused at the same time"),
				ErrCodeSickExcusedConflict,
			))
			return
		}
		if errors.Is(err, errStudentReassigned) {
			renderError(w, r, ErrorForbidden(errors.New("you can only update students in groups you supervise")))
			return
		}
		if errors.Is(err, errStudentNotFoundUnderLock) {
			renderError(w, r, ErrorNotFound(errors.New("student not found")))
			return
		}
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	// Get updated student with person data
	updatedStudent, err := rs.PersonService.GetStudentByID(r.Context(), student.ID)
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

	// Capture the photo path from the locked row (not the pre-tx snapshot)
	// so a concurrent upload can't orphan a new file on disk. The unlink
	// itself runs after the OUTER tenant tx commits.
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		// FOR UPDATE row-locks against any in-flight upload tx. We either
		// observe its committed photo_path or it sees our deleted row and
		// aborts.
		fresh, err := rs.StudentService.GetByIDForUpdate(ctx, student.ID)
		if err != nil {
			return err
		}
		var photoToRemove string
		if fresh.PhotoPath != nil {
			photoToRemove = *fresh.PhotoPath
		}

		if err := rs.StudentService.Delete(ctx, student.ID); err != nil {
			return err
		}

		// Person delete failure must not fail the request. The student row
		// is already gone, leaving the person orphaned is recoverable.
		if err := rs.PersonService.Delete(ctx, student.PersonID); err != nil {
			slog.Default().Error("failed to delete associated person record",
				slog.Int64("person_id", student.PersonID),
				slog.String("error", err.Error()))
		}

		rs.StudentPhotos.ScheduleUnlinkAfterCommit(ctx, photoToRemove)
		return nil
	}); err != nil {
		if common.IsConstraintViolation(err) {
			renderError(w, r, common.ErrorConflictMessage("Kind kann nicht gelöscht werden: Kind hat aktive Besuche, Einschreibungen oder andere verknüpfte Daten"))
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
