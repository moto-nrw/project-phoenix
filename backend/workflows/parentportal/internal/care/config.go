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
	// ChildDataAccess records the reads of the holiday registration prefill
	// (#4662) in the school's data access log.
	ChildDataAccess ChildDataAccessLog
	// HolidayWishes states the holiday care wish of a prefilled child
	// (#4703) in the school's tenant transaction.
	HolidayWishes HolidayWishStatements
	// HolidayAnnouncements lists the published holiday periods the start
	// page announces (#6671), in the administrative transaction.
	HolidayAnnouncements HolidayRegistrationAnnouncements

	// Document inbox (#4671, ADR 0126): the documents of the Elternabrechnung
	// in the Träger transaction, the Träger of a school, the address of an
	// account, and the push that a document arrived.
	BillingDocuments BillingDocuments
	// ContractDocuments are the sent care contracts of the Träger in the
	// same inbox (#4678, ADR 0153).
	ContractDocuments ContractDocuments
	// ContractNotices withdraw and terminate those contracts (#4682).
	ContractNotices ContractNotices
	// SharedDocuments are the documents a Träger released for parents in
	// the same inbox (#4692).
	SharedDocuments TraegerSharedDocuments
	// SEPAMandates are the mandates of the families on those contracts
	// (#4696).
	SEPAMandates     SEPAMandates
	TraegerRuntime   TraegerRuntime
	SchoolTraeger    SchoolTraeger
	ParentAccounts   ParentAccounts
	DocumentNotifier BillingDocumentNotifier
	// DocumentPushPorts serve the pushes of a sent contract and of a
	// release for parents (#5941).
	DocumentPushPorts

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
	// PickupSignatures stores the signatures of pickup permissions, collectors
	// and the way home alone (#4780, Audit Platform).
	PickupSignatures PickupSignatureLedger

	// Identity & Access: the guardian's school membership a child write holds
	// until commit (#6529), inviting and removing further guardians, and the
	// open invitations shown as pending.
	Memberships         SchoolMembershipLock
	InvitationAccounts  InvitationAccountLock
	GuardianInvites     GuardianAccess
	GuardianInvitations GuardianInvitationReads
	InvalidateInvites   func(context.Context, int64) error

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
	EmitSelfServicePill(ctx context.Context, tenantID, studentID, accountID int64, eventType, body, refTable string, refID *int64) error
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
// WithinAdminThenTenant is the one-way handoff of ADR 0161: one transaction,
// discovery as admin, then the discovered school's tenant scope.
// AmbientScope reports whether a unit of any scope is open in ctx and the
// school ctx is bound to (0 for none). A bundled read decides from it, before
// it starts the bundled unit, whether to take the unbundled way (ADR 0161
// point 7); it never decides from a start error.
type Runtime interface {
	WithinTenant(context.Context, int64, func(context.Context) error) error
	WithinAdmin(context.Context, func(context.Context) error) error
	WithinAdminThenTenant(ctx context.Context, discover func(context.Context) (int64, error), fn func(context.Context) error) error
	AmbientScope(ctx context.Context) (unitOpen bool, boundSchool int64)
	RegisterAfterCommit(context.Context, func())
	WithTenantID(context.Context, int64) context.Context
}
