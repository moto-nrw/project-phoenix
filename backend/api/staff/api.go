package staff

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/active"
	authmodel "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

var errScheduleValidation = errors.New("schedule validation")

type scheduleValidationError struct {
	message string
}

func (e scheduleValidationError) Error() string {
	return e.message
}

func (e scheduleValidationError) Is(target error) bool {
	return target == errScheduleValidation
}

func scheduleValidationErrorf(format string, args ...any) error {
	return scheduleValidationError{message: fmt.Sprintf(format, args...)}
}

// Resource defines the staff API resource
type Resource struct {
	PersonService           usersSvc.PersonService
	StaffOffboardingService usersSvc.StaffOffboardingService
	StaffRepo               users.StaffRepository
	TeacherRepo             users.TeacherRepository
	EducationService        educationSvc.Service
	AuthService             authSvc.AuthService
	GroupSupervisorRepo     active.GroupSupervisorRepository
	WorkSessionService      activeSvc.WorkSessionService
	StaffAbsenceService     activeSvc.StaffAbsenceService
	AbsenceRepo             active.StaffAbsenceRepository
	ScheduleRepo            config.StaffWorkScheduleRepository
	WorkTimeModelRepo       config.WorkTimeModelRepository
	db                      *bun.DB
	logger                  *slog.Logger
}

// NewResource creates a new staff resource
func NewResource(
	personService usersSvc.PersonService,
	staffOffboardingService usersSvc.StaffOffboardingService,
	educationService educationSvc.Service,
	authService authSvc.AuthService,
	groupSupervisorRepo active.GroupSupervisorRepository,
	workSessionService activeSvc.WorkSessionService,
	staffAbsenceService activeSvc.StaffAbsenceService,
	absenceRepo active.StaffAbsenceRepository,
	scheduleRepo config.StaffWorkScheduleRepository,
	workTimeModelRepo config.WorkTimeModelRepository,
	db *bun.DB,
	logger *slog.Logger,
) *Resource {
	return &Resource{
		PersonService:           personService,
		StaffOffboardingService: staffOffboardingService,
		StaffRepo:               personService.StaffRepository(),
		TeacherRepo:             personService.TeacherRepository(),
		EducationService:        educationService,
		AuthService:             authService,
		GroupSupervisorRepo:     groupSupervisorRepo,
		WorkSessionService:      workSessionService,
		StaffAbsenceService:     staffAbsenceService,
		AbsenceRepo:             absenceRepo,
		ScheduleRepo:            scheduleRepo,
		WorkTimeModelRepo:       workTimeModelRepo,
		db:                      db,
		logger:                  logger,
	}
}

// getLogger returns the injected logger, falling back to slog.Default()
func (rs *Resource) getLogger() *slog.Logger {
	if rs.logger != nil {
		return rs.logger
	}
	return slog.Default()
}

// Router returns a configured router for staff endpoints
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

		// Read operations only require users:read permission
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/", rs.listStaff)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}", rs.getStaff)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/avatar", rs.serveStaffAvatar)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/groups", rs.getStaffGroups)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/substitutions", rs.getStaffSubstitutions)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/available", rs.getAvailableStaff)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/available-for-substitution", rs.getAvailableForSubstitution)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/by-role", rs.getStaffByRole)

		// Write operations require users:create, users:update, or users:delete permission
		r.With(authorize.RequiresPermission(permissions.UsersCreate), withTx).Post("/", rs.createStaff)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}", rs.updateStaff)
		r.With(authorize.RequiresPermission(permissions.UsersDelete), withTx).Delete("/{id}", rs.deleteStaff)

		// Work schedule endpoints expose contractual target hours.
		r.With(authorize.RequiresAnyPermission(permissions.TimeTrackingManage, permissions.TimeTrackingOwn), withTx).Get("/{id}/schedule", rs.getSchedule)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Put("/{id}/schedule", rs.updateSchedule)

		// Time tracking history for a specific staff member (admin read)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/{id}/time-tracking/history", rs.getStaffHistory)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/{id}/time-tracking/export", rs.exportStaffSessions)

		// Vacation workflow admin-side (Tranche 4)
		r.With(authorize.RequiresPermission(permissions.VacationApprove), withTx).Get("/absences/pending", rs.listPendingAbsenceRequests)
		r.With(authorize.RequiresPermission(permissions.VacationApprove), withTx).Post("/absences/{absenceId}/approve", rs.approveAbsence)
		r.With(authorize.RequiresPermission(permissions.VacationApprove), withTx).Post("/absences/{absenceId}/deny", rs.denyAbsence)
		r.With(authorize.RequiresPermission(permissions.VacationApprove), withTx).Get("/{id}/vacation/quota", rs.getStaffVacationQuota)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/vacation/quota", rs.setStaffVacationQuota)
		// Audit trail of a single work session, admin-facing. The MA-side
		// /api/time-tracking/{id}/edits enforces session-staff ownership
		// against the JWT subject; here the route guarantees the session
		// belongs to the staff named in the URL instead.
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/{id}/time-tracking/sessions/{sessionId}/edits", rs.getStaffSessionEdits)

		// Admin cross-staff corrections. time_tracking:manage is the gate.
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Put("/{id}/time-tracking/sessions/{sessionId}", rs.adminUpdateStaffSession)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Post("/{id}/time-tracking/sessions", rs.adminCreateStaffSession)

		// Absences for a specific staff member (admin read). The MA-Sicht uses
		// /api/time-tracking/absences which is scoped to the caller; this route
		// lets an admin see Krank/Urlaub for any staff in the same tenant.
		r.With(authorize.RequiresPermission(permissions.VacationApprove), withTx).Get("/{id}/absences", rs.getStaffAbsences)

		// PIN management endpoints - staff can manage their own PIN
		r.With(withTx).Get("/pin", rs.getPINStatus)
		r.With(withTx).Put("/pin", rs.updatePIN)
	})

	return r
}

