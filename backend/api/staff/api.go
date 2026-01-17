package staff

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// Resource defines the staff API resource
type Resource struct {
	PersonService       usersSvc.PersonService
	StaffRepo           users.StaffRepository
	TeacherRepo         users.TeacherRepository
	EducationService    educationSvc.Service
	AuthService         authSvc.AuthService
	GroupSupervisorRepo active.GroupSupervisorRepository
}

// NewResource creates a new staff resource
func NewResource(
	personService usersSvc.PersonService,
	educationService educationSvc.Service,
	authService authSvc.AuthService,
	groupSupervisorRepo active.GroupSupervisorRepository,
) *Resource {
	return &Resource{
		PersonService:       personService,
		StaffRepo:           personService.StaffRepository(),
		TeacherRepo:         personService.TeacherRepository(),
		EducationService:    educationService,
		AuthService:         authService,
		GroupSupervisorRepo: groupSupervisorRepo,
	}
}

// Router returns a configured router for staff endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Read operations only require users:read permission
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/", rs.listStaff)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}", rs.getStaff)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}/groups", rs.getStaffGroups)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}/substitutions", rs.getStaffSubstitutions)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/available", rs.getAvailableStaff)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/available-for-substitution", rs.getAvailableForSubstitution)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/by-role", rs.getStaffByRole)

		// Write operations require users:create, users:update, or users:delete permission
		r.With(authorize.RequiresPermission(permissions.UsersCreate)).Post("/", rs.createStaff)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate)).Put("/{id}", rs.updateStaff)
		r.With(authorize.RequiresPermission(permissions.UsersDelete)).Delete("/{id}", rs.deleteStaff)

		// PIN management endpoints - staff can manage their own PIN
		r.Get("/pin", rs.getPINStatus)
		r.Put("/pin", rs.updatePIN)
	})

	return r
}

// PersonResponse represents a simplified person response
type PersonResponse struct {
	ID        int64     `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email,omitempty"`
	TagID     string    `json:"tag_id,omitempty"`
	AccountID *int64    `json:"account_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StaffResponse represents a staff response
type StaffResponse struct {
	ID              int64           `json:"id"`
	PersonID        int64           `json:"person_id"`
	StaffNotes      string          `json:"staff_notes,omitempty"`
	Person          *PersonResponse `json:"person,omitempty"`
	IsTeacher       bool            `json:"is_teacher"`
	WasPresentToday bool            `json:"was_present_today"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// TeacherResponse represents a teacher response (extends staff)
type TeacherResponse struct {
	StaffResponse
	TeacherID      int64  `json:"teacher_id"`
	Specialization string `json:"specialization,omitempty"`
	Role           string `json:"role,omitempty"`
	Qualifications string `json:"qualifications,omitempty"`
}

// GroupResponse represents a simplified group response
type GroupResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// StaffRequest represents a staff creation/update request
type StaffRequest struct {
	PersonID   int64  `json:"person_id"`
	StaffNotes string `json:"staff_notes,omitempty"`
	// Teacher-specific fields for creating a teacher
	IsTeacher      bool   `json:"is_teacher,omitempty"`
	Specialization string `json:"specialization,omitempty"`
	Role           string `json:"role,omitempty"`
	Qualifications string `json:"qualifications,omitempty"`
}

// PINStatusResponse represents the PIN status response
type PINStatusResponse struct {
	HasPIN      bool       `json:"has_pin"`
	LastChanged *time.Time `json:"last_changed,omitempty"`
}

// PINUpdateRequest represents a PIN update request
type PINUpdateRequest struct {
	CurrentPIN *string `json:"current_pin,omitempty"` // null for first-time setup
	NewPIN     string  `json:"new_pin"`
}

// Bind validates the staff request
func (req *StaffRequest) Bind(_ *http.Request) error {
	if req.PersonID <= 0 {
		return errors.New("person ID is required")
	}

	req.Specialization = strings.TrimSpace(req.Specialization)
	req.Role = strings.TrimSpace(req.Role)
	req.Qualifications = strings.TrimSpace(req.Qualifications)

	return nil
}

// Bind validates the PIN update request
func (req *PINUpdateRequest) Bind(_ *http.Request) error {
	if req.NewPIN == "" {
		return errors.New("new PIN is required")
	}

	// Validate PIN format (4 digits)
	if len(req.NewPIN) != 4 {
		return errors.New("PIN must be exactly 4 digits")
	}

	// Check if PIN contains only digits
	for _, char := range req.NewPIN {
		if char < '0' || char > '9' {
			return errors.New("PIN must contain only digits")
		}
	}

	return nil
}

// newPersonResponse creates a simplified person response
func newPersonResponse(person *users.Person) *PersonResponse {
	if person == nil {
		return nil
	}

	response := &PersonResponse{
		ID:        person.ID,
		FirstName: person.FirstName,
		LastName:  person.LastName,
		AccountID: person.AccountID,
		CreatedAt: person.CreatedAt,
		UpdatedAt: person.UpdatedAt,
	}

	if person.TagID != nil {
		response.TagID = *person.TagID
	}

	if person.Account != nil {
		response.Email = person.Account.Email
	}

	return response
}

// =============================================================================
// HELPER METHODS - Reduce code duplication for common parsing/validation
// =============================================================================

// parseAndGetStaff parses staff ID from URL and returns the staff if it exists.
// Returns nil and false if parsing fails or staff doesn't exist (error already rendered).
func (rs *Resource) parseAndGetStaff(w http.ResponseWriter, r *http.Request) (*users.Staff, bool) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidStaffID)))
		return nil, false
	}

	staff, err := rs.StaffRepo.FindByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New(common.MsgStaffNotFound)))
		return nil, false
	}

	return staff, true
}

