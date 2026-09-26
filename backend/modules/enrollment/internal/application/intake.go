package application

import (
	"context"
	"log/slog"
	"strings"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The parent-facing intake (#3565): submission and edits, the status link,
// the public form loads and the late invites. Every dependency outside the
// owner's own records arrives through the ports below.

// Names of the tenant settings the intake resolves, as its error texts name
// them.
const (
	settingCollectGradeLevel    = "enrollment.collect_grade_level"
	settingCollectSchoolClass   = "enrollment.collect_school_class"
	settingCareOfferingsEnabled = "enrollment.care_offerings_enabled"
	settingGradeLevelMax        = "enrollment.grade_level_max"
)

// Values of enrollment.duplicate_handling.
const (
	duplicateHandlingBlock  = "block"
	duplicateHandlingWarn   = "warn"
	duplicateHandlingIgnore = "ignore"
)

// Values of enrollment.legal_agb_display_mode.
const (
	legalAGBDisplayModeText = "text"
	legalAGBDisplayModePDF  = "pdf"
)

// guardianPermissionEnrollmentSubmit is the parent-portal permission a
// guardian needs on a student to renew its enrollment (#1663).
const guardianPermissionEnrollmentSubmit = "parent_portal.enrollment.submit"

// Enrollment owner ports of the intake and its edits.
type (
	// IntakeRequests reads and writes the submitted requests.
	IntakeRequests interface {
		InsertRequest(context.Context, *enrollment.Request) error
		RequestByID(context.Context, int64, bool) (*enrollment.Request, error)
		RequestByToken(context.Context, string, bool) (*enrollment.Request, error)
		RequestsByID(context.Context, []int64) ([]*enrollment.Request, error)
		UpdateRequestGuardian(context.Context, *enrollment.Request, bool) error
		SetRequestWithdrawal(context.Context, int64, *time.Time) error
		AcquireSubmissionDedupLock(context.Context, int64, uint64) error
		AcquireExistingStudentMatchLock(context.Context, int64) error
		ActiveDuplicateChildren(context.Context, int64, string, []enrollment.DuplicateChildKey, int64) ([]enrollment.DuplicateChildKey, error)
		HasActiveRequestForMatchedStudent(context.Context, int64, int64, int64) (bool, error)
		PinDecisionNotificationMode(context.Context, int64, string) (string, error)
	}
	// IntakeChildren reads and writes the children of the requests with
	// their submitted offering choices.
	IntakeChildren interface {
		enrollment.SubmittedOfferingCommands
		RequestChildOfferingHistoryForChildren(context.Context, []int64) ([]*enrollment.RequestChildOffering, error)
		RequestChildOfferingsForChildrenAtDate(context.Context, []int64, enrollment.Date) ([]*enrollment.RequestChildOffering, error)
		OfferingCapacityPeak(context.Context, int64, []int64, enrollment.Date, enrollment.Date) (int, error)
		InsertChild(context.Context, *enrollment.RequestChild) error
		ChildrenForRequest(context.Context, int64, bool) ([]*enrollment.RequestChild, error)
		ChildrenForRequests(context.Context, []int64) ([]*enrollment.RequestChild, error)
		DeleteRequestChildren(context.Context, int64) error
		UpdateChildStatus(context.Context, int64, string, *string, int64) error
		UpdateChildData(context.Context, *enrollment.RequestChild) error
		UpdateMatchedStudent(context.Context, int64, *int64) error
	}
	// IntakeGuardians keeps the co-guardians of the requests.
	IntakeGuardians interface {
		CreateRequestGuardian(context.Context, *enrollment.RequestGuardian) error
		RequestGuardians(context.Context, []int64) ([]*enrollment.RequestGuardian, error)
		DeleteRequestGuardians(context.Context, int64) error
	}
	// IntakeLateInvites issues, resolves and consumes late invites.
	IntakeLateInvites interface {
		InsertLateInvite(context.Context, *enrollment.LateInvite) error
		UsableLateInvite(context.Context, string, int64, time.Time, bool) (*enrollment.LateInvite, error)
		MarkLateInviteUsed(context.Context, int64, int64, time.Time) error
		LateInviteByUsedRequestID(context.Context, int64) (*enrollment.LateInvite, error)
	}
	// IntakeCatalog loads the phase and the immutable schema a submission
	// works with.
	IntakeCatalog interface {
		Phase(context.Context, int64) (*enrollment.Phase, error)
		Schema(context.Context, int64) (*enrollment.FormSchema, error)
	}
	// SubmissionRateLimiter counts submission attempts per IP and email.
	SubmissionRateLimiter interface {
		IncrementAttempts(context.Context, int64, string, string, time.Duration) (*enrollment.SubmissionRateLimitState, error)
	}
)

// IntakeOfferings reads Care Plan's offering catalog in Enrollment's
// offering rows.
type IntakeOfferings interface {
	ListActiveByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error)
	ListByIDs(ctx context.Context, ids []int64) ([]*enrollmentModels.CareOffering, error)
}

// StudentMatches is the People Directory's name-and-birthday match of
// enrolled (active or pending) students. FindEnrolledStudentIDByNameAndBirthday
// answers nil for no match and for more than one.
type StudentMatches interface {
	ExistsEnrolledByNameAndBirthday(ctx context.Context, tenantID int64, firstName, lastName string, dateOfBirth calendar.Date) (bool, error)
	FindEnrolledStudentIDByNameAndBirthday(ctx context.Context, tenantID int64, firstName, lastName string, dateOfBirth calendar.Date) (*int64, error)
}

// GuardianStudentAuthorizer probes a guardian's parent-portal permission on
// one student, by the authenticated account or by the email a late invite
// was issued to.
type GuardianStudentAuthorizer interface {
	AccountHasStudentPermission(ctx context.Context, accountID, studentID, tenantID int64, permission string) (bool, error)
	GuardianEmailHasStudentPermission(ctx context.Context, email string, studentID, tenantID int64, permission string) (bool, error)
}

// IntakeSettings resolves the tenant settings of the intake. The *Default
// reads fall back to the registry default on any failure, the others report
// it.
type IntakeSettings interface {
	// EnrollmentEnabled defaults to false.
	EnrollmentEnabled(ctx context.Context) bool
	// AllowSubmissionEdit defaults to true.
	AllowSubmissionEdit(ctx context.Context) bool
	// StatusTokenTTLDays defaults to 365.
	StatusTokenTTLDays(ctx context.Context) int
	// AdminNotificationEmails is the comma-separated recipient list, empty
	// when unset.
	AdminNotificationEmails(ctx context.Context) string
	DuplicateHandling(ctx context.Context) (string, error)
	CollectGradeLevel(ctx context.Context) (bool, error)
	CollectSchoolClass(ctx context.Context) (bool, error)
	CareOfferingsEnabled(ctx context.Context) (bool, error)
	GradeLevelMax(ctx context.Context) (int, error)
	// LegalSettings resolves the tenant-wide legal texts and toggles. A
	// toggle without a tenant override is off.
	LegalSettings(ctx context.Context) (LegalSettings, error)
	// ChangeRequestMailsEnabled is false on any failure.
	ChangeRequestMailsEnabled(ctx context.Context) bool
}

// LegalSettings are the tenant-wide legal texts and their toggles as stored.
type LegalSettings struct {
	AGB                 string
	AGBDocumentURL      string
	AGBDisplayMode      string
	DSGVO               string
	EmailContact        string
	Photo               string
	TermsEnabled        bool
	DSGVOEnabled        bool
	EmailContactEnabled bool
	PhotoEnabled        bool
}

// ManualEnrollmentDecider approves the freshly submitted request of a manual
// enrollment in the same transaction.
type ManualEnrollmentDecider interface {
	Decide(ctx context.Context, input enrollment.DecideInput) (*enrollment.DecideOutcome, error)
}

// IntakeDependencies bind the intake to its owners. Guardians, LateInvites,
// RateLimits, Students, GuardianAuthorizer and Outbox are optional where the
// intake degrades without them; the intake reports every other missing
// dependency when it needs it.
type IntakeDependencies struct {
	Requests           IntakeRequests
	Children           IntakeChildren
	Guardians          IntakeGuardians
	LateInvites        IntakeLateInvites
	Catalog            IntakeCatalog
	RateLimits         SubmissionRateLimiter
	Offerings          IntakeOfferings
	Bookings           enrollment.CareBookingCommands
	Capacity           enrollment.OfferingCapacity
	Schools            enrollment.SchoolDirectory
	Notifications      enrollment.Notifications
	Students           StudentMatches
	GuardianAuthorizer GuardianStudentAuthorizer
	Outbox             MailOutbox
	Settings           IntakeSettings
	ManualDecider      ManualEnrollmentDecider
	// Random fills the status and late-invite tokens; Fingerprint hashes a
	// late-invite token into its stored identity.
	Random      func([]byte) error
	Fingerprint Fingerprint
	// FrontendURL is the base of staff links, ParentsURL the base of the
	// parent-facing links; it falls back to FrontendURL.
	FrontendURL string
	ParentsURL  string
	Runtime     Runtime
	Logger      *slog.Logger
}

// Intake is Enrollment's parent-facing intake. It implements the public
// IntakeSubmissions, IntakeStatus and IntakeForms capabilities.
type Intake struct {
	deps IntakeDependencies
}

var (
	_ enrollment.IntakeSubmissions = (*Intake)(nil)
	_ enrollment.IntakeStatus      = (*Intake)(nil)
	_ enrollment.IntakeForms       = (*Intake)(nil)
)

// NewIntake composes the intake. A nil logger falls back to slog.Default().
func NewIntake(deps IntakeDependencies) *Intake {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	deps.FrontendURL = strings.TrimRight(strings.TrimSpace(deps.FrontendURL), "/")
	deps.ParentsURL = strings.TrimRight(strings.TrimSpace(deps.ParentsURL), "/")
	if deps.ParentsURL == "" {
		deps.ParentsURL = deps.FrontendURL
	}
	return &Intake{deps: deps}
}

func (s *Intake) logger() *slog.Logger { return s.deps.Logger }

func (s *Intake) intakePhase(ctx context.Context, id int64) (*enrollment.Phase, error) {
	return s.deps.Catalog.Phase(ctx, id)
}

func (s *Intake) intakeSchema(ctx context.Context, id int64) (*enrollment.FormSchema, error) {
	value, err := s.deps.Catalog.Schema(ctx, id)
	return enrollment.CopyFormSchema(value), err
}