// PersonResponse represents a simplified person response
type PersonResponse struct {
	ID        int64     `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email,omitempty"`
	Avatar    string    `json:"avatar,omitempty"`
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
	WorkStatus      string          `json:"work_status,omitempty"`
	AbsenceType     string          `json:"absence_type,omitempty"`
	AccountRole     string          `json:"account_role,omitempty"`
	EmploymentType  *string         `json:"employment_type,omitempty"`
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
func newPersonResponse(person *users.Person, email string, avatar string) *PersonResponse {
	if person == nil {
		return nil
	}

	response := &PersonResponse{
		ID:        person.ID,
		FirstName: person.FirstName,
		LastName:  person.LastName,
		Email:     email,
		Avatar:    avatar,
		AccountID: person.AccountID,
		CreatedAt: person.CreatedAt,
		UpdatedAt: person.UpdatedAt,
	}

	if person.TagID != nil {
		response.TagID = *person.TagID
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

func (rs *Resource) listActiveCaregivers(ctx context.Context) ([]*users.ActiveCaregiver, error) {
	directory, err := usersSvc.CaregiverDirectoryFromPersonService(rs.PersonService)
	if err != nil {
		return nil, fmt.Errorf("resolve caregiver directory: %w", err)
	}

	return directory.ListActiveCaregivers(ctx)
}

func caregiverStaffIDSet(caregivers []*users.ActiveCaregiver) map[int64]struct{} {
	result := make(map[int64]struct{}, len(caregivers))
	for _, caregiver := range caregivers {
		result[caregiver.StaffID] = struct{}{}
	}
	return result
}

func requestedCaregiverPool(roles []string) bool {
	if len(roles) != 1 {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(roles[0])) {
	case "user", "staff":
		return true
	default:
		return false
	}
}

// =============================================================================
// RESPONSE HELPERS
// =============================================================================

// newStaffResponse creates a staff response
func newStaffResponse(staff *users.Staff, isTeacher bool, wasPresentToday bool, workStatus string, absenceType string, accountRole string, email string, avatar string) StaffResponse {
	response := StaffResponse{
		ID:              staff.ID,
		PersonID:        staff.PersonID,
		StaffNotes:      staff.StaffNotes,
		IsTeacher:       isTeacher,
		WasPresentToday: wasPresentToday,
		WorkStatus:      workStatus,
		AbsenceType:     absenceType,
		AccountRole:     accountRole,
		EmploymentType:  staff.EmploymentType,
		CreatedAt:       staff.CreatedAt,
		UpdatedAt:       staff.UpdatedAt,
	}

	if staff.Person != nil {
		response.Person = newPersonResponse(staff.Person, email, avatar)
	}

	return response
}

// newTeacherResponse creates a teacher response
func newTeacherResponse(staff *users.Staff, teacher *users.Teacher, wasPresentToday bool, workStatus string, absenceType string, accountRole string, email string, avatar string) TeacherResponse {
	staffResponse := newStaffResponse(staff, true, wasPresentToday, workStatus, absenceType, accountRole, email, avatar)

	response := TeacherResponse{
		StaffResponse:  staffResponse,
		TeacherID:      teacher.ID,
		Specialization: strings.TrimSpace(teacher.Specialization),
		Role:           teacher.Role,
		Qualifications: teacher.Qualifications,
	}

	return response
}

// loadWorkStatusMap loads work session status map (non-critical, returns empty map on error)
func (rs *Resource) loadWorkStatusMap(ctx context.Context) map[int64]string {
	if rs.WorkSessionService == nil {
		return make(map[int64]string)
	}
	wsm, err := rs.WorkSessionService.GetTodayPresenceMap(ctx)
	if err != nil {
		rs.getLogger().Warn("failed to fetch work status map", slog.String("error", err.Error()))
		return make(map[int64]string)
	}
	return wsm
}

// loadAbsenceMap loads absence status map (non-critical, returns empty map on error)
func (rs *Resource) loadAbsenceMap(ctx context.Context) map[int64]string {
	if rs.AbsenceRepo == nil {
		return make(map[int64]string)
	}
	am, err := rs.AbsenceRepo.GetTodayAbsenceMap(ctx)
	if err != nil {
		rs.getLogger().Warn("failed to fetch absence map", slog.String("error", err.Error()))
		return make(map[int64]string)
	}
	return am
}

// loadAccountRoleMap batch-loads auth role names for all staff members (non-critical, returns empty map on error)
func (rs *Resource) loadAccountRoleMap(ctx context.Context, staffMembers []*users.Staff) map[int64]string {
	if rs.AuthService == nil {
		return make(map[int64]string)
	}

	// Collect account IDs from staff members
	accountIDs := make([]int64, 0, len(staffMembers))
	for _, s := range staffMembers {
		if s.Person != nil && s.Person.AccountID != nil {
			accountIDs = append(accountIDs, *s.Person.AccountID)
		}
	}

	if len(accountIDs) == 0 {
		return make(map[int64]string)
	}

	roleMap, err := rs.AuthService.GetAccountRoleNames(ctx, accountIDs)
	if err != nil {
		rs.getLogger().Warn("failed to fetch account role map", slog.String("error", err.Error()))
		return make(map[int64]string)
	}
	return roleMap
}

// loadAccountEmailMap batch-loads email addresses for all staff members (non-critical, returns empty map on error)
func (rs *Resource) loadAccountEmailMap(ctx context.Context, staffMembers []*users.Staff) map[int64]string {
	if rs.AuthService == nil {
		return make(map[int64]string)
	}

	// Collect account IDs from staff members
	accountIDs := make([]int64, 0, len(staffMembers))
	for _, s := range staffMembers {
		if s.Person != nil && s.Person.AccountID != nil {
			accountIDs = append(accountIDs, *s.Person.AccountID)
		}
	}

	if len(accountIDs) == 0 {
		return make(map[int64]string)
	}

	emailMap, err := rs.AuthService.GetAccountEmailsByIDs(ctx, accountIDs)
	if err != nil {
		rs.getLogger().Warn("failed to fetch account email map", slog.String("error", err.Error()))
		return make(map[int64]string)
	}
	return emailMap
}

// loadAccountAvatarMap batch-loads avatar paths for all staff members (non-critical, returns empty map on error)
func (rs *Resource) loadAccountAvatarMap(ctx context.Context, staffMembers []*users.Staff) map[int64]string {
	if rs.AuthService == nil {
		return make(map[int64]string)
	}

	// Collect account IDs from staff members
	accountIDs := make([]int64, 0, len(staffMembers))
	for _, s := range staffMembers {
		if s.Person != nil && s.Person.AccountID != nil {
			accountIDs = append(accountIDs, *s.Person.AccountID)
		}
	}

	if len(accountIDs) == 0 {
		return make(map[int64]string)
	}

	avatarMap, err := rs.AuthService.GetAccountAvatarsByIDs(ctx, accountIDs)
	if err != nil {
		rs.getLogger().Warn("failed to fetch account avatar map", slog.String("error", err.Error()))
		return make(map[int64]string)
	}
	return avatarMap
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
		rs.getLogger().Warn("failed to fetch present staff IDs", slog.String("error", err.Error()))
		presentStaffIDs = []int64{}
	}

	// Build presentMap for O(1) lookup
	presentMap := make(map[int64]bool, len(presentStaffIDs))
	for _, id := range presentStaffIDs {
		presentMap[id] = true
	}

	// Batch-load work session and absence status (non-critical)
	workStatusMap := rs.loadWorkStatusMap(ctx)
	absenceMap := rs.loadAbsenceMap(ctx)

	// Batch-load account roles, emails, and avatars for all staff members (non-critical)
	accountRoleMap := rs.loadAccountRoleMap(ctx, staffMembers)
	accountEmailMap := rs.loadAccountEmailMap(ctx, staffMembers)
	accountAvatarMap := rs.loadAccountAvatarMap(ctx, staffMembers)

	// Build response objects using pre-loaded data
	responses := make([]interface{}, 0, len(staffMembers))
	for _, staff := range staffMembers {
		if response, include := rs.processStaffForListOptimized(ctx, staff, teacherMap, presentMap, workStatusMap, absenceMap, accountRoleMap, accountEmailMap, accountAvatarMap, filters); include {
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
			rs.getLogger().Warn("failed to get person data for staff member",
				slog.Int64("staff_id", id),
				slog.String("error", err.Error()))
			// Don't fail the request, just log the warning
		} else {
			staff.Person = person
		}
	}

	// Load account role, email, and avatar with tenant scoping (non-critical)
	var accountRole string
	var accountEmail string
	var accountAvatar string
	if staff.Person != nil && staff.Person.AccountID != nil && rs.AuthService != nil {
		accountID := *staff.Person.AccountID
		roleMap, roleErr := rs.AuthService.GetAccountRoleNames(r.Context(), []int64{accountID})
		if roleErr == nil {
			accountRole = roleMap[accountID]
		}
		emailMap, emailErr := rs.AuthService.GetAccountEmailsByIDs(r.Context(), []int64{accountID})
		if emailErr == nil {
			accountEmail = emailMap[accountID]
		}
		avatarMap, avatarErr := rs.AuthService.GetAccountAvatarsByIDs(r.Context(), []int64{accountID})
		if avatarErr == nil {
			accountAvatar = avatarMap[accountID]
		}
	}

	// Resolve presence/work-status/absence to keep detail consistent with the list view.
	// These maps cover all staff today; we just look up our single ID.
	wasPresentToday := false
	if presentIDs, presentErr := rs.GroupSupervisorRepo.GetStaffIDsWithSupervisionToday(r.Context()); presentErr == nil {
		wasPresentToday = slices.Contains(presentIDs, staff.ID)
	} else {
		rs.getLogger().Warn("failed to fetch present staff IDs",
			slog.Int64("staff_id", staff.ID),
			slog.String("error", presentErr.Error()))
	}
	workStatus := rs.loadWorkStatusMap(r.Context())[staff.ID]
	absenceType := rs.loadAbsenceMap(r.Context())[staff.ID]

	// Check if this staff member is also a teacher
	isTeacher := false
	var teacher *users.Teacher

	teacher, err = rs.TeacherRepo.FindByStaffID(r.Context(), staff.ID)
	if err == nil && teacher != nil {
		response := newTeacherResponse(staff, teacher, wasPresentToday, workStatus, absenceType, accountRole, accountEmail, accountAvatar)
		common.Respond(w, r, http.StatusOK, response, "Teacher retrieved successfully")
		return
	}

	response := newStaffResponse(staff, isTeacher, wasPresentToday, workStatus, absenceType, accountRole, accountEmail, accountAvatar)
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
			rs.getLogger().Error("failed to grant groups:read permission",
				slog.String("role", role),
				slog.Int64("account_id", accountID),
				slog.String("error", err.Error()))
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

	// Create staff record (and optionally teacher) in tenant transaction
	isTeacher := req.IsTeacher
	var teacher *users.Teacher
	var teacherCreationFailed bool

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := rs.StaffRepo.Create(ctx, staff); err != nil {
			return err
		}

		// If request indicates this is a teacher, create teacher record as well
		if isTeacher {
			teacher = &users.Teacher{
				StaffID:        staff.ID,
				Specialization: strings.TrimSpace(req.Specialization),
				Role:           req.Role,
				Qualifications: req.Qualifications,
			}

			if rs.TeacherRepo.Create(ctx, teacher) != nil {
				// Still return staff member even if teacher creation fails
				isTeacher = false
				teacherCreationFailed = true
			}
		}

		// Grant groups:read permission if they have an account
		if person.AccountID != nil {
			if isTeacher {
				rs.grantDefaultPermissions(ctx, *person.AccountID, "teacher")
			} else if !teacherCreationFailed {
				rs.grantDefaultPermissions(ctx, *person.AccountID, "staff")
			}
		}

		return nil
	}); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Set person data for response
	staff.Person = person

	if teacherCreationFailed {
		response := newStaffResponse(staff, false, false, "", "", "", "", "")
		common.Respond(w, r, http.StatusCreated, response, "Staff member created successfully, but failed to create teacher record")
		return
	}

	if isTeacher {
		// Return teacher response
		response := newTeacherResponse(staff, teacher, false, "", "", "", "", "")
		common.Respond(w, r, http.StatusCreated, response, "Teacher created successfully")
		return
	}

	// Return staff response
	response := newStaffResponse(staff, isTeacher, false, "", "", "", "", "")
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

	tenantID := tenant.FromContext(r.Context())
	var response interface{}
	var message string
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := rs.StaffRepo.Update(ctx, staff); err != nil {
			return err
		}

		// Reload staff with person data
		rs.reloadStaffWithPerson(ctx, staff, id)

		// Get existing teacher record if any
		teacher, _ := rs.TeacherRepo.FindByStaffID(ctx, staff.ID)

		// Handle teacher record based on request
		response, message = rs.buildUpdateStaffResponse(ctx, staff, req, teacher)
		return nil
	}); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

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
		return newTeacherResponse(staff, existingTeacher, false, "", "", "", "", ""), "Teacher updated successfully"
	}

	return newStaffResponse(staff, false, false, "", "", "", "", ""), "Staff member updated successfully"
}