// =============================================================================
// RESPONSE HELPERS
// =============================================================================

// newStaffResponse creates a staff response
func newStaffResponse(staff *users.Staff, isTeacher bool, wasPresentToday bool) StaffResponse {
	response := StaffResponse{
		ID:              staff.ID,
		PersonID:        staff.PersonID,
		StaffNotes:      staff.StaffNotes,
		IsTeacher:       isTeacher,
		WasPresentToday: wasPresentToday,
		CreatedAt:       staff.CreatedAt,
		UpdatedAt:       staff.UpdatedAt,
	}

	if staff.Person != nil {
		response.Person = newPersonResponse(staff.Person)
	}

	return response
}

// newTeacherResponse creates a teacher response
func newTeacherResponse(staff *users.Staff, teacher *users.Teacher, wasPresentToday bool) TeacherResponse {
	staffResponse := newStaffResponse(staff, true, wasPresentToday)

	response := TeacherResponse{
		StaffResponse:  staffResponse,
		TeacherID:      teacher.ID,
		Specialization: strings.TrimSpace(teacher.Specialization),
		Role:           teacher.Role,
		Qualifications: teacher.Qualifications,
	}

	return response
}

// listStaff handles listing all staff members with optional filtering
// Optimized to avoid N+1 queries by batch-loading Person and Teacher data
func (rs *Resource) listStaff(w http.ResponseWriter, r *http.Request) {
	filters := parseListStaffFilters(r)
	ctx := r.Context()

	// Get all staff members with person data in a single query (avoids N+1)
	staffMembers, err := rs.StaffRepo.ListAllWithPerson(ctx)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Collect all staff IDs for batch teacher lookup
	staffIDs := make([]int64, len(staffMembers))
	for i, s := range staffMembers {
		staffIDs[i] = s.ID
	}

	// Batch-load all teachers in a single query (avoids N+1)
	teacherMap, err := rs.TeacherRepo.FindByStaffIDs(ctx, staffIDs)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Batch-load staff who had supervision activity today (for "Anwesend" status)
	presentStaffIDs, err := rs.GroupSupervisorRepo.GetStaffIDsWithSupervisionToday(ctx)
	if err != nil {
		// Log warning but continue - presence status is non-critical
		log.Printf("Warning: failed to fetch present staff IDs: %v", err)
		presentStaffIDs = []int64{}
	}

	// Build presentMap for O(1) lookup
	presentMap := make(map[int64]bool, len(presentStaffIDs))
	for _, id := range presentStaffIDs {
		presentMap[id] = true
	}

	// Build response objects using pre-loaded data
	responses := make([]interface{}, 0, len(staffMembers))
	for _, staff := range staffMembers {
		if response, include := rs.processStaffForListOptimized(ctx, staff, teacherMap, presentMap, filters); include {
			responses = append(responses, response)
		}
	}

	common.Respond(w, r, http.StatusOK, responses, "Staff members retrieved successfully")
}

