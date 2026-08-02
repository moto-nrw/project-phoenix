package active

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/analytics"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
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
)

// Broadcaster interface (re-exported from realtime for convenience)
type Broadcaster = realtime.Broadcaster

const (
	// sseErrorMessage is the standard error message for SSE broadcast failures
	sseErrorMessage = "SSE broadcast failed"

	activeSupervisionReasonActivityStarted = "activity_started"
	activeSupervisionReasonActivityEnded   = "activity_ended"
	activeSupervisionReasonStudentMoved    = "student_moved"
)

// defaultDeviceOnlineWindow is the fallback online/offline cutoff used by the
// abandoned-session cleanup when no tenant override
// (iot.device_online_window_minutes) is configured. Moved off the iot.Device
// model per issue #586 (Rule 12: models hold data, not decisions).
const defaultDeviceOnlineWindow = 5 * time.Minute

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
	ResolveInt(ctx context.Context, key string) (int, error)
}

// TimetableBridgeCompleter marks timetable instances that are still bridged
// to active groups as completed. Implemented by schedule.ActivityInstanceRepository.
type TimetableBridgeCompleter interface {
	CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error)
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
	StudentStatusRepo active.StudentStatusDayRepository

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

	// Optional: Product analytics tracker (nil-safe, no student PII)
	Tracker analytics.Tracker

	// Optional: Work session service for NFC auto-check-in
	WorkSessionService WorkSessionService

	// Optional: Attendance sync (WP-B10). When non-nil, visit create/end
	// calls mirror into schedule.instance_students and enrich check-in/out
	// SSE events with attendance status/substatus/note.
	AttendanceSyncer AttendanceSyncer

	// Optional: Timetable bridge cleanup for force-ended IoT sessions.
	TimetableBridgeCompleter TimetableBridgeCompleter

	// Optional: Structured logger (nil-safe, Phase 2b will add logging calls)
	Logger *slog.Logger
}

// Service implements the Active Service interface
type service struct {
	ServiceDependencies

	// Optional: Tenant-scoped settings resolver for auto-clear logic.
	// When nil, auto-clear falls back to the registry default behavior.
	// Injected post-construction via SetSettingsService.
	settings SettingsResolver
}

// SetSettingsService injects the tenant-scoped settings resolver.
// Called from the factory after settingsService is constructed.
func (s *service) SetSettingsService(resolver SettingsResolver) {
	s.settings = resolver
}

// GetPresenceMode returns the tenant's resolved presence mode
// ("detailed" | "binary"), falling back to "detailed" whenever the settings
// resolver is nil, the key is unset/empty, or the lookup errors. Mirrors the
// fallback logic of services/config.ResolvePresenceModeForTenant but avoids
// importing the config package here (it would create a dep cycle through the
// factory).
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
	return cmp.Or(s.Logger, slog.Default())
}

// trackProductEvent captures a product analytics event scoped to the tenant
// from context. Fire-and-forget and nil-safe. GDPR: props must never contain
// student IDs or any student PII — only tenant-level properties.
//
// The capture is deferred via tenant.RegisterAfterCommit so events fire only
// after the surrounding tenant transaction commits — a rolled-back check-in
// must not appear in analytics. Outside a tenant tx the capture runs
// immediately.
func (s *service) trackProductEvent(ctx context.Context, event string, props map[string]any) {
	if s.Tracker == nil {
		return
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return
	}
	if props == nil {
		props = map[string]any{}
	}
	school := strconv.FormatInt(tenantID, 10)
	props["school_id"] = tenantID
	props["$groups"] = map[string]any{"school": school}
	tenant.RegisterAfterCommit(ctx, func() {
		s.Tracker.Capture("school:"+school, event, props)
	})
}

// checkout_type values for the student_checked_out analytics event — which
// flow ended the attendance, orthogonal to method (rfid vs manual).
const (
	checkoutTypeDaily  = "daily"  // daily-checkout kiosk flow (CheckOutStudentFromDevice)
	checkoutTypeToggle = "toggle" // kiosk toggle-out (ToggleStudentAttendance)
	checkoutTypeWeb    = "web"    // web/staff-UI checkout (CheckOutStudent)
)

// dailyCheckoutSource is the `source` carried by the student_checkout event of
// the kiosk daily-checkout flow. Kept as its own value (rather than the
// checkoutTypeDaily label) because it is a wire field consumers may already
// match on.
const dailyCheckoutSource = "daily_checkout"

// checkoutSourceLabel maps an internal checkout_type onto the student_checkout
// `source` wire field, preserving the historical value of the daily flow.
func checkoutSourceLabel(checkoutType string) string {
	if checkoutType == checkoutTypeDaily {
		return dailyCheckoutSource
	}
	return checkoutType
}

// attendanceMethod derives how an attendance change was triggered: RFID/kiosk
// requests carry device auth in context, everything else is web/manual.
func attendanceMethod(ctx context.Context) string {
	if device.IsIoTDeviceRequest(ctx) {
		return "rfid"
	}
	return "manual"
}

// NewService creates a new active service instance
func NewService(deps ServiceDependencies) Service {
	return &service{ServiceDependencies: deps}
}

