package enrollment

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"maps"
	"net/mail"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// RequestService sentinel errors. The HTTP layer maps these to status
// codes; tests assert on them via errors.Is.
var (
	ErrEnrollmentDisabled      = errors.New("enrollment is not enabled for this tenant")
	ErrEnrollmentWindowClosed  = errors.New("enrollment window is closed")
	ErrLateInviteInvalid       = errors.New("late invite is invalid")
	ErrInvalidSubmission       = errors.New("invalid submission")
	ErrCareOfferingClosed      = errors.New("one or more selected care offerings are not currently accepting applications")
	ErrCareOfferingUnavailable = errors.New("one or more selected care offerings are not available for this child")
	ErrCareOfferingFull        = errors.New("one or more selected care offerings are at capacity")
	ErrCareOfferingsDisabled   = errors.New("care offerings are disabled for this tenant")
	// ErrCareOfferingMissing is returned when a phase requires at least
	// one care offering per child but a child has no offering selected.
	// Mapped to 400 with a stable code so the parent form can highlight
	// the right child.
	ErrCareOfferingMissing = errors.New("care offering selection is required for every child")
	// ErrCareOfferingExactlyOneRequired is returned when a phase requires
	// exactly one care offering per child but a child selected none or
	// more than one.
	ErrCareOfferingExactlyOneRequired = errors.New("exactly one care offering must be selected for every child")
	// ErrRequiredCareOfferingMissing is returned when a care offering
	// flagged is_required is not selected for one of the children. Unlike
	// ErrCareOfferingMissing (the phase-wide "at least one" gate), this
	// targets a specific mandatory offering. Mapped to 400 with a stable
	// code so the parent form can highlight the right child.
	ErrRequiredCareOfferingMissing = errors.New("a required care offering was not selected for every child")
	// ErrCareOfferingRule wraps ErrInvalidSubmission (so the HTTP layer
	// maps it to 400) and is returned when a child's offering selection
	// violates a group's selection rule (exactly_one / at_least_one /
	// at_most_one). Defense-in-depth: the parent form enforces the same.
	ErrCareOfferingRule     = fmt.Errorf("%w: care offering selection rule not satisfied", ErrInvalidSubmission)
	ErrRateLimited          = errors.New("too many submission attempts; please retry later")
	ErrRequestNotFound      = errors.New("enrollment request not found")
	ErrInvalidGuardianPhone = errors.New("guardian phone number has an invalid format")
	// ErrInvalidGuardianEmail wraps ErrInvalidSubmission so callers that match
	// the broad category keep working, while the HTTP layer maps the specific
	// case to a stable code (enrollment.invalid_email) for per-field marking.
	ErrInvalidGuardianEmail = fmt.Errorf("%w: guardian email has an invalid format", ErrInvalidSubmission)
	// ErrPickupTimeNotAllowed wraps ErrInvalidSubmission (so the HTTP layer
	// still maps it to 400) and is returned when a submitted weekday_schedule
	// time falls outside the field's configured fixed pickup times. The
	// specific identity lets the handler attach a stable code
	// (enrollment.pickup_time_not_allowed) so the parent form can localize the
	// message and highlight the offending schedule field.
	ErrPickupTimeNotAllowed = fmt.Errorf("%w: pickup time not allowed", ErrInvalidSubmission)
	// The three parent-input day errors (#1846/#1885) wrap
	// ErrInvalidSubmission (HTTP 400) and carry their own identity so the
	// handler can attach stable codes for localized form messages.
	ErrSelectedDayNotAvailable = fmt.Errorf("%w: selected day is not available for this offering", ErrInvalidSubmission)
	ErrDaySelectionRequired    = fmt.Errorf("%w: offering requires the parent to pick at least one day", ErrInvalidSubmission)
	ErrDaySelectionNotAllowed  = fmt.Errorf("%w: offering does not allow parent day selection (days_of_week_mode=fixed)", ErrInvalidSubmission)
	ErrEditNotAllowed          = errors.New("request can no longer be edited")
	ErrWithdrawNotAllowed      = errors.New("child cannot be withdrawn in its current state")
	ErrDuplicateEnrollment     = errors.New("an active enrollment already exists for this parent and child in this phase")
	// ErrExistingStudentAlreadyRequested rejects an existing_students
	// submission (or parent edit) whose child matched an already-enrolled
	// student that ANOTHER active request in the same phase already targets.
	// The email-based duplicate check keys on guardian_email, so two guardians
	// with different emails submitting the same child both slip through it yet
	// pin the same matched_student_id; approving both would renew/overwrite one
	// live student twice and duplicate its care-offering enrollments. Enforced
	// unconditionally (independent of the block/warn/ignore duplicate policy)
	// because it protects a live student record, not just parent convenience
	// (#1663). Mapped to 409 Conflict.
	ErrExistingStudentAlreadyRequested = errors.New("another active enrollment request already targets this student in this phase")
	// Phase eligibility sentinels (#1663). ErrPhaseNotEligible is the
	// audience gate: a linked_parents phase rejects anonymous submissions
	// (the parent handler additionally verifies the guardian link before
	// stamping GuardianAccountID). The two child-level errors carry stable
	// codes so the form can explain which child is affected.
	ErrPhaseNotEligible      = errors.New("phase is not open for this applicant")
	ErrChildClassNotEligible = fmt.Errorf("%w: child school class is not eligible for this phase", ErrInvalidSubmission)
	// ErrChildGradeNotEligible is the grade-level counterpart of
	// ErrChildClassNotEligible: a phase aimed at whole grades (e.g. all
	// grade-3 applicants) rejects a child declaring any other grade.
	ErrChildGradeNotEligible = fmt.Errorf("%w: child grade level is not eligible for this phase", ErrInvalidSubmission)
	ErrChildAlreadyEnrolled  = fmt.Errorf("%w: child is already enrolled at this school", ErrInvalidSubmission)
	// ErrChildNotEnrolled backs the existing_students audience — the
	// inverse of ErrChildAlreadyEnrolled: a phase open only to already
	// enrolled students rejects a child with no matching enrolled record.
	ErrChildNotEnrolled = fmt.Errorf("%w: child is not enrolled at this school", ErrInvalidSubmission)
	// ErrChildEnrollmentAmbiguous rejects an existing_students submission
	// whose child matches MORE THAN ONE already-enrolled record by
	// name+birthday. The enrolled gate passes (at least one match exists) but
	// the matched-student resolver refuses to guess which record to renew, so
	// approval would silently take the fresh-create path and add yet another
	// duplicate on top of the colliding records. The school must resolve the
	// duplicate students first (#1663).
	ErrChildEnrollmentAmbiguous = fmt.Errorf("%w: child matches multiple enrolled students at this school", ErrInvalidSubmission)
	// ErrChildEnrollmentNotPermitted rejects an existing_students re-enrollment
	// submitted from the parents portal when the authenticated guardian account
	// does NOT hold parent_portal.enrollment.submit on the SPECIFIC already
	// enrolled student the child matched (#1663). Guardian parent-portal
	// permissions are relationship-scoped: a parent authorized to re-enroll one
	// child must not be able to renew a DIFFERENT child at the same school just
	// because the school-wide GuardianSubmitEligible audience flag is set. It is
	// an authorization failure (mapped to 403), NOT an ErrInvalidSubmission.
	ErrChildEnrollmentNotPermitted = errors.New("guardian is not permitted to re-enroll this child")
	// ErrPhaseAudienceRestricted is the public form-load gate for
	// audience-restricted phases (#1663): a linked_parents or
	// existing_students phase cannot be bootstrapped anonymously, so the
	// unauthenticated public path rejects it (the same set Submit refuses —
	// see audienceRequiresGuardianAccount). It maps to a plain 404 so an
	// anonymous caller cannot distinguish
	// a restricted phase from a non-existent one; the parents portal loads
	// these phases through its own authenticated bootstrap path instead.
	ErrPhaseAudienceRestricted = errors.New("phase is not available for public enrollment")
)

// Rate-limit thresholds. Hardcoded for now - if individual schools
// need different limits we can promote these to settings, but the
// defaults need to work for "small school with families of 3 kids
// submitting once" without ever needing tuning.
const (
	rateLimitWindowIP        = time.Hour
	rateLimitWindowEmail     = 24 * time.Hour
	rateLimitMaxAttemptsIP   = 10
	rateLimitMaxAttemptsMail = 5
)

// SubmitRequest is the data the public submission handler hands to the
// service. PR 7's HTTP layer translates the JSON wire shape into this
// struct.
type SubmitRequest struct {
	TenantID int64
	PhaseID  int64

	// RemoteIP is the parent's source IP (handler resolves X-Forwarded-For
	// when set, falls back to RemoteAddr). Empty disables IP-based rate
	// limiting for that submission - emit it from the handler when at
	// all possible.
	RemoteIP string

	// LateInviteToken opens a closed phase only for the token-bound guardian
	// email. It is validated and consumed inside Submit's write transaction.
	LateInviteToken string

	// Internal admin paths set these. Public HTTP callers never deserialize
	// them directly; handlers construct the service request.
	SkipRateLimit            bool
	AllowClosedPhase         bool
	SuppressSubmissionEmails bool
	ExternalConsentConfirmed bool
	SubmissionSource         string
	SourceMetadata           map[string]any

	GuardianFirstName string
	GuardianLastName  string
	GuardianEmail     string
	GuardianPhone     *string
	ConsentFlags      map[string]any
	CustomData        map[string]any

	// GuardianAccountID is set when the submission comes from an
	// authenticated parent on the parents portal. Stamped onto the
	// request row so PR 11/4 can skip the invitation when an account
	// already exists. nil = anonymous public submission.
	GuardianAccountID *int64

	// GuardianSubmitEligible is set by the parent handler alongside
	// GuardianAccountID: true when the account holds a guardian
	// relationship at the tenant granting parent_portal.enrollment.submit
	// (#1663). Phases with audience=linked_parents require it; open and
	// new_students phases accept authenticated parents without any
	// guardian link (applying to a new school is the point of the parent
	// picker). Always false for anonymous submissions.
	GuardianSubmitEligible bool

	Children []SubmitChild

	// AdditionalGuardians are the co-guardians the parent added beyond
	// the primary guardian above. Stored in enrollment.request_guardians
	// and materialized as additional users.students_guardians links on
	// approval. Email/phone are optional per co-guardian.
	AdditionalGuardians []SubmitGuardian
}

// SubmitGuardian is one additional guardian (Erziehungsberechtigte:r)
// within a SubmitRequest. Only first/last name are required; email and
// phone are optional (a co-guardian may be a contact-only record).
type SubmitGuardian struct {
	FirstName string
	LastName  string
	Email     *string
	Phone     *string
}

// SubmitChild is one child within a SubmitRequest.
//
// OfferingDays is the optional per-offering day-selection refinement
// for offerings whose days_of_week_mode is "parent_choice". Entries
// in OfferingDays MUST also appear in OfferingIDs; missing entries
// inherit the offering's default (admin-fixed) day set, written as
// NULL on the resulting request_child_offerings row. The service
// validates subset/non-empty before inserting.
type SubmitChild struct {
	ID                int64
	FirstName         string
	LastName          string
	DateOfBirth       timezone.Date
	TargetGradeLevel  *int16
	TargetSchoolClass *string
	CustomData        map[string]any
	OfferingIDs       []int64
	OfferingDays      []SubmitOfferingDays
}

// SubmitOfferingDays is one row of SubmitChild.OfferingDays.
type SubmitOfferingDays struct {
	OfferingID   int64
	SelectedDays []string
}

// SubmitResult bundles what the handler needs after Submit returns.
type SubmitResult struct {
	Request   *enrollmentModels.Request
	Children  []*enrollmentModels.RequestChild
	StatusURL string
	Warnings  []SubmissionWarning
}

type SubmissionWarning struct {
	Code string `json:"code"`
}

const WarningCodeDuplicateEnrollment = "enrollment.duplicate_detected"

// FormCapabilities is the effective tenant configuration for enrollment
// inputs. It is resolved inside the tenant transaction and is authoritative
// for both bootstrap visibility and submission validation.
type FormCapabilities struct {
	CollectGradeLevel    bool
	CollectSchoolClass   bool
	CareOfferingsEnabled bool
}

type CreateLateInviteInput struct {
	PhaseID           int64
	GuardianEmail     string
	GuardianFirstName string
	GuardianLastName  string
	Reason            string
	ExpiresAt         *time.Time
	CreatedBy         int64
}

type CreateLateInviteResult struct {
	Invite *enrollmentModels.LateInvite
	Token  string
}

type materializedOfferingSelection struct {
	OfferingID            int64
	SelectedDays          []string
	ManualSelectedDays    []string
	AutomaticSelectedDays []string
}

// EditDraft is the complete persisted request shape needed to reopen the
// public enrollment form for a submitted request.
type EditDraft struct {
	Request          *enrollmentModels.Request
	Children         []*enrollmentModels.RequestChild
	Guardians        []*enrollmentModels.RequestGuardian
	OfferingsByChild map[int64][]*enrollmentModels.RequestChildOffering
	Phase            *enrollmentModels.Phase
	School           *platformModels.School
	Schema           *enrollmentModels.FormSchema
	OpenOfferings    []*enrollmentModels.CareOffering
	LegalTexts       LegalTexts
	EditMode         string
	// CollectSchoolClass mirrors the tenant's enrollment.collect_school_class
	// setting (#1833) so the reopened form knows whether to show the
	// concrete-class field. Combined with the phase's AvailableSchoolClasses
	// and RequireSchoolClass it forms the public concrete-class config.
	CollectSchoolClass   bool
	CollectGradeLevel    bool
	CareOfferingsEnabled bool
	// GradeLevelMax is the tenant's server-authoritative upper bound for
	// target grades. Token-based edit pages have no reliable tenant context
	// of their own, so the bootstrap must carry the value resolved inside the
	// same tenant transaction as the rest of the draft.
	GradeLevelMax int
}

const (
	EditModeDirectEdit    = "direct_edit"
	EditModeChangeRequest = "change_request"
	EditModeNone          = "none"
)

// EditPatch carries the fields the parent can edit between submission
// and the first admin decision. Pointer fields = "leave alone unless
// set"; map fields replace wholesale.
type EditPatch struct {
	GuardianFirstName *string
	GuardianLastName  *string
	GuardianPhone     *string
	ConsentFlags      map[string]any
	CustomData        map[string]any
}

