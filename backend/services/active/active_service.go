package active

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// Broadcaster interface (re-exported from realtime for convenience)
type Broadcaster = realtime.Broadcaster

const (
	// sseErrorMessage is the standard error message for SSE broadcast failures
	sseErrorMessage = "SSE broadcast failed"
)

// RoomConflictStrategy defines how to handle room conflicts when determining room ID
type RoomConflictStrategy int

const (
	// RoomConflictFail returns error if room has conflicts
	RoomConflictFail RoomConflictStrategy = iota
	// RoomConflictIgnore skips conflict checking entirely
	RoomConflictIgnore
	// RoomConflictWarn logs warning but continues
	RoomConflictWarn
)

// CrossTenantRepo defines the interface for cross-tenant student queries.
type CrossTenantRepo interface {
	FindCrossTenantStudents(ctx context.Context, hostingTenantID int64) ([]active.CrossTenantStudent, error)
}

// SettingsResolver resolves tenant-scoped settings. Implemented by config.SettingsService.
// Optional dependency — when nil, auto-clear behavior falls back to the registry default.
type SettingsResolver interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

// ServiceDependencies contains all dependencies required by the active service
type ServiceDependencies struct {
	// Active domain repositories
	GroupRepo         active.GroupRepository
	VisitRepo         active.VisitRepository
	SupervisorRepo    active.GroupSupervisorRepository
	CombinedGroupRepo active.CombinedGroupRepository
	GroupMappingRepo  active.GroupMappingRepository
	AttendanceRepo    active.AttendanceRepository

	// Cross-tenant query repository (optional - nil-safe)
	CrossTenantRepo CrossTenantRepo

	// User domain repositories
	StudentRepo userModels.StudentRepository
	PersonRepo  userModels.PersonRepository
	TeacherRepo userModels.TeacherRepository
	StaffRepo   userModels.StaffRepository

	// Supporting domain repositories
	RoomRepo           facilityModels.RoomRepository
	ActivityGroupRepo  activitiesModels.GroupRepository
	ActivityCatRepo    activitiesModels.CategoryRepository
	EducationGroupRepo educationModels.GroupRepository
	DeviceRepo         iotModels.DeviceRepository

	// External services
	EducationService education.Service
	UsersService     users.PersonService

	// Infrastructure
	DB          *bun.DB
	Broadcaster Broadcaster // SSE event broadcaster (optional - can be nil for testing)

	// Optional: Work session service for NFC auto-check-in
	WorkSessionService WorkSessionService

	// Optional: Attendance sync (WP-B10). When non-nil, visit create/end
	// calls mirror into schedule.instance_students and enrich check-in/out
	// SSE events with attendance status/substatus/note.
	AttendanceSyncer AttendanceSyncer

	// Optional: Structured logger (nil-safe, Phase 2b will add logging calls)
	Logger *slog.Logger
}

// Service implements the Active Service interface
type service struct {
	groupRepo         active.GroupRepository
	visitRepo         active.VisitRepository
	supervisorRepo    active.GroupSupervisorRepository
	combinedGroupRepo active.CombinedGroupRepository
	groupMappingRepo  active.GroupMappingRepository

	// Cross-tenant query repository (optional - nil-safe)
	crossTenantRepo CrossTenantRepo

	// Additional repositories for dashboard analytics
	studentRepo        userModels.StudentRepository
	roomRepo           facilityModels.RoomRepository
	activityGroupRepo  activitiesModels.GroupRepository
	activityCatRepo    activitiesModels.CategoryRepository
	educationGroupRepo educationModels.GroupRepository
	personRepo         userModels.PersonRepository
	deviceRepo         iotModels.DeviceRepository

	// New dependencies for attendance tracking
	attendanceRepo   active.AttendanceRepository
	educationService education.Service
	usersService     users.PersonService
	teacherRepo      userModels.TeacherRepository
	staffRepo        userModels.StaffRepository

	db *bun.DB

	// SSE real-time event broadcasting (optional - can be nil for testing)
	broadcaster Broadcaster

	// Optional: Work session service for NFC auto-check-in
	workSessionService WorkSessionService

	// Optional: Attendance sync (WP-B10)
	attendanceSyncer AttendanceSyncer

	// Optional: Tenant-scoped settings resolver for auto-clear logic.
	// When nil, auto-clear falls back to the registry default behavior.
	settings SettingsResolver

	// Structured logger (nil-safe)
	logger *slog.Logger
}

// SetSettingsService injects the tenant-scoped settings resolver.
// Called from the factory after settingsService is constructed.
func (s *service) SetSettingsService(resolver SettingsResolver) {
	s.settings = resolver
}