// getStaff handles getting a staff member by ID
func (rs *Resource) getStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}

	// Get staff member with person data using FindWithPerson method
	staff, err := rs.StaffRepo.FindWithPerson(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New(common.MsgStaffNotFound)))
		return
	}

	// If person data was not loaded by FindWithPerson, try to fetch it separately
	if staff.Person == nil && staff.PersonID > 0 {
		person, err := rs.PersonService.Get(r.Context(), staff.PersonID)
		if err != nil {
			log.Printf("Warning: failed to get person data for staff member %d: %v", id, err)
			// Don't fail the request, just log the warning
		} else {
			staff.Person = person
		}
	}

	// Check if this staff member is also a teacher
	isTeacher := false
	var teacher *users.Teacher

	teacher, err = rs.TeacherRepo.FindByStaffID(r.Context(), staff.ID)
	if err == nil && teacher != nil {
		// Create teacher response (false for wasPresentToday - individual GET doesn't need this)
		response := newTeacherResponse(staff, teacher, false)
		common.Respond(w, r, http.StatusOK, response, "Teacher retrieved successfully")
		return
	}

	// Create staff response (false for wasPresentToday - individual GET doesn't need this)
	response := newStaffResponse(staff, isTeacher, false)
	common.Respond(w, r, http.StatusOK, response, "Staff member retrieved successfully")
}

// grantDefaultPermissions grants default permissions to a newly created account
func (rs *Resource) grantDefaultPermissions(ctx context.Context, accountID int64, role string) {
	if rs.AuthService == nil {
		return
	}

	// Get the groups:read permission
	perm, err := rs.AuthService.GetPermissionByName(ctx, permissions.GroupsRead)
	if err == nil && perm != nil {
		// Grant the permission to the account
		if err := rs.AuthService.GrantPermissionToAccount(ctx, int(accountID), int(perm.ID)); err != nil {
			log.Printf("Failed to grant groups:read permission to %s account %d: %v", role, accountID, err)
		}
	}
}

// createStaff handles creating a new staff member
func (rs *Resource) createStaff(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &StaffRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Verify person exists
	person, err := rs.PersonService.Get(r.Context(), req.PersonID)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New("person not found")))
		return
	}

	// Create staff
	staff := &users.Staff{
		PersonID:   req.PersonID,
		StaffNotes: req.StaffNotes,
	}

	// Create staff record
	if err := rs.StaffRepo.Create(r.Context(), staff); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Set person data for response
	staff.Person = person

	// If request indicates this is a teacher, create teacher record as well
	isTeacher := req.IsTeacher
	var teacher *users.Teacher

	if isTeacher {
		teacher = &users.Teacher{
			StaffID:        staff.ID,
			Specialization: strings.TrimSpace(req.Specialization),
			Role:           req.Role,
			Qualifications: req.Qualifications,
		}

		if rs.TeacherRepo.Create(r.Context(), teacher) != nil {
			// Still return staff member even if teacher creation fails
			isTeacher = false
			response := newStaffResponse(staff, isTeacher, false)
			common.Respond(w, r, http.StatusCreated, response, "Staff member created successfully, but failed to create teacher record")
			return
		}

		// Grant groups:read permission to teacher if they have an account
		if person.AccountID != nil {
			rs.grantDefaultPermissions(r.Context(), *person.AccountID, "teacher")
		}

		// Return teacher response
		response := newTeacherResponse(staff, teacher, false)
		common.Respond(w, r, http.StatusCreated, response, "Teacher created successfully")
		return
	}

	// Grant groups:read permission to staff if they have an account
	if person.AccountID != nil {
		rs.grantDefaultPermissions(r.Context(), *person.AccountID, "staff")
	}

	// Return staff response
	response := newStaffResponse(staff, isTeacher, false)
	common.Respond(w, r, http.StatusCreated, response, "Staff member created successfully")
}

// updateStaff handles updating a staff member
func (rs *Resource) updateStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}

	req := &StaffRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	staff, err := rs.StaffRepo.FindByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New(common.MsgStaffNotFound)))
		return
	}

	// Update basic fields
	staff.StaffNotes = req.StaffNotes

	// Handle person ID change
	if staff.PersonID != req.PersonID {
		if rs.updateStaffPerson(r.Context(), staff, req.PersonID) != nil {
			common.RenderError(w, r, ErrorNotFound(errors.New("person not found")))
			return
		}
	}

	if err := rs.StaffRepo.Update(r.Context(), staff); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Reload staff with person data
	rs.reloadStaffWithPerson(r.Context(), staff, id)

	// Get existing teacher record if any
	teacher, _ := rs.TeacherRepo.FindByStaffID(r.Context(), staff.ID)

	// Handle teacher record based on request
	response, message := rs.buildUpdateStaffResponse(r.Context(), staff, req, teacher)
	common.Respond(w, r, http.StatusOK, response, message)
}

