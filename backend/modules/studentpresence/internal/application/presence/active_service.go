package presence

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const (
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

type SchoolQuery interface {
	ListSchoolsByID(context.Context, []int64) ([]School, error)
}

type School struct {
	ID   int64
	Slug string
}

// SettingsResolver resolves tenant-scoped settings. Implemented by config.SettingsService.
// Optional dependency — when nil, auto-clear behavior falls back to the registry default.
type SettingsResolver interface {
	// PresenceMode is the tenant's attendance granularity, one of
	// PresenceModeDetailed or PresenceModeBinary.
	PresenceMode(ctx context.Context) (string, error)
	// SickClearMode and ExcusedClearMode say when a reported sick or excused
	// flag is cleared; ClearModeNextCheckin is the value the check-in acts on.
	SickClearMode(ctx context.Context) (string, error)
	ExcusedClearMode(ctx context.Context) (string, error)
	// SessionInactivityTimeoutMinutes is how long a kiosk session may idle.
	SessionInactivityTimeoutMinutes(ctx context.Context) (int, error)
	// AttendanceEditScope and OperationalOverviewScope gate who may edit
	// attendance and who sees the tenant-wide overview.
	AttendanceEditScope(ctx context.Context) (string, error)
	OperationalOverviewScope(ctx context.Context) (string, error)
}

// Settings values the presence flows compare against. They are the stored
// column values of the settings registry, so they are named here the same way
// the wire vocabulary is.
const (
	ClearModeNextCheckin        = "next_checkin"
	AttendanceEditScopeOwn      = "own"
	AttendanceEditScopeAllStaff = "all_staff"
	OverviewScopeAllStaff       = "all_staff"
)

// TimetableBridgeCompleter marks timetable instances that are still bridged
// to active groups as completed. Implemented by schedule.ActivityInstanceRepository.
type TimetableBridgeCompleter interface {
	CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error)
}

// ServiceDependencies contains all dependencies required by the active service
type ServiceDependencies struct {
	PrincipalReader func(context.Context) RequestPrincipal
	// Active domain repositories
	GroupRepo        active.GroupRepository
	SessionStartLock interface {
		LockSessionStart(context.Context, int64) error
	}
	SupervisorRepo    active.GroupSupervisorRepository
	SchoolPresence    StudentPresence
	StudentDisplay    StudentDisplayReader
	StudentStatusRepo StudentStatusDayRepository

	// Cross-tenant query repository (optional - nil-safe)
	CrossTenantRepo CrossTenantRepo
	Schools         SchoolQuery

	// User domain repositories
	StudentRepo PresenceStudents
	StaffRepo   AttendanceStaff

	// Supporting domain repositories
	RoomRepo           AttendanceRooms
	YardRoomColor      func(context.Context) (*string, error)
	ActivityGroupRepo  AttendanceActivityGroups
	ActivityCatRepo    AttendanceActivityCategories
	EducationGroupRepo AttendanceEducationGroups
	DeviceRepo         SessionDeviceDirectory

	// External services
	StaffNames AttendanceStaffNames

	// Infrastructure
	DB          DatabaseHandle
	Broadcaster EventPublisher // SSE event broadcaster (optional - can be nil for testing)

	// Optional: Product analytics tracker (nil-safe, no student PII)
	Tracker productEventTracker

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
	Now    func() time.Time
}

// Service implements the Active Service interface
type service struct {
	ServiceDependencies
	tenantRuntime *tenant.UnitOfWork

	// Optional: Tenant-scoped settings resolver for auto-clear logic.
	// When nil, auto-clear falls back to the registry default behavior.
	// Injected post-construction via SetSettingsService.
	settings SettingsResolver

	// Optional: weckt die Sorgeberechtigten eines Kindes nach einer
	// Anwesenheitsaenderung, damit die Eltern-App ihren Tagesstatus (#2252)
	// live nachlaedt. Injiziert via SetGuardianWaker; nil ist ein No-op.
	guardianWaker GuardianWaker
}

func (s *service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *service) todayDate() timezone.Date {
	return timezone.DateFromTime(s.now())
}

// GetPresenceMode resolves the tenant value or registry default and propagates failures.
func (s *service) GetPresenceMode(ctx context.Context) (string, error) {
	if s.settings == nil {
		return "", errors.New("resolve presence mode: settings service is not configured")
	}
	mode, err := s.settings.PresenceMode(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve presence mode: %w", err)
	}
	if mode != PresenceModeDetailed && mode != PresenceModeBinary {
		return "", fmt.Errorf("resolve presence mode: invalid value %q", mode)
	}
	return mode, nil
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (s *service) getLogger() *slog.Logger {
	return loggerOrDefault(s.Logger)
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

// checkin_type values mirroring the checkout_type set above. They ride the
// student_checkin SSE event as its `source`; the checkout constants are kept
// separate because their values are pinned to the checkout_type analytics
// property.
const (
	checkinTypeToggle = "toggle" // kiosk toggle-in (ToggleStudentAttendance)
	checkinTypeWeb    = "web"    // web/staff-UI check-in (CheckInStudent)
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
func (s *service) attendanceMethod(ctx context.Context) string {
	if s.attendancePrincipal(ctx).IsIoT {
		return "rfid"
	}
	return "manual"
}

// NewService creates a new active service instance
func NewService(deps ServiceDependencies, options ...ServiceOption) Service {
	svc := &service{ServiceDependencies: deps}
	for _, option := range options {
		option(svc)
	}
	return svc
}
