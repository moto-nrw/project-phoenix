package care

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// StudentChangeRecorder appends the per-child change history for a field the
// flow changed, attributed to the authenticated actor (People Directory, bound
// by the composition root).
type StudentChangeRecorder interface {
	RecordChangesForActor(ctx context.Context, before, after *PortalStudent, editedBy int64) error
}

// Config is the dependency bundle of the guardian portal's child flows. The
// unit of work comes from the request context; the package holds no database.
type Config struct {
	CarePlanPolicy  CarePlanPolicy
	GuardianPolicy  GuardianPolicy
	DeparturePolicy DeparturePolicy
	Runtime         Runtime
	Logger          *slog.Logger
	Now             func() time.Time

	// Reads of the child, its school and its guardians.
	ChildRepo             ChildReads
	EnrollablePhaseRepo   EnrollablePhaseReads
	EnrollmentSettings    EnrollmentSettings
	EnrollmentRequestRepo EnrollmentRequestReads
	StudentRepo           StudentReads
	PersonRepo            PersonReads
	GuardianProfileRepo   GuardianProfileReads
	GuardianPhoneRepo     GuardianPhoneReads
	StudentGuardianRepo   StudentGuardianReads
	ChangeRequestRepo     DataRequestReads
	StatusDayRepo         StatusDayReads
	Settings              SettingsService

	// Attendance is the only presence source for parents. Visits and room
	// locations are deliberately excluded from this consumer contract.
	Attendance AttendanceReader

	// Care Plan: the weekly plan, its requests, the absence requests and the
	// day exceptions. CareExceptions shares the caller's tenant transaction;
	// writes follow the student/day lock.
	ArrivalSchedules ArrivalSchedules
	PickupSchedules  PickupSchedules
	CareRequests     carerequests.Service
	ExcusedRequests  ExcusedAbsenceRequests
	CareExceptions   CareExceptions

	// Enrollment: the booked care offerings behind the child and the
	// post-enrollment change-request lifecycle.
	CarePeriods      CarePeriodReads
	OfferingHistory  OfferingHistoryReads
	CareOfferingRepo CareOfferingReads
	OfferingChanges  OfferingChangeRequests

	// People Directory: the child's change history, consent projection and
	// photo lifecycle, and the parent-request ledger.
	StudentAudit        StudentChangeRecorder
	StudentConsents     StudentConsentService
	ParentRequestEvents RequestEventWriter
	// StudentPhotos unlinks the stored photo a withdrawal leaves behind.
	StudentPhotos StudentPhotoUnlinker

	// Owner commands. Care Plan records absences, the guardian leg of a
	// day's pickup exception and the Stammdaten requests; People Directory
	// writes the guardian and student rows; Audit Platform appends the
	// guardian change trail. All of them join the flow's tenant unit of work.
	GuardianAbsences GuardianAbsenceReports
	GuardianPickups  GuardianPickupExceptions
	DataRequests     StudentDataRequestCommands
	Guardians        GuardianRecords
	Students         StudentRecords
	GuardianChanges  GuardianChangeLog

	// Identity & Access: inviting and removing further guardians, and the
	// open invitations shown as pending.
	GuardianInvites     GuardianAccess
	GuardianInvitations GuardianInvitationReads

	// Meal Plan of the child's school, and the portal languages.
	MealPlan MealPlan
	Locales  Locales

	// Best-effort notifiers, called after the commit only.
	AbsenceNotifier AbsenceNotifier
	Emitter         ChildEvents
	StudentUpdates  StudentUpdates
	SelfService     SelfServiceEvents

	// RequestSharing is the parent request-sharing ledger. Required.
	RequestSharing RequestSharer
}

// EnrollmentSettings reports which schools have enrollment switched on.
type EnrollmentSettings interface {
	EnrollmentEnabledForTenants(ctx context.Context, tenantIDs []int64) (map[int64]bool, error)
}

// SelfServiceEvents is the port to the parent-OGS conversation: the chat pill
// a self-service action posts and the wake-up of every guardian's open tab.
type SelfServiceEvents interface {
	EmitSelfServicePill(tenantID, studentID, accountID int64, eventType, body, refTable string, refID *int64)
	WakeChildGuardians(tenantID, studentID int64)
}

// StudentUpdates wakes the school's live views after a parent-side change of
// a child. source names who changed it; the event carries no payload.
type StudentUpdates interface {
	StudentUpdated(tenantID int64, source string) error
}

// Locales is the port to the portal's language catalog.
type Locales interface {
	IsSupported(locale string) bool
	Normalize(locale string) string
	Default() string
}

// Service implements the guardian portal's child flows.
type Service struct {
	Config
}

// New wires the child flows. Runtime, GuardianPolicy and RequestSharing are required.
func New(cfg Config) *Service {
	if cfg.Runtime == nil {
		panic("care: runtime is required")
	}
	if cfg.RequestSharing == nil {
		panic("care: request sharing is required")
	}
	if cfg.GuardianPolicy == nil {
		panic("care: guardian policy is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = calendar.Now
	}
	return &Service{Config: cfg}
}

func (s *Service) now() time.Time {
	if s.Now == nil {
		return calendar.Now()
	}
	return s.Now()
}

func (s *Service) todayDate() calendar.Date {
	return calendar.DateFromTime(s.now())
}

// Runtime is the workflow-owned transaction and after-commit boundary.
// Implementations retain the caller's context and the platform's nested
// transaction, tenant validation and callback lifecycle semantics.
type Runtime interface {
	WithinTenant(context.Context, int64, func(context.Context) error) error
	WithinAdmin(context.Context, func(context.Context) error) error
	RegisterAfterCommit(context.Context, func())
	WithTenantID(context.Context, int64) context.Context
}