// updateStaffPerson validates and updates the person ID for a staff member
func (rs *Resource) updateStaffPerson(ctx context.Context, staff *users.Staff, personID int64) error {
	person, err := rs.PersonService.Get(ctx, personID)
	if err != nil {
		return err
	}
	staff.PersonID = personID
	staff.Person = person
	return nil
}

// reloadStaffWithPerson attempts to reload staff with person data
func (rs *Resource) reloadStaffWithPerson(ctx context.Context, staff *users.Staff, id int64) {
	reloaded, err := rs.StaffRepo.FindWithPerson(ctx, id)
	if err == nil {
		*staff = *reloaded
		return
	}
	// Fallback: load person separately
	if staff.Person == nil && staff.PersonID > 0 {
		if person, err := rs.PersonService.Get(ctx, staff.PersonID); err == nil {
			staff.Person = person
		}
	}
}

// buildUpdateStaffResponse builds the appropriate response for staff update
func (rs *Resource) buildUpdateStaffResponse(
	ctx context.Context,
	staff *users.Staff,
	req *StaffRequest,
	existingTeacher *users.Teacher,
) (interface{}, string) {
	// Handle teacher record creation/update
	if req.IsTeacher {
		response, message, _ := rs.handleTeacherRecordUpdate(ctx, staff, req, existingTeacher)
		return response, message
	}

	// Return existing teacher response if they have a teacher record
	if existingTeacher != nil {
		return newTeacherResponse(staff, existingTeacher, false), "Teacher updated successfully"
	}

	return newStaffResponse(staff, false, false), "Staff member updated successfully"
}

// deleteStaff handles deleting a staff member
func (rs *Resource) deleteStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}

	// Check if this staff member is also a teacher
	teacher, err := rs.TeacherRepo.FindByStaffID(r.Context(), id)
	if err == nil && teacher != nil {
		// Delete teacher record first
		if rs.TeacherRepo.Delete(r.Context(), teacher.ID) != nil {
			common.RenderError(w, r, ErrorInternalServer(errors.New("failed to delete teacher record")))
			return
		}
	}

	// Delete staff member
	if err := rs.StaffRepo.Delete(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Staff member deleted successfully")
}