// GetPresenceMode returns the tenant's resolved presence mode
// ("detailed" | "binary"), falling back to "detailed" whenever the settings
// resolver is nil, the key is unset/empty, or the lookup errors. Mirrors the
// fallback logic of services/config.ResolvePresenceMode but avoids importing
// the config package here (it would create a dep cycle through the factory).
func (s *service) GetPresenceMode(ctx context.Context) string {
	const (
		keyPresenceMode      = "operations.presence_mode"
		presenceModeDetailed = "detailed"
	)
	if s.settings == nil {
		return presenceModeDetailed
	}
	val, err := s.settings.ResolveString(ctx, keyPresenceMode)
	if err != nil {
		s.getLogger().Warn("presence_mode resolve failed, using default",
			slog.String("error", err.Error()),
		)
		return presenceModeDetailed
	}
	if val == "" {
		return presenceModeDetailed
	}
	return val
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (s *service) getLogger() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// NewService creates a new active service instance
func NewService(deps ServiceDependencies) Service {
	return &service{
		groupRepo:          deps.GroupRepo,
		visitRepo:          deps.VisitRepo,
		supervisorRepo:     deps.SupervisorRepo,
		combinedGroupRepo:  deps.CombinedGroupRepo,
		groupMappingRepo:   deps.GroupMappingRepo,
		crossTenantRepo:    deps.CrossTenantRepo,
		studentRepo:        deps.StudentRepo,
		roomRepo:           deps.RoomRepo,
		activityGroupRepo:  deps.ActivityGroupRepo,
		activityCatRepo:    deps.ActivityCatRepo,
		educationGroupRepo: deps.EducationGroupRepo,
		personRepo:         deps.PersonRepo,
		deviceRepo:         deps.DeviceRepo,
		attendanceRepo:     deps.AttendanceRepo,
		educationService:   deps.EducationService,
		usersService:       deps.UsersService,
		teacherRepo:        deps.TeacherRepo,
		staffRepo:          deps.StaffRepo,
		db:                 deps.DB,
		broadcaster:        deps.Broadcaster,
		workSessionService: deps.WorkSessionService,
		attendanceSyncer:   deps.AttendanceSyncer,
		logger:             deps.Logger,
	}
}

// Active Group operations
func (s *service) GetActiveGroup(ctx context.Context, id int64) (*active.Group, error) {
	group, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroup", Err: ErrActiveGroupNotFound}
	}

	// Ensure we always have room metadata so downstream callers
	// (location resolver, SSE payloads) can render friendly labels.
	if group != nil && group.Room == nil && group.RoomID > 0 {
		if room, roomErr := s.roomRepo.FindByID(ctx, group.RoomID); roomErr == nil {
			group.Room = room
		}
	}

	return group, nil
}

func (s *service) GetActiveGroupsByIDs(ctx context.Context, groupIDs []int64) (map[int64]*active.Group, error) {
	if len(groupIDs) == 0 {
		return map[int64]*active.Group{}, nil
	}

	groups, err := s.groupRepo.FindByIDs(ctx, groupIDs)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupsByIDs", Err: ErrDatabaseOperation}
	}

	if groups == nil {
		groups = make(map[int64]*active.Group)
	}

	return groups, nil
}

func (s *service) CreateActiveGroup(ctx context.Context, group *active.Group) error {
	if group == nil || group.Validate() != nil {
		return &ActiveError{Op: "CreateActiveGroup", Err: ErrInvalidData}
	}

	// Check for room conflicts if room is assigned
	if group.RoomID > 0 {
		hasConflict, _, err := s.groupRepo.CheckRoomConflict(ctx, group.RoomID, 0)
		if err != nil {
			return &ActiveError{Op: "CreateActiveGroup", Err: fmt.Errorf("check room conflict: %w", err)}
		}
		if hasConflict {
			return &ActiveError{Op: "CreateActiveGroup", Err: ErrRoomConflict}
		}
	}

	group.SetTenantID(tenant.FromContext(ctx))
	if err := s.groupRepo.Create(ctx, group); err != nil {
		return &ActiveError{Op: "CreateActiveGroup", Err: fmt.Errorf("create failed: %w", err)}
	}

	return nil
}

func (s *service) UpdateActiveGroup(ctx context.Context, group *active.Group) error {
	if group == nil || group.Validate() != nil {
		return &ActiveError{Op: "UpdateActiveGroup", Err: ErrInvalidData}
	}

	// Check for room conflicts if room is assigned (exclude current group)
	if group.RoomID > 0 {
		hasConflict, _, err := s.groupRepo.CheckRoomConflict(ctx, group.RoomID, group.ID)
		if err != nil {
			return &ActiveError{Op: "UpdateActiveGroup", Err: fmt.Errorf("check room conflict: %w", err)}
		}
		if hasConflict {
			return &ActiveError{Op: "UpdateActiveGroup", Err: ErrRoomConflict}
		}
	}

	if err := s.groupRepo.Update(ctx, group); err != nil {
		return &ActiveError{Op: "UpdateActiveGroup", Err: fmt.Errorf("update failed: %w", err)}
	}

	return nil
}

func (s *service) DeleteActiveGroup(ctx context.Context, id int64) error {
	// Check if there are any active visits for this group
	visits, err := s.visitRepo.FindByActiveGroupID(ctx, id)
	if err != nil {
		return &ActiveError{Op: "DeleteActiveGroup", Err: fmt.Errorf("find visits: %w", err)}
	}

	// Check if any of the visits are still active
	for _, visit := range visits {
		if visit.IsActive() {
			return &ActiveError{Op: "DeleteActiveGroup", Err: ErrCannotDeleteActiveGroup}
		}
	}

	// Delete the active group
	_, err = s.groupRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &ActiveError{Op: "DeleteActiveGroup", Err: ErrActiveGroupNotFound}
		}
		return &ActiveError{Op: "DeleteActiveGroup", Err: fmt.Errorf("find group: %w", err)}
	}

	if err := s.groupRepo.Delete(ctx, id); err != nil {
		return &ActiveError{Op: "DeleteActiveGroup", Err: fmt.Errorf("delete failed: %w", err)}
	}

	return nil
}

