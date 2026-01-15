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
	"github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
	userService "github.com/moto-nrw/project-phoenix/services/users"
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
		StudentResponse: newStudentResponseWithOpts(r.Context(), StudentResponseOpts{
			Student:       student,
			Person:        person,
			Group:         group,
			HasFullAccess: hasFullAccess,
		}, StudentResponseServices{
			ActiveService: rs.ActiveService,
			PersonService: rs.PersonService,
		}),
		HasFullAccess: hasFullAccess,
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
	if err := rs.StudentService.Create(r.Context(), student); err != nil {
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
	if err := rs.StudentService.Update(r.Context(), student); err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	// Get updated student with person data
	updatedStudent, err := rs.StudentService.Get(r.Context(), student.ID)
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

	// Delete the student first
	if err := rs.StudentService.Delete(r.Context(), student.ID); err != nil {
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
	response := newStudentResponseWithOpts(r.Context(), StudentResponseOpts{
		Student:       student,
		Person:        person,
		Group:         group,
		HasFullAccess: hasFullAccess,
	}, StudentResponseServices{
		ActiveService: rs.ActiveService,
		PersonService: rs.PersonService,
	})

	// Create location response structure
	locationResponse := struct {
		Location    string `json:"current_location"`
		CurrentRoom string `json:"current_room,omitempty"`
	}{
		Location: response.Location,
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

	student, err := rs.StudentService.Get(r.Context(), id)
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