// getStaffGroups handles getting groups for a staff member
func (rs *Resource) getStaffGroups(w http.ResponseWriter, r *http.Request) {
	// Parse and get staff
	staff, ok := rs.parseAndGetStaff(w, r)
	if !ok {
		return
	}

	// Check if this staff member is a teacher
	teacher, err := rs.TeacherRepo.FindByStaffID(r.Context(), staff.ID)
	if err != nil || teacher == nil {
		// If not a teacher, return empty groups list
		common.Respond(w, r, http.StatusOK, []GroupResponse{}, "Staff member is not a teacher and has no assigned groups")
		return
	}

	// Check if we have a reference to the Education service
	if rs.EducationService == nil {
		// If not, return an error
		common.RenderError(w, r, ErrorInternalServer(errors.New("education service not available")))
		return
	}

	// Get groups for this teacher
	groups, err := rs.EducationService.GetTeacherGroups(r.Context(), teacher.ID)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response
	responses := make([]GroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, GroupResponse{
			ID:   group.ID,
			Name: group.Name,
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Teacher groups retrieved successfully")
}

// getAvailableStaff handles getting available staff members (teachers) for assignments
// Optimized to avoid N+1 queries by using ListAllWithStaffAndPerson
func (rs *Resource) getAvailableStaff(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get all teachers with staff and person data in a single query (avoids N+1)
	teachers, err := rs.TeacherRepo.ListAllWithStaffAndPerson(ctx)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response objects - all returned items are teachers by definition
	responses := make([]TeacherResponse, 0, len(teachers))

	for _, teacher := range teachers {
		// Skip if staff or person data is missing
		if teacher.Staff == nil || teacher.Staff.Person == nil {
			continue
		}

		// Create teacher response using pre-loaded data (false for wasPresentToday - not needed here)
		responses = append(responses, newTeacherResponse(teacher.Staff, teacher, false))
	}

	common.Respond(w, r, http.StatusOK, responses, "Available staff members retrieved successfully")
}

// getStaffSubstitutions handles getting substitutions for a staff member
func (rs *Resource) getStaffSubstitutions(w http.ResponseWriter, r *http.Request) {
	// Parse and get staff
	staff, ok := rs.parseAndGetStaff(w, r)
	if !ok {
		return
	}

	// Check if we have a reference to the Education service
	if rs.EducationService == nil {
		// If not, return an error
		common.RenderError(w, r, ErrorInternalServer(errors.New("education service not available")))
		return
	}

	// Get substitutions where this staff member is the substitute
	substitutions, err := rs.EducationService.GetStaffSubstitutions(r.Context(), staff.ID, false)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, substitutions, "Staff substitutions retrieved successfully")
}

// SubstitutionInfo represents a single substitution with transfer indicator
type SubstitutionInfo struct {
	ID         int64            `json:"id"`
	GroupID    int64            `json:"group_id"`
	GroupName  string           `json:"group_name,omitempty"`
	IsTransfer bool             `json:"is_transfer"`
	StartDate  string           `json:"start_date"`
	EndDate    string           `json:"end_date"`
	Group      *education.Group `json:"group,omitempty"`
}

// StaffWithSubstitutionStatus represents a staff member with their substitution status
type StaffWithSubstitutionStatus struct {
	*StaffResponse
	IsSubstituting    bool               `json:"is_substituting"`
	SubstitutionCount int                `json:"substitution_count"`
	Substitutions     []SubstitutionInfo `json:"substitutions,omitempty"`
	CurrentGroup      *education.Group   `json:"current_group,omitempty"`
	RegularGroup      *education.Group   `json:"regular_group,omitempty"`
	TeacherID         int64              `json:"teacher_id,omitempty"`
	Specialization    string             `json:"specialization,omitempty"`
	Role              string             `json:"role,omitempty"`
	Qualifications    string             `json:"qualifications,omitempty"`
}

// buildSubstitutionInfoList creates substitution info from active substitutions
func buildSubstitutionInfoList(subs []*education.GroupSubstitution) []SubstitutionInfo {
	result := make([]SubstitutionInfo, 0, len(subs))
	for _, sub := range subs {
		info := SubstitutionInfo{
			ID:         sub.ID,
			GroupID:    sub.GroupID,
			IsTransfer: sub.Duration() == 1,
			StartDate:  sub.StartDate.Format(common.DateFormatISO),
			EndDate:    sub.EndDate.Format(common.DateFormatISO),
		}
		if sub.Group != nil {
			info.GroupName = sub.Group.Name
			info.Group = sub.Group
		}
		result = append(result, info)
	}
	return result
}

// buildStaffSubstitutionStatus creates a staff status entry with substitution data
func (rs *Resource) buildStaffSubstitutionStatus(
	ctx context.Context,
	staff *users.Staff,
	teacher *users.Teacher,
	subs []*education.GroupSubstitution,
) StaffWithSubstitutionStatus {
	staffResp := newStaffResponse(staff, false, false)
	result := StaffWithSubstitutionStatus{
		StaffResponse:     &staffResp,
		IsSubstituting:    len(subs) > 0,
		SubstitutionCount: len(subs),
		Substitutions:     []SubstitutionInfo{},
		TeacherID:         teacher.ID,
		Specialization:    teacher.Specialization,
		Role:              teacher.Role,
		Qualifications:    teacher.Qualifications,
	}

	if len(subs) > 0 {
		result.Substitutions = buildSubstitutionInfoList(subs)
		if subs[0].Group != nil {
			result.CurrentGroup = subs[0].Group
		}
	}

	// Find regular group for this teacher
	if rs.EducationService != nil {
		groups, err := rs.EducationService.GetTeacherGroups(ctx, teacher.ID)
		if err == nil && len(groups) > 0 {
			result.RegularGroup = groups[0]
		}
	}

	return result
}

// matchesSearchTerm checks if staff member matches the search filter
func matchesSearchTerm(person *users.Person, searchTerm string) bool {
	if searchTerm == "" {
		return true
	}
	return containsIgnoreCase(person.FirstName, searchTerm) ||
		containsIgnoreCase(person.LastName, searchTerm)
}

// getAvailableForSubstitution handles getting staff available for substitution with their current status
// Optimized to avoid N+1 queries by using ListAllWithStaffAndPerson
func (rs *Resource) getAvailableForSubstitution(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	searchTerm := r.URL.Query().Get("search")
	ctx := r.Context()

	date := time.Now()
	if dateStr != "" {
		if parsedDate, err := time.Parse(common.DateFormatISO, dateStr); err == nil {
			date = parsedDate
		}
	}

	// Get all teachers with staff and person data in a single query (avoids N+1)
	teachers, err := rs.TeacherRepo.ListAllWithStaffAndPerson(ctx)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	substitutingStaffMap := rs.buildSubstitutionMap(ctx, date)
	results := rs.filterAndBuildTeacherResults(ctx, teachers, substitutingStaffMap, searchTerm)

	common.Respond(w, r, http.StatusOK, results, "Available staff for substitution retrieved successfully")
}

// buildSubstitutionMap creates a map of staff IDs to their active substitutions
func (rs *Resource) buildSubstitutionMap(ctx context.Context, date time.Time) map[int64][]*education.GroupSubstitution {
	result := make(map[int64][]*education.GroupSubstitution)
	if rs.EducationService == nil {
		return result
	}

	activeSubstitutions, _ := rs.EducationService.GetActiveSubstitutions(ctx, date)
	for _, sub := range activeSubstitutions {
		result[sub.SubstituteStaffID] = append(result[sub.SubstituteStaffID], sub)
	}
	return result
}

// filterAndBuildTeacherResults filters teachers and builds response entries
// Optimized version that uses pre-loaded Teacher/Staff/Person data
func (rs *Resource) filterAndBuildTeacherResults(
	ctx context.Context,
	teachers []*users.Teacher,
	subsMap map[int64][]*education.GroupSubstitution,
	searchTerm string,
) []StaffWithSubstitutionStatus {
	results := make([]StaffWithSubstitutionStatus, 0, len(teachers))

	for _, teacher := range teachers {
		result := rs.processTeacherForSubstitution(ctx, teacher, subsMap, searchTerm)
		if result != nil {
			results = append(results, *result)
		}
	}
	return results
}

// processTeacherForSubstitution processes a teacher with pre-loaded data for the substitution list
// Uses pre-loaded Staff and Person data to avoid N+1 queries
func (rs *Resource) processTeacherForSubstitution(
	ctx context.Context,
	teacher *users.Teacher,
	subsMap map[int64][]*education.GroupSubstitution,
	searchTerm string,
) *StaffWithSubstitutionStatus {
	// Skip if staff or person data is missing
	if teacher.Staff == nil || teacher.Staff.Person == nil {
		return nil
	}

	// Apply search filter using pre-loaded person data
	if !matchesSearchTerm(teacher.Staff.Person, searchTerm) {
		return nil
	}

	subs := subsMap[teacher.Staff.ID]
	result := rs.buildStaffSubstitutionStatus(ctx, teacher.Staff, teacher, subs)
	return &result
}

// Helper function to check if a string contains another string, ignoring case
func containsIgnoreCase(s, substr string) bool {
	s, substr = strings.ToLower(s), strings.ToLower(substr)
	return strings.Contains(s, substr)
}

// getPINStatus handles getting the current user's PIN status
func (rs *Resource) getPINStatus(w http.ResponseWriter, r *http.Request) {
	// Get user from JWT context
	userClaims := jwt.ClaimsFromCtx(r.Context())
	if userClaims.ID == 0 {
		common.RenderError(w, r, ErrorUnauthorized(errors.New("invalid token")))
		return
	}

	// Get account directly
	account, err := rs.AuthService.GetAccountByID(r.Context(), userClaims.ID)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New("account not found")))
		return
	}

	// Ensure the account belongs to a staff member (admins without person records are allowed)
	person, err := rs.PersonService.FindByAccountID(r.Context(), int64(account.ID))
	if err == nil && person != nil {
		if _, err := rs.StaffRepo.FindByPersonID(r.Context(), person.ID); err != nil {
			common.RenderError(w, r, ErrorForbidden(errors.New("only staff members can access PIN settings")))
			return
		}
	}

	// Build response using account PIN data
	response := PINStatusResponse{
		HasPIN: account.HasPIN(),
	}

	// Include last changed timestamp if available (use UpdatedAt as proxy)
	if account.HasPIN() {
		response.LastChanged = &account.UpdatedAt
	}

	common.Respond(w, r, http.StatusOK, response, "PIN status retrieved successfully")
}