func (s *service) ListActiveGroups(ctx context.Context, options *base.QueryOptions) ([]*active.Group, error) {
	groups, err := s.groupRepo.List(ctx, options)
	if err != nil {
		return nil, &ActiveError{Op: "ListActiveGroups", Err: fmt.Errorf("list failed: %w", err)}
	}
	return groups, nil
}

func (s *service) FindActiveGroupsByRoomID(ctx context.Context, roomID int64) ([]*active.Group, error) {
	groups, err := s.groupRepo.FindActiveByRoomID(ctx, roomID)
	if err != nil {
		return nil, &ActiveError{Op: "FindActiveGroupsByRoomID", Err: fmt.Errorf("find by room: %w", err)}
	}
	return groups, nil
}

func (s *service) FindDeviceActiveGroupInRoom(ctx context.Context, roomID int64, deviceID int64) (*active.Group, error) {
	group, err := s.groupRepo.FindActiveByRoomIDAndDeviceID(ctx, roomID, deviceID)
	if err != nil {
		return nil, &ActiveError{Op: "FindDeviceActiveGroupInRoom", Err: fmt.Errorf("find by room and device: %w", err)}
	}
	return group, nil
}

func (s *service) FindActiveGroupsByGroupID(ctx context.Context, groupID int64) ([]*active.Group, error) {
	groups, err := s.groupRepo.FindActiveByGroupID(ctx, groupID)
	if err != nil {
		return nil, &ActiveError{Op: "FindActiveGroupsByGroupID", Err: ErrDatabaseOperation}
	}
	return groups, nil
}

func (s *service) FindActiveGroupsByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Group, error) {
	if start.After(end) {
		return nil, &ActiveError{Op: "FindActiveGroupsByTimeRange", Err: ErrInvalidTimeRange}
	}

	groups, err := s.groupRepo.FindByTimeRange(ctx, start, end)
	if err != nil {
		return nil, &ActiveError{Op: "FindActiveGroupsByTimeRange", Err: ErrDatabaseOperation}
	}
	return groups, nil
}

func (s *service) EndActiveGroupSession(ctx context.Context, id int64) error {
	// Delegate to EndActivitySession which properly ends visits and broadcasts SSE
	if err := s.EndActivitySession(ctx, id); err != nil {
		// Wrap the error with our operation name for clarity
		if activeErr, ok := err.(*ActiveError); ok {
			return &ActiveError{Op: "EndActiveGroupSession", Err: activeErr.Err}
		}
		return &ActiveError{Op: "EndActiveGroupSession", Err: err}
	}
	return nil
}

func (s *service) GetActiveGroupWithVisits(ctx context.Context, id int64) (*active.Group, error) {
	// Get the active group
	group, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupWithVisits", Err: ErrActiveGroupNotFound}
	}

	// Get visits for this group
	visits, err := s.visitRepo.FindByActiveGroupID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupWithVisits", Err: ErrDatabaseOperation}
	}

	group.Visits = visits
	return group, nil
}

func (s *service) GetActiveGroupWithSupervisors(ctx context.Context, id int64) (*active.Group, error) {
	// Get the active group
	group, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupWithSupervisors", Err: ErrActiveGroupNotFound}
	}

	// Get supervisors for this group (only active ones)
	supervisors, err := s.supervisorRepo.FindByActiveGroupID(ctx, id, true)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupWithSupervisors", Err: ErrDatabaseOperation}
	}

	group.Supervisors = supervisors
	return group, nil
}

// Visit operations
func (s *service) GetVisit(ctx context.Context, id int64) (*active.Visit, error) {
	visit, err := s.visitRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetVisit", Err: ErrVisitNotFound}
	}
	return visit, nil
}

