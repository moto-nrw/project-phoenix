package students

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/education"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	activeService "github.com/moto-nrw/project-phoenix/internal/core/service/active"
	educationService "github.com/moto-nrw/project-phoenix/internal/core/service/education"
	iotSvc "github.com/moto-nrw/project-phoenix/internal/core/service/iot"
	userContextService "github.com/moto-nrw/project-phoenix/internal/core/service/usercontext"
	userService "github.com/moto-nrw/project-phoenix/internal/core/service/users"
)

// renderError writes an error response to the HTTP response writer
// Logs rendering errors but doesn't propagate them (already in error state)
func renderError(w http.ResponseWriter, r *http.Request, errorResponse render.Renderer) {
	if err := render.Render(w, r, errorResponse); err != nil {
		if logger.Logger != nil {
			logger.Logger.WithError(err).Error("failed to render error response")
		}
	}
}

// Resource defines the students API resource
type Resource struct {
	PersonService      userService.PersonService
	StudentService     userService.StudentService
	EducationService   educationService.Service
	UserContextService userContextService.UserContextService
	ActiveService      activeService.Service
	IoTService         iotSvc.Service
}

// NewResource creates a new students resource
func NewResource(personService userService.PersonService, studentService userService.StudentService, educationService educationService.Service, userContextService userContextService.UserContextService, activeService activeService.Service, iotService iotSvc.Service) *Resource {
	return &Resource{
		PersonService:      personService,
		StudentService:     studentService,
		EducationService:   educationService,
		UserContextService: userContextService,
		ActiveService:      activeService,
		IoTService:         iotService,
	}
}

// Router returns a configured router for student endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustTokenAuth()

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
		if logger.Logger != nil {
			logger.Logger.WithError(err).Error("failed to load student data snapshot")
		}
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
		students, err := rs.StudentService.FindByGroupIDs(ctx, []int64{params.groupID})
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
	totalCount, err := rs.StudentService.CountWithOptions(ctx, countOptions)
	if err != nil {
		return nil, 0, err
	}

	// Get students
	students, err := rs.StudentService.ListWithOptions(ctx, queryOptions)
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