// Active Group operations
func (s *service) GetActiveGroup(ctx context.Context, id int64) (*active.Group, error) {
	group, err := s.GroupRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &ActiveError{Op: "GetActiveGroup", Err: ErrActiveGroupNotFound}
		}
		return nil, &ActiveError{Op: "GetActiveGroup", Err: ErrDatabaseOperation}
	}
	if group == nil {
		return nil, &ActiveError{Op: "GetActiveGroup", Err: ErrActiveGroupNotFound}
	}

	// Ensure we always have room metadata so downstream callers
	// (location resolver, SSE payloads) can render friendly labels.
	if group != nil && group.Room == nil && group.RoomID > 0 {
		if room, roomErr := s.RoomRepo.FindByID(ctx, group.RoomID); roomErr == nil {
			group.Room = room
		}
	}

	return group, nil
}

func (s *service) GetActiveGroupsByIDs(ctx context.Context, groupIDs []int64) (map[int64]*active.Group, error) {
	if len(groupIDs) == 0 {
		return map[int64]*active.Group{}, nil
	}

	groups, err := s.GroupRepo.FindByIDs(ctx, groupIDs)
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
		hasConflict, _, err := s.GroupRepo.CheckRoomConflict(ctx, group.RoomID, 0)
		if err != nil {
			return &ActiveError{Op: "CreateActiveGroup", Err: fmt.Errorf("check room conflict: %w", err)}
		}
		if hasConflict {
			return &ActiveError{Op: "CreateActiveGroup", Err: ErrRoomConflict}
		}
	}

	group.SetTenantID(tenant.FromContext(ctx))
	if err := s.GroupRepo.Create(ctx, group); err != nil {
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
		hasConflict, _, err := s.GroupRepo.CheckRoomConflict(ctx, group.RoomID, group.ID)
		if err != nil {
			return &ActiveError{Op: "UpdateActiveGroup", Err: fmt.Errorf("check room conflict: %w", err)}
		}
		if hasConflict {
			return &ActiveError{Op: "UpdateActiveGroup", Err: ErrRoomConflict}
		}
	}

	if err := s.GroupRepo.Update(ctx, group); err != nil {
		return &ActiveError{Op: "UpdateActiveGroup", Err: fmt.Errorf("update failed: %w", err)}
	}

	return nil
}

func (s *service) DeleteActiveGroup(ctx context.Context, id int64) error {
	// Check if there are any active visits for this group
	visits, err := s.VisitRepo.FindByActiveGroupID(ctx, id)
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
	_, err = s.GroupRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &ActiveError{Op: "DeleteActiveGroup", Err: ErrActiveGroupNotFound}
		}
		return &ActiveError{Op: "DeleteActiveGroup", Err: fmt.Errorf("find group: %w", err)}
	}

	if err := s.GroupRepo.Delete(ctx, id); err != nil {
		return &ActiveError{Op: "DeleteActiveGroup", Err: fmt.Errorf("delete failed: %w", err)}
	}

	return nil
}

func (s *service) ListActiveGroups(ctx context.Context, options *base.QueryOptions) ([]*active.Group, error) {
	groups, err := s.GroupRepo.List(ctx, options)
	if err != nil {
		return nil, &ActiveError{Op: "ListActiveGroups", Err: fmt.Errorf("list failed: %w", err)}
	}
	return groups, nil
}

func (s *service) FindActiveGroupsByRoomID(ctx context.Context, roomID int64) ([]*active.Group, error) {
	groups, err := s.GroupRepo.FindActiveByRoomID(ctx, roomID)
	if err != nil {
		return nil, &ActiveError{Op: "FindActiveGroupsByRoomID", Err: fmt.Errorf("find by room: %w", err)}
	}
	return groups, nil
}

func (s *service) FindDeviceActiveGroupInRoom(ctx context.Context, roomID int64, deviceID int64) (*active.Group, error) {
	group, err := s.GroupRepo.FindActiveByRoomIDAndDeviceID(ctx, roomID, deviceID)
	if err != nil {
		return nil, &ActiveError{Op: "FindDeviceActiveGroupInRoom", Err: fmt.Errorf("find by room and device: %w", err)}
	}
	return group, nil
}

func (s *service) FindActiveGroupsByGroupID(ctx context.Context, groupID int64) ([]*active.Group, error) {
	groups, err := s.GroupRepo.FindActiveByGroupID(ctx, groupID)
	if err != nil {
		return nil, &ActiveError{Op: "FindActiveGroupsByGroupID", Err: ErrDatabaseOperation}
	}
	return groups, nil
}