// deleteStaff handles offboarding a staff member: soft-deletes the staff and
// teacher rows and revokes the linked account's access for this tenant.
func (rs *Resource) deleteStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}

	deletedBy := jwt.ClaimsFromCtx(r.Context()).Username

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.StaffOffboardingService.OffboardStaff(ctx, id, deletedBy)
	}); err != nil {
		if errors.Is(err, usersSvc.ErrStaffInUse) {
			common.RenderError(w, r, common.ErrorConflictMessage(usersSvc.ErrStaffInUse.Error()))
			return
		}
		if common.IsConstraintViolation(err) {
			common.RenderError(w, r, common.ErrorConflictMessage("Personal kann nicht gelöscht werden: Mitarbeiter/in wird noch in anderen Bereichen referenziert"))
			return
		}
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Staff member deleted successfully")
}

// serveStaffAvatar serves the avatar image for a staff member.
// Requires users:read permission, any authenticated user in the same tenant can view staff avatars.
func (rs *Resource) serveStaffAvatar(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}

	staff, err := rs.StaffRepo.FindWithPerson(r.Context(), id)
	if err != nil || staff == nil {
		http.NotFound(w, r)
		return
	}

	if staff.Person == nil || staff.Person.AccountID == nil {
		http.NotFound(w, r)
		return
	}

	accountID := *staff.Person.AccountID
	avatarMap, err := rs.AuthService.GetAccountAvatarsByIDs(r.Context(), []int64{accountID})
	if err != nil || avatarMap[accountID] == "" {
		http.NotFound(w, r)
		return
	}

	avatarPath := avatarMap[accountID]
	filePath, err := common.ResolveStoredPath("public", avatarPath, "/uploads/avatars/")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	common.ServeImage(w, r, filepath.Dir(filePath), filepath.Base(filePath), "private, max-age=86400")
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
		responses = append(responses, newTeacherResponse(teacher.Staff, teacher, false, "", "", "", "", ""))
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
	staffResp := newStaffResponse(staff, false, false, "", "", "", "", "")
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

	caregivers, err := rs.listActiveCaregivers(ctx)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Keep the richer teacher payload, but only for canonical caregivers.
	teachers, err := rs.TeacherRepo.ListAllWithStaffAndPerson(ctx)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	activeCaregiverStaffIDs := caregiverStaffIDSet(caregivers)
	substitutingStaffMap := rs.buildSubstitutionMap(ctx, date)
	results := rs.filterAndBuildTeacherResults(ctx, teachers, activeCaregiverStaffIDs, substitutingStaffMap, searchTerm)

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
	activeCaregiverStaffIDs map[int64]struct{},
	subsMap map[int64][]*education.GroupSubstitution,
	searchTerm string,
) []StaffWithSubstitutionStatus {
	results := make([]StaffWithSubstitutionStatus, 0, len(teachers))

	for _, teacher := range teachers {
		result := rs.processTeacherForSubstitution(ctx, teacher, activeCaregiverStaffIDs, subsMap, searchTerm)
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
	activeCaregiverStaffIDs map[int64]struct{},
	subsMap map[int64][]*education.GroupSubstitution,
	searchTerm string,
) *StaffWithSubstitutionStatus {
	// Skip if staff or person data is missing
	if teacher.Staff == nil || teacher.Staff.Person == nil {
		return nil
	}
	if _, ok := activeCaregiverStaffIDs[teacher.Staff.ID]; !ok {
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
	if renderErr := rs.checkAccountLocked(r.Context(), account); renderErr != nil {
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
		// Only increment attempts for actual verification failures, not missing input.
		// Routed through the service so the atomic increment closes the
		// read-modify-write lockout race (issue #586).
		if result == pinVerificationFailed {
			if updateErr := rs.AuthService.RecordFailedPINAttempt(r.Context(), int64(account.ID)); updateErr != nil {
				rs.getLogger().Error("failed to update account PIN attempts",
					slog.String("error", updateErr.Error()))
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

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if updateErr := rs.AuthService.UpdateAccount(ctx, account); updateErr != nil {
			return updateErr
		}
		// Clear any prior lockout via the atomic reset rather than mutating
		// the in-memory account and persisting it again.
		return rs.AuthService.ResetPINLockout(ctx, int64(account.ID))
	}); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "PIN updated successfully",
	}, "PIN updated successfully")
}

// checkAccountLocked checks if account is PIN locked. The lockout decision is
// resolved by the auth service (clock injected) rather than on the model
// (issue #586, Rule 12).
func (rs *Resource) checkAccountLocked(_ context.Context, account *authmodel.Account) render.Renderer {
	if rs.AuthService.IsPINLocked(account, time.Now()) {
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
	ID                int64     `json:"id"`
	PersonID          int64     `json:"person_id"`
	TeacherID         int64     `json:"teacher_id,omitempty"`
	FirstName         string    `json:"first_name"`
	LastName          string    `json:"last_name"`
	FullName          string    `json:"full_name"`
	AccountID         int64     `json:"account_id"`
	Email             string    `json:"email"`
	IsActiveCaregiver bool      `json:"is_active_caregiver"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// getStaffByRole handles GET /api/staff/by-role?role=user or ?roles=teacher,staff,user
// Returns staff members filtered by account role (useful for group transfer dropdowns)
func (rs *Resource) getStaffByRole(w http.ResponseWriter, r *http.Request) {
	// Support both ?role=teacher (singular, backward compat) and ?roles=teacher,staff,user (multi)
	rolesParam := r.URL.Query().Get("roles")
	roleParam := r.URL.Query().Get("role")

	var roles []string
	if rolesParam != "" {
		// Parse comma-separated roles
		for _, role := range strings.Split(rolesParam, ",") {
			role = strings.TrimSpace(role)
			if role != "" {
				roles = append(roles, role)
			}
		}
	} else if roleParam != "" {
		roles = []string{strings.TrimSpace(roleParam)}
	}

	if len(roles) == 0 {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("role or roles parameter is required")))
		return
	}

	ctx := r.Context()

	if requestedCaregiverPool(roles) {
		caregivers, err := rs.listActiveCaregivers(ctx)
		if err != nil {
			common.RenderError(w, r, ErrorInternalServer(err))
			return
		}

		results := make([]StaffWithRoleResponse, 0, len(caregivers))
		for _, caregiver := range caregivers {
			results = append(results, StaffWithRoleResponse{
				ID:                caregiver.StaffID,
				PersonID:          caregiver.PersonID,
				TeacherID:         caregiver.TeacherID,
				FirstName:         caregiver.FirstName,
				LastName:          caregiver.LastName,
				FullName:          caregiver.FullName(),
				AccountID:         caregiver.AccountID,
				Email:             caregiver.Email,
				IsActiveCaregiver: true,
				CreatedAt:         caregiver.CreatedAt,
				UpdatedAt:         caregiver.UpdatedAt,
			})
		}

		common.Respond(w, r, http.StatusOK, results, "Active caregivers retrieved successfully")
		return
	}

	staffByRoles, err := rs.StaffRepo.ListStaffByRoles(ctx, roles)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Deduplicate by staff ID (a staff member with multiple matching roles appears once)
	seen := make(map[int64]bool)
	var results []StaffWithRoleResponse
	for _, s := range staffByRoles {
		if seen[s.StaffID] {
			continue
		}
		seen[s.StaffID] = true
		results = append(results, StaffWithRoleResponse{
			ID:        s.StaffID,
			PersonID:  s.PersonID,
			FirstName: s.FirstName,
			LastName:  s.LastName,
			FullName:  s.FirstName + " " + s.LastName,
			AccountID: s.AccountID,
			Email:     s.Email,
			CreatedAt: s.CreatedAt,
			UpdatedAt: s.UpdatedAt,
		})
	}

	common.Respond(w, r, http.StatusOK, results, "Staff members with role retrieved successfully")
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

// ============================================================================
// Work Schedule Handlers
// ============================================================================

// ScheduleEntryRequest represents a single day in the schedule
type ScheduleEntryRequest struct {
	WeekIndex     int `json:"week_index"`
	DayOfWeek     int `json:"day_of_week"`
	TargetMinutes int `json:"target_minutes"`
}

// ScheduleEntryResponse represents a single day in the schedule response
type ScheduleEntryResponse struct {
	WeekIndex     int `json:"week_index"`
	DayOfWeek     int `json:"day_of_week"`
	TargetMinutes int `json:"target_minutes"`
}

// ScheduleModelInfo describes the assigned work-time template, when there is one.
type ScheduleModelInfo struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	RotationLength     int    `json:"rotation_length"`
	RotationAnchorDate string `json:"rotation_anchor_date"`
}

// ScheduleResponse wraps the resolved schedule with rotation metadata.
type ScheduleResponse struct {
	Mode               string                  `json:"mode"`
	Model              *ScheduleModelInfo      `json:"model,omitempty"`
	RotationLength     int                     `json:"rotation_length"`
	RotationAnchorDate string                  `json:"rotation_anchor_date"`
	Entries            []ScheduleEntryResponse `json:"entries"`
	WeeklyTotals       []int                   `json:"weekly_totals"`
	ValidFrom          string                  `json:"valid_from,omitempty"`
}

// scheduleUpdateRequest is the union body for PUT /api/staff/{id}/schedule.
//
// Mode "template": only ModelID is required. Existing custom entries are
// archived and the staff is bound to the template.
// Mode "custom":   RotationLength + Entries describe a per-staff pattern.
//
//	Entries that exceed the rotation are rejected.
//	SaveAsTemplateName is optional; when set we additionally
//	create a tenant template from the same payload and bind it.
type scheduleUpdateRequest struct {
	Mode               string                 `json:"mode"`
	ModelID            *int64                 `json:"model_id,omitempty"`
	RotationLength     int                    `json:"rotation_length,omitempty"`
	RotationAnchorDate string                 `json:"rotation_anchor_date,omitempty"`
	Entries            []ScheduleEntryRequest `json:"entries"`
	SaveAsTemplateName string                 `json:"save_as_template,omitempty"`
}

// getSchedule handles GET /api/staff/{id}/schedule
func (rs *Resource) getSchedule(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if !rs.canReadSchedule(r.Context(), staffID) {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("insufficient permission to read schedule")))
		return
	}

	staff, err := rs.StaffRepo.FindByID(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		return
	}

	resp, err := rs.buildScheduleResponse(r.Context(), staff)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, resp, "Schedule retrieved successfully")
}

func (rs *Resource) canReadSchedule(ctx context.Context, staffID int64) bool {
	userPermissions := jwt.PermissionsFromCtx(ctx)
	if authorize.HasPermission(permissions.TimeTrackingManage, userPermissions) {
		return true
	}
	if !authorize.HasPermission(permissions.TimeTrackingOwn, userPermissions) {
		return false
	}
	claims := jwt.ClaimsFromCtx(ctx)
	if claims.ID == 0 {
		return false
	}
	person, err := rs.PersonService.FindByAccountID(ctx, int64(claims.ID))
	if err != nil {
		return false
	}
	staff, err := rs.StaffRepo.FindByPersonID(ctx, person.ID)
	if err != nil {
		return false
	}
	return staff.ID == staffID
}

// updateSchedule handles PUT /api/staff/{id}/schedule
func (rs *Resource) updateSchedule(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	staff, err := rs.StaffRepo.FindByID(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		return
	}

	var req scheduleUpdateRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	mode := req.Mode
	if mode == "" {
		// Backwards compatibility: missing mode + flat entries means the
		// caller still uses the single-week, no-rotation contract.
		mode = "custom"
		if req.RotationLength == 0 {
			req.RotationLength = 1
		}
	}

	switch mode {
	case "template":
		if req.ModelID == nil || *req.ModelID == 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("model_id is required for mode=template")))
			return
		}
		if err := rs.assignTemplateToStaff(r.Context(), staff, *req.ModelID); err != nil {
			common.RenderError(w, r, common.ErrorInternalServer(err))
			return
		}
	case "custom":
		if err := rs.applyCustomSchedule(r.Context(), staff, req); err != nil {
			if errors.Is(err, errScheduleValidation) {
				common.RenderError(w, r, common.ErrorInvalidRequest(err))
			} else {
				common.RenderError(w, r, common.ErrorInternalServer(err))
			}
			return
		}
	default:
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("invalid mode %q", mode)))
		return
	}

	refreshed, err := rs.StaffRepo.FindByID(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	resp, err := rs.buildScheduleResponse(r.Context(), refreshed)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, resp, "Schedule updated successfully")
}

func (rs *Resource) buildScheduleResponse(ctx context.Context, staff *users.Staff) (*ScheduleResponse, error) {
	if staff.WorkTimeModelID != nil && *staff.WorkTimeModelID > 0 {
		model, err := rs.WorkTimeModelRepo.FindByID(ctx, *staff.WorkTimeModelID)
		if err != nil {
			return nil, fmt.Errorf("load assigned model: %w", err)
		}
		anchor := model.RotationAnchorDate
		if staff.RotationAnchorDate != nil {
			anchor = *staff.RotationAnchorDate
		}

		rows, err := rs.ScheduleRepo.GetCurrentByStaffID(ctx, staff.ID)
		if err != nil {
			return nil, fmt.Errorf("load assigned schedule snapshot: %w", err)
		}
		rotation := model.RotationLength
		var entries []ScheduleEntryResponse
		var totals []int
		if len(rows) > 0 {
			entries, totals, rotation = scheduleRowsToResponseParts(rows)
		} else {
			entries, totals = modelEntriesToResponseParts(model.Entries, rotation)
		}
		return &ScheduleResponse{
			Mode: "template",
			Model: &ScheduleModelInfo{
				ID:                 model.ID,
				Name:               model.Name,
				RotationLength:     model.RotationLength,
				RotationAnchorDate: model.RotationAnchorDate.Format("2006-01-02"),
			},
			RotationLength:     rotation,
			RotationAnchorDate: anchor.Format("2006-01-02"),
			Entries:            entries,
			WeeklyTotals:       totals,
		}, nil
	}

	rows, err := rs.ScheduleRepo.GetCurrentByStaffID(ctx, staff.ID)
	if err != nil {
		return nil, fmt.Errorf("load custom schedule: %w", err)
	}

	entries, totals, rotation := scheduleRowsToResponseParts(rows)
	var earliest *time.Time
	for _, row := range rows {
		if earliest == nil || row.ValidFrom.Before(*earliest) {
			vf := row.ValidFrom
			earliest = &vf
		}
	}
	anchor := time.Time{}
	if staff.RotationAnchorDate != nil {
		anchor = *staff.RotationAnchorDate
	} else if earliest != nil {
		anchor = *earliest
	}
	resp := &ScheduleResponse{
		Mode:               "custom",
		RotationLength:     rotation,
		RotationAnchorDate: anchor.Format("2006-01-02"),
		Entries:            entries,
		WeeklyTotals:       totals,
	}
	if earliest != nil {
		resp.ValidFrom = earliest.Format("2006-01-02")
	}
	return resp, nil
}

func (rs *Resource) assignTemplateToStaff(ctx context.Context, staff *users.Staff, modelID int64) error {
	model, err := rs.WorkTimeModelRepo.FindByID(ctx, modelID)
	if err != nil {
		return fmt.Errorf("template not found: %w", err)
	}

	entries := modelEntriesToScheduleRows(model.Entries, model.RotationLength)
	if err := rs.ScheduleRepo.ReplaceSchedule(ctx, staff.ID, entries); err != nil {
		return fmt.Errorf("write assigned schedule snapshot: %w", err)
	}

	staff.WorkTimeModelID = &model.ID
	anchor := model.RotationAnchorDate
	staff.RotationAnchorDate = &anchor
	if err := rs.StaffRepo.Update(ctx, staff); err != nil {
		return fmt.Errorf("bind template to staff: %w", err)
	}
	return nil
}

func (rs *Resource) applyCustomSchedule(ctx context.Context, staff *users.Staff, req scheduleUpdateRequest) error {
	rotation := req.RotationLength
	if rotation == 0 {
		rotation = 1
	}
	if rotation < 1 || rotation > config.WorkTimeModelMaxRotation {
		return scheduleValidationErrorf("rotation_length must be between 1 and %d", config.WorkTimeModelMaxRotation)
	}

	anchor := time.Time{}
	if req.RotationAnchorDate != "" {
		parsed, err := time.Parse("2006-01-02", req.RotationAnchorDate)
		if err != nil {
			return scheduleValidationErrorf("invalid rotation_anchor_date: %v", err)
		}
		anchor = parsed
	}

	entries, templateEntries, err := buildScheduleEntries(req.Entries, rotation)
	if err != nil {
		return err
	}

	if req.SaveAsTemplateName != "" {
		if err := rs.saveCustomAsTemplate(ctx, staff, req.SaveAsTemplateName, rotation, anchor, templateEntries); err != nil {
			return fmt.Errorf("save as template: %w", err)
		}
		return nil
	}

	if err := rs.ScheduleRepo.ReplaceSchedule(ctx, staff.ID, entries); err != nil {
		return fmt.Errorf("write custom schedule: %w", err)
	}

	staff.WorkTimeModelID = nil
	if !anchor.IsZero() {
		staff.RotationAnchorDate = &anchor
	}
	if err := rs.StaffRepo.Update(ctx, staff); err != nil {
		return fmt.Errorf("unbind template: %w", err)
	}

	return nil
}

func buildScheduleEntries(reqEntries []ScheduleEntryRequest, rotation int) ([]*config.StaffWorkSchedule, []*config.WorkTimeModelEntry, error) {
	entries := make([]*config.StaffWorkSchedule, 0, len(reqEntries))
	templateEntries := make([]*config.WorkTimeModelEntry, 0, len(reqEntries))
	seenSlots := make(map[string]struct{}, len(reqEntries))
	for _, e := range reqEntries {
		if e.TargetMinutes <= 0 {
			continue
		}
		if err := validateScheduleEntryRequest(e, rotation, seenSlots); err != nil {
			return nil, nil, err
		}
		entries = append(entries, &config.StaffWorkSchedule{
			WeekIndex:      e.WeekIndex,
			RotationLength: rotation,
			DayOfWeek:      e.DayOfWeek,
			TargetMinutes:  e.TargetMinutes,
		})
		templateEntries = append(templateEntries, &config.WorkTimeModelEntry{
			WeekIndex:     e.WeekIndex,
			DayOfWeek:     e.DayOfWeek,
			TargetMinutes: e.TargetMinutes,
		})
	}
	return entries, templateEntries, nil
}

func (rs *Resource) saveCustomAsTemplate(ctx context.Context, staff *users.Staff, name string, rotation int, anchor time.Time, entries []*config.WorkTimeModelEntry) error {
	if anchor.IsZero() {
		anchor = time.Now().Truncate(24 * time.Hour)
	}
	model := &config.WorkTimeModel{
		Name:               name,
		RotationLength:     rotation,
		RotationAnchorDate: anchor,
	}
	if err := rs.WorkTimeModelRepo.Create(ctx, model, entries); err != nil {
		return err
	}
	scheduleRows := modelEntriesToScheduleRows(entries, rotation)
	if err := rs.ScheduleRepo.ReplaceSchedule(ctx, staff.ID, scheduleRows); err != nil {
		return fmt.Errorf("write saved template schedule snapshot: %w", err)
	}

	staff.WorkTimeModelID = &model.ID
	staff.RotationAnchorDate = &anchor
	if err := rs.StaffRepo.Update(ctx, staff); err != nil {
		return fmt.Errorf("bind freshly created template: %w", err)
	}
	return nil
}

func scheduleRowsToResponseParts(rows []*config.StaffWorkSchedule) ([]ScheduleEntryResponse, []int, int) {
	rotation := 1
	for _, row := range rows {
		if row.RotationLength > rotation {
			rotation = row.RotationLength
		}
	}
	if rotation < 1 {
		rotation = 1
	}
	totals := make([]int, rotation)
	entries := make([]ScheduleEntryResponse, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, ScheduleEntryResponse{
			WeekIndex:     row.WeekIndex,
			DayOfWeek:     row.DayOfWeek,
			TargetMinutes: row.TargetMinutes,
		})
		if row.WeekIndex >= 0 && row.WeekIndex < rotation {
			totals[row.WeekIndex] += row.TargetMinutes
		}
	}
	return entries, totals, rotation
}

func modelEntriesToResponseParts(modelEntries []*config.WorkTimeModelEntry, rotation int) ([]ScheduleEntryResponse, []int) {
	if rotation < 1 {
		rotation = 1
	}
	entries := make([]ScheduleEntryResponse, 0, len(modelEntries))
	totals := make([]int, rotation)
	for _, e := range modelEntries {
		entries = append(entries, ScheduleEntryResponse{
			WeekIndex:     e.WeekIndex,
			DayOfWeek:     e.DayOfWeek,
			TargetMinutes: e.TargetMinutes,
		})
		if e.WeekIndex >= 0 && e.WeekIndex < rotation {
			totals[e.WeekIndex] += e.TargetMinutes
		}
	}
	return entries, totals
}

func modelEntriesToScheduleRows(modelEntries []*config.WorkTimeModelEntry, rotation int) []*config.StaffWorkSchedule {
	rows := make([]*config.StaffWorkSchedule, 0, len(modelEntries))
	for _, e := range modelEntries {
		if e.TargetMinutes <= 0 {
			continue
		}
		rows = append(rows, &config.StaffWorkSchedule{
			WeekIndex:      e.WeekIndex,
			RotationLength: rotation,
			DayOfWeek:      e.DayOfWeek,
			TargetMinutes:  e.TargetMinutes,
		})
	}
	return rows
}

func validateScheduleEntryRequest(e ScheduleEntryRequest, rotation int, seenSlots map[string]struct{}) error {
	if e.WeekIndex < 0 || e.WeekIndex >= rotation {
		return scheduleValidationErrorf("week_index %d outside rotation_length %d", e.WeekIndex, rotation)
	}
	if e.DayOfWeek < config.DayMonday || e.DayOfWeek > config.DaySunday {
		return scheduleValidationErrorf("day_of_week must be between 0 and 6")
	}
	if e.TargetMinutes > 720 {
		return scheduleValidationErrorf("target_minutes must be between 0 and 720")
	}
	slot := fmt.Sprintf("%d:%d", e.WeekIndex, e.DayOfWeek)
	if _, ok := seenSlots[slot]; ok {
		return scheduleValidationErrorf("duplicate schedule entry for week_index %d and day_of_week %d", e.WeekIndex, e.DayOfWeek)
	}
	seenSlots[slot] = struct{}{}
	return nil
}

// getStaffHistory handles GET /api/staff/{id}/time-tracking/history?from=YYYY-MM-DD&to=YYYY-MM-DD
func (rs *Resource) getStaffHistory(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Verify staff exists
	if _, err := rs.StaffRepo.FindByID(r.Context(), staffID); err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		return
	}

	// Parse date range from query parameters
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if fromStr == "" || toStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("from and to query parameters are required")))
		return
	}

	from, err := time.Parse(common.DateFormatISO, fromStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid from date format, expected YYYY-MM-DD")))
		return
	}

	to, err := time.Parse(common.DateFormatISO, toStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid to date format, expected YYYY-MM-DD")))
		return
	}

	historyResp, err := rs.WorkSessionService.GetHistory(r.Context(), staffID, from, to)
	if err != nil {
		rs.getLogger().Error("failed to get staff history",
			"staff_id", staffID,
			"error", err.Error(),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, historyResp, "Staff session history retrieved successfully")
}

// exportStaffSessions handles GET /api/staff/{id}/time-tracking/export?from=...&to=...&format=csv|xlsx
func (rs *Resource) exportStaffSessions(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if _, err := rs.StaffRepo.FindByID(r.Context(), staffID); err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		return
	}

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if fromStr == "" || toStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("from and to query parameters are required")))
		return
	}
	from, err := time.Parse(common.DateFormatISO, fromStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid from date format, expected YYYY-MM-DD")))
		return
	}
	to, err := time.Parse(common.DateFormatISO, toStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid to date format, expected YYYY-MM-DD")))
		return
	}

	format := r.URL.Query().Get("format")
	if format != "csv" && format != "xlsx" {
		format = "csv"
	}

	fileBytes, filename, err := rs.WorkSessionService.ExportSessions(r.Context(), staffID, from, to, format)
	if err != nil {
		rs.getLogger().Error("failed to export staff sessions",
			"staff_id", staffID,
			"error", err.Error(),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	switch format {
	case "xlsx":
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	default:
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	}
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("Content-Length", strconv.Itoa(len(fileBytes)))
	if _, err := w.Write(fileBytes); err != nil {
		rs.getLogger().Error("failed to write export response", "error", err.Error())
	}
}

// listPendingAbsenceRequests handles GET /api/staff/absences/pending
func (rs *Resource) listPendingAbsenceRequests(w http.ResponseWriter, r *http.Request) {
	resp, err := rs.StaffAbsenceService.ListPendingRequests(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, resp, "Pending absence requests retrieved")
}

// approveAbsence handles POST /api/staff/absences/{absenceId}/approve
func (rs *Resource) approveAbsence(w http.ResponseWriter, r *http.Request) {
	absenceID, err := parseInt64Param(r, "absenceId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	decidedBy, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("invalid token")))
		return
	}
	var req activeSvc.VacationDecisionRequest
	if r.ContentLength > 0 {
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
	}
	resp, err := rs.StaffAbsenceService.ApproveAbsence(r.Context(), absenceID, int64(claims.ID), decidedBy, req.DecisionNote)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, resp, "Absence approved")
}

// denyAbsence handles POST /api/staff/absences/{absenceId}/deny
func (rs *Resource) denyAbsence(w http.ResponseWriter, r *http.Request) {
	absenceID, err := parseInt64Param(r, "absenceId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	decidedBy, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("invalid token")))
		return
	}
	var req activeSvc.VacationDecisionRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	resp, err := rs.StaffAbsenceService.DenyAbsence(r.Context(), absenceID, int64(claims.ID), decidedBy, req.DecisionNote)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, resp, "Absence declined")
}

// getStaffVacationQuota handles GET /api/staff/{id}/vacation/quota?year=YYYY
func (rs *Resource) getStaffVacationQuota(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	year, err := parseYearQuery(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	summary, err := rs.StaffAbsenceService.GetVacationQuotaSummary(r.Context(), staffID, year)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, summary, "Vacation quota retrieved")
}

// setStaffVacationQuota handles PUT /api/staff/{id}/vacation/quota
func (rs *Resource) setStaffVacationQuota(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	var body struct {
		Year          int     `json:"year"`
		EntitledDays  float64 `json:"entitled_days"`
		CarryoverDays float64 `json:"carryover_days"`
	}
	if err := render.DecodeJSON(r.Body, &body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if body.Year == 0 {
		body.Year = time.Now().Year()
	}
	if err := rs.StaffAbsenceService.UpsertVacationQuota(r.Context(), staffID, body.Year, body.EntitledDays, body.CarryoverDays); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	summary, err := rs.StaffAbsenceService.GetVacationQuotaSummary(r.Context(), staffID, body.Year)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, summary, "Vacation quota saved")
}

func parseInt64Param(r *http.Request, name string) (int64, error) {
	str := chi.URLParam(r, name)
	v, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0, errors.New("invalid " + name + " parameter")
	}
	return v, nil
}

func parseYearQuery(r *http.Request) (int, error) {
	yearStr := r.URL.Query().Get("year")
	if yearStr == "" {
		return time.Now().Year(), nil
	}
	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 2000 || year > 2100 {
		return 0, errors.New("invalid year parameter")
	}
	return year, nil
}

// resolveEditorStaffID maps the JWT account id to a staff id, the staff
// record of the admin currently making the request. Lands in
// audit.work_session_edits.edited_by so the audit trail can name a real
// person, not an opaque account.
func (rs *Resource) resolveEditorStaffID(ctx context.Context) (int64, error) {
	claims := jwt.ClaimsFromCtx(ctx)
	if claims.ID == 0 {
		return 0, errors.New("invalid token")
	}
	person, err := rs.PersonService.FindByAccountID(ctx, int64(claims.ID))
	if err != nil {
		return 0, fmt.Errorf("person not found for account: %w", err)
	}
	staff, err := rs.StaffRepo.FindByPersonID(ctx, person.ID)
	if err != nil {
		return 0, fmt.Errorf("staff not found for editor account: %w", err)
	}
	return staff.ID, nil
}

// adminUpdateStaffSession handles PUT /api/staff/{id}/time-tracking/sessions/{sessionId}
// Admin counterpart to /api/time-tracking/{id}, gated on
// time_tracking:manage at the router level, so a Betreuer with only
// time_tracking:own gets 403 before reaching the handler.
func (rs *Resource) adminUpdateStaffSession(w http.ResponseWriter, r *http.Request) {
	targetStaffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	editorStaffID, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	sessionIDStr := chi.URLParam(r, "sessionId")
	sessionID, err := strconv.ParseInt(sessionIDStr, 10, 64)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid session ID")))
		return
	}

	var updates activeSvc.SessionUpdateRequest
	if err := render.DecodeJSON(r.Body, &updates); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	session, err := rs.WorkSessionService.UpdateSessionAsAdmin(r.Context(), editorStaffID, targetStaffID, sessionID, updates)
	if err != nil {
		rs.getLogger().Error("admin session update failed",
			"target_staff_id", targetStaffID,
			"editor_staff_id", editorStaffID,
			"session_id", sessionID,
			"error", err.Error(),
		)
		common.RenderError(w, r, classifyAdminEditError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, session, "Session updated")
}

// adminCreateStaffSession handles POST /api/staff/{id}/time-tracking/sessions
// the "nachtragen" flow when an MA forgot to stamp.
func (rs *Resource) adminCreateStaffSession(w http.ResponseWriter, r *http.Request) {
	targetStaffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	editorStaffID, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	var req activeSvc.AdminCreateSessionRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	session, err := rs.WorkSessionService.CreateSessionAsAdmin(r.Context(), editorStaffID, targetStaffID, req)
	if err != nil {
		rs.getLogger().Error("admin session create failed",
			"target_staff_id", targetStaffID,
			"editor_staff_id", editorStaffID,
			"error", err.Error(),
		)
		common.RenderError(w, r, classifyAdminEditError(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, session, "Session created")
}

// classifyAdminEditError maps service errors to HTTP responses. Validation
// problems (notes missing, range invalid) land as 400; ownership mismatches
// and "not found" as 404 to avoid leaking session existence.
func classifyAdminEditError(err error) render.Renderer {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "notes required"),
		strings.Contains(msg, "must be"),
		strings.Contains(msg, "must not"),
		strings.Contains(msg, "is required"):
		return common.ErrorInvalidRequest(err)
	case strings.Contains(msg, "session not found"),
		strings.Contains(msg, "does not belong"):
		return common.ErrorNotFound(err)
	default:
		return common.ErrorInternalServer(err)
	}
}

// getStaffSessionEdits handles GET /api/staff/{id}/time-tracking/sessions/{sessionId}/edits
// Admin-facing counterpart to /api/time-tracking/{id}/edits. The MA endpoint
// verifies the session belongs to the JWT subject; here we verify it belongs
// to the staff named in the URL (RLS still scopes the tenant).
func (rs *Resource) getStaffSessionEdits(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	if _, err := rs.StaffRepo.FindByID(r.Context(), staffID); err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		return
	}

	sessionIDStr := chi.URLParam(r, "sessionId")
	sessionID, err := strconv.ParseInt(sessionIDStr, 10, 64)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid session ID")))
		return
	}

	edits, err := rs.WorkSessionService.GetSessionEditsForStaff(r.Context(), staffID, sessionID)
	if err != nil {
		rs.getLogger().Error("failed to get staff session edits",
			"staff_id", staffID,
			"session_id", sessionID,
			"error", err.Error(),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, edits, "Session edits retrieved successfully")
}

// getStaffAbsences handles GET /api/staff/{id}/absences?from=YYYY-MM-DD&to=YYYY-MM-DD
// Admin-facing counterpart to /api/time-tracking/absences (which is scoped to
// the caller). Lets the staff-detail view render Krank/Urlaub badges for any
// staff in the tenant.
func (rs *Resource) getStaffAbsences(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	if _, err := rs.StaffRepo.FindByID(r.Context(), staffID); err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		return
	}

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if fromStr == "" || toStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("from and to query parameters are required")))
		return
	}

	from, err := time.Parse(common.DateFormatISO, fromStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid from date format, expected YYYY-MM-DD")))
		return
	}

	to, err := time.Parse(common.DateFormatISO, toStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid to date format, expected YYYY-MM-DD")))
		return
	}

	absences, err := rs.StaffAbsenceService.GetAbsencesForRange(r.Context(), staffID, from, to)
	if err != nil {
		rs.getLogger().Error("failed to get staff absences",
			"staff_id", staffID,
			"error", err.Error(),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, absences, "Staff absences retrieved successfully")
}