// RequestService manages the parent-facing submission lifecycle. PR 7
// ships Submit / GetByStatusToken / Edit / Withdraw; PR 8's decision
// service builds on the same data.
type RequestService interface {
	Submit(ctx context.Context, req SubmitRequest) (*SubmitResult, error)
	CreateLateInvite(ctx context.Context, input CreateLateInviteInput) (*CreateLateInviteResult, error)
	GetByStatusToken(ctx context.Context, token string) (*enrollmentModels.Request, []*enrollmentModels.RequestChild, error)
	EditModeForStatus(ctx context.Context, req *enrollmentModels.Request, children []*enrollmentModels.RequestChild) (string, error)
	GetEditDraft(ctx context.Context, token string) (*EditDraft, error)
	ReplaceEditable(ctx context.Context, token string, req SubmitRequest) (*SubmitResult, error)
	// GuardiansByStatusToken returns the additional guardians (co-guardians)
	// for the request behind a public status token. Kept separate from
	// GetByStatusToken so the edit/withdraw callers stay untouched.
	GuardiansByStatusToken(ctx context.Context, token string) ([]*enrollmentModels.RequestGuardian, error)
	Edit(ctx context.Context, token string, patch EditPatch) error
	Withdraw(ctx context.Context, token string, childID int64) error

	// ConfirmRenewal transitions every pending_renewal child under
	// this request to submitted, so the admin's regular review queue
	// picks it up. Used by the parent-facing "Anmeldung bestätigen"
	// button in opt-in rollover mode. No-op when the request has no
	// pending_renewal rows (idempotent on double-clicks).
	ConfirmRenewal(ctx context.Context, token string) (int, error)
	// IsEnrollmentEnabled reports whether the per-tenant master toggle
	// (enrollment.enabled setting) is on for the tenant in ctx. Public
	// form-load endpoints call this so a deactivated tenant returns a
	// 404 at the picker/schema step instead of letting the parent fill
	// in a whole form before being rejected at submit. Caller must
	// already be inside a tenant-tx so the settings repo can read the
	// per-tenant override.
	IsEnrollmentEnabled(ctx context.Context) bool

	// CollectsSchoolClass reports whether the tenant collects the
	// concrete future class (e.g. "2a") in addition to the grade level
	// (enrollment.collect_school_class setting, issue #1833). Public
	// form-load endpoints call this to decide whether to surface the
	// class field. Caller must already be inside a tenant-tx. A settings
	// resolution failure is returned, not swallowed as "disabled".
	CollectsSchoolClass(ctx context.Context) (bool, error)
	FormCapabilities(ctx context.Context) (FormCapabilities, error)

	// LegalTexts returns the tenant's configured legal texts and derived
	// public blocks for the enrollment form. Empty strings mean the admin
	// hasn't filled the text in; such blocks are not rendered.
	// Caller must already be inside a tenant-tx so the settings repo
	// reads the per-tenant override. A non-nil error means a real
	// settings/DB/JSON failure — the caller MUST fail the request rather
	// than fall back to an incomplete legal state, because these texts sit
	// behind legally relevant blocks.
	LegalTexts(ctx context.Context) (LegalTexts, error)

	// LegalTextsForPhaseWithLateInvite returns the legal block contract for
	// the selected phase. When the phase's template carries at least one
	// ENABLED legal block those blocks win; otherwise it falls back to the
	// tenant-wide legal settings. Runs the same public phase gate as
	// LoadPublicPhaseWithLateInvite, so stale or closed phases surface the
	// sentinel errors instead of an incomplete legal contract.
	LegalTextsForPhaseWithLateInvite(ctx context.Context, phaseID int64, lateInviteToken string) (LegalTexts, error)
	LegalTextsForManualEnrollmentPhase(ctx context.Context, phaseID int64) (LegalTexts, error)

	// LoadPublicPhaseWithLateInvite is the shared anonymous public phase
	// gate: every public form-load endpoint (schema, offerings, legal texts,
	// bootstrap) calls this so a direct or stale parent link cannot load
	// detail data for a phase the picker would hide. Returns
	// ErrEnrollmentDisabled when the tenant toggle is off or the phase is
	// inactive, ErrInvalidSubmission when the id is unknown,
	// ErrEnrollmentWindowClosed outside the phase's enrollment window, and
	// ErrPhaseAudienceRestricted for an audience-restricted (linked_parents /
	// existing_students) phase — those are reachable only through the
	// authenticated parents-portal gate (LoadEnrolleePhaseWithLateInvite).
	// Caller must be inside a tenant-tx.
	LoadPublicPhaseWithLateInvite(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (*enrollmentModels.Phase, error)
	// LoadEnrolleePhaseWithLateInvite is the authenticated parents-portal
	// counterpart to LoadPublicPhaseWithLateInvite; identical except it also
	// admits the audience-restricted (linked_parents / existing_students)
	// phases the caller's resolved guardian facts cover — the caller passes
	// them as EnrolleeAudienceAccess, whose zero value is the public gate.
	LoadEnrolleePhaseWithLateInvite(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string, access EnrolleeAudienceAccess) (*enrollmentModels.Phase, error)
	LoadManualEnrollmentPhase(ctx context.Context, phaseID int64) (*enrollmentModels.Phase, error)

	// LoadPublicFormBootstrap assembles everything the public enrollment
	// form-load endpoint needs inside one tenant transaction: the gated
	// phase, its pinned schema (degraded to nil when stale), the active
	// care offerings, the resolved raw form capabilities, and the legal
	// contract. Capability- and legal-resolution failures come back wrapped
	// in *BootstrapStageError so the caller can map them to 500 instead of
	// the public 404 gate mapping. Caller must be inside a tenant-tx.
	LoadPublicFormBootstrap(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (*PublicFormBootstrapData, error)
	// LoadEnrolleeFormBootstrap mirrors LoadPublicFormBootstrap for the
	// authenticated parents-portal form load, using the enrollee phase gate
	// so the audience-restricted phases the caller's access covers load for a
	// logged-in guardian. Passing the zero access value makes it behave
	// exactly like LoadPublicFormBootstrap. Caller must be inside a tenant-tx.
	LoadEnrolleeFormBootstrap(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string, access EnrolleeAudienceAccess) (*PublicFormBootstrapData, error)
	// LoadPublicCareOfferings is the offering-only projection of the public
	// bootstrap: phase gate + capabilities + active offerings, no schema,
	// legal texts or captcha. Capability failures are wrapped in
	// *BootstrapStageError. Caller must be inside a tenant-tx.
	LoadPublicCareOfferings(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (*PublicFormBootstrapData, error)
	// LoadManualEnrollmentBootstrap mirrors LoadPublicFormBootstrap for the
	// admin manual-enrollment flow, using the manual phase gate and legal
	// loader. It does not wrap stage errors — the admin caller maps the raw
	// service errors itself. Caller must be inside a tenant-tx.
	LoadManualEnrollmentBootstrap(ctx context.Context, phaseID int64) (*PublicFormBootstrapData, error)

	// CreateManualApprovedEnrollment submits a privileged admin-created
	// enrollment (rate-limit skipped, closed window allowed, submission
	// emails suppressed, source = AdminManual) and approves it in the same
	// transaction. Caller must be inside a tenant-tx.
	CreateManualApprovedEnrollment(ctx context.Context, input ManualApprovedEnrollmentInput) (*ManualApprovedEnrollmentResult, error)
	// PublicActiveSchema resolves the form schema a public parent form
	// should render for a (phase, tenant) pair. It runs the shared public
	// phase gate first (LoadPublicPhaseWithLateInvite). A Basis phase (no
	// pinned schema) returns ErrNoActiveSchema so the form renders core
	// fields only — it deliberately does NOT fall back to the tenant's
	// currently-active schema, or a custom form would leak its fields into
	// every Basis phase. A pinned-but-deleted schema also returns
	// ErrNoActiveSchema. Caller must be inside a tenant-tx.
	PublicActiveSchema(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (*enrollmentModels.FormSchema, error)
}

// LegalTexts bundles the per-tenant legal texts surfaced on the public
// enrollment form. Standard blocks render only when their toggle is enabled
// and the matching text is non-empty.
type LegalTexts struct {
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
	Blocks              []LegalBlock
}

// LegalBlock is one configured legal row shown on the public enrollment
// form. Required checkbox blocks must be accepted; notice blocks only display
// information.
type LegalBlock struct {
	Key       string `json:"key"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Label     string `json:"label"`
	Text      string `json:"text"`
	Required  bool   `json:"required"`
	SortOrder int    `json:"sort_order,omitempty"`
	Source    string `json:"source,omitempty"`
}

// RequestSettingsResolver is the narrow contract the service needs from
// the platform settings service.
type RequestSettingsResolver interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// GuardianStudentAuthorizer is the narrow slice of the student-guardian
// repository the submit path needs: a per-child parent-portal permission probe.
// It backs the existing_students re-enrollment authorization gate (#1663) — the
// only place Submit resolves a concrete existing student a submission could
// renew. Two probes, one per identity the enrollment flows can carry: the
// authenticated portal account, and (for the accountless late-invite path) the
// guardian email the request is bound to.
type GuardianStudentAuthorizer interface {
	AccountHasStudentPermission(ctx context.Context, accountID, studentID, tenantID int64, permission string) (bool, error)
	GuardianEmailHasStudentPermission(ctx context.Context, email string, studentID, tenantID int64, permission string) (bool, error)
}

// RequestServiceConfig is the dep-injection bundle.
type RequestServiceConfig struct {
	RequestRepo              enrollmentModels.RequestRepository
	RequestChildRepo         enrollmentModels.RequestChildRepository
	RequestGuardianRepo      enrollmentModels.RequestGuardianRepository
	LateInviteRepo           enrollmentModels.LateInviteRepository
	RequestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	CareOfferingRepo         enrollmentModels.CareOfferingRepository
	FormSchemaRepo           enrollmentModels.FormSchemaRepository
	PhaseRepo                enrollmentModels.PhaseRepository
	SchoolRepo               platformModels.SchoolRepository
	// StudentRepo backs the new_students audience check (#1663): a
	// submission for a child who is already an enrolled student at the
	// school is rejected. Nil-safe — without the repo the check is
	// skipped (relevant for narrow test setups only; the factory always
	// wires it).
	StudentRepo users.StudentRepository
	// GuardianAuthorizer verifies per-child parent-portal permissions for every
	// submit path that can pin an existing student. It gates existing_students
	// re-enrollment (#1663): the matched student may be renewed only when the
	// request's own guardian identity — the authenticated account, or the bound
	// guardian email on the accountless late-invite path — holds
	// parent_portal.enrollment.submit on ITS relationship to that student. The
	// school-wide GuardianSubmitEligible flag and a late-invite token both admit
	// the submitter to the phase but neither proves authority over one child. The
	// submit path fails closed when the authorizer is nil and a submission
	// resolves an existing student, so only tests that never pin one leave it
	// unset (the factory always wires it).
	GuardianAuthorizer GuardianStudentAuthorizer
	RateLimitRepo      enrollmentModels.SubmissionRateLimitRepository
	OutboxEnqueuer     platformModels.OutboxEnqueuer
	Settings           RequestSettingsResolver
	// ManualDecider approves the freshly submitted request in the manual
	// admin-enrollment flow. Narrow slice of DecisionService so the request
	// service does not depend on the whole decision surface.
	ManualDecider manualEnrollmentDecider
	FrontendURL   string // staff/admin URLs only (admin notification email link)
	ParentsURL    string // parent-facing URLs (status link, logo). Falls back to FrontendURL when empty.
	DB            *bun.DB
	Logger        *slog.Logger
}

type requestService struct {
	RequestServiceConfig
	txHandler *modelBase.TxHandler
}

// NewRequestService builds the service. A nil logger falls back to
// slog.Default().
func NewRequestService(cfg RequestServiceConfig) RequestService {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	cfg.FrontendURL = strings.TrimRight(strings.TrimSpace(cfg.FrontendURL), "/")
	cfg.ParentsURL = strings.TrimRight(strings.TrimSpace(cfg.ParentsURL), "/")
	if cfg.ParentsURL == "" {
		cfg.ParentsURL = cfg.FrontendURL
	}
	return &requestService{
		RequestServiceConfig: cfg,
		txHandler:            modelBase.NewTxHandler(cfg.DB),
	}
}

// Submit is the workhorse. One DB transaction writes the request, all
// child rows, all care-offering selections, and enqueues the two
// confirmation emails. Either everything lands or nothing does.
//
// Phase-model: req.PhaseID identifies the parent's chosen phase. The
// phase carries its own enrollment window, form-schema reference, and
// care-overflow mode - all the things that used to be tenant-wide.
func (s *requestService) Submit(ctx context.Context, req SubmitRequest) (*SubmitResult, error) {
	if !req.SkipRateLimit {
		if err := s.enforceRateLimit(ctx, req); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(req.LateInviteToken) != "" {
		req.AllowClosedPhase = true
		req.SubmissionSource = enrollmentModels.RequestSourceLateInvite
	}
	submissionSource := normalizedSubmissionSource(req.SubmissionSource)
	sourceMetadata := cloneSourceMetadata(req.SourceMetadata)

	phase, err := s.loadPhaseForSubmission(ctx, req.PhaseID)
	if err != nil {
		return nil, err
	}
	// Bind the phase to the submission's tenant. The parent submit path
	// resolves the phase under an admin transaction (RLS bypassed), so a
	// phase_id belonging to another school would otherwise load cleanly and
	// get stamped with THIS school's tenant_id. Reject the cross-tenant
	// reference before any window / eligibility / permission decision
	// consumes it — mirroring the "not found" shape so a probe can't
	// distinguish another tenant's phase from a nonexistent one (#1663).
	if phase.TenantID != req.TenantID {
		return nil, fmt.Errorf("%w: phase %d not found", ErrInvalidSubmission, req.PhaseID)
	}
	now := time.Now()
	if !req.AllowClosedPhase && !IsEnrollmentWindowOpen(phase, now) {
		return nil, ErrEnrollmentWindowClosed
	}
	capabilities, err := s.FormCapabilities(ctx)
	if err != nil {
		return nil, fmt.Errorf("submit: resolve form capabilities: %w", err)
	}
	duplicatePolicy, err := s.Settings.ResolveString(ctx, configModel.KeyEnrollmentDuplicateHandling)
	if err != nil {
		return nil, fmt.Errorf("submit: resolve duplicate handling: %w", err)
	}

	// Build the dedup key list once; the actual check + insert happens
	// inside the write tx below, after we've taken an advisory lock to
	// serialize concurrent submits for the same (phase, email).
	dupKeys := make([]enrollmentModels.DuplicateChildKey, 0, len(req.Children))
	for _, c := range req.Children {
		dupKeys = append(dupKeys, enrollmentModels.DuplicateChildKey{
			FirstName: c.FirstName,
			LastName:  c.LastName,
		})
	}

	openOfferings := []*enrollmentModels.CareOffering{}
	if capabilities.CareOfferingsEnabled {
		openOfferings, err = s.CareOfferingRepo.ListActiveByPhase(ctx, phase.ID)
		if err != nil {
			return nil, fmt.Errorf("submit: load phase offerings: %w", err)
		}
	}
	capabilities = EffectiveFormCapabilities(capabilities, openOfferings)
	if err := normalizeSubmissionForCapabilities(&req, capabilities); err != nil {
		return nil, err
	}
	openByID := make(map[int64]*enrollmentModels.CareOffering, len(openOfferings))
	for _, o := range openOfferings {
		openByID[o.ID] = o
	}
	selectionMode := enrollmentModels.PhaseCareOfferingSelectionOptional
	if capabilities.CareOfferingsEnabled {
		selectionMode = phase.CareOfferingSelectionMode
	}
	materializedSelections, err := materializeAndValidateChildrenOfferingSelections(req.Children, openByID, selectionMode)
	if err != nil {
		return nil, err
	}
	capacityChildren := childrenWithMaterializedOfferingSelections(req.Children, materializedSelections)

	// childStatusOverrides[i] is set when capacity logic forces a
	// non-default status (e.g. waitlisted under mode=waitlist). It is resolved
	// inside the write transaction below, so the offering locks protect both
	// the count and the request-child offering rows that consume the slot.
	childStatusOverrides := map[int]string{}

	// Pin the schema version to whichever schema the phase points at,
	// or the tenant's currently-active schema if the phase has no
	// override. When neither resolves (Basis phase + no tenant schema
	// ever published), we submit without a schema_id - the column is
	// nullable since migration 1.15.69.
	schema, err := s.resolveSubmissionSchema(ctx, phase)
	if err != nil {
		return nil, fmt.Errorf("submit: load schema: %w", err)
	}
	legalBlocks, err := s.resolveSubmissionLegalBlocks(ctx, schema)
	if err != nil {
		return nil, fmt.Errorf("submit: resolve legal blocks: %w", err)
	}
	if req.ExternalConsentConfirmed {
		req.ConsentFlags = ensureRequiredConsentFlags(req.ConsentFlags, legalBlocks)
	}
	// Normalize + validate additional guardians (co-guardians the parent
	// added beyond the primary). Mutates req.AdditionalGuardians in place:
	// trims, drops fully-blank cards, errors on a half-filled card or a
	// bad email/phone, and dedups against the primary + each other. The
	// cleaned slice is what the insert loop below persists.
	if err := normalizeAdditionalGuardians(&req); err != nil {
		return nil, err
	}
	if err := s.validateSubmission(ctx, req, legalBlocks, capabilities); err != nil {
		return nil, err
	}
	// Concrete-class rules need the phase (pick list + require flag) and
	// the tenant setting, so they run here rather than in validateSubmission.
	// Mutates req.Children[i].TargetSchoolClass to the persisted value.
	if err := s.validateAndNormalizeSchoolClasses(ctx, phase, req.Children); err != nil {
		return nil, err
	}
	// Phase eligibility runs AFTER class canonicalization so it validates
	// the persisted TargetSchoolClass, not the raw client value. Otherwise a
	// crafted request could declare an eligible class for a grade-1 child (or
	// with concrete-class collection disabled), pass the class gate, and then
	// have validateAndNormalizeSchoolClasses erase the class — letting an
	// ineligible submission through (#1663). No DB writes happen before the
	// write tx below, so the later rejection stays clean.
	if err := s.validatePhaseEligibility(ctx, phase, req); err != nil {
		return nil, err
	}
	// consent_flags is legally meaningful data: persist only keys the
	// resolved legal-block contract declares so a stale or manipulated
	// client cannot smuggle arbitrary consent entries into the request.
	req.ConsentFlags = filterConsentFlags(req.ConsentFlags, legalBlocks)

	// Single required-field gate: enforces required core + custom fields
	// server-side (defense-in-depth; the client checks the same), while
	// exempting fields hidden by a visibility condition. A field hidden by
	// its show-if condition must never block an otherwise valid submit.
	if err := s.validateRequiredCustomFields(schema, req, openByID); err != nil {
		return nil, err
	}
	// Reject an accompanied departure plan with no companion note before
	// persisting, so submitted requests can always be approved (#1694).
	if err := s.validateAccompaniedCompanionNote(schema, req, openByID); err != nil {
		return nil, err
	}
	// Reject any pickup time outside a weekday_schedule field's fixed
	// AllowedTimes list (defense-in-depth behind the dropdown the public
	// form renders). No-op when no schedule field constrains its times.
	if err := s.validateConstrainedSchedules(schema, req, openByID); err != nil {
		return nil, err
	}

	// Defense-in-depth: drop answers for fields the parent couldn't see
	// (hidden by a show-if condition) and any keys not declared in the
	// schema before persisting. A stale or manipulated client must not be
	// able to smuggle a value for a hidden field — with a field Target that
	// value would otherwise be written into student data on approval.
	// Conditions are evaluated against the raw submitted answers (matching
	// the client + the required-field check above); only the persisted copy
	// is filtered.
	if schema != nil {
		byKey := buildFieldsByKey(schema)
		rawGuardian := req.CustomData
		for i := range req.Children {
			childCtx := fieldVisibilityContext{
				guardianAnswers: rawGuardian,
				childAnswers:    req.Children[i].CustomData,
				gradeLevel:      req.Children[i].TargetGradeLevel,
				offeringNames:   selectedOfferingNames(req.Children[i], openByID),
				fieldsByKey:     byKey,
			}
			req.Children[i].CustomData = sanitizeVisibleAnswers(
				schema, true, req.Children[i].CustomData, childCtx,
			)
			// Server-side care-day enforcement: strip schedule entries for
			// weekdays the child can't schedule so a stale/scripted submit
			// can't persist (and later dispatch) a pickup on a non-care day.
			pruneChildScheduleAnswers(
				schema, req.Children[i].CustomData,
				relevantCareDaysForChild(req.Children[i], openByID),
			)
		}
		req.CustomData = sanitizeVisibleAnswers(
			schema, false, rawGuardian,
			fieldVisibilityContext{guardianAnswers: rawGuardian, fieldsByKey: byKey},
		)
	}

	statusToken, err := newStatusToken()
	if err != nil {
		return nil, fmt.Errorf("submit: generate status token: %w", err)
	}
	statusExpiry := s.resolveStatusTokenExpiry(ctx)
	statusExpiresAt := time.Now().Add(statusExpiry)

	var (
		createdRequest  *enrollmentModels.Request
		createdChildren []*enrollmentModels.RequestChild
		warnings        []SubmissionWarning
	)
	txErr := s.txHandler.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
		// Serialize concurrent submits for the same (phase, guardian
		// email) so two parallel requests can't both pass the dedup
		// check and then both insert. The lock auto-releases at tx
		// commit/rollback. Phase ID is the first key, FNV-64 hash of
		// the lowercased email is the second - pg_advisory_xact_lock
		// takes two int4s OR one int8.
		emailLC := strings.ToLower(strings.TrimSpace(req.GuardianEmail))
		emailHash := fnvHash64(emailLC)
		if err := s.RequestRepo.AcquireSubmissionDedupLock(txCtx, phase.ID, emailHash); err != nil {
			return fmt.Errorf("submit: acquire dedup lock: %w", err)
		}

		var lateInvite *enrollmentModels.LateInvite
		if strings.TrimSpace(req.LateInviteToken) != "" {
			invite, inviteErr := s.findLateInviteForSubmit(txCtx, req.LateInviteToken, phase.ID, emailLC, now)
			if inviteErr != nil {
				return inviteErr
			}
			lateInvite = invite
			sourceMetadata["late_invite_id"] = invite.ID
		}

		// Identity the per-child re-enrollment gate below authorizes against.
		// emailLC is the invite's own guardian email on the late-invite path —
		// findLateInviteForSubmit rejects the submission when the two differ — so a
		// token holder cannot shop for an email that happens to be an authorized
		// guardian of the child they want to claim (#1663).
		reEnrollSubmitter := reEnrollmentSubmitterFor(submissionSource, req.GuardianAccountID, emailLC)

		// Dedup check runs inside the lock so the result is stable for
		// the rest of the tx. Different parents or different child
		// names slip past untouched; rejected/withdrawn rows are
		// ignored, so a parent can re-apply after a denial.
		dupes, dupErr := s.RequestRepo.FindActiveDuplicate(txCtx, phase.ID, req.GuardianEmail, dupKeys)
		if dupErr != nil {
			return fmt.Errorf("submit: duplicate check: %w", dupErr)
		}
		if len(dupes) > 0 {
			switch duplicatePolicy {
			case configModel.EnrollmentDuplicateHandlingBlock:
				return ErrDuplicateEnrollment
			case configModel.EnrollmentDuplicateHandlingWarn:
				warnings = []SubmissionWarning{{Code: WarningCodeDuplicateEnrollment}}
			case configModel.EnrollmentDuplicateHandlingIgnore:
			default:
				return fmt.Errorf("submit: unsupported duplicate handling %q", duplicatePolicy)
			}
		}

		if capabilities.CareOfferingsEnabled {
			overrides, capacityErr := s.applyCapacityOverflow(txCtx, phase, capacityChildren, openByID)
			if capacityErr != nil {
				return capacityErr
			}
			childStatusOverrides = overrides
		}

		var schemaID *int64
		if schema != nil {
			id := schema.ID
			schemaID = &id
		}
		request := &enrollmentModels.Request{
			SchemaID:           schemaID,
			PhaseID:            phase.ID,
			GuardianAccountID:  req.GuardianAccountID,
			GuardianFirstName:  strings.TrimSpace(req.GuardianFirstName),
			GuardianLastName:   strings.TrimSpace(req.GuardianLastName),
			GuardianEmail:      strings.ToLower(strings.TrimSpace(req.GuardianEmail)),
			GuardianPhone:      req.GuardianPhone,
			ConsentFlags:       req.ConsentFlags,
			CustomData:         req.CustomData,
			SubmissionSource:   submissionSource,
			SourceMetadata:     sourceMetadata,
			StatusToken:        statusToken,
			StatusTokenExpires: &statusExpiresAt,
			SubmittedAt:        time.Now(),
		}
		if err := s.RequestRepo.Create(txCtx, request); err != nil {
			return fmt.Errorf("submit: create request: %w", err)
		}
		if lateInvite != nil {
			if err := s.LateInviteRepo.MarkUsed(txCtx, lateInvite.ID, request.ID, time.Now()); err != nil {
				return fmt.Errorf("submit: mark late invite used: %w", err)
			}
		}
		createdRequest = request

		// Additional guardians (co-guardians). Already normalized +
		// validated above. Stored here; materialized as students_guardians
		// links on approval by the decision service.
		if s.RequestGuardianRepo != nil {
			for i, g := range req.AdditionalGuardians {
				row := &enrollmentModels.RequestGuardian{
					RequestID: request.ID,
					FirstName: g.FirstName,
					LastName:  g.LastName,
					Email:     g.Email,
					Phone:     g.Phone,
					SortOrder: i,
				}
				if err := s.RequestGuardianRepo.Create(txCtx, row); err != nil {
					return fmt.Errorf("submit: create request guardian %d: %w", i, err)
				}
			}
		}

		for i, child := range req.Children {
			status := enrollmentModels.ChildStatusSubmitted
			if override, ok := childStatusOverrides[i]; ok {
				status = override
			}
			row := &enrollmentModels.RequestChild{
				RequestID:         request.ID,
				FirstName:         strings.TrimSpace(child.FirstName),
				LastName:          strings.TrimSpace(child.LastName),
				DateOfBirth:       child.DateOfBirth,
				TargetGradeLevel:  child.TargetGradeLevel,
				TargetSchoolClass: child.TargetSchoolClass,
				CustomData:        child.CustomData,
				Status:            status,
				ActivationMode:    enrollmentModels.ChildActivationScheduled,
				SortOrder:         i,
			}
			matchedStudentID, err := s.resolveMatchedStudentID(txCtx, req.TenantID, phase, i, child)
			if err != nil {
				return err
			}
			// !AllowClosedPhase == validatePhaseEligibility ran above, so a
			// vanished match here is a race, not a fresh create (#1663).
			if err := assertExistingStudentMatchResolved(phase, matchedStudentID, !req.AllowClosedPhase, i); err != nil {
				return err
			}
			if err := s.assertGuardianMayReEnrollStudent(txCtx, reEnrollSubmitter, matchedStudentID, req.TenantID, i); err != nil {
				return err
			}
			if err := s.guardMatchedStudentUnique(txCtx, phase.ID, matchedStudentID, 0, i); err != nil {
				return err
			}
			row.MatchedStudentID = matchedStudentID
			if err := s.RequestChildRepo.Create(txCtx, row); err != nil {
				return fmt.Errorf("submit: create request child %d: %w", i, err)
			}

			for _, selection := range materializedSelections[i] {
				link := &enrollmentModels.RequestChildOffering{
					RequestChildID:        row.ID,
					CareOfferingID:        selection.OfferingID,
					SelectedDays:          selection.SelectedDays,
					ManualSelectedDays:    selection.ManualSelectedDays,
					AutomaticSelectedDays: selection.AutomaticSelectedDays,
				}
				if err := s.RequestChildOfferingRepo.Create(txCtx, link); err != nil {
					return fmt.Errorf("submit: create child-offering link: %w", err)
				}
			}
			createdChildren = append(createdChildren, row)
		}
		if len(childStatusOverrides) > 0 && !req.SuppressSubmissionEmails {
			if err := enqueueDecisionNotifications(txCtx, decisionNotificationDependencies{
				requests:   s.RequestRepo,
				settings:   s.Settings,
				outbox:     s.OutboxEnqueuer,
				schools:    s.SchoolRepo,
				parentsURL: s.ParentsURL,
			}, request, createdChildren, phase, childIDsForStatus(createdChildren, enrollmentModels.ChildStatusWaitlisted)); err != nil {
				return fmt.Errorf("submit: notify capacity decisions: %w", err)
			}
		}
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	statusURL := enrollmentStatusURL(s.ParentsURL, statusToken)
	if !req.SuppressSubmissionEmails {
		s.enqueueSubmissionEmails(ctx, req.TenantID, createdRequest, createdChildren, statusURL)
	}

	s.Logger.Info("enrollment request submitted",
		slog.Int64("request_id", createdRequest.ID),
		slog.Int64("tenant_id", req.TenantID),
		slog.Int("children", len(createdChildren)))

	return &SubmitResult{
		Request:   createdRequest,
		Children:  createdChildren,
		StatusURL: statusURL,
		Warnings:  warnings,
	}, nil
}

func childrenWithMaterializedOfferingSelections(
	children []SubmitChild,
	materialized [][]materializedOfferingSelection,
) []SubmitChild {
	withMaterialized := make([]SubmitChild, len(children))
	for i, child := range children {
		withMaterialized[i] = child
		if i >= len(materialized) {
			continue
		}
		ids := make([]int64, 0, len(materialized[i]))
		for _, selection := range materialized[i] {
			ids = append(ids, selection.OfferingID)
		}
		withMaterialized[i].OfferingIDs = ids
	}
	return withMaterialized
}

func (s *requestService) CreateLateInvite(ctx context.Context, input CreateLateInviteInput) (*CreateLateInviteResult, error) {
	if s.LateInviteRepo == nil {
		return nil, fmt.Errorf("late invite repository is not configured")
	}
	if input.PhaseID <= 0 {
		return nil, fmt.Errorf("%w: phase_id is required", ErrInvalidSubmission)
	}
	if input.CreatedBy <= 0 {
		return nil, fmt.Errorf("%w: created_by is required", ErrInvalidSubmission)
	}
	email, err := normalizeGuardianEmail(input.GuardianEmail)
	if err != nil {
		return nil, err
	}
	phase, err := s.loadPhaseForEditableRequest(ctx, input.PhaseID)
	if err != nil {
		return nil, err
	}
	token, err := newStatusToken()
	if err != nil {
		return nil, fmt.Errorf("late invite: generate token: %w", err)
	}
	expiresAt := time.Now().Add(14 * 24 * time.Hour)
	if input.ExpiresAt != nil {
		expiresAt = *input.ExpiresAt
	}
	if !expiresAt.After(time.Now()) {
		return nil, fmt.Errorf("%w: expires_at must be in the future", ErrInvalidSubmission)
	}
	firstName := strutil.TrimToNil(input.GuardianFirstName)
	lastName := strutil.TrimToNil(input.GuardianLastName)
	reason := strutil.TrimToNil(input.Reason)
	invite := &enrollmentModels.LateInvite{
		PhaseID:           phase.ID,
		TokenHash:         lateInviteTokenHash(token),
		GuardianEmail:     email,
		GuardianFirstName: firstName,
		GuardianLastName:  lastName,
		ExpiresAt:         expiresAt,
		CreatedBy:         input.CreatedBy,
		Reason:            reason,
	}
	if err := s.LateInviteRepo.Create(ctx, invite); err != nil {
		return nil, err
	}
	return &CreateLateInviteResult{Invite: invite, Token: token}, nil
}

func (s *requestService) findLateInviteForSubmit(ctx context.Context, token string, phaseID int64, guardianEmail string, now time.Time) (*enrollmentModels.LateInvite, error) {
	if s.LateInviteRepo == nil {
		return nil, fmt.Errorf("%w: late invite support is not configured", ErrLateInviteInvalid)
	}
	invite, err := s.LateInviteRepo.FindUsableByTokenHashForUpdate(ctx, lateInviteTokenHash(token), phaseID, now)
	if err != nil {
		// Same split as the form-load gate: only a genuinely unusable token is
		// an invalid invite. A lookup failure must not tell the parent their
		// invite is invalid — surface it as a server error (#1663).
		if errors.Is(err, enrollmentModels.ErrLateInviteNotFound) {
			return nil, ErrLateInviteInvalid
		}
		return nil, fmt.Errorf("submit: resolve late invite: %w", err)
	}
	if strings.ToLower(strings.TrimSpace(invite.GuardianEmail)) != guardianEmail {
		return nil, ErrLateInviteInvalid
	}
	return invite, nil
}

func (s *requestService) validateSubmission(ctx context.Context, req SubmitRequest, legalBlocks []LegalBlock, capabilities FormCapabilities) error {
	if !s.isEnrollmentEnabled(ctx) {
		return ErrEnrollmentDisabled
	}
	// Phase-window enforcement happens after we load the phase row in
	// Submit(). The here check used to enforce a tenant-wide window -
	// that setting is gone in the phase model.
	if req.PhaseID <= 0 {
		return fmt.Errorf("%w: phase_id is required", ErrInvalidSubmission)
	}
	if strings.TrimSpace(req.GuardianFirstName) == "" {
		return fmt.Errorf("%w: guardian first name is required", ErrInvalidSubmission)
	}
	if strings.TrimSpace(req.GuardianLastName) == "" {
		return fmt.Errorf("%w: guardian last name is required", ErrInvalidSubmission)
	}
	emailAddr := strings.TrimSpace(req.GuardianEmail)
	if emailAddr == "" {
		return fmt.Errorf("%w: guardian email is required", ErrInvalidSubmission)
	}
	// Validate against the shared canonical pattern (same rule student
	// creation enforces at approval) so a submittable email can never get
	// stuck at the approval step. mail.ParseAddress was too lenient here
	// (it accepts e.g. "a@b" / "test@localhost" that approval rejects).
	if err := users.ValidateOptionalEmail(emailAddr); err != nil {
		return ErrInvalidGuardianEmail
	}
	// Guardian phone is optional, but when present it must match the same
	// canonical format student creation enforces on approval. Validating
	// here stops an invalid value (e.g. "12345") from being stored and
	// then permanently blocking approval with a 500.
	if req.GuardianPhone != nil {
		if err := users.ValidateOptionalPhone(*req.GuardianPhone); err != nil {
			return ErrInvalidGuardianPhone
		}
	}
	for _, key := range requiredConsentKeys(legalBlocks) {
		accepted, ok := req.ConsentFlags[key].(bool)
		if !ok || !accepted {
			return fmt.Errorf("%w: consent %s is required", ErrInvalidSubmission, key)
		}
	}
	if len(req.Children) == 0 {
		return fmt.Errorf("%w: at least one child is required", ErrInvalidSubmission)
	}
	gradeMax, err := s.resolveGradeMax(ctx)
	if err != nil {
		return err
	}
	for i, child := range req.Children {
		if strings.TrimSpace(child.FirstName) == "" || strings.TrimSpace(child.LastName) == "" {
			return fmt.Errorf("%w: child %d missing name", ErrInvalidSubmission, i)
		}
		if child.DateOfBirth.IsZero() {
			return fmt.Errorf("%w: child %d missing date_of_birth", ErrInvalidSubmission, i)
		}
		if capabilities.CollectGradeLevel && child.TargetGradeLevel == nil {
			return fmt.Errorf("%w: child %d missing target_grade_level", ErrInvalidSubmission, i)
		}
		if capabilities.CollectGradeLevel && (*child.TargetGradeLevel < 1 || int(*child.TargetGradeLevel) > gradeMax) {
			return fmt.Errorf("%w: child %d grade out of range 1..%d", ErrInvalidSubmission, i, gradeMax)
		}
	}
	return nil
}

// normalizeAdditionalGuardians cleans req.AdditionalGuardians in place:
// trims every field, drops fully-blank cards the parent added but never
// filled, returns a typed error for a half-filled card (one name missing)
// or a malformed email/phone, and dedups co-guardians against the primary
// guardian and each other. Email/phone are optional per co-guardian; only
// first + last name are required. The primary guardian is the source of
// truth — a co-guardian that duplicates it is dropped so approval never
// creates two students_guardians links to the same profile.
func normalizeAdditionalGuardians(req *SubmitRequest) error {
	if len(req.AdditionalGuardians) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(req.AdditionalGuardians)+1)
	primaryPhone := ""
	if req.GuardianPhone != nil {
		primaryPhone = strings.TrimSpace(*req.GuardianPhone)
	}
	if primary := guardianDedupKey(req.GuardianEmail, primaryPhone, req.GuardianFirstName, req.GuardianLastName); primary != "" {
		seen[primary] = struct{}{}
	}
	cleaned := make([]SubmitGuardian, 0, len(req.AdditionalGuardians))
	for i, g := range req.AdditionalGuardians {
		first := strings.TrimSpace(g.FirstName)
		last := strings.TrimSpace(g.LastName)
		email := ""
		if g.Email != nil {
			email = strings.ToLower(strings.TrimSpace(*g.Email))
		}
		phone := ""
		if g.Phone != nil {
			phone = strings.TrimSpace(*g.Phone)
		}

		// Fully blank card: the parent added a row but never filled it.
		if first == "" && last == "" && email == "" && phone == "" {
			continue
		}
		// Name is the only hard requirement for a co-guardian.
		if first == "" || last == "" {
			return fmt.Errorf("%w: additional guardian %d missing name", ErrInvalidSubmission, i)
		}
		if email != "" {
			if err := users.ValidateOptionalEmail(email); err != nil {
				return ErrInvalidGuardianEmail
			}
		}
		if phone != "" {
			if err := users.ValidateOptionalPhone(phone); err != nil {
				return ErrInvalidGuardianPhone
			}
		}

		key := guardianDedupKey(email, phone, first, last)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}

		out := SubmitGuardian{FirstName: first, LastName: last}
		if email != "" {
			e := email
			out.Email = &e
		}
		if phone != "" {
			p := phone
			out.Phone = &p
		}
		cleaned = append(cleaned, out)
	}
	req.AdditionalGuardians = cleaned
	return nil
}

// guardianDedupKey builds a stable dedup key for a guardian: prefer the
// lowercased email when present (the identity anchor), else fall back to
// the case-folded full name PLUS the phone number. Including the phone in
// the email-less key keeps two same-name co-guardians with different
// phone numbers (e.g. relatives sharing a surname) as distinct people
// instead of silently collapsing the later row into the first. Returns ""
// only when all inputs are empty.
func guardianDedupKey(email, phone, first, last string) string {
	if e := strings.ToLower(strings.TrimSpace(email)); e != "" {
		return "e:" + e
	}
	f := strings.ToLower(strings.TrimSpace(first))
	l := strings.ToLower(strings.TrimSpace(last))
	p := strings.TrimSpace(phone)
	if f == "" && l == "" && p == "" {
		return ""
	}
	return "n:" + f + "|" + l + "|p:" + p
}

// validateOfferingSelections cross-checks every offering id against the
// live open-window catalog. Defense-in-depth against stale clients.
func validateOfferingSelections(children []SubmitChild, openByID map[int64]*enrollmentModels.CareOffering) error {
	for _, child := range children {
		for _, offeringID := range child.OfferingIDs {
			if _, ok := openByID[offeringID]; !ok {
				return ErrCareOfferingClosed
			}
		}
	}
	return nil
}

func materializeAndValidateChildrenOfferingSelections(
	children []SubmitChild,
	openByID map[int64]*enrollmentModels.CareOffering,
	selectionMode string,
) ([][]materializedOfferingSelection, error) {
	out := make([][]materializedOfferingSelection, len(children))
	for i := range children {
		availableByID, err := availableCareOfferingsForGrade(openByID, children[i].TargetGradeLevel)
		if err != nil {
			return nil, fmt.Errorf("child %d: %w", i, err)
		}
		if err := validateOfferingSelectionsForChild(children[i], openByID, availableByID); err != nil {
			return nil, fmt.Errorf("child %d: %w", i, err)
		}
		manualChild := cloneSubmitChildrenOfferingSelections([]SubmitChild{children[i]})[0]
		selections, err := materializeOfferingSelections(children[i], availableByID)
		if err != nil {
			return nil, fmt.Errorf("child %d: %w", i, err)
		}
		children[i].OfferingIDs, children[i].OfferingDays = selectionPayload(selections, availableByID)
		if err := validateOfferingGroupRules([]SubmitChild{children[i]}, availableByID); err != nil {
			return nil, err
		}
		if err := validateRequiredOfferings([]SubmitChild{children[i]}, availableByID); err != nil {
			return nil, err
		}
		if len(openByID) == 0 || hasChoosableCareOffering(availableByID) {
			if err := validateCareOfferingSelectionMode([]SubmitChild{manualChild}, availableByID, selectionMode); err != nil {
				return nil, err
			}
		}
		out[i] = selections
	}
	return out, nil
}

func availableCareOfferingsForGrade(catalog map[int64]*enrollmentModels.CareOffering, grade *int16) (map[int64]*enrollmentModels.CareOffering, error) {
	available := make(map[int64]*enrollmentModels.CareOffering, len(catalog))
	for id, offering := range catalog {
		matches, err := offering.AvailabilityRule.MatchesGradeLevel(grade)
		if err != nil {
			return nil, fmt.Errorf("offering %d has invalid availability rule: %w", id, err)
		}
		if matches {
			available[id] = offering
		}
	}
	return available, nil
}

func validateOfferingSelectionsForChild(child SubmitChild, catalog, available map[int64]*enrollmentModels.CareOffering) error {
	check := func(id int64) error {
		if _, ok := catalog[id]; !ok {
			return ErrCareOfferingClosed
		}
		if _, ok := available[id]; !ok {
			return ErrCareOfferingUnavailable
		}
		return nil
	}
	for _, id := range child.OfferingIDs {
		if err := check(id); err != nil {
			return err
		}
	}
	for _, row := range child.OfferingDays {
		if err := check(row.OfferingID); err != nil {
			return err
		}
	}
	return nil
}

func cloneSubmitChildrenOfferingSelections(children []SubmitChild) []SubmitChild {
	out := make([]SubmitChild, len(children))
	for i, child := range children {
		out[i] = child
		out[i].OfferingIDs = append([]int64(nil), child.OfferingIDs...)
		if len(child.OfferingDays) > 0 {
			out[i].OfferingDays = make([]SubmitOfferingDays, len(child.OfferingDays))
			for j, row := range child.OfferingDays {
				out[i].OfferingDays[j] = SubmitOfferingDays{
					OfferingID:   row.OfferingID,
					SelectedDays: copyDays(row.SelectedDays),
				}
			}
		}
	}
	return out
}

func materializeOfferingSelections(child SubmitChild, openByID map[int64]*enrollmentModels.CareOffering) ([]materializedOfferingSelection, error) {
	daysByOffering := make(map[int64][]string, len(child.OfferingDays))
	for _, row := range child.OfferingDays {
		daysByOffering[row.OfferingID] = row.SelectedDays
	}

	selectionByID := make(map[int64]*materializedOfferingSelection, len(child.OfferingIDs))
	for _, offeringID := range child.OfferingIDs {
		offering, ok := openByID[offeringID]
		if !ok {
			return nil, ErrCareOfferingClosed
		}
		manual, err := resolveManualSelectedDays(offering, daysByOffering[offeringID])
		if err != nil {
			return nil, fmt.Errorf("offering %d: %w", offeringID, err)
		}
		selectionByID[offeringID] = &materializedOfferingSelection{
			OfferingID:            offeringID,
			SelectedDays:          copyDays(manual),
			ManualSelectedDays:    copyDays(manual),
			AutomaticSelectedDays: nil,
		}
	}

	targets := sortedCareOfferings(openByID)
	for _, target := range targets {
		if careOfferingCanAutoAddDays(target) && autoAddAppliesToGrade(child.TargetGradeLevel, target.AutoAddGradeLevels) && target.DaysOfWeekMode != enrollmentModels.DaysOfWeekModeParentChoice {
			return nil, fmt.Errorf("offering %d cannot be automatically added because it does not allow day selection", target.ID)
		}
	}

	changed := true
	for changed {
		changed = false
		for _, target := range targets {
			if !careOfferingCanAutoAddDays(target) || !autoAddAppliesToGrade(child.TargetGradeLevel, target.AutoAddGradeLevels) {
				continue
			}
			autoDays := unionDaysInOfferingOrder(
				target.AvailableDays,
				autoDaysForTarget(target, target.AutoAddTriggerOfferingIDs, selectionByID, openByID),
				autoLunchDaysForTarget(target, selectionByID, openByID),
			)
			if len(autoDays) == 0 {
				continue
			}
			selection := selectionByID[target.ID]
			if selection == nil {
				selection = &materializedOfferingSelection{OfferingID: target.ID}
				selectionByID[target.ID] = selection
				changed = true
			}
			if !slices.Equal(selection.AutomaticSelectedDays, autoDays) {
				selection.AutomaticSelectedDays = autoDays
				changed = true
			}
			selectedDays := unionDaysInOfferingOrder(target.AvailableDays, selection.ManualSelectedDays, selection.AutomaticSelectedDays)
			if !slices.Equal(selection.SelectedDays, selectedDays) {
				selection.SelectedDays = selectedDays
				changed = true
			}
		}
	}

	for offeringID, selection := range selectionByID {
		offering := openByID[offeringID]
		if offering == nil || offering.DaysOfWeekMode != enrollmentModels.DaysOfWeekModeParentChoice {
			continue
		}
		if len(selection.SelectedDays) == 0 {
			return nil, fmt.Errorf("offering %d: %w", offeringID, errParentChoiceOfferingMissingDays)
		}
	}

	out := make([]materializedOfferingSelection, 0, len(selectionByID))
	for _, selection := range selectionByID {
		out = append(out, *selection)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left := openByID[out[i].OfferingID]
		right := openByID[out[j].OfferingID]
		if left == nil || right == nil {
			return out[i].OfferingID < out[j].OfferingID
		}
		if left.SortOrder == right.SortOrder {
			return left.ID < right.ID
		}
		return left.SortOrder < right.SortOrder
	})
	return out, nil
}

func sortedCareOfferings(openByID map[int64]*enrollmentModels.CareOffering) []*enrollmentModels.CareOffering {
	out := make([]*enrollmentModels.CareOffering, 0, len(openByID))
	for _, offering := range openByID {
		if offering != nil {
			out = append(out, offering)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SortOrder == out[j].SortOrder {
			return out[i].ID < out[j].ID
		}
		return out[i].SortOrder < out[j].SortOrder
	})
	return out
}

func selectionPayload(selections []materializedOfferingSelection, openByID map[int64]*enrollmentModels.CareOffering) ([]int64, []SubmitOfferingDays) {
	ids := make([]int64, 0, len(selections))
	dayRows := make([]SubmitOfferingDays, 0, len(selections))
	for _, selection := range selections {
		ids = append(ids, selection.OfferingID)
		if offering := openByID[selection.OfferingID]; offering != nil && offering.DaysOfWeekMode == enrollmentModels.DaysOfWeekModeParentChoice {
			dayRows = append(dayRows, SubmitOfferingDays{
				OfferingID:   selection.OfferingID,
				SelectedDays: copyDays(selection.SelectedDays),
			})
		}
	}
	return ids, dayRows
}

// preservedOfferingSelections copies the persisted links onto replacement
// child rows while the catalog is disabled. The parent cannot see or change
// these values, but re-enabling the setting restores them intact.
func preservedOfferingSelections(
	existingChildren []*enrollmentModels.RequestChild,
	incoming []SubmitChild,
	links []*enrollmentModels.RequestChildOffering,
) [][]materializedOfferingSelection {
	byChild := make(map[int64][]materializedOfferingSelection, len(existingChildren))
	for _, link := range links {
		byChild[link.RequestChildID] = append(byChild[link.RequestChildID], materializedOfferingSelection{
			OfferingID:            link.CareOfferingID,
			SelectedDays:          copyDays(link.SelectedDays),
			ManualSelectedDays:    copyDays(link.ManualSelectedDays),
			AutomaticSelectedDays: copyDays(link.AutomaticSelectedDays),
		})
	}
	result := make([][]materializedOfferingSelection, len(incoming))
	for i, child := range matchExistingChildrenBySubmittedIdentity(existingChildren, incoming) {
		if child != nil {
			result[i] = byChild[child.ID]
		}
	}
	return result
}

func autoAddAppliesToGrade(grade *int16, levels []int) bool {
	if len(levels) == 0 {
		return true
	}
	if grade == nil {
		return false
	}
	for _, level := range levels {
		if int(*grade) == level {
			return true
		}
	}
	return false
}

func careOfferingCanAutoAddDays(offering *enrollmentModels.CareOffering) bool {
	if offering == nil {
		return false
	}
	return len(offering.AutoAddTriggerOfferingIDs) > 0 || isRequiredLunchOffering(offering)
}

func isRequiredLunchOffering(offering *enrollmentModels.CareOffering) bool {
	return offering != nil &&
		offering.IsRequired &&
		offering.IncludesLunch &&
		offering.DaysOfWeekMode == enrollmentModels.DaysOfWeekModeParentChoice
}

func autoDaysForTarget(
	target *enrollmentModels.CareOffering,
	triggerIDs []int64,
	selectionByID map[int64]*materializedOfferingSelection,
	openByID map[int64]*enrollmentModels.CareOffering,
) []string {
	selected := make(map[string]bool, len(target.AvailableDays))
	targetDays := make(map[string]bool, len(target.AvailableDays))
	for _, day := range target.AvailableDays {
		targetDays[day] = true
	}
	for _, triggerID := range triggerIDs {
		triggerSelection := selectionByID[triggerID]
		if triggerSelection == nil {
			continue
		}
		trigger := openByID[triggerID]
		if trigger == nil {
			continue
		}
		triggerDays := triggerSelection.SelectedDays
		if trigger.DaysOfWeekMode == enrollmentModels.DaysOfWeekModeFixed {
			triggerDays = trigger.AvailableDays
		}
		for _, day := range triggerDays {
			if targetDays[day] {
				selected[day] = true
			}
		}
	}
	return daysFromSetInOrder(target.AvailableDays, selected)
}

func autoLunchDaysForTarget(
	target *enrollmentModels.CareOffering,
	selectionByID map[int64]*materializedOfferingSelection,
	openByID map[int64]*enrollmentModels.CareOffering,
) []string {
	if !isRequiredLunchOffering(target) {
		return nil
	}
	selected := make(map[string]bool, len(target.AvailableDays))
	targetDays := make(map[string]bool, len(target.AvailableDays))
	for _, day := range target.AvailableDays {
		targetDays[day] = true
	}
	for offeringID, selection := range selectionByID {
		if offeringID == target.ID || selection == nil {
			continue
		}
		offering := openByID[offeringID]
		if offering == nil || !offering.CountsAsCare {
			continue
		}
		days := selection.SelectedDays
		if offering.DaysOfWeekMode == enrollmentModels.DaysOfWeekModeFixed {
			days = offering.AvailableDays
		}
		for _, day := range days {
			if targetDays[day] {
				selected[day] = true
			}
		}
	}
	return daysFromSetInOrder(target.AvailableDays, selected)
}

func unionDaysInOfferingOrder(available []string, groups ...[]string) []string {
	seen := make(map[string]bool, len(available))
	for _, group := range groups {
		for _, day := range group {
			seen[day] = true
		}
	}
	return daysFromSetInOrder(available, seen)
}

func daysFromSetInOrder(order []string, selected map[string]bool) []string {
	out := make([]string, 0, len(selected))
	for _, day := range order {
		if selected[day] {
			out = append(out, day)
		}
	}
	return out
}

func copyDays(days []string) []string {
	if len(days) == 0 {
		return nil
	}
	out := make([]string, len(days))
	copy(out, days)
	return out
}

// validateRequiredOfferings enforces that every offering flagged
// is_required in the phase's open catalog is selected by every child.
// The day-level requirement for parent_choice offerings is already
// enforced at insert time by the manual-selected-days resolution, so this only checks
// presence in child.OfferingIDs.
func validateRequiredOfferings(children []SubmitChild, openByID map[int64]*enrollmentModels.CareOffering) error {
	requiredIDs := make([]int64, 0)
	for id, offering := range openByID {
		if offering.IsRequired {
			requiredIDs = append(requiredIDs, id)
		}
	}
	if len(requiredIDs) == 0 {
		return nil
	}
	for i, child := range children {
		selected := make(map[int64]bool, len(child.OfferingIDs))
		for _, id := range child.OfferingIDs {
			selected[id] = true
		}
		for _, requiredID := range requiredIDs {
			if !selected[requiredID] {
				return fmt.Errorf("%w: child %d offering %d", ErrRequiredCareOfferingMissing, i, requiredID)
			}
		}
	}
	return nil
}

// validateCareOfferingSelectionMode enforces the phase's selection mode
// over the *choosable* (non-required) offerings only. Required offerings
// are always-on and orthogonal to the mode: they are forced by
// validateRequiredOfferings and must not count toward "at least one" /
// "exactly one". Counting them would make exactly_one impossible whenever
// the phase also carries a required base offering (e.g. a mandatory care
// package plus a choose-one-time-slot rule): the contradiction this
// function is designed to avoid.
func validateCareOfferingSelectionMode(children []SubmitChild, openByID map[int64]*enrollmentModels.CareOffering, mode string) error {
	if mode == "" || mode == enrollmentModels.PhaseCareOfferingSelectionOptional {
		return nil
	}
	if mode != enrollmentModels.PhaseCareOfferingSelectionAtLeastOne &&
		mode != enrollmentModels.PhaseCareOfferingSelectionExactlyOne {
		return fmt.Errorf("%w: invalid care offering selection mode %q", ErrInvalidSubmission, mode)
	}

	for i, child := range children {
		selected := make(map[int64]bool, len(child.OfferingIDs))
		for _, id := range child.OfferingIDs {
			selected[id] = true
		}
		for _, dayPick := range child.OfferingDays {
			if !selected[dayPick.OfferingID] {
				return fmt.Errorf("%w: child %d offering %d has days but is not selected", ErrInvalidSubmission, i, dayPick.OfferingID)
			}
		}
		// Count only the choosable (non-required) selected offerings. An
		// offering absent from openByID is rejected earlier by
		// validateOfferingSelections, so treat unknown ids as choosable.
		choosableCount := 0
		for _, id := range child.OfferingIDs {
			if o, ok := openByID[id]; !ok || !o.IsRequired {
				choosableCount++
			}
		}
		switch mode {
		case enrollmentModels.PhaseCareOfferingSelectionAtLeastOne:
			if choosableCount == 0 {
				return fmt.Errorf("%w: child %d", ErrCareOfferingMissing, i)
			}
		case enrollmentModels.PhaseCareOfferingSelectionExactlyOne:
			if choosableCount != 1 {
				return fmt.Errorf("%w: child %d", ErrCareOfferingExactlyOneRequired, i)
			}
		}
	}
	return nil
}

func hasChoosableCareOffering(catalog map[int64]*enrollmentModels.CareOffering) bool {
	for _, offering := range catalog {
		if !offering.IsRequired {
			return true
		}
	}
	return false
}

// GetByStatusToken loads a request + its children for the public
// status page. Caller is responsible for setting an admin-tx context
// (token-only auth - RLS would block unprivileged SELECTs).
func (s *requestService) GetByStatusToken(ctx context.Context, token string) (*enrollmentModels.Request, []*enrollmentModels.RequestChild, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, nil, ErrRequestNotFound
	}
	req, err := s.RequestRepo.FindByStatusToken(ctx, token)
	if err != nil {
		return nil, nil, ErrRequestNotFound
	}
	if req.StatusTokenExpires != nil && time.Now().After(*req.StatusTokenExpires) {
		return nil, nil, ErrRequestNotFound
	}

	tenantCtx := tenant.WithTenantID(ctx, req.GetTenantID())
	children, err := s.RequestChildRepo.ListByRequestID(tenantCtx, req.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("status: list children: %w", err)
	}

	// status_reason is admin-internal free text. It may only reach a
	// parent when the request's phase opts in via
	// show_status_reason_to_parent. Strip it otherwise so the public
	// status page (and the parents-portal status view that shares this
	// endpoint) never expose an internal rejection/waitlist note. The
	// decision service applies the same gate to the parent email.
	if !s.statusReasonVisibleToParent(tenantCtx, req.PhaseID) {
		for _, c := range children {
			c.StatusReason = nil
		}
	}
	return req, children, nil
}

// EditModeForStatus returns the edit path the public status page may offer.
// It intentionally mirrors GetEditDraft's gates without loading the full draft.
func (s *requestService) EditModeForStatus(ctx context.Context, req *enrollmentModels.Request, children []*enrollmentModels.RequestChild) (string, error) {
	if req == nil {
		return EditModeNone, nil
	}
	tenantCtx := tenant.WithTenantID(ctx, req.GetTenantID())
	mode := editModeForChildren(children)
	switch mode {
	case EditModeDirectEdit:
		if err := s.ensureRequestEditable(tenantCtx, req, children); err != nil {
			return EditModeNone, nil
		}
		if _, err := s.loadPhaseForEditableRequest(tenantCtx, req.PhaseID); err != nil {
			if errors.Is(err, ErrEnrollmentDisabled) || errors.Is(err, ErrInvalidSubmission) {
				return EditModeNone, nil
			}
			return EditModeNone, err
		}
		return EditModeDirectEdit, nil
	case EditModeChangeRequest:
		if err := s.ensureChangeRequestDraftAvailable(tenantCtx, req, children); err != nil {
			return EditModeNone, nil
		}
		phase, err := s.loadPhaseForEditableRequest(tenantCtx, req.PhaseID)
		if err != nil {
			if errors.Is(err, ErrEnrollmentDisabled) || errors.Is(err, ErrInvalidSubmission) {
				return EditModeNone, nil
			}
			return EditModeNone, err
		}
		if !IsEnrollmentWindowOpen(phase, time.Now()) {
			return EditModeNone, nil
		}
		return EditModeChangeRequest, nil
	default:
		return EditModeNone, nil
	}
}

// GuardiansByStatusToken loads the additional guardians (co-guardians)
// behind a public status token. Same token/expiry gate as
// GetByStatusToken; caller sets an admin-tx context (token-only auth).
func (s *requestService) GuardiansByStatusToken(ctx context.Context, token string) ([]*enrollmentModels.RequestGuardian, error) {
	if s.RequestGuardianRepo == nil {
		return nil, nil
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrRequestNotFound
	}
	req, err := s.RequestRepo.FindByStatusToken(ctx, token)
	if err != nil {
		return nil, ErrRequestNotFound
	}
	if req.StatusTokenExpires != nil && time.Now().After(*req.StatusTokenExpires) {
		return nil, ErrRequestNotFound
	}
	tenantCtx := tenant.WithTenantID(ctx, req.GetTenantID())
	return s.RequestGuardianRepo.ListByRequestID(tenantCtx, req.ID)
}

// statusReasonVisibleToParent reports whether the given phase allows a
// per-child status_reason to be surfaced to the parent. Fail-closed: if
// the phase can't be loaded it returns false, so an internal note is
// redacted rather than risk leaking when the setting can't be confirmed.
func (s *requestService) statusReasonVisibleToParent(ctx context.Context, phaseID int64) bool {
	phase, err := s.PhaseRepo.FindByID(ctx, phaseID)
	if err != nil {
		s.Logger.Warn("status: phase load failed; redacting status reason",
			slog.Int64("phase_id", phaseID),
			slog.String("error", err.Error()),
		)
		return false
	}
	return phase.ShowStatusReasonToParent
}

// GetEditDraft loads the full persisted request needed to reopen the public
// enrollment form. It is token-gated like GetByStatusToken, but also enforces
// the edit window up front so a locked request never leaks a stale editable
// draft to the client.
func (s *requestService) GetEditDraft(ctx context.Context, token string) (*EditDraft, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrRequestNotFound
	}

	var (
		req       *enrollmentModels.Request
		children  []*enrollmentModels.RequestChild
		guardians []*enrollmentModels.RequestGuardian
		links     []*enrollmentModels.RequestChildOffering
		school    *platformModels.School
	)
	if err := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		loadedReq, err := s.RequestRepo.FindByStatusToken(adminCtx, token)
		if err != nil {
			return ErrRequestNotFound
		}
		if loadedReq.StatusTokenExpires != nil && time.Now().After(*loadedReq.StatusTokenExpires) {
			return ErrRequestNotFound
		}
		req = loadedReq

		tenantCtx := tenant.WithTenantID(adminCtx, req.GetTenantID())
		loadedChildren, err := s.RequestChildRepo.ListByRequestID(tenantCtx, req.ID)
		if err != nil {
			return fmt.Errorf("edit draft: list children: %w", err)
		}
		children = loadedChildren
		if s.RequestGuardianRepo != nil {
			loadedGuardians, err := s.RequestGuardianRepo.ListByRequestID(tenantCtx, req.ID)
			if err != nil {
				return fmt.Errorf("edit draft: list guardians: %w", err)
			}
			guardians = loadedGuardians
		}
		childIDs := make([]int64, 0, len(children))
		for _, c := range children {
			childIDs = append(childIDs, c.ID)
		}
		loadedLinks, err := s.RequestChildOfferingRepo.ListByRequestChildIDs(tenantCtx, childIDs)
		if err != nil {
			return fmt.Errorf("edit draft: list child offerings: %w", err)
		}
		links = loadedLinks
		if s.SchoolRepo != nil {
			loadedSchool, err := s.SchoolRepo.FindByID(adminCtx, req.GetTenantID())
			if err != nil {
				return fmt.Errorf("edit draft: load school: %w", err)
			}
			school = loadedSchool
		}
		return nil
	}); err != nil {
		return nil, err
	}

	linksByChild := make(map[int64][]*enrollmentModels.RequestChildOffering, len(children))
	for _, link := range links {
		linksByChild[link.RequestChildID] = append(linksByChild[link.RequestChildID], link)
	}

	var (
		phase         *enrollmentModels.Phase
		schema        *enrollmentModels.FormSchema
		openOfferings []*enrollmentModels.CareOffering
		legalTexts    LegalTexts
		editMode      string
		capabilities  FormCapabilities
		gradeLevelMax int
	)
	if err := tenant.WithTenantTx(ctx, s.DB, req.GetTenantID(), func(txCtx context.Context, _ bun.Tx) error {
		resolvedCapabilities, capabilityErr := s.FormCapabilities(txCtx)
		if capabilityErr != nil {
			return fmt.Errorf("edit draft: resolve form capabilities: %w", capabilityErr)
		}
		resolvedGradeLevelMax, gradeLevelErr := s.resolveGradeMax(txCtx)
		if gradeLevelErr != nil {
			return fmt.Errorf("edit draft: %w", gradeLevelErr)
		}
		gradeLevelMax = resolvedGradeLevelMax
		capabilities = resolvedCapabilities
		editMode = editModeForChildren(children)
		if editMode == EditModeDirectEdit {
			if err := s.ensureRequestEditable(txCtx, req, children); err != nil {
				return err
			}
		} else {
			if err := s.ensureChangeRequestDraftAvailable(txCtx, req, children); err != nil {
				return err
			}
		}
		loadedPhase, err := s.loadPhaseForEditableRequest(txCtx, req.PhaseID)
		if err != nil {
			return err
		}
		if editMode == EditModeChangeRequest && !IsEnrollmentWindowOpen(loadedPhase, time.Now()) {
			return ErrEnrollmentWindowClosed
		}
		phase = loadedPhase
		// Reopening the form is a self-service load like the public one, so it
		// must present exactly the world the save will accept. Both edit paths
		// (ReplaceEditable, prepareProposed) re-run the per-child eligibility
		// gates under this very predicate, so an enforced draft gets the
		// offered classes narrowed to the eligible subset — otherwise the
		// parent picks an available-but-ineligible class and the save fails
		// with class_not_eligible after the whole form was filled in. An exempt
		// draft (trusted source / rollover-generated) bypasses those gates, so
		// it keeps the full offered list and drops the grade restriction the
		// form would otherwise narrow its grade select to. Mirrors the public
		// form gate loadEditablePhaseWithLateInvite (#1663).
		if isTrustedEnrollmentSource(req.SubmissionSource) || hasRolloverGeneratedChild(children) {
			clearGradeRestrictionForEligibilityExemptForm(phase)
		} else {
			narrowOfferedClassesToEligibleForForm(phase)
		}
		loadedSchema, err := s.schemaForEditableRequest(txCtx, req, phase)
		if err != nil {
			return err
		}
		schema = loadedSchema
		list := []*enrollmentModels.CareOffering{}
		if capabilities.CareOfferingsEnabled {
			list, err = s.CareOfferingRepo.ListActiveByPhase(txCtx, phase.ID)
			if err != nil {
				return fmt.Errorf("edit draft: list active offerings: %w", err)
			}
		}
		openOfferings = list
		openByID := make(map[int64]*enrollmentModels.CareOffering, len(openOfferings))
		for _, offering := range openOfferings {
			openByID[offering.ID] = offering
		}
		missingLinkedIDs := make(map[int64]struct{})
		for _, link := range links {
			if !capabilities.CareOfferingsEnabled {
				continue
			}
			if _, ok := openByID[link.CareOfferingID]; ok {
				continue
			}
			if editMode != EditModeChangeRequest {
				return ErrEditNotAllowed
			}
			missingLinkedIDs[link.CareOfferingID] = struct{}{}
		}
		if len(missingLinkedIDs) > 0 {
			ids := slices.Collect(maps.Keys(missingLinkedIDs))
			currentOfferings, err := s.CareOfferingRepo.ListByIDs(txCtx, ids)
			if err != nil {
				return fmt.Errorf("edit draft: list current inactive offerings: %w", err)
			}
			for _, offering := range currentOfferings {
				if offering == nil || offering.PhaseID != phase.ID {
					continue
				}
				openByID[offering.ID] = offering
				openOfferings = append(openOfferings, offering)
				delete(missingLinkedIDs, offering.ID)
			}
			if len(missingLinkedIDs) > 0 {
				return ErrEditNotAllowed
			}
		}
		capabilities = EffectiveFormCapabilities(capabilities, openOfferings)
		texts, err := s.legalTextsForEditableRequest(txCtx, schema)
		if err != nil {
			return err
		}
		legalTexts = texts
		return nil
	}); err != nil {
		return nil, err
	}

	return &EditDraft{
		Request:              req,
		Children:             children,
		Guardians:            guardians,
		OfferingsByChild:     linksByChild,
		Phase:                phase,
		School:               school,
		Schema:               schema,
		OpenOfferings:        openOfferings,
		LegalTexts:           legalTexts,
		EditMode:             editMode,
		CollectSchoolClass:   capabilities.CollectSchoolClass,
		CollectGradeLevel:    capabilities.CollectGradeLevel,
		CareOfferingsEnabled: capabilities.CareOfferingsEnabled,
		GradeLevelMax:        gradeLevelMax,
	}, nil
}

// ReplaceEditable rewrites the editable payload of an existing request while
// preserving the request id, status token, guardian account link, and original
// submitted_at timestamp. It only runs while every child is still submitted.
func (s *requestService) ReplaceEditable(ctx context.Context, token string, incoming SubmitRequest) (*SubmitResult, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrRequestNotFound
	}

	var tenantID int64
	if err := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		req, err := s.RequestRepo.FindByStatusToken(adminCtx, token)
		if err != nil {
			return ErrRequestNotFound
		}
		if req.StatusTokenExpires != nil && time.Now().After(*req.StatusTokenExpires) {
			return ErrRequestNotFound
		}
		tenantID = req.GetTenantID()
		return nil
	}); err != nil {
		return nil, err
	}

	var (
		updatedRequest  *enrollmentModels.Request
		createdChildren []*enrollmentModels.RequestChild
		warnings        []SubmissionWarning
	)
	err := tenant.WithTenantTx(ctx, s.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		req, err := s.RequestRepo.FindByStatusTokenForUpdate(txCtx, token)
		if err != nil {
			return ErrRequestNotFound
		}
		if req.StatusTokenExpires != nil && time.Now().After(*req.StatusTokenExpires) {
			return ErrRequestNotFound
		}
		children, err := s.RequestChildRepo.ListByRequestIDForUpdate(txCtx, req.ID)
		if err != nil {
			return fmt.Errorf("edit replace: lock children: %w", err)
		}
		if err := s.ensureRequestEditable(txCtx, req, children); err != nil {
			return err
		}

		editReq := incoming
		editReq.TenantID = tenantID
		editReq.PhaseID = req.PhaseID
		editReq.GuardianEmail = req.GuardianEmail
		editReq.GuardianAccountID = req.GuardianAccountID
		if editReq.ConsentFlags == nil {
			editReq.ConsentFlags = map[string]any{}
		}
		if editReq.CustomData == nil {
			editReq.CustomData = map[string]any{}
		}
		if editReq.Children == nil {
			editReq.Children = []SubmitChild{}
		}

		phase, err := s.loadPhaseForEditableRequest(txCtx, req.PhaseID)
		if err != nil {
			return err
		}
		duplicatePolicy, err := s.Settings.ResolveString(txCtx, configModel.KeyEnrollmentDuplicateHandling)
		if err != nil {
			return fmt.Errorf("edit replace: resolve duplicate handling: %w", err)
		}
		capabilities, err := s.FormCapabilities(txCtx)
		if err != nil {
			return fmt.Errorf("edit replace: resolve form capabilities: %w", err)
		}
		openOfferings := []*enrollmentModels.CareOffering{}
		if capabilities.CareOfferingsEnabled {
			openOfferings, err = s.CareOfferingRepo.ListActiveByPhase(txCtx, phase.ID)
			if err != nil {
				return fmt.Errorf("edit replace: load phase offerings: %w", err)
			}
		}
		capabilities = EffectiveFormCapabilities(capabilities, openOfferings)
		if err := normalizeSubmissionForCapabilities(&editReq, capabilities); err != nil {
			return err
		}
		openByID := make(map[int64]*enrollmentModels.CareOffering, len(openOfferings))
		for _, o := range openOfferings {
			openByID[o.ID] = o
		}
		selectionMode := enrollmentModels.PhaseCareOfferingSelectionOptional
		if capabilities.CareOfferingsEnabled {
			selectionMode = phase.CareOfferingSelectionMode
		}
		materializedSelections, err := materializeAndValidateChildrenOfferingSelections(editReq.Children, openByID, selectionMode)
		if err != nil {
			return err
		}

		schema, err := s.schemaForEditableRequest(txCtx, req, phase)
		if err != nil {
			return err
		}
		legalBlocks, err := s.legalBlocksForEditableRequest(txCtx, schema)
		if err != nil {
			return err
		}
		if err := normalizeAdditionalGuardians(&editReq); err != nil {
			return err
		}
		if err := s.validateSubmission(txCtx, editReq, legalBlocks, capabilities); err != nil {
			return err
		}
		if err := s.validateAndNormalizeSchoolClasses(txCtx, phase, editReq.Children); err != nil {
			return err
		}
		// Reapply the per-child eligibility gates (eligible_school_classes +
		// new_students already-enrolled) so a status-token holder cannot edit a
		// previously eligible request into an ineligible class or an
		// already-enrolled child's identity (#1663). Runs after class
		// canonicalization, mirroring Submit's ordering. The linked_parents
		// audience authorization is preserved from the original submission —
		// the request's existence proves it passed and the status token is held
		// by that same guardian — so it is not re-evaluated here. Trusted-path
		// requests (late invite / admin manual) bypassed eligibility
		// deliberately at creation and keep that override on edit.
		// Generated rollover requests carry submission_source='public' (the DB
		// default) yet their children were carried forward from an already-
		// approved enrollment, so they necessarily FAIL the self-service gates
		// on renewal: a new_students successor trips ErrChildAlreadyEnrolled,
		// and a grade-bumped class-restricted successor has its concrete class
		// cleared and trips class_not_eligible. Exempt them here alongside the
		// trusted-source bypass — the identity of a rollover edit is already
		// pinned by validateRolloverEditIdentity, so no arbitrary child can be
		// slipped in behind this exemption (#1663).
		eligibilityEnforced := !isTrustedEnrollmentSource(req.SubmissionSource) && !hasRolloverGeneratedChild(children)
		if eligibilityEnforced {
			if err := s.validatePhaseChildEligibility(txCtx, phase, editReq); err != nil {
				return err
			}
		}
		editReq.ConsentFlags = filterConsentFlags(editReq.ConsentFlags, legalBlocks)
		if err := s.validateRequiredCustomFields(schema, editReq, openByID); err != nil {
			return err
		}
		// Same accompanied-requires-note gate as Submit, so an edited request
		// can't be saved into an un-approvable state (#1694).
		if err := s.validateAccompaniedCompanionNote(schema, editReq, openByID); err != nil {
			return err
		}
		if err := s.validateConstrainedSchedules(schema, editReq, openByID); err != nil {
			return err
		}
		byKey := buildFieldsByKey(schema)
		rawGuardian := editReq.CustomData
		matchedExistingChildren := matchExistingChildrenBySubmittedIdentity(children, editReq.Children)
		existingCustomData := existingChildCustomDataBySubmittedIdentity(children, editReq.Children)
		for i := range editReq.Children {
			childCtx := fieldVisibilityContext{
				guardianAnswers: rawGuardian,
				childAnswers:    editReq.Children[i].CustomData,
				gradeLevel:      editReq.Children[i].TargetGradeLevel,
				offeringNames:   selectedOfferingNames(editReq.Children[i], openByID),
				fieldsByKey:     byKey,
			}
			sanitizedChild := sanitizeVisibleAnswers(schema, true, editReq.Children[i].CustomData, childCtx)
			pruneChildScheduleAnswers(
				schema, sanitizedChild,
				relevantCareDaysForChild(editReq.Children[i], openByID),
			)
			editReq.Children[i].CustomData = mergeEditableCustomData(existingCustomData[i], sanitizedChild, schema, true)
		}
		editReq.CustomData = mergeEditableCustomData(
			req.CustomData,
			sanitizeVisibleAnswers(schema, false, rawGuardian, fieldVisibilityContext{
				guardianAnswers: rawGuardian,
				fieldsByKey:     byKey,
			}),
			schema,
			false,
		)

		emailLC := strings.ToLower(strings.TrimSpace(req.GuardianEmail))
		emailHash := fnvHash64(emailLC)
		if err := s.RequestRepo.AcquireSubmissionDedupLock(txCtx, phase.ID, emailHash); err != nil {
			return fmt.Errorf("edit replace: acquire dedup lock: %w", err)
		}

		existingChildIDs := make([]int64, 0, len(children))
		hasRolloverChild := false
		for _, child := range children {
			existingChildIDs = append(existingChildIDs, child.ID)
			if child.RolloverSourceChildID != nil {
				hasRolloverChild = true
			}
		}
		if hasRolloverChild {
			if err := validateRolloverEditIdentity(children, editReq.Children); err != nil {
				return err
			}
		}
		existingLinks, err := s.RequestChildOfferingRepo.ListByRequestChildIDs(txCtx, existingChildIDs)
		if err != nil {
			return fmt.Errorf("edit replace: load existing child offerings: %w", err)
		}
		preservedClaims := make(map[int64]int, len(existingLinks))
		for _, link := range existingLinks {
			preservedClaims[link.CareOfferingID]++
		}
		if !capabilities.CareOfferingsEnabled {
			materializedSelections = preservedOfferingSelections(children, editReq.Children, existingLinks)
		}

		if s.RequestGuardianRepo != nil {
			if err := s.RequestGuardianRepo.DeleteByRequestID(txCtx, req.ID); err != nil {
				return err
			}
		}
		if err := s.RequestChildRepo.DeleteByRequestID(txCtx, req.ID); err != nil {
			return err
		}

		dupKeys := make([]enrollmentModels.DuplicateChildKey, 0, len(editReq.Children))
		for _, c := range editReq.Children {
			dupKeys = append(dupKeys, enrollmentModels.DuplicateChildKey{FirstName: c.FirstName, LastName: c.LastName})
		}
		dupes, dupErr := s.RequestRepo.FindActiveDuplicate(txCtx, phase.ID, req.GuardianEmail, dupKeys)
		if dupErr != nil {
			return fmt.Errorf("edit replace: duplicate check: %w", dupErr)
		}
		if len(dupes) > 0 && duplicatePolicy == configModel.EnrollmentDuplicateHandlingBlock {
			return ErrDuplicateEnrollment
		}
		if len(dupes) > 0 && duplicatePolicy == configModel.EnrollmentDuplicateHandlingWarn {
			warnings = []SubmissionWarning{{Code: WarningCodeDuplicateEnrollment}}
		}
		if duplicatePolicy != configModel.EnrollmentDuplicateHandlingBlock &&
			duplicatePolicy != configModel.EnrollmentDuplicateHandlingWarn &&
			duplicatePolicy != configModel.EnrollmentDuplicateHandlingIgnore {
			return fmt.Errorf("edit replace: unsupported duplicate handling %q", duplicatePolicy)
		}

		childStatusOverrides := map[int]string{}
		if capabilities.CareOfferingsEnabled {
			childStatusOverrides, err = s.applyCapacityOverflowWithPreservedClaims(txCtx, phase, editReq.Children, openByID, preservedClaims)
			if err != nil {
				return err
			}
		}

		req.GuardianFirstName = strings.TrimSpace(editReq.GuardianFirstName)
		req.GuardianLastName = strings.TrimSpace(editReq.GuardianLastName)
		req.GuardianPhone = editReq.GuardianPhone
		req.ConsentFlags = editReq.ConsentFlags
		req.CustomData = editReq.CustomData
		if err := s.RequestRepo.UpdateGuardianData(txCtx, req); err != nil {
			return err
		}
		if s.RequestGuardianRepo != nil {
			for i, g := range editReq.AdditionalGuardians {
				row := &enrollmentModels.RequestGuardian{
					RequestID: req.ID,
					FirstName: g.FirstName,
					LastName:  g.LastName,
					Email:     g.Email,
					Phone:     g.Phone,
					SortOrder: i,
				}
				if err := s.RequestGuardianRepo.Create(txCtx, row); err != nil {
					return fmt.Errorf("edit replace: create request guardian %d: %w", i, err)
				}
			}
		}

		for i, child := range editReq.Children {
			status := enrollmentModels.ChildStatusSubmitted
			if override, ok := childStatusOverrides[i]; ok {
				status = override
			}
			row := &enrollmentModels.RequestChild{
				RequestID:         req.ID,
				FirstName:         strings.TrimSpace(child.FirstName),
				LastName:          strings.TrimSpace(child.LastName),
				DateOfBirth:       child.DateOfBirth,
				TargetGradeLevel:  child.TargetGradeLevel,
				TargetSchoolClass: child.TargetSchoolClass,
				CustomData:        child.CustomData,
				Status:            status,
				ActivationMode:    enrollmentModels.ChildActivationScheduled,
				SortOrder:         i,
			}
			matched := matchedExistingChildren[i]
			if matched != nil {
				row.ActivationMode = matched.ActivationMode
				row.ActivateOn = matched.ActivateOn
				row.RolloverSourceChildID = matched.RolloverSourceChildID
				row.ReviewReason = matched.ReviewReason
			}
			// Resolve the matched existing student for existing_students edits so
			// approval still renews the right record instead of duplicating
			// (#1663). Skipped for rollover children: their existing student is
			// resolved through the rollover source chain, which takes precedence at
			// approval, so a redundant match here would be dead data.
			if row.RolloverSourceChildID == nil {
				// Default to the submission-time pin carried on the prior row. The
				// original submission pinned it while the phase was
				// existing_students; if the phase audience is later flipped away,
				// re-resolution below no-ops and must NOT drop the pin FOR AN
				// UNCHANGED IDENTITY — otherwise approval takes the fresh-create
				// branch and duplicates the very Person/Student the existing_students
				// audience exists to renew. An edited identity is handled just below.
				var matchedStudentID *int64
				if matched != nil {
					matchedStudentID = matched.MatchedStudentID
				}
				// Replacement edits pair to the prior row by persisted ID, so an
				// edited name/birthday keeps the SAME row — and its pin — even though
				// it now describes a different child. Drop the carried pin when the
				// submitted identity no longer matches what was originally pinned:
				// keeping it would renew/overwrite the originally matched student even
				// though the review screen shows the new identity. The unchanged-
				// identity case still preserves the pin (the anti-duplication reason
				// above), and re-resolution below re-pins by the NEW identity when the
				// phase is still existing_students; otherwise the child falls through
				// to a clean fresh create (#1663).
				if matchedStudentID != nil && !sameSubmittedIdentity(matched, child) {
					matchedStudentID = nil
				}
				// While the phase is still existing_students, re-resolve against the
				// (possibly edited) name/birthday. A concrete re-resolution wins; a
				// nil result keeps the carried pin rather than dropping it.
				if phase.Audience == enrollmentModels.PhaseAudienceExistingStudents {
					resolved, err := s.resolveMatchedStudentID(txCtx, req.TenantID, phase, i, child)
					if err != nil {
						return err
					}
					if resolved != nil {
						matchedStudentID = resolved
					}
				}
				// Same race guard as Submit: when the eligibility gate ran a few
				// statements earlier it proved this child is enrolled, so an
				// unpinned existing_students child means the student changed
				// status underneath the edit — reject instead of letting approval
				// duplicate it (#1663).
				if err := assertExistingStudentMatchResolved(phase, matchedStudentID, eligibilityEnforced, i); err != nil {
					return err
				}
				// Authorized against the PERSISTED request's identity, so an edit
				// cannot re-point the pin at a child the request's guardian holds no
				// re-enrollment permission on (#1663).
				if err := s.assertGuardianMayReEnrollStudent(txCtx,
					reEnrollmentSubmitterFor(req.SubmissionSource, req.GuardianAccountID, req.GuardianEmail),
					matchedStudentID, req.TenantID, i); err != nil {
					return err
				}
				if err := s.guardMatchedStudentUnique(txCtx, phase.ID, matchedStudentID, 0, i); err != nil {
					return err
				}
				row.MatchedStudentID = matchedStudentID
			}
			if err := s.RequestChildRepo.Create(txCtx, row); err != nil {
				return fmt.Errorf("edit replace: create request child %d: %w", i, err)
			}
			for _, selection := range materializedSelections[i] {
				link := &enrollmentModels.RequestChildOffering{
					RequestChildID:        row.ID,
					CareOfferingID:        selection.OfferingID,
					SelectedDays:          selection.SelectedDays,
					ManualSelectedDays:    selection.ManualSelectedDays,
					AutomaticSelectedDays: selection.AutomaticSelectedDays,
				}
				if err := s.RequestChildOfferingRepo.Create(txCtx, link); err != nil {
					return fmt.Errorf("edit replace: create child-offering link: %w", err)
				}
			}
			createdChildren = append(createdChildren, row)
		}
		updatedRequest = req
		if len(childStatusOverrides) > 0 && !editReq.SuppressSubmissionEmails {
			if err := enqueueDecisionNotifications(txCtx, decisionNotificationDependencies{
				requests:   s.RequestRepo,
				settings:   s.Settings,
				outbox:     s.OutboxEnqueuer,
				schools:    s.SchoolRepo,
				parentsURL: s.ParentsURL,
			}, req, createdChildren, phase, childIDsForStatus(createdChildren, enrollmentModels.ChildStatusWaitlisted)); err != nil {
				return fmt.Errorf("edit replace: notify capacity decisions: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.Logger.Info("enrollment request edited by parent",
		slog.Int64("request_id", updatedRequest.ID),
		slog.Int64("tenant_id", tenantID),
		slog.Int("children", len(createdChildren)))

	return &SubmitResult{
		Request:   updatedRequest,
		Children:  createdChildren,
		StatusURL: enrollmentStatusURL(s.ParentsURL, updatedRequest.StatusToken),
		Warnings:  warnings,
	}, nil
}

func (s *requestService) ensureRequestEditable(ctx context.Context, req *enrollmentModels.Request, children []*enrollmentModels.RequestChild) error {
	if req == nil || req.WithdrawnAt != nil {
		return ErrEditNotAllowed
	}
	if !s.allowSubmissionEdit(ctx) {
		return ErrEditNotAllowed
	}
	if len(children) == 0 {
		return ErrEditNotAllowed
	}
	for _, c := range children {
		if c.Status != enrollmentModels.ChildStatusSubmitted {
			return ErrEditNotAllowed
		}
	}
	return nil
}

func editModeForChildren(children []*enrollmentModels.RequestChild) string {
	if len(children) == 0 {
		return EditModeDirectEdit
	}
	for _, c := range children {
		if c.Status != enrollmentModels.ChildStatusSubmitted {
			return EditModeChangeRequest
		}
	}
	return EditModeDirectEdit
}

func (s *requestService) ensureChangeRequestDraftAvailable(ctx context.Context, req *enrollmentModels.Request, children []*enrollmentModels.RequestChild) error {
	if req == nil || req.WithdrawnAt != nil {
		return ErrEditNotAllowed
	}
	if !s.allowSubmissionEdit(ctx) {
		return ErrEditNotAllowed
	}
	if len(children) == 0 {
		return ErrEditNotAllowed
	}
	if editModeForChildren(children) != EditModeChangeRequest {
		return ErrEditNotAllowed
	}
	for _, c := range children {
		if c.Status == enrollmentModels.ChildStatusWithdrawn {
			return ErrEditNotAllowed
		}
	}
	return nil
}

func (s *requestService) schemaForEditableRequest(ctx context.Context, req *enrollmentModels.Request, phase *enrollmentModels.Phase) (*enrollmentModels.FormSchema, error) {
	if req != nil && req.SchemaID != nil {
		return s.FormSchemaRepo.FindByID(ctx, *req.SchemaID)
	}
	return nil, nil
}

func validateRolloverEditIdentity(existing []*enrollmentModels.RequestChild, incoming []SubmitChild) error {
	if len(incoming) != len(existing) {
		return ErrEditNotAllowed
	}
	for i, child := range existing {
		next := incoming[i]
		if strings.TrimSpace(next.FirstName) != strings.TrimSpace(child.FirstName) ||
			strings.TrimSpace(next.LastName) != strings.TrimSpace(child.LastName) ||
			next.DateOfBirth != child.DateOfBirth {
			return ErrEditNotAllowed
		}
	}
	return nil
}

func (s *requestService) legalTextsForEditableRequest(ctx context.Context, schema *enrollmentModels.FormSchema) (LegalTexts, error) {
	texts, err := s.LegalTexts(ctx)
	if err != nil {
		return LegalTexts{}, err
	}
	return applyTemplateLegalBlocks(texts, schema), nil
}

// applyTemplateLegalBlocks replaces the settings-derived legal blocks with the
// template's blocks. Template blocks win only when at least one of them is
// enabled: a template whose blocks are all disabled (saved before the
// Rechtstexte were configured, or via the API) must not erase the tenant's
// consent contract - that would let parents submit without the expected DSGVO
// acknowledgment.
func applyTemplateLegalBlocks(texts LegalTexts, schema *enrollmentModels.FormSchema) LegalTexts {
	if schema != nil && len(schema.LegalBlocks) > 0 {
		if blocks := buildTemplateLegalBlocks(schema.LegalBlocks); len(blocks) > 0 {
			texts.Blocks = blocks
		}
	}
	return texts
}

func (s *requestService) legalBlocksForEditableRequest(ctx context.Context, schema *enrollmentModels.FormSchema) ([]LegalBlock, error) {
	texts, err := s.legalTextsForEditableRequest(ctx, schema)
	if err != nil {
		return nil, fmt.Errorf("edit replace: resolve legal blocks: %w", err)
	}
	return texts.Blocks, nil
}

// Edit applies the patch only when every child is still `submitted`.
// PR 7 ships a minimal in-place mutation via raw bun on the tenant
// transaction; PR 8's admin update path may extend the repo with a
// dedicated Update method when it ships its own admin-edit endpoint.
func (s *requestService) Edit(ctx context.Context, token string, patch EditPatch) error {
	req, children, err := s.GetByStatusToken(ctx, token)
	if err != nil {
		return err
	}
	if !s.allowSubmissionEdit(ctx) {
		return ErrEditNotAllowed
	}
	for _, c := range children {
		if c.Status != enrollmentModels.ChildStatusSubmitted {
			return ErrEditNotAllowed
		}
	}

	if patch.GuardianFirstName != nil {
		req.GuardianFirstName = strings.TrimSpace(*patch.GuardianFirstName)
	}
	if patch.GuardianLastName != nil {
		req.GuardianLastName = strings.TrimSpace(*patch.GuardianLastName)
	}
	if patch.GuardianPhone != nil {
		v := strings.TrimSpace(*patch.GuardianPhone)
		// Same canonical phone check as submit — an admin/parent edit must
		// not be able to reintroduce a value approval would later reject.
		if err := users.ValidateOptionalPhone(v); err != nil {
			return ErrInvalidGuardianPhone
		}
		req.GuardianPhone = &v
	}
	if patch.ConsentFlags != nil {
		req.ConsentFlags = patch.ConsentFlags
	}
	if patch.CustomData != nil {
		req.CustomData = patch.CustomData
	}

	// Same hidden-answer sanitizing as Submit: an edit must not be able to
	// (re)introduce a value for a guardian field the parent couldn't see.
	// Children aren't edited here, so only the guardian scope is filtered.
	var schema *enrollmentModels.FormSchema
	if req.SchemaID != nil {
		if loaded, schemaErr := s.FormSchemaRepo.FindByID(ctx, *req.SchemaID); schemaErr == nil {
			schema = loaded
			byKey := buildFieldsByKey(schema)
			req.CustomData = sanitizeVisibleAnswers(
				schema, false, req.CustomData,
				fieldVisibilityContext{guardianAnswers: req.CustomData, fieldsByKey: byKey},
			)
		}
	}

	tenantID := req.GetTenantID()
	tenantCtx := tenant.WithTenantID(ctx, tenantID)
	return tenant.WithTenantTx(tenantCtx, s.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		// Same consent-key allowlist as Submit. Resolved inside the tenant
		// tx because the settings fallback needs the per-tenant override.
		if patch.ConsentFlags != nil {
			blocks, blockErr := s.resolveSubmissionLegalBlocks(txCtx, schema)
			if blockErr != nil {
				return fmt.Errorf("edit: resolve legal blocks: %w", blockErr)
			}
			req.ConsentFlags = filterConsentFlags(req.ConsentFlags, blocks)
		}
		return s.RequestRepo.UpdateGuardianData(txCtx, req)
	})
}

// Withdraw transitions a child to `withdrawn` (or every non-terminal
// child when childID is 0). Approved children must go through the
// admin (terminal student records exist) - returns ErrWithdrawNotAllowed.
func (s *requestService) Withdraw(ctx context.Context, token string, childID int64) error {
	req, _, err := s.GetByStatusToken(ctx, token)
	if err != nil {
		return err
	}

	tenantID := req.GetTenantID()
	tenantCtx := tenant.WithTenantID(ctx, tenantID)
	return tenant.WithTenantTx(tenantCtx, s.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		lockedReq, err := s.RequestRepo.FindByStatusTokenForUpdate(txCtx, strings.TrimSpace(token))
		if err != nil || lockedReq == nil {
			return ErrRequestNotFound
		}
		if lockedReq.StatusTokenExpires != nil && time.Now().After(*lockedReq.StatusTokenExpires) {
			return ErrRequestNotFound
		}
		children, err := s.RequestChildRepo.ListByRequestIDForUpdate(txCtx, lockedReq.ID)
		if err != nil {
			return fmt.Errorf("withdraw: lock request children: %w", err)
		}
		anyWithdrawn := false
		for _, c := range children {
			if childID != 0 && c.ID != childID {
				continue
			}
			if c.Status == enrollmentModels.ChildStatusApproved {
				if childID != 0 {
					return ErrWithdrawNotAllowed
				}
				continue
			}
			if c.IsTerminal() {
				continue // already approved/rejected/withdrawn
			}
			if err := s.RequestChildRepo.UpdateStatus(txCtx, c.ID, enrollmentModels.ChildStatusWithdrawn, nil, 0); err != nil {
				return err
			}
			c.Status = enrollmentModels.ChildStatusWithdrawn
			c.StatusReason = nil
			anyWithdrawn = true
		}
		if childID == 0 && anyWithdrawn {
			if err := s.RequestRepo.MarkWithdrawn(txCtx, lockedReq.ID, time.Now()); err != nil {
				return err
			}
		}
		if !anyWithdrawn {
			return nil
		}

		if !allChildrenParentResolved(children) {
			return nil
		}
		children, err = s.RequestChildRepo.ListByRequestID(txCtx, lockedReq.ID)
		if err != nil {
			return fmt.Errorf("withdraw: refresh children for decision digest: %w", err)
		}
		phase, err := s.PhaseRepo.FindByID(txCtx, lockedReq.PhaseID)
		if err != nil {
			return fmt.Errorf("withdraw: load phase for decision digest: %w", err)
		}
		if err := enqueueDecisionNotifications(txCtx, decisionNotificationDependencies{
			requests:   s.RequestRepo,
			settings:   s.Settings,
			outbox:     s.OutboxEnqueuer,
			schools:    s.SchoolRepo,
			parentsURL: s.ParentsURL,
		}, lockedReq, children, phase, nil); err != nil {
			return fmt.Errorf("withdraw: notify completed decision state: %w", err)
		}
		return nil
	})
}

func childIDsForStatus(children []*enrollmentModels.RequestChild, status string) map[int64]struct{} {
	ids := make(map[int64]struct{})
	for _, child := range children {
		if child != nil && child.Status == status {
			ids[child.ID] = struct{}{}
		}
	}
	return ids
}

// ConfirmRenewal transitions every pending_renewal row under the
// request token to submitted. Idempotent — rows that are no longer
// pending_renewal are skipped, so a parent who double-clicks doesn't
// get an error.
func (s *requestService) ConfirmRenewal(ctx context.Context, token string) (int, error) {
	req, children, err := s.GetByStatusToken(ctx, token)
	if err != nil {
		return 0, err
	}
	tenantID := req.GetTenantID()
	tenantCtx := tenant.WithTenantID(ctx, tenantID)

	var confirmed int
	txErr := tenant.WithTenantTx(tenantCtx, s.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		for _, c := range children {
			if c.Status != enrollmentModels.ChildStatusPendingRenewal {
				continue
			}
			if err := s.RequestChildRepo.UpdateStatus(txCtx, c.ID, enrollmentModels.ChildStatusSubmitted, nil, 0); err != nil {
				return err
			}
			confirmed++
		}
		return nil
	})
	if txErr != nil {
		return 0, txErr
	}
	if confirmed > 0 {
		s.Logger.Info("renewal confirmed by parent",
			slog.Int64("request_id", req.ID),
			slog.Int("children_confirmed", confirmed),
		)
	}
	return confirmed, nil
}

// enqueueSubmissionEmails fires off the parent confirmation + admin
// notifications. Best-effort - failures log but don't fail the
// submission (the rows are already committed).
func (s *requestService) enqueueSubmissionEmails(ctx context.Context, tenantID int64, request *enrollmentModels.Request, children []*enrollmentModels.RequestChild, statusURL string) {
	if s.OutboxEnqueuer == nil {
		return
	}

	schoolName, logoURL := emailBrandForSchool(ctx, s.SchoolRepo, tenantID, s.ParentsURL)
	footerLogoURL := motoLogoURL(s.ParentsURL)
	childNames := make([]string, 0, len(children))
	for _, c := range children {
		childNames = append(childNames, fmt.Sprintf("%s %s", c.FirstName, c.LastName))
	}

	parentPayload := map[string]any{
		EnrollmentPayloadGuardianFirstName: request.GuardianFirstName,
		EnrollmentPayloadGuardianLastName:  request.GuardianLastName,
		EnrollmentPayloadGuardianEmail:     request.GuardianEmail,
		EnrollmentPayloadSchoolName:        schoolName,
		EnrollmentPayloadStatusURL:         statusURL,
		EnrollmentPayloadLogoURL:           logoURL,
		EnrollmentPayloadMotoLogoURL:       footerLogoURL,
		EnrollmentPayloadChildNames:        childNames,
		EnrollmentPayloadRecipientEmail:    request.GuardianEmail,
	}
	if err := s.OutboxEnqueuer.EnqueueOutbox(ctx, platformModels.OutboxEnqueueRequest{
		Kind:              platformModels.EmailKindEnrollmentSubmitted,
		Payload:           parentPayload,
		RelatedEntityType: platformModels.EmailRelatedTypeEnrollmentRequest,
		RelatedEntityID:   request.ID,
	}); err != nil {
		s.Logger.Error("submit: enqueue parent confirmation failed",
			slog.Int64("request_id", request.ID),
			slog.String("error", err.Error()))
	}

	for _, admin := range s.resolveAdminEmails(ctx) {
		adminPayload := map[string]any{
			EnrollmentPayloadGuardianFirstName: request.GuardianFirstName,
			EnrollmentPayloadGuardianLastName:  request.GuardianLastName,
			EnrollmentPayloadGuardianEmail:     request.GuardianEmail,
			EnrollmentPayloadSchoolName:        schoolName,
			EnrollmentPayloadAdminURL:          fmt.Sprintf("%s/enrollments/%d", s.FrontendURL, request.ID),
			EnrollmentPayloadLogoURL:           logoURL,
			EnrollmentPayloadMotoLogoURL:       footerLogoURL,
			EnrollmentPayloadChildNames:        childNames,
			EnrollmentPayloadRecipientEmail:    admin,
		}
		if request.GuardianPhone != nil {
			adminPayload[EnrollmentPayloadGuardianPhone] = *request.GuardianPhone
		}
		if err := s.OutboxEnqueuer.EnqueueOutbox(ctx, platformModels.OutboxEnqueueRequest{
			Kind:              platformModels.EmailKindEnrollmentAdminNotify,
			Payload:           adminPayload,
			RelatedEntityType: platformModels.EmailRelatedTypeEnrollmentRequest,
			RelatedEntityID:   request.ID,
		}); err != nil {
			s.Logger.Error("submit: enqueue admin notification failed",
				slog.Int64("request_id", request.ID),
				slog.String("admin", admin),
				slog.String("error", err.Error()))
		}
	}
}

// --- helpers ---

// loadPhaseForSubmission fetches the phase the parent selected and
// validates it's enabled. Returns ErrEnrollmentDisabled when the phase
// is inactive (admin marked it hidden) and ErrInvalidSubmission when
// the id is unknown. The window check happens in Submit() - kept
// separate so error mapping in the handler stays specific.
func (s *requestService) loadPhaseForSubmission(ctx context.Context, phaseID int64) (*enrollmentModels.Phase, error) {
	if s.PhaseRepo == nil {
		return nil, fmt.Errorf("submit: phase repo not wired")
	}
	phase, err := s.PhaseRepo.FindByID(ctx, phaseID)
	if err != nil {
		return nil, fmt.Errorf("%w: phase %d not found", ErrInvalidSubmission, phaseID)
	}
	if !phase.IsActive {
		return nil, ErrEnrollmentDisabled
	}
	return phase, nil
}

// resolveSubmissionSchema returns the phase's pinned form schema, or
// nil when the phase is "Basis" (form_schema_id IS NULL). Returning
// nil writes the request with a NULL schema_id, which is allowed
// since migration 1.15.69. We deliberately do NOT fall back to the
// tenant's currently-active schema for Basis phases - the admin UI
// promises "nur die Standardfelder", so silently inheriting the
// latest custom schema would leak its fields into every Basis phase.
func (s *requestService) resolveSubmissionSchema(ctx context.Context, phase *enrollmentModels.Phase) (*enrollmentModels.FormSchema, error) {
	if phase.FormSchemaID == nil {
		return nil, nil
	}
	schema, err := s.FormSchemaRepo.FindByID(ctx, *phase.FormSchemaID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Pinned schema was deleted out from under the phase.
			// Treat as Basis rather than 500 - submission still
			// succeeds with NULL schema_id and the admin can repin.
			s.Logger.Warn("phase form_schema_id pointed at missing schema; submitting as Basis",
				slog.Int64("phase_id", phase.ID),
				slog.Int64("form_schema_id", *phase.FormSchemaID))
			return nil, nil
		}
		return nil, err
	}
	return schema, nil
}

func newStatusToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func lateInviteTokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func normalizedSubmissionSource(source string) string {
	switch strings.TrimSpace(source) {
	case enrollmentModels.RequestSourceLateInvite:
		return enrollmentModels.RequestSourceLateInvite
	case enrollmentModels.RequestSourceAdminManual:
		return enrollmentModels.RequestSourceAdminManual
	default:
		return enrollmentModels.RequestSourcePublic
	}
}

func cloneSourceMetadata(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func ensureRequiredConsentFlags(flags map[string]any, legalBlocks []LegalBlock) map[string]any {
	out := cloneSourceMetadata(flags)
	for _, key := range requiredConsentKeys(legalBlocks) {
		out[key] = true
	}
	return out
}

func normalizeGuardianEmail(email string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(email))
	if trimmed == "" {
		return "", fmt.Errorf("%w: guardian email is required", ErrInvalidSubmission)
	}
	if err := users.ValidateOptionalEmail(trimmed); err != nil {
		return "", ErrInvalidGuardianEmail
	}
	return trimmed, nil
}

// enrollmentStatusURL builds the parent-facing status link - sent in the
// submitted/approved/waitlisted/rejected emails. Routes to the parents portal.
func enrollmentStatusURL(parentsURL, token string) string {
	host := parentsURL
	if host == "" {
		host = "http://localhost:3000"
	}
	return fmt.Sprintf("%s/enroll/status/%s", host, token)
}

// IsEnrollmentEnabled is the public counterpart of isEnrollmentEnabled
// so HTTP handlers can gate their form-load endpoints without reaching
// into the settings package directly.
func (s *requestService) IsEnrollmentEnabled(ctx context.Context) bool {
	return s.isEnrollmentEnabled(ctx)
}

// fnvHash64 returns a 64-bit FNV-1a hash of the input. Used to derive
// a stable advisory-lock key from the lowercased guardian email.
func fnvHash64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func (s *requestService) isEnrollmentEnabled(ctx context.Context) bool {
	return config.ResolveBoolOrDefault(ctx, s.Settings, configModel.KeyEnrollmentEnabled, false, nil)
}

// LegalTexts resolves configured legal Markdown for the tenant in context
// and derives the public block list. No env var fallback: these settings
// were registered from the start, so a plain Resolve (tenant override →
// registry default of "") is correct. Whitespace-only values normalize to
// "" so the frontend treats them as "not configured".
//
// A resolve error (settings/DB/JSON failure) is propagated, NOT swallowed:
// these texts drive legally relevant blocks, so the endpoint must fail rather
// than let the form collect an incomplete legal state. An unconfigured
// (empty) text is not an error; it returns "" and produces no public block.
func (s *requestService) LegalTexts(ctx context.Context) (LegalTexts, error) {
	if s.Settings == nil {
		return LegalTexts{}, nil
	}
	agb, err := s.Settings.ResolveString(ctx, configModel.KeyEnrollmentLegalAGBText)
	if err != nil {
		return LegalTexts{}, fmt.Errorf("resolve AGB legal text: %w", err)
	}
	agbDocumentURL, err := s.Settings.ResolveString(ctx, configModel.KeyEnrollmentLegalAGBDocumentURL)
	if err != nil {
		return LegalTexts{}, fmt.Errorf("resolve AGB legal document: %w", err)
	}
	agbDisplayMode, err := s.Settings.ResolveString(ctx, configModel.KeyEnrollmentLegalAGBDisplayMode)
	if err != nil {
		return LegalTexts{}, fmt.Errorf("resolve AGB legal display mode: %w", err)
	}
	dsgvo, err := s.Settings.ResolveString(ctx, configModel.KeyEnrollmentLegalDSGVOText)
	if err != nil {
		return LegalTexts{}, fmt.Errorf("resolve DSGVO legal text: %w", err)
	}
	emailContact, err := s.Settings.ResolveString(ctx, configModel.KeyEnrollmentLegalEmailContactText)
	if err != nil {
		return LegalTexts{}, fmt.Errorf("resolve email contact legal text: %w", err)
	}
	photo, err := s.Settings.ResolveString(ctx, configModel.KeyEnrollmentLegalPhotoText)
	if err != nil {
		return LegalTexts{}, fmt.Errorf("resolve photo legal text: %w", err)
	}
	termsEnabled, err := s.legalTermsEnabled(ctx)
	if err != nil {
		return LegalTexts{}, err
	}
	dsgvoEnabled, err := s.legalBlockEnabled(ctx, configModel.KeyEnrollmentLegalDSGVOEnabled, "DSGVO legal block")
	if err != nil {
		return LegalTexts{}, err
	}
	emailContactEnabled, err := s.legalBlockEnabled(ctx, configModel.KeyEnrollmentLegalEmailContactEnabled, "email contact legal block")
	if err != nil {
		return LegalTexts{}, err
	}
	photoEnabled, err := s.legalBlockEnabled(ctx, configModel.KeyEnrollmentLegalPhotoEnabled, "photo legal block")
	if err != nil {
		return LegalTexts{}, err
	}
	texts := LegalTexts{
		AGB:                 strings.TrimSpace(agb),
		AGBDocumentURL:      strings.TrimSpace(agbDocumentURL),
		AGBDisplayMode:      legalAGBDisplayMode(strings.TrimSpace(agbDisplayMode)),
		DSGVO:               strings.TrimSpace(dsgvo),
		EmailContact:        strings.TrimSpace(emailContact),
		Photo:               strings.TrimSpace(photo),
		TermsEnabled:        termsEnabled,
		DSGVOEnabled:        dsgvoEnabled,
		EmailContactEnabled: emailContactEnabled,
		PhotoEnabled:        photoEnabled,
	}
	texts.Blocks = buildLegalBlocks(texts)
	return texts, nil
}

func (s *requestService) LegalTextsForPhaseWithLateInvite(ctx context.Context, phaseID int64, lateInviteToken string) (LegalTexts, error) {
	phase, err := s.LoadPublicPhaseWithLateInvite(ctx, phaseID, time.Now(), lateInviteToken)
	if err != nil {
		return LegalTexts{}, err
	}
	return s.legalTextsForLoadedPhase(ctx, phase)
}

// LegalTextsForEnrolleePhaseWithLateInvite mirrors
// LegalTextsForPhaseWithLateInvite but runs the authenticated enrollee gate
// with the caller's resolved audience access, so the parents-portal bootstrap
// resolves legal texts for exactly the restricted phases it may load.
func (s *requestService) LegalTextsForEnrolleePhaseWithLateInvite(ctx context.Context, phaseID int64, lateInviteToken string, access EnrolleeAudienceAccess) (LegalTexts, error) {
	phase, err := s.LoadEnrolleePhaseWithLateInvite(ctx, phaseID, time.Now(), lateInviteToken, access)
	if err != nil {
		return LegalTexts{}, err
	}
	return s.legalTextsForLoadedPhase(ctx, phase)
}

func (s *requestService) LegalTextsForManualEnrollmentPhase(ctx context.Context, phaseID int64) (LegalTexts, error) {
	phase, err := s.LoadManualEnrollmentPhase(ctx, phaseID)
	if err != nil {
		return LegalTexts{}, err
	}
	return s.legalTextsForLoadedPhase(ctx, phase)
}

func (s *requestService) legalTextsForLoadedPhase(ctx context.Context, phase *enrollmentModels.Phase) (LegalTexts, error) {
	texts, err := s.LegalTexts(ctx)
	if err != nil {
		return LegalTexts{}, err
	}
	schema, err := s.resolveSubmissionSchema(ctx, phase)
	if err != nil {
		return LegalTexts{}, err
	}
	return applyTemplateLegalBlocks(texts, schema), nil
}

// loadEditablePhaseWithLateInvite is the shared form-load phase gate. It
// resolves the phase, enforces the enrollment window (honoring a valid
// late-invite token when the window is closed) and rejects every
// audience-restricted phase the caller's resolved access does not cover, so
// neither a direct anonymous link nor a parent-scoped JWT without the
// matching guardian fact can bootstrap a linked_parents or existing_students
// phase (#1663). The audience check runs before the window check so a
// restricted phase always surfaces the same 404 regardless of window. The
// restricted set is exactly the one Submit refuses
// (audienceRequiresGuardianAccount): loading a form whose save can only ever
// fail is a dead end that also leaks the phase's existence.
func (s *requestService) loadEditablePhaseWithLateInvite(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string, access EnrolleeAudienceAccess) (*enrollmentModels.Phase, error) {
	phase, err := s.loadPhaseForEditableRequest(ctx, phaseID)
	if err != nil {
		return nil, err
	}
	// A valid late invite is an explicit, per-recipient eligibility override
	// (Submit treats it exactly that way via AllowClosedPhase), so it also
	// lifts the audience restriction: an admin who mints a late invite for a
	// restricted phase hands out the tenant's public form URL, and the
	// recipient must be able to load the form the invite points at. Resolve
	// the invite once here and reuse it for the window check below, so a
	// restricted phase with a valid invite loads instead of 404-ing before
	// the token is ever validated (#1663).
	hasValidLateInvite := false
	if s.LateInviteRepo != nil && strings.TrimSpace(lateInviteToken) != "" {
		_, inviteErr := s.LateInviteRepo.FindUsableByTokenHash(ctx, lateInviteTokenHash(lateInviteToken), phaseID, now)
		switch {
		case inviteErr == nil:
			hasValidLateInvite = true
		case errors.Is(inviteErr, enrollmentModels.ErrLateInviteNotFound):
			// Unknown, used, or expired token: no override, fall through to
			// the normal audience and window gates below.
		default:
			// A database or driver failure leaves the invite status unknown.
			// Treating it as "no invite" would render an outage as a 404
			// (ErrPhaseAudienceRestricted / ErrLateInviteInvalid) and tell a
			// legitimate recipient their link is invalid, so propagate it and
			// let the handler return a 500 (#1663).
			return nil, fmt.Errorf("resolve late invite: %w", inviteErr)
		}
	}
	if !hasValidLateInvite && !access.AllowsAudience(phase.Audience) {
		return nil, ErrPhaseAudienceRestricted
	}
	if !IsEnrollmentWindowOpen(phase, now) {
		if strings.TrimSpace(lateInviteToken) == "" {
			return nil, ErrEnrollmentWindowClosed
		}
		if !hasValidLateInvite {
			return nil, ErrLateInviteInvalid
		}
	}
	// Only offer classes the submit-time eligibility gate will actually accept.
	// A valid late invite bypasses that gate (AllowClosedPhase), so its recipient
	// keeps the full offered list; everyone else sees the eligible subset (#1663).
	if hasValidLateInvite {
		clearGradeRestrictionForEligibilityExemptForm(phase)
	} else {
		narrowOfferedClassesToEligibleForForm(phase)
	}
	return phase, nil
}

// clearGradeRestrictionForEligibilityExemptForm drops the phase's grade
// restriction from a form-load response served to a caller the submit-time
// eligibility gates do not apply to: a valid late-invite recipient, or the
// holder of a status token on a trusted-source / rollover-generated request.
// Such a load is a deliberate eligibility override — Submit honors the invite
// via AllowClosedPhase and the edit paths skip validateChildGradeEligibility
// for exempt requests — so a child from outside the restricted grade is
// accepted. Leaving the restriction in the response would still narrow the
// form's grade select to it, making the caller unable to declare the very
// grade the invite (or the carried-forward enrollment) exists for. This is the
// grade-level mirror of the class handling, where an exempt load skips
// narrowOfferedClassesToEligibleForForm and keeps the full offered class list
// (#1663).
//
// Mutates a phase already cleared for a form load; Submit and the edit paths
// reload the phase independently, so this never reaches validation.
func clearGradeRestrictionForEligibilityExemptForm(phase *enrollmentModels.Phase) {
	if phase == nil {
		return
	}
	phase.EligibleGradeLevels = []int{}
}

// narrowOfferedClassesToEligibleForForm restricts the phase's offered concrete
// classes (available_school_classes) to those that are ALSO eligible, for the
// self-service form-load paths. Phase.Validate only enforces eligible ⊆
// available, so available may legitimately be a wider superset — and a stale
// admin client or a direct API write can widen it further while the eligibility
// list stays narrow. Offering a class the submit-time gate rejects would let a
// parent complete the whole form only to fail with class_not_eligible, so
// present exactly the eligible subset instead. Mutates a phase already cleared
// for a self-service load; the callers exclude eligibility-exempt loads (late
// invite, trusted-source or rollover-generated edit drafts), which bypass the
// gate. No-op when no restriction is active. Submit reloads the phase
// independently (loadPhaseForSubmission), so this narrowing never reaches
// validation — and eligible ⊆ available guarantees every offered class still
// passes the available check anyway (#1663).
func narrowOfferedClassesToEligibleForForm(phase *enrollmentModels.Phase) {
	if phase == nil || len(phase.EligibleSchoolClasses) == 0 {
		return
	}
	eligible := make(map[string]struct{}, len(phase.EligibleSchoolClasses))
	for _, c := range phase.EligibleSchoolClasses {
		if t := strings.TrimSpace(c); t != "" {
			eligible[t] = struct{}{}
		}
	}
	narrowed := make([]string, 0, len(phase.AvailableSchoolClasses))
	for _, c := range phase.AvailableSchoolClasses {
		if _, ok := eligible[strings.TrimSpace(c)]; ok {
			narrowed = append(narrowed, c)
		}
	}
	phase.AvailableSchoolClasses = narrowed
	narrowOfferedGradesToEligibleClassesForForm(phase)
}

// narrowOfferedGradesToEligibleClassesForForm derives the form's grade options
// from a class-only eligibility restriction. A phase restricted to "3a" but with
// an empty eligible_grade_levels leaves the grade select offering every grade
// 1..grade_level_max, while only grade 3 can ever declare an eligible class: the
// form filters the class pick list by the selected grade, so grade 1 shows no
// class at all (or "Klasse offen") and the submit then fails with
// class_not_eligible after the whole form was filled in. That is the same dead
// end narrowOfferedClassesToEligibleForForm removes on the class side, one level
// up (#1663).
//
// Only ever narrows, and only for the self-service loads that narrowing serves:
// an explicit eligible_grade_levels is left untouched (Phase.Validate already
// keeps the two lists consistent, so it is never wider than the class-derived
// set), and an eligible class with no derivable grade ("Bienen") is compatible
// with every grade — exactly as Phase.Validate treats it — so it suppresses the
// narrowing entirely rather than hiding the grade a prefixless class belongs to.
//
// Presentation only: this mutates a phase already cleared for a form load, and
// Submit reloads the phase independently, so the derived list never reaches
// validation or the database. The class gate stays the enforcing side.
func narrowOfferedGradesToEligibleClassesForForm(phase *enrollmentModels.Phase) {
	if phase == nil || len(phase.EligibleGradeLevels) > 0 {
		return
	}
	grades := make([]int, 0, len(phase.EligibleSchoolClasses))
	seen := make(map[int]struct{}, len(phase.EligibleSchoolClasses))
	for _, c := range phase.EligibleSchoolClasses {
		if strings.TrimSpace(c) == "" {
			continue
		}
		prefix := schoolclass.GradePrefix(c)
		if prefix == "" {
			// Grade-agnostic eligible class: every grade stays satisfiable.
			return
		}
		level, err := strconv.Atoi(prefix)
		if err != nil {
			return
		}
		if _, ok := seen[level]; ok {
			continue
		}
		seen[level] = struct{}{}
		grades = append(grades, level)
	}
	if len(grades) == 0 {
		return
	}
	sort.Ints(grades)
	phase.EligibleGradeLevels = grades
}

// LoadPublicPhaseWithLateInvite is the anonymous public form-load gate: it
// rejects every audience-restricted phase (linked_parents and
// existing_students) outright — the zero access value grants neither.
func (s *requestService) LoadPublicPhaseWithLateInvite(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (*enrollmentModels.Phase, error) {
	return s.loadEditablePhaseWithLateInvite(ctx, phaseID, now, lateInviteToken, EnrolleeAudienceAccess{})
}

// LoadEnrolleePhaseWithLateInvite is the authenticated parent-portal
// form-load gate. It behaves exactly like the public gate except it also
// admits the restricted audiences the caller's resolved guardian facts cover
// (see EnrolleeAudienceAccess) — the submit path still enforces
// GuardianSubmitEligible + the per-child eligibility rules on top.
func (s *requestService) LoadEnrolleePhaseWithLateInvite(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string, access EnrolleeAudienceAccess) (*enrollmentModels.Phase, error) {
	return s.loadEditablePhaseWithLateInvite(ctx, phaseID, now, lateInviteToken, access)
}

func (s *requestService) LoadManualEnrollmentPhase(ctx context.Context, phaseID int64) (*enrollmentModels.Phase, error) {
	return s.loadPhaseForEditableRequest(ctx, phaseID)
}

func (s *requestService) PublicActiveSchema(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (*enrollmentModels.FormSchema, error) {
	phase, err := s.LoadPublicPhaseWithLateInvite(ctx, phaseID, now, lateInviteToken)
	if err != nil {
		return nil, err
	}
	if phase.FormSchemaID == nil {
		return nil, ErrNoActiveSchema
	}
	schema, err := s.FormSchemaRepo.FindByID(ctx, *phase.FormSchemaID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoActiveSchema
		}
		return nil, err
	}
	return schema, nil
}

func (s *requestService) loadPhaseForEditableRequest(ctx context.Context, phaseID int64) (*enrollmentModels.Phase, error) {
	if !s.isEnrollmentEnabled(ctx) {
		return nil, ErrEnrollmentDisabled
	}
	return s.loadPhaseForSubmission(ctx, phaseID)
}

func buildLegalBlocks(texts LegalTexts) []LegalBlock {
	blocks := make([]LegalBlock, 0, 4)
	agbText := legalAGBBlockText(texts)
	if texts.TermsEnabled && agbText != "" {
		blocks = append(blocks, LegalBlock{
			Key:       enrollmentModels.ConsentKeyAGB,
			Kind:      "terms",
			Title:     "AGB / Teilnahmebedingungen",
			Label:     "Ich akzeptiere die AGB / Teilnahmebedingungen / den Ganztag Info-Brief.",
			Text:      agbText,
			Required:  true,
			SortOrder: 10,
			Source:    enrollmentModels.LegalBlockSourceStandard,
		})
	}
	if texts.DSGVOEnabled && texts.DSGVO != "" {
		blocks = append(blocks, LegalBlock{
			Key:       enrollmentModels.ConsentKeyDataProcessing,
			Kind:      "privacy_notice",
			Title:     "Datenschutzinformation",
			Label:     "Ich habe die Datenschutzinformation der Schule zur Kenntnis genommen.",
			Text:      texts.DSGVO,
			Required:  true,
			SortOrder: 20,
			Source:    enrollmentModels.LegalBlockSourceStandard,
		})
	}
	if texts.PhotoEnabled && texts.Photo != "" {
		blocks = append(blocks, LegalBlock{
			Key:       enrollmentModels.ConsentKeyPhoto,
			Kind:      "consent",
			Title:     "Fotoeinwilligung",
			Label:     "Mein Kind darf bei Schulveranstaltungen fotografiert werden. Diese Einwilligung ist freiwillig und jederzeit mit Wirkung für die Zukunft widerrufbar.",
			Text:      texts.Photo,
			Required:  false,
			SortOrder: 30,
			Source:    enrollmentModels.LegalBlockSourceStandard,
		})
	}
	if texts.EmailContactEnabled && texts.EmailContact != "" {
		blocks = append(blocks, LegalBlock{
			Key:       enrollmentModels.ConsentKeyEmailContact,
			Kind:      "notice",
			Title:     "E-Mail-Kontakt",
			Label:     "Die Schule nutzt Ihre E-Mail-Adresse für Rückfragen und Status-Benachrichtigungen zu dieser Anmeldung.",
			Text:      texts.EmailContact,
			Required:  false,
			SortOrder: 40,
			Source:    enrollmentModels.LegalBlockSourceStandard,
		})
	}
	return blocks
}

func legalAGBDisplayMode(mode string) string {
	if mode == configModel.EnrollmentLegalAGBDisplayModePDF {
		return configModel.EnrollmentLegalAGBDisplayModePDF
	}
	return configModel.EnrollmentLegalAGBDisplayModeText
}

func legalAGBBlockText(texts LegalTexts) string {
	switch legalAGBDisplayMode(texts.AGBDisplayMode) {
	case configModel.EnrollmentLegalAGBDisplayModePDF:
		if texts.AGBDocumentURL == "" {
			return ""
		}
		return fmt.Sprintf("Die AGB / Teilnahmebedingungen sind als PDF-Datei hinterlegt: [AGB-Dokument öffnen](%s)", PublicEnrollmentLegalDocumentURL(texts.AGBDocumentURL))
	default:
		return texts.AGB
	}
}

// PublicEnrollmentLegalDocumentURL maps stored upload paths of enrollment
// legal documents onto their public serving routes. Non-upload URLs pass
// through unchanged. Shared with api/config's legal-AGB settings handler.
func PublicEnrollmentLegalDocumentURL(storedURL string) string {
	const globalUploadPrefix = "/uploads/enrollment-legal-documents/"
	const globalPublicPrefix = "/api/public/enrollment-legal-documents/"
	if strings.HasPrefix(storedURL, globalUploadPrefix) {
		return globalPublicPrefix + strings.TrimPrefix(storedURL, globalUploadPrefix)
	}
	const formUploadPrefix = "/uploads/enrollment-form-legal-documents/"
	const formPublicPrefix = "/api/public/enrollment-form-legal-documents/"
	if strings.HasPrefix(storedURL, formUploadPrefix) {
		return formPublicPrefix + strings.TrimPrefix(storedURL, formUploadPrefix)
	}
	return storedURL
}

func buildTemplateLegalBlocks(configured []enrollmentModels.FormLegalBlock) []LegalBlock {
	blocks := make([]LegalBlock, 0, len(configured))
	for _, block := range configured {
		if !block.Enabled {
			continue
		}
		blocks = append(blocks, LegalBlock{
			Key:       block.Key,
			Kind:      block.Kind,
			Title:     block.Title,
			Label:     block.Label,
			Text:      templateLegalBlockText(block),
			Required:  block.Required,
			SortOrder: block.SortOrder,
			Source:    block.Source,
		})
	}
	// The editor writes blocks in display order, but API-written templates
	// may store them out of order — enforce sort_order at render time.
	sort.SliceStable(blocks, func(i, j int) bool {
		return blocks[i].SortOrder < blocks[j].SortOrder
	})
	return blocks
}

func templateLegalBlockText(block enrollmentModels.FormLegalBlock) string {
	if block.Key == enrollmentModels.ConsentKeyAGB &&
		block.DisplayMode == enrollmentModels.LegalBlockDisplayModePDF &&
		strings.TrimSpace(block.DocumentURL) != "" {
		return fmt.Sprintf("Die AGB / Teilnahmebedingungen sind als PDF-Datei hinterlegt: [AGB-Dokument öffnen](%s)", PublicEnrollmentLegalDocumentURL(block.DocumentURL))
	}
	return block.Text
}

// legalTermsEnabled reports whether the tenant has switched on the AGB /
// Teilnahmebedingungen block. Missing overrides default off; settings errors
// fail closed because this setting decides whether a required legal block is
// rendered and enforced.
func (s *requestService) legalTermsEnabled(ctx context.Context) (bool, error) {
	return s.legalBlockEnabled(ctx, configModel.KeyEnrollmentLegalTermsEnabled, "AGB terms")
}

func (s *requestService) legalBlockEnabled(ctx context.Context, key string, label string) (bool, error) {
	if s.Settings == nil {
		return false, nil
	}
	has, err := s.Settings.HasTenantOverride(ctx, key)
	if err != nil {
		return false, fmt.Errorf("check %s setting override: %w", label, err)
	}
	if !has {
		return false, nil
	}
	v, err := s.Settings.ResolveBool(ctx, key)
	if err != nil {
		return false, fmt.Errorf("resolve %s setting: %w", label, err)
	}
	return v, nil
}

// resolveSubmissionLegalBlocks returns the legal blocks a submission is
// validated and persisted against: the template's enabled blocks when the
// pinned schema declares at least one, otherwise the tenant-wide settings
// blocks. Zero enabled template blocks fall back to settings (same rule as
// LegalTextsForPhaseWithLateInvite) so an all-disabled snapshot can never erase the
// tenant's consent contract.
func (s *requestService) resolveSubmissionLegalBlocks(ctx context.Context, schema *enrollmentModels.FormSchema) ([]LegalBlock, error) {
	if schema != nil && len(schema.LegalBlocks) > 0 {
		if blocks := buildTemplateLegalBlocks(schema.LegalBlocks); len(blocks) > 0 {
			return blocks, nil
		}
	}
	texts, err := s.LegalTexts(ctx)
	if err != nil {
		return nil, err
	}
	return texts.Blocks, nil
}

// requiredConsentKeys extracts the keys the parent must accept from the
// resolved block list, so a hidden/empty block never blocks submit
// server-side.
func requiredConsentKeys(blocks []LegalBlock) []string {
	required := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Required {
			required = append(required, block.Key)
		}
	}
	return required
}

// filterConsentFlags drops every consent key the resolved legal-block
// contract doesn't declare. The flags are legally meaningful data, so
// arbitrary client-sent keys must not be persisted.
func filterConsentFlags(flags map[string]any, blocks []LegalBlock) map[string]any {
	if len(flags) == 0 {
		return flags
	}
	allowed := make(map[string]bool, len(blocks))
	for _, block := range blocks {
		allowed[block.Key] = true
	}
	out := make(map[string]any, len(flags))
	for key, value := range flags {
		if allowed[key] {
			out[key] = value
		}
	}
	return out
}

// CollectsSchoolClass resolves the effective school-class capability for
// public form-load endpoints.
func (s *requestService) CollectsSchoolClass(ctx context.Context) (bool, error) {
	if s.Settings == nil {
		return false, errors.New("enrollment settings resolver is not configured")
	}
	collectGrade, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentCollectGradeLevel)
	if err != nil {
		return false, fmt.Errorf("resolve %s: %w", configModel.KeyEnrollmentCollectGradeLevel, err)
	}
	collectClass, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentCollectSchoolClass)
	if err != nil {
		return false, fmt.Errorf("resolve %s: %w", configModel.KeyEnrollmentCollectSchoolClass, err)
	}
	return collectGrade && collectClass, nil
}

// FormCapabilities resolves the three settings that govern the enrollment
// form's core inputs. collect_school_class is deliberately made ineffective
// while grade collection is disabled because a class without its grade is
// ambiguous and cannot be validated safely.
func (s *requestService) FormCapabilities(ctx context.Context) (FormCapabilities, error) {
	if s.Settings == nil {
		return FormCapabilities{}, errors.New("enrollment settings resolver is not configured")
	}
	collectGrade, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentCollectGradeLevel)
	if err != nil {
		return FormCapabilities{}, fmt.Errorf("resolve %s: %w", configModel.KeyEnrollmentCollectGradeLevel, err)
	}
	collectClass, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentCollectSchoolClass)
	if err != nil {
		return FormCapabilities{}, fmt.Errorf("resolve %s: %w", configModel.KeyEnrollmentCollectSchoolClass, err)
	}
	offeringsEnabled, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentCareOfferingsEnabled)
	if err != nil {
		return FormCapabilities{}, fmt.Errorf("resolve %s: %w", configModel.KeyEnrollmentCareOfferingsEnabled, err)
	}
	return FormCapabilities{
		CollectGradeLevel:    collectGrade,
		CollectSchoolClass:   collectGrade && collectClass,
		CareOfferingsEnabled: offeringsEnabled,
	}, nil
}

func normalizeSubmissionForCapabilities(req *SubmitRequest, capabilities FormCapabilities) error {
	for i := range req.Children {
		child := &req.Children[i]
		if !capabilities.CollectGradeLevel {
			child.TargetGradeLevel = nil
			child.TargetSchoolClass = nil
		}
		if !capabilities.CareOfferingsEnabled {
			if len(child.OfferingIDs) > 0 || len(child.OfferingDays) > 0 {
				return ErrCareOfferingsDisabled
			}
			child.OfferingIDs = nil
			child.OfferingDays = nil
		}
	}
	return nil
}

func EffectiveFormCapabilities(capabilities FormCapabilities, offerings []*enrollmentModels.CareOffering) FormCapabilities {
	if capabilities.CareOfferingsEnabled {
		for _, offering := range offerings {
			if offering != nil && offering.AvailabilityRule.RequiresGradeLevel() {
				capabilities.CollectGradeLevel = true
				break
			}
		}
	}
	capabilities.CollectSchoolClass = capabilities.CollectGradeLevel && capabilities.CollectSchoolClass
	return capabilities
}

// validateAndNormalizeSchoolClasses enforces the concrete-class rules
// (issue #1833) and mutates each child's TargetSchoolClass in place to
// the value that should be persisted:
//
//   - setting off: the concrete class is never collected -> force nil.
//   - grade < 2: grade 1 (and grade-less rows) stay grade-level only ->
//     force nil regardless of what the client sent.
//   - grade >= 2, value provided: must be one of the phase's
//     AvailableSchoolClasses (exact, trimmed) -> else reject.
//   - grade >= 2, value empty: allowed as "Klasse offen" unless the
//     phase's RequireSchoolClass makes it mandatory -> then reject.
//
// Trims and collapses empty strings to nil so "" never reaches the DB.
// validatePhaseEligibility enforces the per-phase eligibility config
// (#1663) server-side for the self-service paths (public + parent).
// Trusted paths — admin manual enrollment and late invites, both
// recognizable by AllowClosedPhase — bypass eligibility entirely: an
// admin acts deliberately (exception cases must stay possible, same as
// the paper form), and a late invite is an explicit personal invitation
// that outranks the phase's audience config.
//
// That bypass covers the AUDIENCE gate only. It does not extend to the
// per-child re-enrollment authorization in
// assertGuardianMayReEnrollStudent: whichever path pins a live student,
// the request's guardian identity must already hold
// parent_portal.enrollment.submit on that student (admin manual excepted,
// since staff act with their own authorization). A late-invite token is
// minted per phase and email and proves nothing about WHICH child, so
// letting it through both gates would have handed an invited parent the
// child of anyone whose name and birthday they could read off a class
// list (#1663).
//
//   - linked_parents audience: requires an authenticated parent whose
//     guardian relationship at the tenant grants
//     parent_portal.enrollment.submit. The parent handler resolves that
//     fact into GuardianSubmitEligible; anonymous submissions carry
//     neither and are rejected.
//   - eligible_school_classes: when non-empty, every child must declare
//     one of the listed classes.
//   - new_students audience: a child matching an already-enrolled student
//     (name + birthday) is rejected. "Enrolled" spans active AND pending
//     students, so a child approved-but-not-yet-activated still blocks a
//     second submission. Best-effort by design — it blocks the honest
//     mistake, not a determined false declaration, exactly like the paper
//     form it replaces.
//   - existing_students audience: the inverse — a child with NO matching
//     already-enrolled student is rejected, so every submitted child must
//     already be enrolled (re-enrollment / renewal). Same name+birthday
//     lookup, same best-effort semantics.
//
// existing_students carries the SAME authentication gate as linked_parents.
// Unlike every other audience, a submission there does not create a new
// record: resolveMatchedStudentID pins a LIVE student that approval renews
// and attaches the submitter to as a guardian. Name + date of birth is a
// recognition signal, not an authentication one — a class list or a school
// festival is enough to learn both — so accepting it anonymously handed
// portal access to a stranger's child to anyone who could guess it. Requiring
// an authenticated, submit-eligible guardian account makes the per-student
// probe in assertGuardianMayReEnrollStudent reachable, which is what actually
// decides WHICH child that account may renew. Parents without a portal
// account are served by the two deliberate override paths that already skip
// this gate via AllowClosedPhase: a late invite (a per-recipient token the
// school mints) and admin manual enrollment (#1663).
func (s *requestService) validatePhaseEligibility(ctx context.Context, phase *enrollmentModels.Phase, req SubmitRequest) error {
	if req.AllowClosedPhase {
		return nil
	}
	if audienceRequiresGuardianAccount(phase.Audience) &&
		(req.GuardianAccountID == nil || !req.GuardianSubmitEligible) {
		return ErrPhaseNotEligible
	}
	return s.validatePhaseChildEligibility(ctx, phase, req)
}

// audienceRequiresGuardianAccount reports whether a phase audience may only be
// submitted by an authenticated, submit-eligible guardian account. Both
// audiences it covers are also the ones the parents-portal picker hides from
// accounts without that permission and the ones the public form gate refuses to
// bootstrap, so the three stay in agreement (#1663).
func audienceRequiresGuardianAccount(audience string) bool {
	return audience == enrollmentModels.PhaseAudienceLinkedParents ||
		audience == enrollmentModels.PhaseAudienceExistingStudents
}

// EnrolleeAudienceAccess carries the caller's per-audience form-load
// authority. The restricted audiences do NOT share one gate (#1663):
//
//   - linked_parents needs any guardian relationship at the school that
//     grants parent_portal.enrollment.submit — a relationship to an inactive
//     child still qualifies, because enrolling a new sibling is the point.
//   - existing_students additionally needs that relationship to point at a
//     still-enrolled (active/pending) child: without one, the submit-time
//     student matcher can only fail (ErrChildNotEnrolled /
//     ErrChildEnrollmentNotPermitted).
//
// The two flags therefore mirror guard.has_submit_permission and
// guard.has_enrolled_submit_permission in the parents-portal picker
// (EnrollablePhaseRepository.ListEnrollable) one-for-one, so the form gate
// admits exactly the phases the picker advertises. The zero value is the
// anonymous caller: no restricted audience at all.
type EnrolleeAudienceAccess struct {
	LinkedParents    bool
	ExistingStudents bool
}

// AllowsAudience reports whether the caller may load a form for a phase with
// this audience. Unrestricted audiences (open / new_students) always pass; a
// restricted one passes only on its matching flag. A restricted audience with
// no flag of its own fails closed, so adding one to
// audienceRequiresGuardianAccount can never silently open the form gate.
func (a EnrolleeAudienceAccess) AllowsAudience(audience string) bool {
	if !audienceRequiresGuardianAccount(audience) {
		return true
	}
	switch audience {
	case enrollmentModels.PhaseAudienceLinkedParents:
		return a.LinkedParents
	case enrollmentModels.PhaseAudienceExistingStudents:
		return a.ExistingStudents
	default:
		return false
	}
}

// validatePhaseChildEligibility enforces the per-child eligibility gates
// (eligible_school_classes + new_students already-enrolled) independently of
// the linked_parents *audience* gate. Submit runs the full
// validatePhaseEligibility; the editable-request path calls this directly so
// a status-token holder cannot edit a child into an ineligible class or an
// already-enrolled identity, while the linked_parents authorization stays
// preserved from the original submission (#1663). Callers must run
// validateAndNormalizeSchoolClasses first so this sees the persisted class.
func (s *requestService) validatePhaseChildEligibility(ctx context.Context, phase *enrollmentModels.Phase, req SubmitRequest) error {
	if err := validateChildGradeEligibility(phase, req.Children); err != nil {
		return err
	}
	if err := validateChildClassEligibility(phase, req.Children); err != nil {
		return err
	}

	for i := range req.Children {
		if err := s.validateChildEnrolledStatus(ctx, phase, req.TenantID, req.Children[i], i); err != nil {
			return err
		}
	}
	return nil
}

// validateChildEnrolledStatus applies the audience's enrolled-status gate to a
// single child, reporting failures under childIndex so the form can point at
// the right row (#1663).
//
// new_students and existing_students are the two child-scoped enrolled-status
// gates: new_students rejects a child that already matches an enrolled record;
// existing_students rejects a child that does NOT — every submitted child must
// already be enrolled (a re-enrollment / renewal phase). Both consult the same
// name+birthday lookup, so they share one probe per child.
//
// Split out per child because the change-request path applies it to a SUBSET —
// only the children whose identity an edit rewrites — and still needs the
// original index in the error (see validateChangedChildIdentityEligibility).
func (s *requestService) validateChildEnrolledStatus(
	ctx context.Context,
	phase *enrollmentModels.Phase,
	tenantID int64,
	child SubmitChild,
	childIndex int,
) error {
	if s.StudentRepo == nil ||
		(phase.Audience != enrollmentModels.PhaseAudienceNewStudents &&
			phase.Audience != enrollmentModels.PhaseAudienceExistingStudents) {
		return nil
	}
	exists, err := s.StudentRepo.ExistsEnrolledByNameAndBirthday(ctx, tenantID,
		child.FirstName, child.LastName, child.DateOfBirth)
	if err != nil {
		return fmt.Errorf("submit: check enrolled student for child %d: %w", childIndex, err)
	}
	if phase.Audience == enrollmentModels.PhaseAudienceNewStudents && exists {
		return fmt.Errorf("%w: child %d", ErrChildAlreadyEnrolled, childIndex)
	}
	if phase.Audience == enrollmentModels.PhaseAudienceExistingStudents && !exists {
		return fmt.Errorf("%w: child %d", ErrChildNotEnrolled, childIndex)
	}
	return nil
}

// validateChildClassEligibility enforces the phase's eligible_school_classes
// restriction: when the list is non-empty, every child must declare one of the
// listed classes. Pure — no repos — because it is the one child-eligibility
// gate that stays valid at EVERY point of a request's life, so the
// change-request path applies it too (#1663).
//
// eligible_school_classes may be a proper subset of available_school_classes,
// so without this an approved change request could persist an
// available-but-ineligible class. The enrolled-status gates it sits next to
// cannot run unconditionally on the change-request path — re-running the
// new_students "must not already be enrolled" probe on a child whose approval
// JUST created that student would reject every later change request on it — so
// that path applies them only to children whose identity the edit rewrites
// (validateChangedChildIdentityEligibility).
func validateChildClassEligibility(phase *enrollmentModels.Phase, children []SubmitChild) error {
	eligible := make(map[string]struct{}, len(phase.EligibleSchoolClasses))
	for _, c := range phase.EligibleSchoolClasses {
		if t := strings.TrimSpace(c); t != "" {
			eligible[t] = struct{}{}
		}
	}
	if len(eligible) == 0 {
		return nil
	}
	for i := range children {
		declared := ""
		if children[i].TargetSchoolClass != nil {
			declared = strings.TrimSpace(*children[i].TargetSchoolClass)
		}
		if declared == "" {
			return fmt.Errorf("%w: child %d declares no school class", ErrChildClassNotEligible, i)
		}
		if _, ok := eligible[declared]; !ok {
			return fmt.Errorf("%w: child %d class %q", ErrChildClassNotEligible, i, declared)
		}
	}
	return nil
}

// validateChildGradeEligibility enforces the phase's eligible_grade_levels
// restriction: when the list is non-empty, every child must declare one of the
// listed grades. This is the enforcement half of the whole-grade phase
// ("nur Klasse 3") — a case the concrete-class list cannot express, because a
// school that collects only the grade level never has a class to compare, and
// enumerating 3a/3b goes stale as soon as a class is added or renamed (#1663).
//
// Pure, like validateChildClassEligibility, and applied at exactly the same
// points: submit, the editable-request path, and the change-request path. A
// child's grade is stable data the parent declares in every one of them, so
// re-checking it never rejects an edit that creation allowed — unlike the
// enrolled-status probes.
//
// A missing grade is a rejection, not a pass: normalizeSubmissionForCapabilities
// nils the grade out when collect_grade_level is off, and the phase-side
// collectability guard is what keeps that pair from being configurable. Treating
// nil as "eligible" would silently disable the restriction instead.
func validateChildGradeEligibility(phase *enrollmentModels.Phase, children []SubmitChild) error {
	if len(phase.EligibleGradeLevels) == 0 {
		return nil
	}
	eligible := make(map[int]struct{}, len(phase.EligibleGradeLevels))
	for _, level := range phase.EligibleGradeLevels {
		eligible[level] = struct{}{}
	}
	for i := range children {
		if children[i].TargetGradeLevel == nil {
			return fmt.Errorf("%w: child %d declares no grade level", ErrChildGradeNotEligible, i)
		}
		declared := int(*children[i].TargetGradeLevel)
		if _, ok := eligible[declared]; !ok {
			return fmt.Errorf("%w: child %d grade %d", ErrChildGradeNotEligible, i, declared)
		}
	}
	return nil
}

// isTrustedEnrollmentSource reports whether a persisted request was created
// through a deliberate override path (admin manual enrollment or a late
// invite). Submit skips eligibility for those via AllowClosedPhase, so the
// editable-request path must keep that override when re-checking child
// eligibility rather than newly rejecting an edit the original creation
// allowed.
func isTrustedEnrollmentSource(source string) bool {
	switch strings.TrimSpace(source) {
	case enrollmentModels.RequestSourceLateInvite, enrollmentModels.RequestSourceAdminManual:
		return true
	default:
		return false
	}
}

// resolveMatchedStudentID returns the already-enrolled student a submitted
// child was matched to, but only for existing_students phases and only when the
// name+birthday lookup is unambiguous (#1663). The result is stamped onto
// request_children so approval renews the matched student instead of creating a
// duplicate Person/Student.
//
// The repository returns a non-nil ID ONLY for exactly one enrolled match and
// collapses both "no match" and "ambiguous multi-match" to nil. Those two nil
// cases are NOT equivalent: a genuine zero match is a legitimate fresh-create
// (admin-manual / late-invite bypass the enrolled gate and may deliberately
// create), but an ambiguous match must be rejected — left as nil it would flow
// to the decision service's fresh-create branch and add a THIRD duplicate on
// top of the two records that already collide, the exact data this flow exists
// to prevent. So on a nil ID we probe existence to tell the two apart and
// reject only the ambiguous case.
func (s *requestService) resolveMatchedStudentID(ctx context.Context, tenantID int64, phase *enrollmentModels.Phase, childIndex int, child SubmitChild) (*int64, error) {
	if s.StudentRepo == nil || phase.Audience != enrollmentModels.PhaseAudienceExistingStudents {
		return nil, nil
	}
	id, err := s.StudentRepo.FindEnrolledStudentIDByNameAndBirthday(ctx, tenantID,
		child.FirstName, child.LastName, child.DateOfBirth)
	if err != nil {
		return nil, fmt.Errorf("submit: resolve matched student: %w", err)
	}
	if id != nil {
		return id, nil
	}
	exists, err := s.StudentRepo.ExistsEnrolledByNameAndBirthday(ctx, tenantID,
		child.FirstName, child.LastName, child.DateOfBirth)
	if err != nil {
		return nil, fmt.Errorf("submit: resolve matched student ambiguity: %w", err)
	}
	if exists {
		// A match exists but the resolver could not pin a single record:
		// the identity is ambiguous, so reject rather than duplicate.
		return nil, fmt.Errorf("%w: child %d", ErrChildEnrollmentAmbiguous, childIndex)
	}
	return nil, nil
}

// assertExistingStudentMatchResolved closes the gap between the pre-write
// enrolled-student gate and the pinned match (#1663). On an existing_students
// phase, validatePhaseChildEligibility already proved every child matches an
// enrolled student; resolveMatchedStudentID then deliberately collapses a zero
// match to "no pin" because the TRUSTED paths (admin manual / late invite) skip
// that gate and may legitimately create a fresh record.
//
// For an ORDINARY submission the two reads must agree. When they don't — the
// activation/deactivation scheduler flipped the student out of active/pending
// between them — storing the request with matched_student_id = NULL would make
// approval create a duplicate Person + Student for a child the school already
// has, the exact outcome this audience exists to prevent. Reject instead: the
// submission is no longer valid for this phase.
//
// eligibilityEnforced mirrors whichever gate the caller ran, so a trusted path
// keeps its override untouched.
func assertExistingStudentMatchResolved(
	phase *enrollmentModels.Phase,
	matchedStudentID *int64,
	eligibilityEnforced bool,
	childIndex int,
) error {
	if !eligibilityEnforced ||
		matchedStudentID != nil ||
		phase.Audience != enrollmentModels.PhaseAudienceExistingStudents {
		return nil
	}
	return fmt.Errorf("%w: child %d", ErrChildNotEnrolled, childIndex)
}

// reEnrollmentSubmitter is the identity a pinned existing-student re-enrollment
// is authorized against. Exactly one of the two identities decides, and which
// one is a property of the flow, never of the payload: a parents-portal submit
// carries the authenticated account, every other flow carries only the guardian
// email the request is bound to (for a late invite, the email the school minted
// the token for — findLateInviteForSubmit rejects any mismatch).
//
// AdminManaged marks the staff-authorized manual-enrollment flow, the one path
// that may deliberately pin a student the submitting guardian has no
// relationship with yet. It is derived from the submission source, which is set
// by the service (Submit for late invites, CreateManualApprovedEnrollment for
// admin manual) and is never bound from the wire.
type reEnrollmentSubmitter struct {
	GuardianAccountID *int64
	GuardianEmail     string
	AdminManaged      bool
}

// reEnrollmentSubmitterFor builds the identity from a submission's source and
// guardian fields. Any source other than admin_manual is self-service as far as
// this gate is concerned — a late invite proves the school invited THIS EMAIL
// into a closed phase, which is not the same fact as authority over a specific
// enrolled child.
func reEnrollmentSubmitterFor(submissionSource string, guardianAccountID *int64, guardianEmail string) reEnrollmentSubmitter {
	return reEnrollmentSubmitter{
		GuardianAccountID: guardianAccountID,
		GuardianEmail:     strings.ToLower(strings.TrimSpace(guardianEmail)),
		AdminManaged:      normalizedSubmissionSource(submissionSource) == enrollmentModels.RequestSourceAdminManual,
	}
}

// assertGuardianMayReEnrollStudent enforces the per-child authorization gate for
// existing_students re-enrollment (#1663). resolveMatchedStudentID may pin a
// concrete already-enrolled student that approval would RENEW and attach the
// submitting guardian to. The invariant this gate holds is therefore:
//
//	a request may renew student S only if the guardian identity ON THAT REQUEST
//	already holds parent_portal.enrollment.submit on its own relationship to S.
//
// Because approval attaches that same identity (account ID when present, else
// guardian email — see decision_service), an approved renewal can never widen
// anyone's access: whoever gets attached already had authority over S.
//
// Both identities are checked, never mixed:
//
//   - Authenticated parent submit: the account must hold the permission on S.
//     The coarse school-wide GuardianSubmitEligible flag admits the parent to the
//     phase but would otherwise let a parent permitted for child A renew — and
//     bind themselves to — child B.
//   - Accountless submit (a late invite, whose recipient typically has no portal
//     account): the request's guardian email must resolve to a guardian profile at
//     the school whose relationship to S grants the permission. A late-invite
//     token is minted per phase and email and says nothing about WHICH child, so
//     without this probe an invited parent could type a stranger's enrolled child's
//     name and birthday — both readable off a class list — and be attached to that
//     child on approval.
//
// The single deliberate bypass is AdminManaged: staff act with their own
// authorization and must be able to re-enroll a child whose guardian is not yet
// recorded, exactly like the paper form.
//
// No-ops when there is no match (nil studentID → fresh create). Fails closed
// otherwise: no authorizer wired, or a submission carrying neither identity, is
// a misconfiguration, not a bypass.
func (s *requestService) assertGuardianMayReEnrollStudent(ctx context.Context, submitter reEnrollmentSubmitter, matchedStudentID *int64, tenantID int64, childIndex int) error {
	if matchedStudentID == nil || submitter.AdminManaged {
		return nil
	}
	if s.GuardianAuthorizer == nil {
		return fmt.Errorf("%w: child %d", ErrChildEnrollmentNotPermitted, childIndex)
	}
	var (
		granted bool
		err     error
	)
	switch {
	case submitter.GuardianAccountID != nil:
		granted, err = s.GuardianAuthorizer.AccountHasStudentPermission(ctx, *submitter.GuardianAccountID,
			*matchedStudentID, tenantID, authorize.GuardianPermissionEnrollmentSubmit)
	case submitter.GuardianEmail != "":
		granted, err = s.GuardianAuthorizer.GuardianEmailHasStudentPermission(ctx, submitter.GuardianEmail,
			*matchedStudentID, tenantID, authorize.GuardianPermissionEnrollmentSubmit)
	default:
		// No identity at all on a submission that pinned a live student: nothing
		// to authorize against, so nothing may be renewed.
		return fmt.Errorf("%w: child %d", ErrChildEnrollmentNotPermitted, childIndex)
	}
	if err != nil {
		return fmt.Errorf("submit: verify guardian re-enrollment permission for child %d: %w", childIndex, err)
	}
	if !granted {
		return fmt.Errorf("%w: child %d", ErrChildEnrollmentNotPermitted, childIndex)
	}
	return nil
}

// guardMatchedStudentUnique rejects a submission/edit whose child resolved to an
// already-enrolled student that another active (non-rejected, non-withdrawn)
// request in the same phase already targets. The email-scoped dedup check keys
// on guardian_email, so two guardians with different emails submitting the same
// existing child both slip through it yet pin the same matched_student_id;
// approving both would renew/overwrite one live student twice and duplicate its
// care-offering enrollments. The phase-wide advisory lock makes the
// check-then-insert race-free even across those distinct email locks. No-ops
// when there is no pin (nil → fresh create, nothing to collide on). Enforced
// regardless of the block/warn/ignore duplicate policy — it protects a live
// student record, not just parent convenience (#1663).
//
// excludeRequestChildID (0 = none) is for callers re-checking an ALREADY
// persisted, already-active row — the change-request path — which would
// otherwise collide with its own pin. Insert paths pass 0.
func (s *requestService) guardMatchedStudentUnique(ctx context.Context, phaseID int64, matchedStudentID *int64, excludeRequestChildID int64, childIndex int) error {
	if matchedStudentID == nil {
		return nil
	}
	if err := s.RequestRepo.AcquireExistingStudentMatchLock(ctx, phaseID); err != nil {
		return fmt.Errorf("submit: acquire existing-student match lock: %w", err)
	}
	has, err := s.RequestRepo.HasActiveRequestForMatchedStudent(ctx, phaseID, *matchedStudentID, excludeRequestChildID)
	if err != nil {
		return fmt.Errorf("submit: matched-student duplicate check for child %d: %w", childIndex, err)
	}
	if has {
		return fmt.Errorf("%w: child %d", ErrExistingStudentAlreadyRequested, childIndex)
	}
	return nil
}

// hasRolloverGeneratedChild reports whether any of the persisted children was
// carried forward by the rollover flow (RolloverSourceChildID set). Rollover
// requests are generated with submission_source='public' but must not be held
// to the self-service eligibility gates on renewal (#1663).
func hasRolloverGeneratedChild(children []*enrollmentModels.RequestChild) bool {
	for _, child := range children {
		if child.RolloverSourceChildID != nil {
			return true
		}
	}
	return false
}

func (s *requestService) validateAndNormalizeSchoolClasses(ctx context.Context, phase *enrollmentModels.Phase, children []SubmitChild) error {
	collect, err := s.CollectsSchoolClass(ctx)
	if err != nil {
		return fmt.Errorf("resolve collect_school_class: %w", err)
	}
	allowed := make(map[string]struct{}, len(phase.AvailableSchoolClasses))
	for _, c := range phase.AvailableSchoolClasses {
		if t := strings.TrimSpace(c); t != "" {
			allowed[t] = struct{}{}
		}
	}
	for i := range children {
		// Trim + collapse to nil first so downstream only sees a real value.
		var chosen string
		if children[i].TargetSchoolClass != nil {
			chosen = strings.TrimSpace(*children[i].TargetSchoolClass)
		}
		grade := 0
		if children[i].TargetGradeLevel != nil {
			grade = int(*children[i].TargetGradeLevel)
		}
		// Grade 1 stays grade-level-only (#1833) UNLESS the phase collects a
		// grade-1 class (CollectsGrade1Class), in which case it is collected
		// and validated exactly like grade >= 2 (#1663). Grade-less rows
		// (grade 0) never collect a class. Grade >= 2 is unchanged.
		if !collect || grade < 1 || (grade == 1 && !CollectsGrade1Class(phase)) {
			children[i].TargetSchoolClass = nil
			continue
		}
		if chosen == "" {
			// Only force a pick when this grade actually has a class to
			// pick. A phase may require classes yet only offer them for some
			// grades (e.g. ["3a"] while grade 2 is still selectable). For a
			// grade with no matching offered class the required pick is
			// unsatisfiable — the grade-prefix check below would reject every
			// offered class — so "Klasse offen" is the only valid outcome
			// rather than a submission that can never succeed. Issue #1833.
			if phase.RequireSchoolClass && gradeHasSelectableClass(allowed, grade) {
				return fmt.Errorf("%w: child %d missing target_school_class", ErrInvalidSubmission, i)
			}
			children[i].TargetSchoolClass = nil
			continue
		}
		if _, ok := allowed[chosen]; !ok {
			return fmt.Errorf("%w: child %d target_school_class %q not offered by this phase", ErrInvalidSubmission, i, chosen)
		}
		// The phase's pick list mixes classes from every grade
		// ("2a", "2b", "3a"), and the client sends the whole list for
		// every grade, so "in the list" is not enough: a grade-2 child
		// must not be able to pick "3a". Concrete classes follow the
		// grade-number convention gradeToClass produces, so the class's
		// leading digits are the grade it belongs to. Reject when they
		// disagree; classes without a numeric prefix carry no derivable
		// grade and are left to the plain list check above.
		if prefix := schoolclass.GradePrefix(chosen); prefix != "" && prefix != strconv.Itoa(grade) {
			return fmt.Errorf("%w: child %d target_school_class %q does not match target grade %d", ErrInvalidSubmission, i, chosen, grade)
		}
		children[i].TargetSchoolClass = &chosen
	}
	return nil
}

// gradeHasSelectableClass reports whether at least one offered class can be
// picked by a child in the given grade. A class matches when its numeric
// prefix equals the grade ("2a" for grade 2); a class without a numeric
// prefix ("Bienen") carries no derivable grade and is offered to every
// grade. Used to decide whether RequireSchoolClass can be enforced for a
// grade at all — a required pick with no matching class is unsatisfiable.
// Issue #1833.
func gradeHasSelectableClass(allowed map[string]struct{}, grade int) bool {
	want := strconv.Itoa(grade)
	for class := range allowed {
		if prefix := schoolclass.GradePrefix(class); prefix == "" || prefix == want {
			return true
		}
	}
	return false
}

// CollectsGrade1Class reports whether a grade-1 child's concrete class is
// collected for this phase. Grade 1 is opt-in — the #1833 default keeps it
// grade-level-only — and there are exactly two triggers (#1663):
//
//   - No class restriction: the phase offers a concrete grade-1 class ("1a"),
//     i.e. an admin deliberately added one. Prefixless classes ("Bienen")
//     deliberately do NOT trigger it here, so a phase that never added a
//     grade-1 class keeps the #1833 default.
//   - Class restriction active: the eligible list decides, because it is the
//     only world the submit gate accepts — validateChildClassEligibility
//     rejects every child that declares no class, and the self-service form
//     is narrowed to that same list. A grade-1 child can satisfy the
//     restriction when an eligible class is grade-1-prefixed OR prefixless
//     ("Bienen" carries no derivable grade and belongs to every grade), so
//     the class must be collected. Not collecting it would clear the class
//     and then reject the very submission the config exists for: a phase
//     limited to "Bienen" accepted no grade-1 submission at all.
//
// An eligible list that only names grade >= 2 classes still returns false:
// such a grade-1 child is genuinely ineligible and is rejected by the class
// gate with class_not_eligible, not by a missing-pick error.
//
// Exported because the form-load responses must present the same decision the
// submit path makes — a field the form hides and the validator then demands is
// a dead end. Reads EligibleSchoolClasses (never narrowed) whenever a
// restriction exists, so it returns the same answer on a form-load phase whose
// offered list was narrowed to the eligible subset.
func CollectsGrade1Class(phase *enrollmentModels.Phase) bool {
	if phase == nil {
		return false
	}
	if hasNonEmptyEligibleClass(phase.EligibleSchoolClasses) {
		return listHasClassSelectableByGrade(phase.EligibleSchoolClasses, 1)
	}
	return listHasGradePrefixedClass(phase.AvailableSchoolClasses, 1)
}

// listHasGradePrefixedClass reports whether the list holds a concrete class
// whose numeric prefix equals the grade (e.g. a "1x" class for grade 1).
// Prefixless classes are ignored on purpose — see CollectsGrade1Class.
func listHasGradePrefixedClass(classes []string, grade int) bool {
	want := strconv.Itoa(grade)
	for _, class := range classes {
		if schoolclass.GradePrefix(strings.TrimSpace(class)) == want {
			return true
		}
	}
	return false
}

// listHasClassSelectableByGrade reports whether a child in the given grade can
// pick at least one class from the list: a matching numeric prefix, or a
// prefixless class, which carries no derivable grade and is offered to every
// grade. Same rule as gradeHasSelectableClass, over a slice instead of the
// offered-class set.
func listHasClassSelectableByGrade(classes []string, grade int) bool {
	want := strconv.Itoa(grade)
	for _, class := range classes {
		trimmed := strings.TrimSpace(class)
		if trimmed == "" {
			continue
		}
		if prefix := schoolclass.GradePrefix(trimmed); prefix == "" || prefix == want {
			return true
		}
	}
	return false
}

// resolveGradeMax reads the server-authoritative tenant setting. The setting
// registry supplies its declared default when a tenant has no override; a
// missing resolver, read failure, or out-of-range value is a configuration
// error and must not silently change which grades the form accepts.
func (s *requestService) resolveGradeMax(ctx context.Context) (int, error) {
	if s.Settings == nil {
		return 0, errors.New("resolve enrollment.grade_level_max: settings service is not configured")
	}
	value, err := s.Settings.ResolveInt(ctx, configModel.KeyEnrollmentGradeLevelMax)
	if err != nil {
		return 0, fmt.Errorf("resolve enrollment.grade_level_max: %w", err)
	}
	if value < schoolclass.MinGradeLevel || value > schoolclass.MaxGradeLevel {
		return 0, fmt.Errorf(
			"resolve enrollment.grade_level_max: value %d is outside %d..%d",
			value,
			schoolclass.MinGradeLevel,
			schoolclass.MaxGradeLevel,
		)
	}
	return value, nil
}

func (s *requestService) allowSubmissionEdit(ctx context.Context) bool {
	return config.ResolveBoolOrDefault(ctx, s.Settings, configModel.KeyEnrollmentAllowSubmissionEdit, true, nil)
}

func (s *requestService) resolveStatusTokenExpiry(ctx context.Context) time.Duration {
	const defaultDays = 365
	days := config.ResolveIntOrDefault(ctx, s.Settings, configModel.KeyEnrollmentStatusTokenTTLDays, defaultDays, nil)
	if days <= 0 {
		days = defaultDays
	}
	return time.Duration(days) * 24 * time.Hour
}

func (s *requestService) resolveSettingString(ctx context.Context, key, fallback string) string {
	return config.ResolveStringOrDefault(ctx, s.Settings, key, fallback, nil)
}

// resolveAdminEmails parses the comma-separated notification_emails
// setting and returns a list of valid email addresses. Invalid entries
// are silently dropped - admins shouldn't receive errors because of a
// trailing comma in their config.
func (s *requestService) resolveAdminEmails(ctx context.Context) []string {
	csv := s.resolveSettingString(ctx, configModel.KeyEnrollmentNotificationEmails, "")
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		if _, err := mail.ParseAddress(trimmed); err != nil {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

// applyCapacityOverflow inspects each selected offering's capacity vs
// current claimants. Per the phase's care_overflow_mode it either
// rejects the whole submission, returns per-child status overrides
// (waitlist), or lets everything pass through unchanged (allow).
//
// The map[childIndex]status return is intentionally sparse - entries
// only appear for children that need a non-default status. Callers
// fall back to ChildStatusSubmitted when the index isn't in the map.
//
// Counting model: a child counts when EITHER it already has a
// non-terminal status in the DB, OR it appears earlier in the same
// submission selecting the same offering. The earlier-in-submission
// case prevents one large family from sneaking past a tight capacity
// because the per-offering DB count was taken once at the top of the
// loop.
func (s *requestService) applyCapacityOverflow(
	ctx context.Context,
	phase *enrollmentModels.Phase,
	children []SubmitChild,
	openByID map[int64]*enrollmentModels.CareOffering,
) (map[int]string, error) {
	return s.applyCapacityOverflowWithPreservedClaims(ctx, phase, children, openByID, nil)
}

func (s *requestService) applyCapacityOverflowWithPreservedClaims(
	ctx context.Context,
	phase *enrollmentModels.Phase,
	children []SubmitChild,
	openByID map[int64]*enrollmentModels.CareOffering,
	preservedClaims map[int64]int,
) (map[int]string, error) {
	overrides := make(map[int]string)
	if s.RequestChildOfferingRepo == nil || len(children) == 0 {
		return overrides, nil
	}
	// Serialize this count with offering-change approvals and other
	// submissions. Both paths lock care offerings by ascending id before they
	// inspect capacity and write the booking links.
	if s.CareOfferingRepo == nil {
		return nil, errors.New("care offering repository is not configured")
	}
	selectedIDs := make([]int64, 0)
	seen := make(map[int64]bool)
	for _, child := range children {
		for _, offeringID := range child.OfferingIDs {
			if offeringID > 0 && !seen[offeringID] {
				seen[offeringID] = true
				selectedIDs = append(selectedIDs, offeringID)
			}
		}
	}
	sort.Slice(selectedIDs, func(i, j int) bool { return selectedIDs[i] < selectedIDs[j] })
	if _, err := s.CareOfferingRepo.ListByIDsForUpdate(ctx, selectedIDs); err != nil {
		return nil, fmt.Errorf("lock care offering capacity: %w", err)
	}

	mode := phase.CareOverflowMode
	if mode == "" {
		mode = enrollmentModels.PhaseCareOverflowWaitlist
	}
	if mode == enrollmentModels.PhaseCareOverflowWaitlist {
		if s.Settings == nil {
			return nil, errors.New("enrollment settings resolver is not configured")
		}
		waitlistEnabled, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentWaitlistEnabled)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", configModel.KeyEnrollmentWaitlistEnabled, err)
		}
		if !waitlistEnabled {
			// Tenant-wide disable wins. Overflow remains deterministic and safe:
			// accept above capacity instead of manufacturing a forbidden status.
			mode = enrollmentModels.PhaseCareOverflowAllow
		}
	}

	// Cache per-offering current count + capacity. Avoid hitting the DB
	// once per (child, offering) pair when one offering is shared.
	type slot struct {
		capacity  *int // nil = unlimited
		current   int  // pre-existing claimants (DB)
		preserved int  // claims this edit already held before replacement
		queued    int  // count from earlier children in this submission
	}
	slots := make(map[int64]*slot)

	getSlot := func(offeringID int64) (*slot, error) {
		if cached, ok := slots[offeringID]; ok {
			return cached, nil
		}
		offering, ok := openByID[offeringID]
		if !ok {
			// Should be impossible (validateOfferingSelections ran first).
			return nil, fmt.Errorf("submit: offering %d not in open catalog", offeringID)
		}
		capacityFrom := timezone.TodayDate()
		if phase.ServiceStartDate.After(capacityFrom) {
			capacityFrom = phase.ServiceStartDate
		}
		capacityUntil := phase.ServiceEndDate.AddDays(1)
		count, err := s.RequestChildOfferingRepo.CountMaxActiveByCareOfferingInRange(ctx, offeringID, capacityFrom, capacityUntil)
		if err != nil {
			return nil, fmt.Errorf("submit: count offering %d: %w", offeringID, err)
		}
		preserved := preservedClaims[offeringID]
		current := count - preserved
		if current < 0 {
			current = 0
		}
		s := &slot{
			capacity:  offering.Capacity,
			current:   current,
			preserved: preserved,
		}
		slots[offeringID] = s
		return s, nil
	}

	for childIdx, child := range children {
		childOver := false
		for _, offeringID := range child.OfferingIDs {
			sl, err := getSlot(offeringID)
			if err != nil {
				return nil, err
			}
			if sl.capacity == nil {
				sl.queued++
				continue
			}
			if sl.preserved > 0 {
				sl.preserved--
				sl.queued++
				continue
			}
			if sl.current+sl.queued+1 > *sl.capacity {
				childOver = true
				if mode == enrollmentModels.PhaseCareOverflowReject {
					return nil, fmt.Errorf("%w: offering %d", ErrCareOfferingFull, offeringID)
				}
			}
			sl.queued++
		}
		if childOver && mode == enrollmentModels.PhaseCareOverflowWaitlist {
			overrides[childIdx] = enrollmentModels.ChildStatusWaitlisted
		}
	}

	return overrides, nil
}

// enforceRateLimit increments the per-IP and per-email buckets and
// returns ErrRateLimited as soon as either crosses its threshold. The
// repository is optional - when not wired, this is a no-op (mainly for
// tests that don't care about rate limiting). Tenant-scoped: each
// school owns its own counters.
func (s *requestService) enforceRateLimit(ctx context.Context, req SubmitRequest) error {
	if s.RateLimitRepo == nil || req.TenantID <= 0 {
		return nil
	}

	if s.DB != nil {
		rateCtx := modelBase.ContextWithoutTx(ctx)
		var limitErr error
		err := tenant.WithTenantTx(rateCtx, s.DB, req.TenantID, func(txCtx context.Context, _ bun.Tx) error {
			limitErr = s.enforceRateLimitBuckets(txCtx, req)
			if errors.Is(limitErr, ErrRateLimited) {
				return nil
			}
			return limitErr
		})
		if err != nil {
			s.Logger.Warn("enrollment submit: rate-limit transaction failed; allowing through",
				slog.String("error", err.Error()),
				slog.Int64("tenant_id", req.TenantID))
		}
		if errors.Is(limitErr, ErrRateLimited) {
			return limitErr
		}
		return nil
	}

	return s.enforceRateLimitBuckets(ctx, req)
}

func (s *requestService) enforceRateLimitBuckets(ctx context.Context, req SubmitRequest) error {
	if s.RateLimitRepo == nil || req.TenantID <= 0 {
		return nil
	}

	ip := strings.TrimSpace(req.RemoteIP)
	email := strings.ToLower(strings.TrimSpace(req.GuardianEmail))

	if ip != "" {
		state, err := s.RateLimitRepo.IncrementAttempts(ctx, req.TenantID, enrollmentModels.SubmissionRateLimitKeyTypeIP, ip, rateLimitWindowIP)
		if err != nil {
			s.Logger.Warn("enrollment submit: rate-limit IP increment failed; allowing through",
				slog.String("error", err.Error()))
		} else if state.Attempts > rateLimitMaxAttemptsIP {
			s.Logger.Info("enrollment submit rate-limited",
				slog.String("key_type", enrollmentModels.SubmissionRateLimitKeyTypeIP),
				slog.Int("attempts", state.Attempts),
				slog.Int64("tenant_id", req.TenantID))
			return ErrRateLimited
		}
	}

	if email != "" {
		state, err := s.RateLimitRepo.IncrementAttempts(ctx, req.TenantID, enrollmentModels.SubmissionRateLimitKeyTypeEmail, email, rateLimitWindowEmail)
		if err != nil {
			s.Logger.Warn("enrollment submit: rate-limit email increment failed; allowing through",
				slog.String("error", err.Error()))
		} else if state.Attempts > rateLimitMaxAttemptsMail {
			s.Logger.Info("enrollment submit rate-limited",
				slog.String("key_type", enrollmentModels.SubmissionRateLimitKeyTypeEmail),
				slog.Int("attempts", state.Attempts),
				slog.Int64("tenant_id", req.TenantID))
			return ErrRateLimited
		}
	}

	return nil
}

// errParentChoiceOfferingMissingDays aliases ErrDaySelectionRequired; the
// sentinel became exported for the HTTP error-code mapping (#1885).
var errParentChoiceOfferingMissingDays = ErrDaySelectionRequired

func resolveManualSelectedDays(offering *enrollmentModels.CareOffering, picks []string) ([]string, error) {
	switch offering.DaysOfWeekMode {
	case enrollmentModels.DaysOfWeekModeFixed:
		if len(picks) > 0 {
			return nil, ErrDaySelectionNotAllowed
		}
		return nil, nil
	case enrollmentModels.DaysOfWeekModeParentChoice:
		allowed := make(map[string]bool, len(offering.AvailableDays))
		for _, d := range offering.AvailableDays {
			allowed[d] = true
		}
		seen := make(map[string]bool, len(picks))
		dedup := make([]string, 0, len(picks))
		for _, d := range picks {
			if !allowed[d] {
				return nil, fmt.Errorf("%w: day %q is not in the offering's available_days", ErrSelectedDayNotAvailable, d)
			}
			if seen[d] {
				continue
			}
			seen[d] = true
			dedup = append(dedup, d)
		}
		return dedup, nil
	default:
		return nil, fmt.Errorf("offering has unknown days_of_week_mode %q", offering.DaysOfWeekMode)
	}
}
