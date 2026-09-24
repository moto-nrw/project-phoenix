package care

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	careplan "github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	notificationsSvc "github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Config is the dependency bundle of the guardian portal's child flows. The
// unit of work comes from the request context; the package holds no database.
type Config struct {
	Logger *slog.Logger
	Now    func() time.Time

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
	Settings              configService.SettingsService

	// Attendance is the only presence source for parents. Visits and room
	// locations are deliberately excluded from this consumer contract.
	Attendance AttendanceReader

	// Care Plan: the weekly plan, its requests, the absence requests and the
	// day exceptions. CareExceptions shares the caller's tenant transaction;
	// writes follow the student/day lock.
	ArrivalSchedules careplan.ArrivalScheduleService
	PickupSchedules  careplan.PickupScheduleService
	CareRequests     carerequests.Service
	ExcusedRequests  careplan.ExcusedAbsenceRequests
	CareExceptions   CareExceptions
	// PickupAutoExcusal couples pulled-forward day pickup times with the
	// per-block partial-absence mechanics (#2360). Optional in tests; nil
	// skips the coupling.
	PickupAutoExcusal careplan.PickupAutoExcusal

	// Enrollment: the booked care offerings behind the child and the
	// post-enrollment change-request lifecycle.
	CarePeriods      enrollmentSvc.StudentCarePeriodReader
	OfferingHistory  enrollmentSvc.OfferingHistoryReader
	CareOfferingRepo CareOfferingReads
	OfferingChanges  OfferingChangeRequests

	// People Directory: the child's change history, consent projection and
	// photo lifecycle, and the parent-request ledger.
	StudentAudit        usersSvc.StudentChangeRecorder
	StudentConsents     StudentConsentService
	ParentRequestEvents usersSvc.ParentRequestEventRecorder
	// StudentPhotos resolves the photo lifecycle when a withdrawal needs it.
	// The API bootstrap builds that service after this one, so the
	// composition passes a resolver instead of setting it afterwards.
	StudentPhotos func() StudentPhotoUnlinker

	// Owner commands. Care Plan records absences, the guardian leg of a
	// day's pickup exception and the Stammdaten requests; People Directory
	// writes the guardian and student rows; Audit Platform appends the
	// guardian change trail. All of them join the flow's tenant unit of work.
	GuardianAbsences careplan.GuardianAbsenceReports
	GuardianPickups  careplan.GuardianPickupExceptions
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
	AbsenceNotifier notificationsSvc.AbsenceNotifier
	Emitter         *parentmessaging.Emitter
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

// New wires the child flows. RequestSharing is required.
func New(cfg Config) *Service {
	if cfg.RequestSharing == nil {
		panic("care: request sharing is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = timezone.Now
	}
	return &Service{Config: cfg}
}

func (s *Service) now() time.Time {
	if s.Now == nil {
		return timezone.Now()
	}
	return s.Now()
}

func (s *Service) todayDate() timezone.Date {
	return timezone.DateFromTime(s.now())
}

// InTenant runs fn in the tenant unit of work of the child's school. Nested
// calls join the ambient transaction; after-commit work runs once, after the
// outermost commit.
func InTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	id, err := tenant.NewTenantID(tenantID)
	if err != nil {
		return err
	}
	return tenant.WithinTenant(ctx, id, fn)
}