func (s *service) EndActiveGroupSession(ctx context.Context, id int64) error {
	// Ending a group by hand (POST /active/groups/{id}/end, and the Schulhof
	// stale-session sweep) is a session end like any other, so it closes the
	// timetable side too (#1747 review). Skipping it leaves the linked instance
	// active with its expected rows unfinalized, and nothing repairs that
	// afterwards: the nightly bridge only looks at active.groups that are still
	// running, and this one is not.
	//
	// The state check runs BEFORE the bridge on purpose. EndActivitySession
	// rejects an already-ended group as ErrActiveGroupAlreadyEnded, which the
	// handler renders as 4xx — and the tenant middleware only rolls back on its
	// own for 5xx, so bridging first would commit a completed instance beside
	// that rejection.
	group, err := s.GroupRepo.FindByID(ctx, id)
	if err != nil || group == nil {
		return &ActiveError{Op: "EndActiveGroupSession", Err: ErrActiveGroupNotFound}
	}
	if !group.IsActive() {
		return &ActiveError{Op: "EndActiveGroupSession", Err: ErrActiveGroupAlreadyEnded}
	}

	// Both writes are one transition (#1747 review). runInSessionTx joins the
	// caller's transaction when there is one (the HTTP path) and opens its own
	// when there is none — the Schulhof sweep calls this straight from the
	// scheduler, where a committed bridge write beside a failed session end
	// would leave a completed instance next to an open group that no later run
	// repairs.
	return s.runInSessionTx(ctx, func(txCtx context.Context) error {
		// Ordered ahead of the session end for the same reason as everywhere
		// else: EndActivitySession emits its checkout and activity-ended SSE
		// events eagerly, so the failure-prone half has to run before the
		// announcing one.
		if err := s.completeTimetableMirrorsForEndedSessions(txCtx, []int64{id}); err != nil {
			return &ActiveError{Op: "EndActiveGroupSession", Err: err}
		}

		// Delegate to EndActivitySession which properly ends visits and broadcasts SSE
		if err := s.EndActivitySession(txCtx, id); err != nil {
			// The bridge already completed the mirrored instance, so the
			// transaction has to go. Joining the request's transaction means
			// nothing here can roll it back, and the tenant middleware only
			// rolls back on its own for 5xx — ErrActiveGroupAlreadyEnded and
			// friends render as 4xx and would commit exactly that split state.
			tenant.MarkRollback(txCtx)
			// Wrap the error with our operation name for clarity
			if activeErr, ok := err.(*ActiveError); ok {
				return &ActiveError{Op: "EndActiveGroupSession", Err: activeErr.Err}
			}
			return &ActiveError{Op: "EndActiveGroupSession", Err: err}
		}
		return nil
	})
}

func (s *service) GetActiveGroupWithVisits(ctx context.Context, id int64) (*active.Group, error) {
	// Get the active group
	group, err := s.GroupRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupWithVisits", Err: ErrActiveGroupNotFound}
	}

	// Get visits for this group
	visits, err := s.VisitRepo.FindByActiveGroupID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupWithVisits", Err: ErrDatabaseOperation}
	}

	group.Visits = visits
	return group, nil
}

func (s *service) GetActiveGroupWithSupervisors(ctx context.Context, id int64) (*active.Group, error) {
	// Get the active group
	group, err := s.GroupRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupWithSupervisors", Err: ErrActiveGroupNotFound}
	}

	// Get supervisors for this group (only active ones)
	supervisors, err := s.SupervisorRepo.FindByActiveGroupID(ctx, id, true)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupWithSupervisors", Err: ErrDatabaseOperation}
	}

	group.Supervisors = supervisors
	return group, nil
}

// Visit operations
func (s *service) GetVisit(ctx context.Context, id int64) (*active.Visit, error) {
	visit, err := s.VisitRepo.FindByID(ctx, id)
	if err != nil {
		if base.IsNoRows(err) {
			return nil, &ActiveError{Op: "GetVisit", Err: ErrVisitNotFound}
		}
		return nil, &ActiveError{Op: "GetVisit", Err: ErrDatabaseOperation}
	}
	if visit == nil {
		return nil, &ActiveError{Op: "GetVisit", Err: ErrVisitNotFound}
	}
	return visit, nil
}