// updatePIN handles updating the current user's PIN
func (rs *Resource) updatePIN(w http.ResponseWriter, r *http.Request) {
	req := &PINUpdateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	userClaims := jwt.ClaimsFromCtx(r.Context())
	if userClaims.ID == 0 {
		common.RenderError(w, r, ErrorUnauthorized(errors.New("invalid token")))
		return
	}

	account, err := rs.AuthService.GetAccountByID(r.Context(), userClaims.ID)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New("account not found")))
		return
	}

	// Validate access
	if renderErr := rs.checkAccountLocked(account); renderErr != nil {
		common.RenderError(w, r, renderErr)
		return
	}
	if renderErr := rs.checkStaffPINAccess(r.Context(), int64(account.ID)); renderErr != nil {
		common.RenderError(w, r, renderErr)
		return
	}

	// Verify current PIN if exists
	result, renderErr := verifyCurrentPIN(account, req.CurrentPIN)
	if renderErr != nil {
		// Only increment attempts for actual verification failures, not missing input
		if result == pinVerificationFailed {
			account.IncrementPINAttempts()
			if updateErr := rs.AuthService.UpdateAccount(r.Context(), account); updateErr != nil {
				log.Printf("Failed to update account PIN attempts: %v", updateErr)
			}
		}
		common.RenderError(w, r, renderErr)
		return
	}

	// Set new PIN
	if account.HashPIN(req.NewPIN) != nil {
		common.RenderError(w, r, ErrorInternalServer(errors.New("failed to hash PIN")))
		return
	}
	account.ResetPINAttempts()

	if err := rs.AuthService.UpdateAccount(r.Context(), account); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "PIN updated successfully",
	}, "PIN updated successfully")
}