func (s *service) CreateVisit(ctx context.Context, visit *active.Visit) error {
	if visit == nil || visit.Validate() != nil {
		return &ActiveError{Op: "CreateVisit", Err: ErrInvalidData}
	}

	// Binary-mode tenants don't track room visits — attendance is the only
	// surface. Short-circuit here so every caller (IoT, web, scheduler) stays
	// consistent without having to resolve the mode at each call site.
	if s.GetPresenceMode(ctx) == "binary" {
		return nil
	}

	// Validate student exists before INSERT (prevents FK constraint errors in logs)
	if err := s.validateStudentExists(ctx, visit.StudentID); err != nil {
		return &ActiveError{Op: "CreateVisit", Err: err}
	}

	// Validate active group exists before INSERT (prevents FK constraint errors in logs)
	if err := s.validateActiveGroupExists(ctx, visit.ActiveGroupID); err != nil {
		return &ActiveError{Op: "CreateVisit", Err: err}
	}

	deviceID, staffID := s.extractContextIDs(ctx)

	// Ensure no existing active visit for this student
	if err := s.ensureStudentHasNoActiveVisit(ctx, visit.StudentID); err != nil {
		if activeErr, ok := err.(*ActiveError); ok {
			return activeErr
		}
		return &ActiveError{Op: "CreateVisit", Err: ErrDatabaseOperation}
	}

	// Handle attendance (create new or update on re-entry)
	if err := s.ensureOrUpdateAttendance(ctx, visit, staffID, deviceID); err != nil {
		if activeErr, ok := err.(*ActiveError); ok {
			return activeErr
		}
		return &ActiveError{Op: "CreateVisit", Err: ErrDatabaseOperation}
	}

	// Auto-clear sickness / excused flags when student checks in
	// (only triggers when the tenant's clear_mode setting is "next_checkin").
	s.autoClearStudentSickness(ctx, visit.StudentID)
	s.autoClearStudentExcused(ctx, visit.StudentID)

	// Create the visit record
	visit.SetTenantID(tenant.FromContext(ctx))
	if err := s.visitRepo.Create(ctx, visit); err != nil {
		// The partial unique index uniq_active_visits_open_per_student is the
		// race-safety net behind ensureStudentHasNoActiveVisit above. When
		// two concurrent requests both pass the read-then-write check, the
		// loser hits 23505 here. Translate to ErrStudentAlreadyActive so the
		// IoT handler maps it to 409 Conflict instead of 500.
		if isDuplicateActiveVisitViolation(err) {
			return &ActiveError{Op: "CreateVisit", Err: ErrStudentAlreadyActive}
		}
		return &ActiveError{Op: "CreateVisit", Err: ErrDatabaseOperation}
	}

	// WP-B10: mirror the check-in into schedule.instance_students when the
	// visit corresponds to a planned instance. The mirror is graceful-
	// degradation-by-design — it returns nil for walk-ins, pre-start
	// races, or any error — so we never block a visit write on it.
	// Snapshot is threaded into the broadcast so the SSE event carries
	// attendance fields when applicable.
	var snapshot *AttendanceSnapshot
	if s.attendanceSyncer != nil {
		snapshot = s.attendanceSyncer.MirrorCheckInForVisit(ctx, visit)
	}

	// Broadcast SSE event (fire-and-forget, outside transaction)
	s.broadcastVisitCreated(ctx, visit, snapshot)

	return nil
}

// isDuplicateActiveVisitViolation returns true when err carries PostgreSQL
// error code 23505 (unique_violation) on the partial unique index
// uniq_active_visits_open_per_student, defined in migration 1.15.45 on
// active.visits (tenant_id, student_id) WHERE exit_time IS NULL.
//
// We match by constraint name (Field 'n') rather than just the error code
// so a future unrelated unique index on active.visits doesn't accidentally
// translate into ErrStudentAlreadyActive.
func isDuplicateActiveVisitViolation(err error) bool {
	if err == nil {
		return false
	}
	var dbErr *base.DatabaseError
	if errors.As(err, &dbErr) {
		err = dbErr.Err
	}
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		return pgErr.Field('C') == "23505" &&
			pgErr.Field('n') == "uniq_active_visits_open_per_student"
	}
	return false
}

// isNotFoundError checks if an error is due to "not found" (sql.ErrNoRows) vs. other database errors
func isNotFoundError(err error) bool {
	var dbErr *base.DatabaseError
	if errors.As(err, &dbErr) {
		return errors.Is(dbErr.Err, sql.ErrNoRows)
	}
	return false
}

// validateStudentExists checks if a student exists, returning appropriate errors
func (s *service) validateStudentExists(ctx context.Context, studentID int64) error {
	if _, err := s.studentRepo.FindByID(ctx, studentID); err != nil {
		if isNotFoundError(err) {
			return ErrStudentNotFound
		}
		return err
	}
	return nil
}

// validateActiveGroupExists checks if an active group exists, returning appropriate errors
func (s *service) validateActiveGroupExists(ctx context.Context, groupID int64) error {
	if _, err := s.groupRepo.FindByID(ctx, groupID); err != nil {
		if isNotFoundError(err) {
			return ErrActiveGroupNotFound
		}
		return err
	}
	return nil
}

// validateStaffExists checks if a staff member exists, returning appropriate errors
func (s *service) validateStaffExists(ctx context.Context, staffID int64) error {
	if _, err := s.staffRepo.FindByID(ctx, staffID); err != nil {
		if isNotFoundError(err) {
			return ErrStaffNotFound
		}
		return err
	}
	return nil
}

// extractContextIDs extracts device and staff IDs from context
func (s *service) extractContextIDs(ctx context.Context) (deviceID, staffID int64) {
	if deviceCtx := device.DeviceFromCtx(ctx); deviceCtx != nil {
		deviceID = deviceCtx.ID
	}
	if staffCtx := device.StaffFromCtx(ctx); staffCtx != nil {
		staffID = staffCtx.ID
	}
	return deviceID, staffID
}

