package application

import (
	"context"
	"log/slog"
	"strings"
	"time"

	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"

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
	// RequestParentChanges records that parents changed a request.
	RequestParentChanges interface {
		MarkRequestParentChanged(context.Context, int64) error
	}
	// IntakeRequests reads and writes the submitted requests.
	IntakeRequests interface {
		InsertRequest(context.Context, *enrollmentNative.Request) error
		RequestByID(context.Context, int64, bool) (*enrollmentNative.Request, error)
		RequestByToken(context.Context, string, bool) (*enrollmentNative.Request, error)
		RequestsByID(context.Context, []int64) ([]*enrollmentNative.Request, error)
		UpdateRequestGuardian(context.Context, *enrollmentNative.Request, bool) error
		SetRequestWithdrawal(context.Context, int64, *time.Time) error
		AcquireSubmissionDedupLock(context.Context, int64, uint64) error
		AcquireExistingStudentMatchLock(context.Context, int64) error
		ActiveDuplicateChildren(context.Context, int64, string, []enrollmentNative.DuplicateChildKey, int64) ([]enrollmentNative.DuplicateChildKey, error)
		HasActiveRequestForMatchedStudent(context.Context, int64, int64, int64) (bool, error)
		PinDecisionNotificationMode(context.Context, int64, string) (string, error)
	}
	// IntakeChildren reads and writes the children of the requests with
	// their submitted offering choices.
	IntakeChildren interface {
		enrollmentNative.SubmittedOfferingCommands
		RequestChildOfferingHistoryForChildren(context.Context, []int64) ([]*enrollmentNative.RequestChildOffering, error)
		RequestChildOfferingsForChildrenAtDate(context.Context, []int64, enrollmentNative.Date) ([]*enrollmentNative.RequestChildOffering, error)
		OfferingCapacityPeak(context.Context, int64, []int64, enrollmentNative.Date, enrollmentNative.Date) (int, error)
		InsertChild(context.Context, *enrollmentNative.RequestChild) error
		ChildrenForRequest(context.Context, int64, bool) ([]*enrollmentNative.RequestChild, error)
		ChildrenForRequests(context.Context, []int64) ([]*enrollmentNative.RequestChild, error)
		DeleteRequestChildren(context.Context, int64) error
		UpdateChildStatus(context.Context, int64, string, *string, int64) error
		UpdateChildData(context.Context, *enrollmentNative.RequestChild) error
		UpdateMatchedStudent(context.Context, int64, *int64) error
	}
	// IntakeGuardians keeps the co-guardians of the requests.
	IntakeGuardians interface {
		CreateRequestGuardian(context.Context, *enrollmentNative.RequestGuardian) error
		RequestGuardians(context.Context, []int64) ([]*enrollmentNative.RequestGuardian, error)
		DeleteRequestGuardians(context.Context, int64) error
	}
	// IntakeLateInvites issues, resolves and consumes late invites.
	IntakeLateInvites interface {
		InsertLateInvite(context.Context, *enrollmentNative.LateInvite) error
		UsableLateInvite(context.Context, string, int64, time.Time, bool) (*enrollmentNative.LateInvite, error)
		MarkLateInviteUsed(context.Context, int64, int64, time.Time) error
		LateInviteByUsedRequestID(context.Context, int64) (*enrollmentNative.LateInvite, error)
	}
	// IntakeCatalog loads the phase and the immutable schema a submission
	// works with.
	IntakeCatalog interface {
		Phase(context.Context, int64) (*enrollmentNative.Phase, error)
		Schema(context.Context, int64) (*enrollmentNative.FormSchema, error)
	}
	// SubmissionRateLimiter counts submission attempts per IP and email.
	SubmissionRateLimiter interface {
		IncrementAttempts(context.Context, int64, string, string, time.Duration) (*enrollmentNative.SubmissionRateLimitState, error)
	}
)

// IntakeOfferings reads Care Plan's offering catalog in Enrollment's
// offering rows.
type IntakeOfferings interface {
	ListActiveByPhase(ctx context.Context, phaseID int64) ([]*offeringContract.CareOfferingRecord, error)
	ListByIDs(ctx context.Context, ids []int64) ([]*offeringContract.CareOfferingRecord, error)
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

// IntakeSettings resolves tenant overrides or registry defaults. Permission
// and token-lifetime reads propagate lookup failures.
type IntakeSettings interface {
	// EnrollmentEnabled defaults to false.
	EnrollmentEnabled(ctx context.Context) bool
	// AllowSubmissionEdit reports lookup failures instead of granting an edit.
	AllowSubmissionEdit(ctx context.Context) (bool, error)
	// StatusTokenTTLDays resolves the token lifetime or reports the read failure.
	StatusTokenTTLDays(ctx context.Context) (int, error)
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

// AdminMailSubscribers adds the staff who switched on the "Neue Anmeldung"
// mail (#3780) to the configured admin recipients.
type AdminMailSubscribers interface {
	// AdminNotificationRecipients returns configured plus every account of
	// the school in context that switched the mail on and may still manage
	// enrollments, deduplicated case-insensitively.
	AdminNotificationRecipients(ctx context.Context, configured []string) ([]string, error)
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
	Decide(ctx context.Context, input enrollmentNative.DecideInput) (*enrollmentNative.DecideOutcome, error)
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
	Bookings           enrollmentNative.CareBookingCommands
	Capacity           enrollmentNative.OfferingCapacity
	Schools            enrollmentNative.SchoolDirectory
	Notifications      enrollmentNative.Notifications
	Students           StudentMatches
	GuardianAuthorizer GuardianStudentAuthorizer
	Outbox             MailOutbox
	Settings           IntakeSettings
	// AdminSubscribers widens the admin notification to the staff who
	// switched it on (#3780). Nil keeps the configured list only.
	AdminSubscribers AdminMailSubscribers
	ManualDecider    ManualEnrollmentDecider
	// Uploads stores the files parents attach (#4783). Nil leaves file
	// answers untouched and refuses uploads.
	Uploads IntakeUploads
	// ParentChanges makes a request unread for the staff again after a
	// parent edit or renewal confirmation (#3778). Nil skips it (focused tests).
	ParentChanges RequestParentChanges
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
	_ enrollmentNative.IntakeSubmissions = (*Intake)(nil)
	_ enrollmentNative.IntakeStatus      = (*Intake)(nil)
	_ enrollmentNative.IntakeForms       = (*Intake)(nil)
)

// NewIntake composes the intake. A nil logger falls back to slog.Default().
func NewIntake(deps IntakeDependencies) *Intake {
	if deps.Settings == nil {
		panic("enrollment intake: settings are required")
	}
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

func (s *Intake) intakePhase(ctx context.Context, id int64) (*enrollmentNative.Phase, error) {
	return s.deps.Catalog.Phase(ctx, id)
}

func (s *Intake) intakeSchema(ctx context.Context, id int64) (*enrollmentNative.FormSchema, error) {
	value, err := s.deps.Catalog.Schema(ctx, id)
	return enrollmentNative.CopyFormSchema(value), err
}