// checkAccountLocked checks if account is PIN locked
func (rs *Resource) checkAccountLocked(account interface{ IsPINLocked() bool }) render.Renderer {
	if account.IsPINLocked() {
		return ErrorForbidden(errors.New("account is temporarily locked due to failed PIN attempts"))
	}
	return nil
}

// checkStaffPINAccess verifies the account belongs to a staff member
func (rs *Resource) checkStaffPINAccess(ctx context.Context, accountID int64) render.Renderer {
	person, err := rs.PersonService.FindByAccountID(ctx, accountID)
	if err != nil || person == nil {
		return nil // No person = likely admin, allow
	}

	if _, err := rs.StaffRepo.FindByPersonID(ctx, person.ID); err != nil {
		return ErrorForbidden(errors.New("only staff members can manage PIN settings"))
	}
	return nil
}

// verifyCurrentPIN validates current PIN when updating existing PIN
// pinVerificationResult indicates the outcome of PIN verification
type pinVerificationResult int

const (
	pinVerificationNotRequired  pinVerificationResult = iota // No PIN exists, verification skipped
	pinVerificationMissingInput                              // PIN required but input was missing (validation error)
	pinVerificationFailed                                    // PIN provided but incorrect (auth failure)
	pinVerificationPassed                                    // PIN verified successfully
)

// verifyCurrentPIN checks the current PIN and returns both the result type and any error
func verifyCurrentPIN(account interface {
	HasPIN() bool
	VerifyPIN(string) bool
}, currentPIN *string) (pinVerificationResult, render.Renderer) {
	if !account.HasPIN() {
		return pinVerificationNotRequired, nil
	}

	if currentPIN == nil || *currentPIN == "" {
		return pinVerificationMissingInput, ErrorInvalidRequest(errors.New("current PIN is required when updating existing PIN"))
	}

	if !account.VerifyPIN(*currentPIN) {
		return pinVerificationFailed, ErrorUnauthorized(errors.New("current PIN is incorrect"))
	}
	return pinVerificationPassed, nil
}

// StaffWithRoleResponse represents a staff member with role information
type StaffWithRoleResponse struct {
	ID        int64     `json:"id"`
	PersonID  int64     `json:"person_id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	FullName  string    `json:"full_name"`
	AccountID int64     `json:"account_id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// getStaffByRole handles GET /api/staff/by-role?role=user
// Returns staff members filtered by account role (useful for group transfer dropdowns)
// Partially optimized: uses ListAllWithPerson to avoid N+1 for person data
// Note: Account and role lookups still require per-staff queries (TODO: batch load accounts/roles)
func (rs *Resource) getStaffByRole(w http.ResponseWriter, r *http.Request) {
	roleName := r.URL.Query().Get("role")
	if roleName == "" {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("role parameter is required")))
		return
	}

	ctx := r.Context()

	// Get all staff with person data in a single query (avoids N+1 for person loading)
	staff, err := rs.StaffRepo.ListAllWithPerson(ctx)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	results := rs.filterStaffByRoleOptimized(ctx, staff, roleName)
	common.Respond(w, r, http.StatusOK, results, "Staff members with role retrieved successfully")
}