func (s *service) UpdateVisit(ctx context.Context, visit *active.Visit) error {
	if visit == nil || visit.Validate() != nil {
		return &ActiveError{Op: "UpdateVisit", Err: ErrInvalidData}
	}

	if s.visitRepo.Update(ctx, visit) != nil {
		return &ActiveError{Op: "UpdateVisit", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) DeleteVisit(ctx context.Context, id int64) error {
	_, err := s.visitRepo.FindByID(ctx, id)
	if err != nil {
		return &ActiveError{Op: "DeleteVisit", Err: ErrVisitNotFound}
	}

	if s.visitRepo.Delete(ctx, id) != nil {
		return &ActiveError{Op: "DeleteVisit", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) ListVisits(ctx context.Context, options *base.QueryOptions) ([]*active.Visit, error) {
	visits, err := s.visitRepo.List(ctx, options)
	if err != nil {
		return nil, &ActiveError{Op: "ListVisits", Err: ErrDatabaseOperation}
	}
	return visits, nil
}

func (s *service) FindVisitsByStudentID(ctx context.Context, studentID int64) ([]*active.Visit, error) {
	visits, err := s.visitRepo.FindActiveByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ActiveError{Op: "FindVisitsByStudentID", Err: ErrDatabaseOperation}
	}
	return visits, nil
}

func (s *service) FindVisitsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.Visit, error) {
	visits, err := s.visitRepo.FindByActiveGroupID(ctx, activeGroupID)
	if err != nil {
		return nil, &ActiveError{Op: "FindVisitsByActiveGroupID", Err: ErrDatabaseOperation}
	}
	return visits, nil
}

func (s *service) FindVisitsByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Visit, error) {
	if start.After(end) {
		return nil, &ActiveError{Op: "FindVisitsByTimeRange", Err: ErrInvalidTimeRange}
	}

	visits, err := s.visitRepo.FindByTimeRange(ctx, start, end)
	if err != nil {
		return nil, &ActiveError{Op: "FindVisitsByTimeRange", Err: ErrDatabaseOperation}
	}
	return visits, nil
}

func (s *service) EndVisit(ctx context.Context, id int64) error {
	// Binary-mode tenants don't keep visit rows, so there's nothing to end.
	// Callers that hold a stale visit ID from before a mode switch hit this
	// no-op path instead of a missing-row error.
	if s.GetPresenceMode(ctx) == "binary" {
		return nil
	}

	endedVisit, err := s.endVisitRecord(ctx, id)
	if err != nil {
		if activeErr, ok := err.(*ActiveError); ok {
			return activeErr
		}
		return &ActiveError{Op: "EndVisit", Err: ErrDatabaseOperation}
	}

	// WP-B10: load (not mutate) attendance snapshot for SSE enrichment.
	// Per spec: check-out does NOT change instance_students.status.
	var snapshot *AttendanceSnapshot
	if s.attendanceSyncer != nil {
		snapshot = s.attendanceSyncer.LoadAttendanceForVisit(ctx, endedVisit)
	}

	s.broadcastVisitCheckout(ctx, endedVisit, snapshot)
	return nil
}

// endVisitRecord ends the visit record and returns the updated visit
func (s *service) endVisitRecord(ctx context.Context, id int64) (*active.Visit, error) {
	visit, err := s.visitRepo.FindByID(ctx, id)
	if err != nil || visit == nil {
		return nil, &ActiveError{Op: "EndVisit", Err: ErrVisitNotFound}
	}

	if s.visitRepo.EndVisit(ctx, id) != nil {
		return nil, &ActiveError{Op: "EndVisit", Err: ErrDatabaseOperation}
	}

	visit, err = s.visitRepo.FindByID(ctx, id)
	if err != nil || visit == nil {
		return nil, &ActiveError{Op: "EndVisit", Err: ErrVisitNotFound}
	}

	return visit, nil
}

// broadcastVisitCheckout broadcasts SSE event for visit checkout.
// snapshot (WP-B10) may be nil — when present, it enriches the event
// with attendance_status/substatus/note so the frontend can display
// the current attendance state alongside the checkout line.
func (s *service) broadcastVisitCheckout(ctx context.Context, endedVisit *active.Visit, snapshot *AttendanceSnapshot) {
	if s.broadcaster == nil || endedVisit == nil {
		return
	}

	activeGroupID := fmt.Sprintf("%d", endedVisit.ActiveGroupID)
	studentID := fmt.Sprintf("%d", endedVisit.StudentID)
	studentName, studentRec := s.getStudentDisplayData(ctx, endedVisit.StudentID)

	data := realtime.EventData{
		StudentID:   &studentID,
		StudentName: &studentName,
	}
	applyAttendanceSnapshot(&data, snapshot)

	event := realtime.NewEvent(
		realtime.EventStudentCheckOut,
		activeGroupID,
		data,
	)

	s.broadcastWithLogging(ctx, activeGroupID, studentID, event, "student_checkout")
	s.broadcastToEducationalGroup(ctx, studentRec, event)

	// Notify all clients so dashboard counts refresh
	_ = s.broadcaster.BroadcastToAll(realtime.NewEvent(realtime.EventDashboardCountsChanged, "", realtime.EventData{}))
}

// broadcastToEducationalGroup mirrors active-group broadcasts to the student's OGS group topic
func (s *service) broadcastToEducationalGroup(ctx context.Context, student *userModels.Student, event realtime.Event) {
	if s.broadcaster == nil || student == nil || student.GroupID == nil {
		return
	}
	groupID := fmt.Sprintf("edu:%d", *student.GroupID)
	if err := s.broadcaster.BroadcastToGroup(tenant.FromContext(ctx), groupID, event); err != nil {
		studentID := ""
		if event.Data.StudentID != nil {
			studentID = *event.Data.StudentID
		}
		s.getLogger().Error(sseErrorMessage+" for educational topic",
			slog.String("error", err.Error()),
			slog.String("event_type", string(event.Type)),
			slog.String("education_group_topic", groupID),
			slog.String("student_id", studentID),
		)
	}
}

// broadcastStudentCheckoutEvents sends checkout SSE events for each visit.
// This helper reduces cognitive complexity in session timeout processing.
//
// TODO(wp-b11-or-later): batch if hot. Each visit triggers two independent
// schedule queries inside LoadAttendanceForVisit (FindByActiveGroupID,
// FindByInstanceAndStudent). A session timeout flushing N students = 2N
// queries. Not catastrophic at v1 scale, but if schools grow we should add
// a batched FindByInstanceAndStudentIDs path. Not scoped into WP-B10.
func (s *service) broadcastStudentCheckoutEvents(ctx context.Context, sessionIDStr string, visitsToNotify []visitSSEData) {
	// Parse session ID once — all visits in this batch share the same
	// active_group_id, which IS the session. We construct a minimal Visit
	// per student for the snapshot lookup.
	var sessionID int64
	if parsed, perr := strconv.ParseInt(sessionIDStr, 10, 64); perr == nil {
		sessionID = parsed
	}

	for _, visitData := range visitsToNotify {
		studentIDStr := fmt.Sprintf("%d", visitData.StudentID)
		studentName := visitData.Name

		data := realtime.EventData{
			StudentID:   &studentIDStr,
			StudentName: &studentName,
		}
		if s.attendanceSyncer != nil && sessionID > 0 {
			snapshot := s.attendanceSyncer.LoadAttendanceForVisit(ctx, &active.Visit{
				StudentID:     visitData.StudentID,
				ActiveGroupID: sessionID,
			})
			applyAttendanceSnapshot(&data, snapshot)
		}

		checkoutEvent := realtime.NewEvent(
			realtime.EventStudentCheckOut,
			sessionIDStr,
			data,
		)

		s.broadcastWithLogging(ctx, sessionIDStr, studentIDStr, checkoutEvent, "student_checkout")
		s.broadcastToEducationalGroup(ctx, visitData.Student, checkoutEvent)
	}

	// Single global broadcast for the entire batch
	if len(visitsToNotify) > 0 && s.broadcaster != nil {
		_ = s.broadcaster.BroadcastToAll(realtime.NewEvent(realtime.EventDashboardCountsChanged, "", realtime.EventData{}))
	}
}

// broadcastActivityEndEvent sends the activity_end SSE event for a completed session.
// This helper reduces cognitive complexity in session timeout processing.
func (s *service) broadcastActivityEndEvent(ctx context.Context, sessionID int64, sessionIDStr string) {
	finalGroup, err := s.groupRepo.FindByID(ctx, sessionID)
	if err != nil || finalGroup == nil {
		return
	}

	roomIDStr := fmt.Sprintf("%d", finalGroup.RoomID)
	activityName := s.getActivityName(ctx, finalGroup.GroupID)
	roomName := s.getRoomName(ctx, finalGroup.RoomID)

	event := realtime.NewEvent(
		realtime.EventActivityEnd,
		sessionIDStr,
		realtime.EventData{
			ActivityName: &activityName,
			RoomID:       &roomIDStr,
			RoomName:     &roomName,
		},
	)

	s.broadcastWithLogging(ctx, sessionIDStr, "", event, "activity_end")

	// Notify all clients (including zero-topic) so dashboard refreshes
	_ = s.broadcaster.BroadcastToAll(realtime.NewEvent(realtime.EventDashboardCountsChanged, "", realtime.EventData{}))
}

// broadcastWithLogging broadcasts an event and logs any errors.
func (s *service) broadcastWithLogging(ctx context.Context, activeGroupID, studentID string, event realtime.Event, eventType string) {
	if err := s.broadcaster.BroadcastToGroup(tenant.FromContext(ctx), activeGroupID, event); err != nil {
		attrs := []slog.Attr{
			slog.String("error", err.Error()),
			slog.String("event_type", eventType),
			slog.String("active_group_id", activeGroupID),
		}
		if studentID != "" {
			attrs = append(attrs, slog.String("student_id", studentID))
		}
		s.getLogger().LogAttrs(context.Background(), slog.LevelError, sseErrorMessage, attrs...)
	}
}

// getActivityName retrieves the activity name by group ID, returning empty string on error.
// A nil groupID marks a spontaneous session (WP-B6): there is no template to look up,
// so we return an empty name and leave the display decision to the caller.
func (s *service) getActivityName(ctx context.Context, groupID *int64) string {
	if groupID == nil {
		return ""
	}
	activity, err := s.activityGroupRepo.FindByID(ctx, *groupID)
	if err != nil || activity == nil {
		return ""
	}
	return activity.Name
}

// getRoomName retrieves the room name by room ID, returning empty string on error.
func (s *service) getRoomName(ctx context.Context, roomID int64) string {
	room, err := s.roomRepo.FindByID(ctx, roomID)
	if err != nil || room == nil {
		return ""
	}
	return room.Name
}

func (s *service) GetStudentCurrentVisit(ctx context.Context, studentID int64) (*active.Visit, error) {
	visit, err := s.visitRepo.GetCurrentByStudentID(ctx, studentID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, &ActiveError{Op: "GetStudentCurrentVisit", Err: ErrVisitNotFound}
		}
		return nil, &ActiveError{Op: "GetStudentCurrentVisit", Err: ErrDatabaseOperation}
	}

	if visit == nil {
		return nil, &ActiveError{Op: "GetStudentCurrentVisit", Err: ErrVisitNotFound}
	}

	return visit, nil
}

