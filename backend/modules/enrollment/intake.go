package enrollment

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The parent-facing intake of an enrollment (#3565): the submission and its
// edits, the status link, the public form loads and the late invites. The
// answers, consent flags and source metadata a family submits travel as
// FormValues (ADR 0173).

// SubmitRequest is one submission as the intake receives it.
type SubmitRequest struct {
	// DisplayedFormSchemaID is the form version the parent's form showed
	// (#4680). Versions never change, so the snapshot proves the shown legal
	// texts are the stored ones only when it equals the version in force.
	DisplayedFormSchemaID int64
	TenantID              int64
	PhaseID               int64
	// RemoteIP is the parent's source IP. Empty disables IP-based rate
	// limiting for that submission.
	RemoteIP string
	// LateInviteToken opens a closed phase for its holder. It is phase-bound,
	// validated, and consumed inside the write transaction, but it does not
	// bind the submitted guardian email to the invitation address (#2164).
	LateInviteToken  string
	SubmissionSource string
	SourceMetadata   FormValues

	GuardianFirstName string
	GuardianLastName  string
	GuardianEmail     string
	GuardianPhone     *string
	ConsentFlags      FormValues
	CustomData        FormValues
	// GuardianAccountID is set when an authenticated parent submits through
	// the parents portal; nil is an anonymous public submission.
	GuardianAccountID *int64
	// GuardianSubmitEligible is true when the account holds a guardian
	// relationship at the tenant granting parent_portal.enrollment.submit
	// (#1663). Always false for anonymous submissions.
	GuardianSubmitEligible bool
	Children               []SubmitChild
	// AdditionalGuardians are the co-guardians the parent added beyond the
	// primary guardian.
	AdditionalGuardians []SubmitGuardian
	// Signature is the parent's signature (#4786); a form that requires one
	// refuses a parent submission or edit without it.
	Signature *SubmitSignature
}

// SubmitGuardian is one additional guardian of a submission. Only the names
// are required; email and phone are optional.
type SubmitGuardian struct {
	FirstName string
	LastName  string
	Email     *string
	Phone     *string
}

// SubmitChild is one child of a submission. OfferingDays refines the days of
// parent_choice offerings; every entry must also appear in OfferingIDs.
type SubmitChild struct {
	ID                int64
	FirstName         string
	LastName          string
	DateOfBirth       calendar.Date
	TargetGradeLevel  *int16
	TargetSchoolClass *string
	// HolidayCareWish answers whether the child needs care in the holidays
	// (#4703); nil when the form does not ask.
	HolidayCareWish *bool
	CustomData      FormValues
	OfferingIDs     []int64
	OfferingDays    []SubmitOfferingDays
	// ExcludedAutoAddTargetIDs switches off the Mitbuchungs-Regel for these
	// target offerings (#2370). Only the staff review decision sets it.
	ExcludedAutoAddTargetIDs map[int64]bool
}

// SubmitOfferingDays is the day selection of one parent_choice offering.
type SubmitOfferingDays struct {
	OfferingID   int64
	SelectedDays []string
}

// SubmitResult is what a submission or an edit stored.
type SubmitResult struct {
	Request   *Request
	Children  []*RequestChild
	StatusURL string
	Warnings  []SubmissionWarning
}

// SubmissionWarning is a non-blocking finding of a stored submission.
type SubmissionWarning struct {
	Code string `json:"code"`
}

// WarningCodeDuplicateEnrollment marks a submission stored although an
// active one exists for the same parent and child in the phase.
const WarningCodeDuplicateEnrollment = "enrollment.duplicate_detected"

// FormCapabilities is the effective tenant configuration of the enrollment
// form's core inputs, authoritative for the form load and the submission.
type FormCapabilities struct {
	CollectGradeLevel    bool
	CollectSchoolClass   bool
	CareOfferingsEnabled bool
}

// Edit modes the status page may offer.
const (
	EditModeDirectEdit    = "direct_edit"
	EditModeChangeRequest = "change_request"
	EditModeNone          = "none"
)

// EditPatch carries the fields a parent may patch before the first decision.
// Pointer fields leave a value alone unless set; set values, even empty
// ones, replace the stored ones.
type EditPatch struct {
	GuardianFirstName *string
	GuardianLastName  *string
	GuardianPhone     *string
	ConsentFlags      *FormValues
	CustomData        *FormValues
}

// CreateLateInviteInput is an admin's late invite for one family.
type CreateLateInviteInput struct {
	PhaseID           int64
	GuardianEmail     string
	GuardianFirstName string
	GuardianLastName  string
	Reason            string
	ExpiresAt         *time.Time
	CreatedBy         int64
}

// CreateLateInviteResult carries the stored invite and its one-time token.
type CreateLateInviteResult struct {
	Invite *LateInvite
	Token  string
}

// ManualApprovedEnrollmentInput is a submission staff create and approve in
// one step.
type ManualApprovedEnrollmentInput struct {
	Request          SubmitRequest
	Reason           string
	SendNotification bool
	ActorID          int64
	// ExistingStudentID and CreateNewStudent are the office's answer when
	// the child may already be enrolled (#6148), passed to the approval as
	// in DecideInput.
	ExistingStudentID *int64
	CreateNewStudent  bool
}

// ManualApprovedEnrollmentResult is the created request, the approved child
// and its status link. The approval's guardian invitation is already created
// in the same transaction.
type ManualApprovedEnrollmentResult struct {
	Request   *Request
	Child     *RequestChild
	StatusURL string
}

// LegalTexts bundles the per-tenant legal texts of the public enrollment
// form. Standard blocks render only when their toggle is on and the text is
// not empty.
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

// LegalBlock is one legal row of the public enrollment form. Required
// checkbox blocks must be accepted; notice blocks only inform.
type LegalBlock struct {
	Attachments []FormAttachment `json:"attachments,omitempty"`
	Key         string           `json:"key"`
	Kind        string           `json:"kind"`
	Title       string           `json:"title"`
	Label       string           `json:"label"`
	Text        string           `json:"text"`
	Required    bool             `json:"required"`
	SortOrder   int              `json:"sort_order,omitempty"`
	Source      string           `json:"source,omitempty"`
	// Translations holds only translations that still match the German
	// text, without their source (#3377).
	Translations Translations `json:"translations,omitempty"`
}

// EnrolleeAudienceAccess carries a caller's per-audience form-load
// authority (#1663): linked_parents needs any guardian relationship granting
// parent_portal.enrollment.submit, existing_students one to a still enrolled
// child. The zero value is the anonymous caller.
type EnrolleeAudienceAccess struct {
	LinkedParents    bool
	ExistingStudents bool
}

// AllowsAudience reports whether the caller may load a form for a phase with
// this audience. A restricted audience without a flag of its own fails
// closed.
func (a EnrolleeAudienceAccess) AllowsAudience(audience string) bool {
	switch audience {
	case PhaseAudienceLinkedParents:
		return a.LinkedParents
	case PhaseAudienceExistingStudents:
		return a.ExistingStudents
	default:
		return true
	}
}

// CareOffering is a care offering as the intake form offers it. The Care
// Plan catalog owns it; AvailabilityRule and Translations stay raw JSON.
type CareOffering struct {
	ID                        int64
	TenantID                  int64
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
	PhaseID                   int64
	ActivityGroupID           *int64
	Name                      string
	Description               *string
	DaysOfWeekMode            string
	AvailableDays             []string
	IncludesHolidayCare       bool
	IncludesLunch             bool
	Capacity                  *int
	PriceCents                *int
	IsActive                  bool
	IsRequired                bool
	CountsAsCare              bool
	AutoAddGradeLevels        []int
	AvailabilityRule          json.RawMessage
	SortOrder                 int
	SelectionGroup            string
	SelectionRule             string
	PickupTimes               map[string]string
	Translations              json.RawMessage
	AutoAddTriggerOfferingIDs []int64
}

// HasFixedDays reports whether the offering books all its available days
// instead of letting the family choose.
func (o *CareOffering) HasFixedDays() bool { return o.DaysOfWeekMode == CareOfferingDaysFixed }

// PublicFormBootstrapData is what a form load assembles in one tenant
// transaction: the gated phase, its pinned schema, the active care
// offerings, the raw and the effective form capabilities, the legal texts
// and the late invite the link carried. Schema and LegalTexts stay zero for
// the offering-only projection.
type PublicFormBootstrapData struct {
	Phase        *Phase
	Schema       *FormSchema
	Offerings    []*CareOffering
	Capabilities FormCapabilities
	// EffectiveCapabilities additionally collects the grade whenever an
	// active offering's availability depends on it.
	EffectiveCapabilities FormCapabilities
	LegalTexts            LegalTexts
	LateInvite            *LateInvite
}

// BootstrapStage identifies the resolution step of a public form load that
// failed, so the caller renders a server error instead of the public 404.
type BootstrapStage string

const (
	BootstrapStageCapabilities BootstrapStage = "capabilities"
	BootstrapStageLegal        BootstrapStage = "legal"
)

// BootstrapStageError marks a form-load failure that must surface as a
// server error rather than the public not-found gate.
type BootstrapStageError struct {
	Stage BootstrapStage
	Err   error
}

func (e *BootstrapStageError) Error() string { return e.Err.Error() }
func (e *BootstrapStageError) Unwrap() error { return e.Err }

// RequestStatus is a request behind a status token with its children.
type RequestStatus struct {
	Request  *Request
	Children []*RequestChild
}

// StatusPage is what the token status page offers beyond the request: the
// edit path and the school's care-facility term (#6042). The page runs on the
// parents host without a school context, so the term travels with it.
type StatusPage struct {
	EditMode         string
	CareFacilityTerm string
	// RenewalTerms are the conditions a renewal asks the parents to accept
	// for the new care phase (#6456); nil while no child of the request
	// waits for a renewal decision.
	RenewalTerms *RenewalTerms
}

// RenewalTerms are the required legal blocks of the form in force for the
// phase of a renewal. A renewal carries no consent of the earlier phase over:
// the parents accept these blocks explicitly, and only that acceptance is
// evidence for the contract of the new phase (#6456).
type RenewalTerms struct {
	// FormSchemaID is the form version the blocks come from; 0 when the
	// phase has no form and the tenant-wide blocks apply.
	FormSchemaID int64
	Blocks       []LegalBlock
	// ConfirmedAt is when the parents last accepted exactly these blocks;
	// nil while they have not.
	ConfirmedAt *time.Time
}

// RenewalConfirmation is the parents' answer to the renewal terms: the
// ticked blocks and the form version the page showed.
type RenewalConfirmation struct {
	ConsentFlags          FormValues
	DisplayedFormSchemaID int64
}

// EditDraft is the persisted request that reopens the public form.
type EditDraft struct {
	Request          *Request
	Children         []*RequestChild
	Guardians        []*RequestGuardian
	OfferingsByChild map[int64][]*RequestChildOfferingRecord
	Phase            *Phase
	School           *School
	Schema           *FormSchema
	OpenOfferings    []*CareOffering
	LegalTexts       LegalTexts
	EditMode         string
	// CollectSchoolClass mirrors enrollment.collect_school_class (#1833).
	CollectSchoolClass   bool
	CollectGradeLevel    bool
	CareOfferingsEnabled bool
	// GradeLevelMax is the tenant's upper bound of target grades, resolved in
	// the same transaction as the rest of the draft.
	GradeLevelMax int
	// CareFacilityTerm is the school's term for its care in the fixed texts
	// of the reopened form (#6042).
	CareFacilityTerm string
}

// IntakeSubmissions stores submissions and the parent's later changes. Each
// write opens or joins the transaction of the request's tenant.
type IntakeSubmissions interface {
	// Submit stores a submission with its children, co-guardians, offering
	// choices and bookings and enqueues the confirmation mails, all in one
	// transaction.
	Submit(ctx context.Context, req SubmitRequest) (*SubmitResult, error)
	// ReplaceEditable rewrites the editable payload of a request whose
	// children are all still submitted, keeping its id and status token.
	ReplaceEditable(ctx context.Context, token string, req SubmitRequest) (*SubmitResult, error)
	// Edit patches the guardian fields while every child is still submitted.
	Edit(ctx context.Context, token string, patch EditPatch) error
	// Withdraw withdraws one child, or every open child when childID is 0.
	Withdraw(ctx context.Context, token string, childID int64) error
	// ConfirmRenewal records the parents' acceptance of the renewal terms
	// as a new legal evidence entry, moves the request's pending_renewal
	// children to submitted and reports how many it moved. auto_renewed
	// children keep their status (#6456).
	ConfirmRenewal(ctx context.Context, token string, confirmation RenewalConfirmation) (int, error)
	// CreateLateInvite issues a late invite and returns its token once.
	CreateLateInvite(ctx context.Context, input CreateLateInviteInput) (*CreateLateInviteResult, error)
	// CreateManualApprovedEnrollment submits a staff-created enrollment and
	// approves it in the caller's transaction.
	CreateManualApprovedEnrollment(ctx context.Context, input ManualApprovedEnrollmentInput) (*ManualApprovedEnrollmentResult, error)
}

// IntakeStatus reads a request behind its public status token. The caller
// runs the token lookup in an administrative transaction.
type IntakeStatus interface {
	// StatusByToken loads the request and its children; the status reason
	// is redacted unless the phase shows it to parents.
	StatusByToken(ctx context.Context, token string) (*RequestStatus, error)
	// StatusPage returns the edit path the status page may offer and the
	// school's term for its care in the page's fixed texts (#6042).
	StatusPage(ctx context.Context, status *RequestStatus) (StatusPage, error)
	// GuardiansByStatusToken loads the request's co-guardians.
	GuardiansByStatusToken(ctx context.Context, token string) ([]*RequestGuardian, error)
	// EditDraft loads everything that reopens the public form.
	EditDraft(ctx context.Context, token string) (*EditDraft, error)
}

// IntakeForms serves the public and manual enrollment form loads. Callers
// run inside the tenant transaction.
type IntakeForms interface {
	// IsEnrollmentEnabled reports the tenant's enrollment master toggle.
	IsEnrollmentEnabled(ctx context.Context) bool
	// LegalTexts resolves the tenant's legal texts and blocks.
	LegalTexts(ctx context.Context) (LegalTexts, error)
	// LegalTextsForPhaseWithLateInvite resolves the legal contract of a
	// phase behind the public phase gate.
	LegalTextsForPhaseWithLateInvite(ctx context.Context, phaseID int64, lateInviteToken string) (LegalTexts, error)
	// LoadPublicFormBootstrap assembles the anonymous public form load.
	LoadPublicFormBootstrap(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (*PublicFormBootstrapData, error)
	// LoadEnrolleeFormBootstrap assembles the parents-portal form load for
	// the restricted audiences the caller's access covers.
	LoadEnrolleeFormBootstrap(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string, access EnrolleeAudienceAccess) (*PublicFormBootstrapData, error)
	// LoadPhasePreviewCareOfferings loads the effective form configuration and
	// active offerings for staff, without public activity, audience or window gates.
	LoadPhasePreviewCareOfferings(ctx context.Context, phaseID int64) (*PublicFormBootstrapData, error)
	// LoadManualEnrollmentBootstrap assembles the staff manual-enrollment
	// form load.
	LoadManualEnrollmentBootstrap(ctx context.Context, phaseID int64) (*PublicFormBootstrapData, error)
}

// Mail kinds and the related entity of the intake and change-request mails.
// The composition root registers the renderers under the same kinds.
const (
	MailKindSubmitted                = "enrollment_submitted"
	MailKindAdminNotification        = "enrollment_admin_notification"
	MailKindChangeRequestSubmitted   = "enrollment_change_request_submitted"
	MailKindChangeRequestQuestion    = "enrollment_change_request_question"
	MailKindChangeRequestParentReply = "enrollment_change_request_parent_reply"
	MailKindChangeRequestApproved    = "enrollment_change_request_approved"
	MailKindChangeRequestRejected    = "enrollment_change_request_rejected"
)