// filterStaffByRoleOptimized filters staff members by role using pre-loaded person data
func (rs *Resource) filterStaffByRoleOptimized(ctx context.Context, staff []*users.Staff, roleName string) []StaffWithRoleResponse {
	var results []StaffWithRoleResponse

	for _, s := range staff {
		entry := rs.buildStaffRoleEntryOptimized(ctx, s, roleName)
		if entry != nil {
			results = append(results, *entry)
		}
	}
	return results
}

// buildStaffRoleEntryOptimized creates a role response entry using pre-loaded person data
// Note: Account and role lookups still require DB queries (TODO: batch load)
func (rs *Resource) buildStaffRoleEntryOptimized(ctx context.Context, s *users.Staff, roleName string) *StaffWithRoleResponse {
	// Person is already loaded via ListAllWithPerson
	if s.Person == nil || s.Person.AccountID == nil {
		return nil
	}

	account, err := rs.AuthService.GetAccountByID(ctx, int(*s.Person.AccountID))
	if err != nil || account == nil {
		return nil
	}

	if !rs.accountHasRole(ctx, account.ID, roleName) {
		return nil
	}

	return &StaffWithRoleResponse{
		ID:        s.ID,
		PersonID:  s.Person.ID,
		FirstName: s.Person.FirstName,
		LastName:  s.Person.LastName,
		FullName:  s.Person.FirstName + " " + s.Person.LastName,
		AccountID: *s.Person.AccountID,
		Email:     account.Email,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

// accountHasRole checks if an account has a specific role
func (rs *Resource) accountHasRole(ctx context.Context, accountID int64, roleName string) bool {
	roles, err := rs.AuthService.GetAccountRoles(ctx, int(accountID))
	if err != nil {
		return false
	}

	for _, role := range roles {
		if role.Name == roleName {
			return true
		}
	}
	return false
}

// =============================================================================
// EXPORTED HANDLERS FOR TESTING
// =============================================================================

// ListStaffHandler returns the listStaff handler for testing.
func (rs *Resource) ListStaffHandler() http.HandlerFunc { return rs.listStaff }

// GetStaffHandler returns the getStaff handler for testing.
func (rs *Resource) GetStaffHandler() http.HandlerFunc { return rs.getStaff }

// CreateStaffHandler returns the createStaff handler for testing.
func (rs *Resource) CreateStaffHandler() http.HandlerFunc { return rs.createStaff }

// UpdateStaffHandler returns the updateStaff handler for testing.
func (rs *Resource) UpdateStaffHandler() http.HandlerFunc { return rs.updateStaff }

// DeleteStaffHandler returns the deleteStaff handler for testing.
func (rs *Resource) DeleteStaffHandler() http.HandlerFunc { return rs.deleteStaff }

// GetStaffGroupsHandler returns the getStaffGroups handler for testing.
func (rs *Resource) GetStaffGroupsHandler() http.HandlerFunc { return rs.getStaffGroups }

// GetStaffSubstitutionsHandler returns the getStaffSubstitutions handler for testing.
func (rs *Resource) GetStaffSubstitutionsHandler() http.HandlerFunc { return rs.getStaffSubstitutions }

// GetAvailableStaffHandler returns the getAvailableStaff handler for testing.
func (rs *Resource) GetAvailableStaffHandler() http.HandlerFunc { return rs.getAvailableStaff }

// GetAvailableForSubstitutionHandler returns the getAvailableForSubstitution handler for testing.
func (rs *Resource) GetAvailableForSubstitutionHandler() http.HandlerFunc {
	return rs.getAvailableForSubstitution
}

// GetStaffByRoleHandler returns the getStaffByRole handler for testing.
func (rs *Resource) GetStaffByRoleHandler() http.HandlerFunc { return rs.getStaffByRole }

// GetPINStatusHandler returns the getPINStatus handler for testing.
func (rs *Resource) GetPINStatusHandler() http.HandlerFunc { return rs.getPINStatus }

// UpdatePINHandler returns the updatePIN handler for testing.
func (rs *Resource) UpdatePINHandler() http.HandlerFunc { return rs.updatePIN }