func (s *service) CreateVisit(ctx context.Context, visit *active.Visit) error {
	if visit == nil || visit.Validate() != nil {
		return &ActiveError{Op: "CreateVisit", Err: ErrInvalidData}
	}

	// Validate the student exists and is not a graduated alumnus before INSERT
	// (prevents FK errors in logs and, via the FOR UPDATE lock, closes the race
	// against a concurrent grade-transition apply — see the helper doc, #405).
	// Runs BEFORE the binary-mode short-circuit: binary-mode check-ins reach
	// CreateVisit via the timetable path and would otherwise mark a departed
	// alumnus present without ever hitting this guard.
	if err := s.ensureStudentCheckinAllowed(ctx, visit.StudentID); err != nil {
		return &ActiveError{Op: "CreateVisit", Err: err}
	}

	// Binary-mode tenants don't track room visits — attendance is the only
	// surface. Short-circuit here so every caller (IoT, web, scheduler) stays
	// consistent without having to resolve the mode at each call site.
	if s.GetPresenceMode(ctx) == "binary" {
		return nil
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
	s.autoClearPlannedStudentStatuses(ctx, visit.StudentID)

	// Create the visit record
	visit.SetTenantID(tenant.FromContext(ctx))
	if err := s.VisitRepo.Create(ctx, visit); err != nil {
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
	if s.AttendanceSyncer != nil {
		snapshot = s.AttendanceSyncer.MirrorCheckInForVisit(ctx, visit)
	}

	// Broadcast SSE event (fire-and-forget, outside transaction)
	s.broadcastVisitCreated(ctx, visit, snapshot)

	return nil
}

// isDuplicateActiveVisitViolation returns true when err carries PostgreSQL
// error code 23505 (unique_violation) on the partial unique index
// uniq_active_visits_open_per_student, defined in migration 1.15.47 on
// active.visits (tenant_id, student_id) WHERE exit_time IS NULL.
//
// We match by constraint name (Field 'n') rather than just the error code
// so a future unrelated unique index on active.visits doesn't accidentally
// translate into ErrStudentAlreadyActive.
func isDuplicateActiveVisitViolation(err error) bool {
	return base.IsUniqueViolationOn(err, "uniq_active_visits_open_per_student")
}

// validateStudentExists checks if a student exists, returning appropriate errors
func (s *service) validateStudentExists(ctx context.Context, studentID int64) error {
	if _, err := s.StudentRepo.FindByID(ctx, studentID); err != nil {
		if base.IsNoRows(err) {
			return ErrStudentNotFound
		}
		return err
	}
	return nil
}

// ensureStudentCheckinAllowed validates the student exists and is not a
// graduated (alumnus) soft-deleted record, taking a FOR UPDATE row lock in the
// process. The lock is held for the caller's request transaction, so it
// serializes against a concurrent grade-transition apply that flips the same
// student to alumnus: the two transactions block on the shared row rather than
// interleaving into a stranded open attendance/visit the kiosk can no longer
// close (#405). Nil-safe for the partial unit-test services that never wire a
// StudentRepo; production always does.
func (s *service) ensureStudentCheckinAllowed(ctx context.Context, studentID int64) error {
	if s.StudentRepo == nil {
		return nil
	}
	student, err := s.StudentRepo.FindByIDForUpdate(ctx, studentID)
	if err != nil {
		if base.IsNoRows(err) {
			return ErrStudentNotFound
		}
		return err
	}
	if student.Status == userModels.StudentStatusAlumnus {
		return ErrStudentGraduated
	}
	return nil
}

// validateActiveGroupExists checks if an active group exists, returning appropriate errors
func (s *service) validateActiveGroupExists(ctx context.Context, groupID int64) error {
	if _, err := s.GroupRepo.FindByID(ctx, groupID); err != nil {
		if base.IsNoRows(err) {
			return ErrActiveGroupNotFound
		}
		return err
	}
	return nil
}

// validateStaffExists checks if a staff member exists, returning appropriate errors
func (s *service) validateStaffExists(ctx context.Context, staffID int64) error {
	if _, err := s.StaffRepo.FindByID(ctx, staffID); err != nil {
		if base.IsNoRows(err) {
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

	existing, err := s.VisitRepo.FindByID(ctx, visit.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &ActiveError{Op: "UpdateVisit", Err: ErrVisitNotFound}
		}
		return &ActiveError{Op: "UpdateVisit", Err: ErrDatabaseOperation}
	}
	if existing == nil {
		return &ActiveError{Op: "UpdateVisit", Err: ErrVisitNotFound}
	}

	isActiveGroupMove, transferAt, err := s.prepareVisitTransfer(ctx, existing, visit)
	if err != nil {
		return err
	}
	intervalChanged := visitIntervalChanged(existing, visit)

	if s.VisitRepo.Update(ctx, visit) != nil {
		return &ActiveError{Op: "UpdateVisit", Err: ErrDatabaseOperation}
	}

	if intervalChanged {
		if err := s.syncAttendanceForVisitRevision(ctx, existing, visit); err != nil {
			return &ActiveError{Op: "UpdateVisit", Err: ErrDatabaseOperation}
		}
	}

	if isActiveGroupMove {
		sourceSnapshot, targetSnapshot := s.syncMovedVisitAttendance(ctx, existing, visit, transferAt)
		s.broadcastVisitMoved(ctx, existing, visit, sourceSnapshot, targetSnapshot)
		s.trackProductEvent(ctx, "room_transfer", nil)
	} else if intervalChanged {
		if s.AttendanceSyncer != nil {
			s.AttendanceSyncer.MirrorVisitRevision(ctx, existing, visit)
		}
	}

	return nil
}

func visitIntervalChanged(previous, updated *active.Visit) bool {
	if previous == nil || updated == nil || !previous.EntryTime.Equal(updated.EntryTime) {
		return true
	}
	if previous.ExitTime == nil || updated.ExitTime == nil {
		return previous.ExitTime != updated.ExitTime
	}
	return !previous.ExitTime.Equal(*updated.ExitTime)
}

func (s *service) prepareVisitTransfer(
	ctx context.Context,
	existing, updated *active.Visit,
) (bool, time.Time, error) {
	if existing.ExitTime != nil || existing.ActiveGroupID == updated.ActiveGroupID {
		return false, time.Time{}, nil
	}

	targetGroup, err := s.GroupRepo.FindByID(ctx, updated.ActiveGroupID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, time.Time{}, &ActiveError{Op: "UpdateVisit", Err: ErrActiveGroupNotFound}
		}
		return false, time.Time{}, &ActiveError{Op: "UpdateVisit", Err: ErrDatabaseOperation}
	}
	if targetGroup == nil || !targetGroup.IsActive() {
		return false, time.Time{}, &ActiveError{Op: "UpdateVisit", Err: ErrActiveGroupNotFound}
	}

	transferAt := time.Now()
	if updated.ExitTime != nil {
		// A combined transfer/checkout has no separate transfer timestamp in the
		// request. Use checkout as the boundary so target check-in precedes close.
		transferAt = *updated.ExitTime
	}
	return true, transferAt, nil
}

// syncMovedVisitAttendance mirrors a transfer independently of SSE. The
// pre-update visit identifies the source slot; a transfer-time copy identifies
// the target slot. This avoids resolving checkout against the already-mutated
// target group.
func (s *service) syncMovedVisitAttendance(
	ctx context.Context,
	previousVisit, movedVisit *active.Visit,
	transferAt time.Time,
) (sourceSnapshot, targetSnapshot *AttendanceSnapshot) {
	if s.AttendanceSyncer == nil || previousVisit == nil || movedVisit == nil {
		return nil, nil
	}

	source := *previousVisit
	source.ExitTime = &transferAt
	sourceSnapshot = s.AttendanceSyncer.MirrorCheckOutForVisit(ctx, &source)

	target := *movedVisit
	target.EntryTime = transferAt
	target.ExitTime = nil
	targetSnapshot = s.AttendanceSyncer.MirrorCheckInForVisit(ctx, &target)

	if movedVisit.ExitTime != nil {
		target.ExitTime = movedVisit.ExitTime
		if closedSnapshot := s.AttendanceSyncer.MirrorCheckOutForVisit(ctx, &target); closedSnapshot != nil {
			targetSnapshot = closedSnapshot
		}
	}
	return sourceSnapshot, targetSnapshot
}

func (s *service) DeleteVisit(ctx context.Context, id int64) error {
	_, err := s.VisitRepo.FindByID(ctx, id)
	if err != nil {
		return &ActiveError{Op: "DeleteVisit", Err: ErrVisitNotFound}
	}

	if s.VisitRepo.Delete(ctx, id) != nil {
		return &ActiveError{Op: "DeleteVisit", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) ListVisits(ctx context.Context, options *base.QueryOptions) ([]*active.Visit, error) {
	visits, err := s.VisitRepo.List(ctx, options)
	if err != nil {
		return nil, &ActiveError{Op: "ListVisits", Err: ErrDatabaseOperation}
	}
	return visits, nil
}

func (s *service) FindVisitsByStudentID(ctx context.Context, studentID int64) ([]*active.Visit, error) {
	visits, err := s.VisitRepo.FindActiveByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ActiveError{Op: "FindVisitsByStudentID", Err: ErrDatabaseOperation}
	}
	return visits, nil
}

func (s *service) FindVisitsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.Visit, error) {
	visits, err := s.VisitRepo.FindByActiveGroupID(ctx, activeGroupID)
	if err != nil {
		return nil, &ActiveError{Op: "FindVisitsByActiveGroupID", Err: ErrDatabaseOperation}
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

	endedVisit, snapshot, err := s.endVisitWithAttendanceSync(ctx, id)
	if err != nil {
		if activeErr, ok := err.(*ActiveError); ok {
			return activeErr
		}
		return &ActiveError{Op: "EndVisit", Err: ErrDatabaseOperation}
	}

	s.broadcastVisitCheckout(ctx, endedVisit, snapshot)
	return nil
}

// endVisitWithAttendanceSync closes one visit and mirrors its exact persisted
// checkout timestamp into slot attendance. Session-end and timeout paths use
// this helper too. The one visit-ending path that bypasses it — the nightly
// bulk close in EndDailySessions — mirrors its checkouts via the scheduler's
// session-end bridge (CloseOpenCheckoutsByActiveGroupIDs).
// AttendanceSyncer owns graceful degradation: a missing bridge or sync failure
// returns a nil snapshot without undoing the successfully ended visit.
func (s *service) endVisitWithAttendanceSync(
	ctx context.Context, id int64,
) (*active.Visit, *AttendanceSnapshot, error) {
	endedVisit, err := s.endVisitRecord(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	var snapshot *AttendanceSnapshot
	if s.AttendanceSyncer != nil {
		snapshot = s.AttendanceSyncer.MirrorCheckOutForVisit(ctx, endedVisit)
	}
	return endedVisit, snapshot, nil
}

// endVisitRecord ends the visit record and returns the updated visit
func (s *service) endVisitRecord(ctx context.Context, id int64) (*active.Visit, error) {
	visit, err := s.VisitRepo.FindByID(ctx, id)
	if err != nil || visit == nil {
		return nil, &ActiveError{Op: "EndVisit", Err: ErrVisitNotFound}
	}
	if visit.ExitTime != nil {
		return nil, &ActiveError{Op: "EndVisit", Err: ErrVisitAlreadyEnded}
	}

	if err := s.VisitRepo.EndVisit(ctx, id); err != nil {
		latest, findErr := s.VisitRepo.FindByID(ctx, id)
		if findErr == nil && latest != nil && latest.ExitTime != nil {
			return nil, &ActiveError{Op: "EndVisit", Err: ErrVisitAlreadyEnded}
		}
		return nil, &ActiveError{Op: "EndVisit", Err: ErrDatabaseOperation}
	}

	visit, err = s.VisitRepo.FindByID(ctx, id)
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
	if s.Broadcaster == nil || endedVisit == nil {
		return
	}

	studentName, studentRec := s.getStudentDisplayData(ctx, endedVisit.StudentID)
	s.emitVisitCheckout(ctx, endedVisit, snapshot, studentName, studentRec)
}

// emitVisitCheckout is broadcastVisitCheckout with the student display data
// already resolved. Split out so the attendance checkout paths can read the
// student inside their request transaction and defer only the emission to a
// tenant.RegisterAfterCommit hook (#2113): by the time hooks run, the tx stored
// in ctx is closed, so a repository call from inside the hook would fail — and
// running it outside the tenant tx would be blocked by RLS.
func (s *service) emitVisitCheckout(
	ctx context.Context,
	endedVisit *active.Visit,
	snapshot *AttendanceSnapshot,
	studentName string,
	studentRec *userModels.Student,
) {
	if s.Broadcaster == nil || endedVisit == nil {
		return
	}

	activeGroupID := fmt.Sprintf("%d", endedVisit.ActiveGroupID)
	studentID := fmt.Sprintf("%d", endedVisit.StudentID)
	eduGroupIDs := eduGroupIDsOf(studentRec)

	data := realtime.EventData{
		StudentID:   &studentID,
		StudentName: &studentName,
	}
	if len(eduGroupIDs) > 0 {
		data.GroupIDs = &eduGroupIDs
	}
	applyAttendanceSnapshot(&data, snapshot)

	event := realtime.NewEvent(
		realtime.EventStudentCheckOut,
		activeGroupID,
		data,
	)

	s.broadcastWithLogging(ctx, activeGroupID, studentID, event, "student_checkout")
	s.broadcastToEducationalGroup(ctx, studentRec, event)

	// Notify every client of the tenant so dashboard counts refresh, scoped to
	// the affected educational group when known (#2057).
	s.broadcastDashboardCountsChanged(ctx, eduGroupIDs)
	s.broadcastActiveSupervisionChanged(ctx, activeGroupID, activeSupervisionReasonStudentMoved)
}

// broadcastVisitMoved publishes a checkout from the source active group and a
// checkin into the target active group. Attendance mutation happens before this
// helper so synchronization does not depend on a broadcaster being configured.
func (s *service) broadcastVisitMoved(
	ctx context.Context,
	previousVisit, movedVisit *active.Visit,
	sourceSnapshot, targetSnapshot *AttendanceSnapshot,
) {
	if s.Broadcaster == nil || previousVisit == nil || movedVisit == nil {
		return
	}

	s.broadcastVisitCheckout(ctx, previousVisit, sourceSnapshot)
	s.broadcastVisitCreated(ctx, movedVisit, targetSnapshot)
}

// broadcastToEducationalGroup mirrors active-group broadcasts to the student's OGS group topic
func (s *service) broadcastToEducationalGroup(ctx context.Context, student *userModels.Student, event realtime.Event) {
	if s.Broadcaster == nil || student == nil || student.GroupID == nil {
		return
	}
	groupID := fmt.Sprintf("edu:%d", *student.GroupID)
	if err := s.Broadcaster.BroadcastToGroup(tenant.FromContext(ctx), groupID, event); err != nil {
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

// broadcastStudentCheckoutEvents sends a batched checkout SSE notification when
// a whole session ends. All visits in the batch share one active_group_id (the
// session). Instead of one student_checkout per student — which produced 2N+
// events on a single client channel and overflowed its 10-slot buffer
// (issue #848) — this emits one bulk_student_checkout per affected topic:
// one to the active-group topic carrying every student, and one per distinct
// educational group carrying that group's students. SSE events are triggers,
// not payloads (the client refetches via bulk endpoints), so the per-student
// attendance snapshot is not carried in this bulk event; only the student IDs
// drive the client's per-student detail-cache invalidation. Slot-attendance
// checkout persistence still runs before this helper for every ended visit.
func (s *service) broadcastStudentCheckoutEvents(ctx context.Context, sessionIDStr string, visitsToNotify []visitSSEData) {
	if len(visitsToNotify) == 0 {
		return
	}

	// Collect every student ID for the active-group topic, and bucket them by
	// educational group for the per-edu-group topics. eduReps keeps one student
	// per group so broadcastToEducationalGroup can derive the edu:{id} topic.
	allStudentIDs := make([]string, 0, len(visitsToNotify))
	eduGroups := make(map[int64][]string)
	eduReps := make(map[int64]*userModels.Student)
	for _, visitData := range visitsToNotify {
		idStr := strconv.FormatInt(visitData.StudentID, 10)
		allStudentIDs = append(allStudentIDs, idStr)
		if visitData.Student != nil && visitData.Student.GroupID != nil {
			gid := *visitData.Student.GroupID
			eduGroups[gid] = append(eduGroups[gid], idStr)
			if eduReps[gid] == nil {
				eduReps[gid] = visitData.Student
			}
		}
	}

	// Every affected educational group id, for scoped client-side invalidation
	// (#2057). Students without an OGS group contribute no id; they appear in
	// no ogs-students-{gid} list, so the scope stays correct.
	allEduGroupIDs := make([]string, 0, len(eduGroups))
	for gid := range eduGroups {
		allEduGroupIDs = append(allEduGroupIDs, strconv.FormatInt(gid, 10))
	}

	// One event to the active-group topic carrying every checked-out student
	// and every affected educational group.
	activeData := realtime.EventData{StudentIDs: &allStudentIDs}
	if len(allEduGroupIDs) > 0 {
		activeData.GroupIDs = &allEduGroupIDs
	}
	activeEvent := realtime.NewEvent(
		realtime.EventBulkStudentCheckOut,
		sessionIDStr,
		activeData,
	)
	s.broadcastWithLogging(ctx, sessionIDStr, "", activeEvent, "bulk_student_checkout")

	// One event per distinct educational group, carrying only that group's
	// students so each subscribed client invalidates the right detail caches.
	for gid, ids := range eduGroups {
		groupIDs := ids
		eduGroupID := []string{strconv.FormatInt(gid, 10)}
		eduEvent := realtime.NewEvent(
			realtime.EventBulkStudentCheckOut,
			sessionIDStr,
			realtime.EventData{StudentIDs: &groupIDs, GroupIDs: &eduGroupID},
		)
		s.broadcastToEducationalGroup(ctx, eduReps[gid], eduEvent)
	}

	// Single tenant-wide broadcast for the entire batch, scoped to the
	// affected educational groups (#2057).
	if s.Broadcaster != nil {
		s.broadcastDashboardCountsChanged(ctx, allEduGroupIDs)
		s.broadcastActiveSupervisionChanged(ctx, sessionIDStr, activeSupervisionReasonStudentMoved)
	}
}

// broadcastActivityEndEvent sends the activity_end SSE event for a completed session.
// This helper reduces cognitive complexity in session timeout processing.
func (s *service) broadcastActivityEndEvent(ctx context.Context, sessionID int64, sessionIDStr string) {
	finalGroup, err := s.GroupRepo.FindByID(ctx, sessionID)
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

	// Notify every client of the tenant (including zero-topic) so dashboards
	// refresh. No group scope: a session end affects room occupancy across
	// groups, so clients fall back to a broad refresh (#2057).
	s.broadcastDashboardCountsChanged(ctx, nil)
	s.broadcastActiveSupervisionChanged(ctx, sessionIDStr, activeSupervisionReasonActivityEnded)
}

// broadcastWithLogging broadcasts an event and logs any errors.
func (s *service) broadcastWithLogging(ctx context.Context, activeGroupID, studentID string, event realtime.Event, eventType string) {
	if err := s.Broadcaster.BroadcastToGroup(tenant.FromContext(ctx), activeGroupID, event); err != nil {
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
	activity, err := s.ActivityGroupRepo.FindByID(ctx, *groupID)
	if err != nil || activity == nil {
		return ""
	}
	return activity.Name
}

// getRoomName retrieves the room name by room ID, returning empty string on error.
func (s *service) getRoomName(ctx context.Context, roomID int64) string {
	room, err := s.RoomRepo.FindByID(ctx, roomID)
	if err != nil || room == nil {
		return ""
	}
	return room.Name
}

func (s *service) GetStudentCurrentVisit(ctx context.Context, studentID int64) (*active.Visit, error) {
	visit, err := s.VisitRepo.GetCurrentByStudentID(ctx, studentID)
	if err != nil {
		if base.IsNoRows(err) {
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
	visit, err := s.VisitRepo.GetCurrentByStudentIDWithRoom(ctx, studentID)
	if err != nil {
		if base.IsNoRows(err) {
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

	visits, err := s.VisitRepo.GetCurrentByStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, &ActiveError{Op: "GetStudentsCurrentVisits", Err: ErrDatabaseOperation}
	}

	if visits == nil {
		visits = make(map[int64]*active.Visit)
	}

	return visits, nil
}

func (s *service) CountActiveVisitsByRoomID(ctx context.Context, roomID int64) (int, error) {
	count, err := s.VisitRepo.CountActiveByRoomID(ctx, roomID)
	if err != nil {
		return 0, &ActiveError{Op: "CountActiveVisitsByRoomID", Err: ErrDatabaseOperation}
	}
	return count, nil
}

func (s *service) CountActiveVisitsByActiveGroupID(ctx context.Context, activeGroupID int64) (int, error) {
	count, err := s.VisitRepo.CountActiveByGroupID(ctx, activeGroupID)
	if err != nil {
		return 0, &ActiveError{Op: "CountActiveVisitsByActiveGroupID", Err: ErrDatabaseOperation}
	}
	return count, nil
}

// ListStudentsPresentInRoom returns the IDs of students currently checked-in
// to any active group in the given room. The student list handler feeds
// these IDs through the standard ListWithOptions pipeline, which applies
// GDPR redaction and pagination.
func (s *service) ListStudentsPresentInRoom(ctx context.Context, roomID int64) ([]int64, error) {
	ids, err := s.VisitRepo.ListActiveStudentIDsByRoomID(ctx, roomID)
	if err != nil {
		return nil, &ActiveError{Op: "ListStudentsPresentInRoom", Err: fmt.Errorf("list active student IDs: %w", err)}
	}
	return ids, nil
}

// Group Supervisor operations
func (s *service) GetGroupSupervisor(ctx context.Context, id int64) (*active.GroupSupervisor, error) {
	supervisor, err := s.SupervisorRepo.FindByID(ctx, id)
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
	supervisors, err := s.SupervisorRepo.FindByActiveGroupID(ctx, supervisor.GroupID, true)
	if err != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: ErrDatabaseOperation}
	}

	for _, s := range supervisors {
		if s.StaffID == supervisor.StaffID {
			return &ActiveError{Op: "CreateGroupSupervisor", Err: ErrStaffAlreadySupervising}
		}
	}

	supervisor.SetTenantID(tenant.FromContext(ctx))
	if s.SupervisorRepo.Create(ctx, supervisor) != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: ErrDatabaseOperation}
	}

	// Taking over a supervision that starts today means the staff member is
	// working right now — auto-open their work session so they show as
	// "Anwesend" (issue #1439). Kiosk-driven session starts already do this
	// in assignMultipleSupervisorsNonCritical; this covers the web app path.
	if supervisor.StartDate == timezone.TodayDate() {
		source := active.WorkSessionSourceApp
		if device.IsIoTDeviceRequest(ctx) {
			source = active.WorkSessionSourceNFC
		}
		s.ensureStaffPresence(ctx, supervisor.StaffID, source)
	}

	return nil
}

func (s *service) UpdateGroupSupervisor(ctx context.Context, supervisor *active.GroupSupervisor) error {
	if supervisor == nil || supervisor.Validate() != nil {
		return &ActiveError{Op: "UpdateGroupSupervisor", Err: ErrInvalidData}
	}

	if s.SupervisorRepo.Update(ctx, supervisor) != nil {
		return &ActiveError{Op: "UpdateGroupSupervisor", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) DeleteGroupSupervisor(ctx context.Context, id int64) error {
	_, err := s.SupervisorRepo.FindByID(ctx, id)
	if err != nil {
		return &ActiveError{Op: "DeleteGroupSupervisor", Err: ErrGroupSupervisorNotFound}
	}

	if s.SupervisorRepo.Delete(ctx, id) != nil {
		return &ActiveError{Op: "DeleteGroupSupervisor", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) ListGroupSupervisors(ctx context.Context, options *base.QueryOptions) ([]*active.GroupSupervisor, error) {
	supervisors, err := s.SupervisorRepo.List(ctx, options)
	if err != nil {
		return nil, &ActiveError{Op: "ListGroupSupervisors", Err: ErrDatabaseOperation}
	}
	return supervisors, nil
}

func (s *service) FindSupervisorsByStaffID(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error) {
	supervisors, err := s.SupervisorRepo.FindActiveByStaffID(ctx, staffID)
	if err != nil {
		return nil, &ActiveError{Op: "FindSupervisorsByStaffID", Err: ErrDatabaseOperation}
	}
	return supervisors, nil
}

func (s *service) FindSupervisorsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.GroupSupervisor, error) {
	supervisors, err := s.SupervisorRepo.FindByActiveGroupID(ctx, activeGroupID, true)
	if err != nil {
		return nil, &ActiveError{Op: "FindSupervisorsByActiveGroupID", Err: ErrDatabaseOperation}
	}
	return supervisors, nil
}

func (s *service) FindSupervisorsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64) ([]*active.GroupSupervisor, error) {
	supervisors, err := s.SupervisorRepo.FindByActiveGroupIDs(ctx, activeGroupIDs, true)
	if err != nil {
		return nil, &ActiveError{Op: "FindSupervisorsByActiveGroupIDs", Err: ErrDatabaseOperation}
	}
	return supervisors, nil
}

func (s *service) EndSupervision(ctx context.Context, id int64) error {
	// Verify supervision exists first
	_, err := s.SupervisorRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &ActiveError{Op: "EndSupervision", Err: ErrGroupSupervisorNotFound}
		}
		return &ActiveError{Op: "EndSupervision", Err: fmt.Errorf("failed to verify supervision: %w", err)}
	}

	if err := s.SupervisorRepo.EndSupervision(ctx, id); err != nil {
		return &ActiveError{Op: "EndSupervision", Err: fmt.Errorf("end supervision failed: %w", err)}
	}
	return nil
}

func (s *service) GetStaffActiveSupervisions(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error) {
	supervisors, err := s.SupervisorRepo.FindActiveByStaffID(ctx, staffID)
	if err != nil {
		return nil, &ActiveError{Op: "GetStaffActiveSupervisions", Err: ErrDatabaseOperation}
	}

	// Filter only active supervisions
	now := time.Now()
	var activeSupervisions []*active.GroupSupervisor
	for _, supervisor := range supervisors {
		if IsSupervisorActive(supervisor, now) {
			activeSupervisions = append(activeSupervisions, supervisor)
		}
	}

	return activeSupervisions, nil
}

// GetCrossTenantStudents returns students visiting from other tenants.
func (s *service) GetCrossTenantStudents(ctx context.Context, hostingTenantID int64) ([]active.CrossTenantStudent, error) {
	if s.CrossTenantRepo == nil {
		return []active.CrossTenantStudent{}, nil
	}

	students, err := s.CrossTenantRepo.FindCrossTenantStudents(ctx, hostingTenantID)
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

	visitNames, err := s.VisitRepo.GetTodayVisitNamesForStudents(ctx, studentIDs)
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

// HasOpenAttendanceOn reports whether any attendance row on the given
// calendar date is still open (check_out_time IS NULL).
func (s *service) HasOpenAttendanceOn(ctx context.Context, date timezone.Date) (bool, error) {
	return s.AttendanceRepo.HasOpenAttendanceOn(ctx, date)
}

// GetRoomsByIDs retrieves rooms by ID.
func (s *service) GetRoomsByIDs(ctx context.Context, ids []int64) ([]*facilityModels.Room, error) {
	return s.RoomRepo.FindByIDs(ctx, ids)
}

// GetActiveGroupVisitsWithDisplay returns the open visits of an active group
// joined with student display data.
func (s *service) GetActiveGroupVisitsWithDisplay(ctx context.Context, activeGroupID int64) ([]*active.VisitWithStudentDisplay, error) {
	return s.VisitRepo.FindActiveWithStudentDisplayByGroup(ctx, activeGroupID)
}