func (s *service) GetStudentCurrentVisitWithRoom(ctx context.Context, studentID int64) (*active.Visit, error) {
	visit, err := s.visitRepo.GetCurrentByStudentIDWithRoom(ctx, studentID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, &ActiveError{Op: "GetStudentCurrentVisitWithRoom", Err: ErrVisitNotFound}
		}
		return nil, &ActiveError{Op: "GetStudentCurrentVisitWithRoom", Err: ErrDatabaseOperation}
	}

	if visit == nil {
		return nil, &ActiveError{Op: "GetStudentCurrentVisitWithRoom", Err: ErrVisitNotFound}
	}

	return visit, nil
}

func (s *service) GetStudentsCurrentVisits(ctx context.Context, studentIDs []int64) (map[int64]*active.Visit, error) {
	if len(studentIDs) == 0 {
		return map[int64]*active.Visit{}, nil
	}

	visits, err := s.visitRepo.GetCurrentByStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, &ActiveError{Op: "GetStudentsCurrentVisits", Err: ErrDatabaseOperation}
	}

	if visits == nil {
		visits = make(map[int64]*active.Visit)
	}

	return visits, nil
}

func (s *service) CountActiveVisitsByRoomID(ctx context.Context, roomID int64) (int, error) {
	count, err := s.visitRepo.CountActiveByRoomID(ctx, roomID)
	if err != nil {
		return 0, &ActiveError{Op: "CountActiveVisitsByRoomID", Err: ErrDatabaseOperation}
	}
	return count, nil
}

func (s *service) CountActiveVisitsByActiveGroupID(ctx context.Context, activeGroupID int64) (int, error) {
	count, err := s.visitRepo.CountActiveByGroupID(ctx, activeGroupID)
	if err != nil {
		return 0, &ActiveError{Op: "CountActiveVisitsByActiveGroupID", Err: ErrDatabaseOperation}
	}
	return count, nil
}

// Group Supervisor operations
func (s *service) GetGroupSupervisor(ctx context.Context, id int64) (*active.GroupSupervisor, error) {
	supervisor, err := s.supervisorRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetGroupSupervisor", Err: ErrGroupSupervisorNotFound}
	}
	return supervisor, nil
}

func (s *service) CreateGroupSupervisor(ctx context.Context, supervisor *active.GroupSupervisor) error {
	if supervisor == nil || supervisor.Validate() != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: ErrInvalidData}
	}

	// Validate active group exists before INSERT (prevents FK constraint errors in logs)
	if err := s.validateActiveGroupExists(ctx, supervisor.GroupID); err != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: err}
	}

	// Validate staff exists before INSERT (prevents FK constraint errors in logs)
	if err := s.validateStaffExists(ctx, supervisor.StaffID); err != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: err}
	}

	// Check if staff is already supervising this group (only check active supervisors)
	supervisors, err := s.supervisorRepo.FindByActiveGroupID(ctx, supervisor.GroupID, true)
	if err != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: ErrDatabaseOperation}
	}

	for _, s := range supervisors {
		if s.StaffID == supervisor.StaffID {
			return &ActiveError{Op: "CreateGroupSupervisor", Err: ErrStaffAlreadySupervising}
		}
	}

	supervisor.SetTenantID(tenant.FromContext(ctx))
	if s.supervisorRepo.Create(ctx, supervisor) != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) UpdateGroupSupervisor(ctx context.Context, supervisor *active.GroupSupervisor) error {
	if supervisor == nil || supervisor.Validate() != nil {
		return &ActiveError{Op: "UpdateGroupSupervisor", Err: ErrInvalidData}
	}

	if s.supervisorRepo.Update(ctx, supervisor) != nil {
		return &ActiveError{Op: "UpdateGroupSupervisor", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) DeleteGroupSupervisor(ctx context.Context, id int64) error {
	_, err := s.supervisorRepo.FindByID(ctx, id)
	if err != nil {
		return &ActiveError{Op: "DeleteGroupSupervisor", Err: ErrGroupSupervisorNotFound}
	}

	if s.supervisorRepo.Delete(ctx, id) != nil {
		return &ActiveError{Op: "DeleteGroupSupervisor", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) ListGroupSupervisors(ctx context.Context, options *base.QueryOptions) ([]*active.GroupSupervisor, error) {
	supervisors, err := s.supervisorRepo.List(ctx, options)
	if err != nil {
		return nil, &ActiveError{Op: "ListGroupSupervisors", Err: ErrDatabaseOperation}
	}
	return supervisors, nil
}

func (s *service) FindSupervisorsByStaffID(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error) {
	supervisors, err := s.supervisorRepo.FindActiveByStaffID(ctx, staffID)
	if err != nil {
		return nil, &ActiveError{Op: "FindSupervisorsByStaffID", Err: ErrDatabaseOperation}
	}
	return supervisors, nil
}

func (s *service) FindSupervisorsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.GroupSupervisor, error) {
	supervisors, err := s.supervisorRepo.FindByActiveGroupID(ctx, activeGroupID, true)
	if err != nil {
		return nil, &ActiveError{Op: "FindSupervisorsByActiveGroupID", Err: ErrDatabaseOperation}
	}
	return supervisors, nil
}

func (s *service) FindSupervisorsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64) ([]*active.GroupSupervisor, error) {
	supervisors, err := s.supervisorRepo.FindByActiveGroupIDs(ctx, activeGroupIDs, true)
	if err != nil {
		return nil, &ActiveError{Op: "FindSupervisorsByActiveGroupIDs", Err: ErrDatabaseOperation}
	}
	return supervisors, nil
}

func (s *service) EndSupervision(ctx context.Context, id int64) error {
	// Verify supervision exists first
	_, err := s.supervisorRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &ActiveError{Op: "EndSupervision", Err: ErrGroupSupervisorNotFound}
		}
		return &ActiveError{Op: "EndSupervision", Err: fmt.Errorf("failed to verify supervision: %w", err)}
	}

	if err := s.supervisorRepo.EndSupervision(ctx, id); err != nil {
		return &ActiveError{Op: "EndSupervision", Err: fmt.Errorf("end supervision failed: %w", err)}
	}
	return nil
}

func (s *service) GetStaffActiveSupervisions(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error) {
	supervisors, err := s.supervisorRepo.FindActiveByStaffID(ctx, staffID)
	if err != nil {
		return nil, &ActiveError{Op: "GetStaffActiveSupervisions", Err: ErrDatabaseOperation}
	}

	// Filter only active supervisions
	var activeSupervisions []*active.GroupSupervisor
	for _, supervisor := range supervisors {
		if supervisor.IsActive() {
			activeSupervisions = append(activeSupervisions, supervisor)
		}
	}

	return activeSupervisions, nil
}

// GetAllActiveSupervisions returns all currently active supervisions across all staff.
// Used by admin supervision overview to display all active rooms.
func (s *service) GetAllActiveSupervisions(ctx context.Context) ([]*active.GroupSupervisor, error) {
	supervisors, err := s.supervisorRepo.FindAllActive(ctx)
	if err != nil {
		return nil, &ActiveError{Op: "GetAllActiveSupervisions", Err: ErrDatabaseOperation}
	}

	var activeSupervisions []*active.GroupSupervisor
	for _, supervisor := range supervisors {
		if supervisor.IsActive() {
			activeSupervisions = append(activeSupervisions, supervisor)
		}
	}

	return activeSupervisions, nil
}

// GetCrossTenantStudents returns students visiting from other tenants.
func (s *service) GetCrossTenantStudents(ctx context.Context, hostingTenantID int64) ([]active.CrossTenantStudent, error) {
	if s.crossTenantRepo == nil {
		return []active.CrossTenantStudent{}, nil
	}

	students, err := s.crossTenantRepo.FindCrossTenantStudents(ctx, hostingTenantID)
	if err != nil {
		return nil, &ActiveError{Op: "GetCrossTenantStudents", Err: fmt.Errorf("query failed: %w", err)}
	}

	s.getLogger().Info("cross-tenant students queried",
		slog.Int64("hosting_tenant_id", hostingTenantID),
		slog.Int("count", len(students)),
	)

	return students, nil
}

// GetTrackingIndicators returns per-student match results for the given labels.
// For each student, it checks today's visits and matches activity group name + room name
// against each label using case-insensitive substring matching.
func (s *service) GetTrackingIndicators(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error) {
	result := make(map[int64][]bool, len(studentIDs))
	if len(studentIDs) == 0 || len(labels) == 0 {
		return result, nil
	}

	visitNames, err := s.visitRepo.GetTodayVisitNamesForStudents(ctx, studentIDs)
	if err != nil {
		return nil, &ActiveError{Op: "GetTrackingIndicators", Err: ErrDatabaseOperation}
	}

	// Build a map of student ID → concatenated visit texts for matching.
	studentVisitTexts := make(map[int64][]string, len(studentIDs))
	for _, vn := range visitNames {
		text := strings.ToLower(strings.TrimSpace(vn.ActivityGroupName + " " + vn.RoomName))
		studentVisitTexts[vn.StudentID] = append(studentVisitTexts[vn.StudentID], text)
	}

	// Lowercase the labels once.
	lowerLabels := make([]string, len(labels))
	for i, l := range labels {
		lowerLabels[i] = strings.ToLower(strings.TrimSpace(l))
	}

	// For each student, check each label against their visit texts.
	for _, sid := range studentIDs {
		matches := make([]bool, len(labels))
		texts := studentVisitTexts[sid]
		for li, ll := range lowerLabels {
			for _, t := range texts {
				if strings.Contains(t, ll) {
					matches[li] = true
					break
				}
			}
		}
		result[sid] = matches
	}

	return result, nil
}

// visitSSEData holds data needed for SSE broadcasts after a visit is ended
type visitSSEData struct {
	VisitID   int64
	StudentID int64
	Name      string
	Student   *userModels.Student
}
