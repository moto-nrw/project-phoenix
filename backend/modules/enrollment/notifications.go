package enrollment

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Payload keys used by the enrollment outbox rows. Every field needed by the
// email template is captured at enqueue time so the renderer is pure (no DB
// lookups). Keeping the keys named here keeps the enqueue side and the
// renderer side in lockstep - renaming a key requires editing both in the
// same diff.
const (
	EnrollmentPayloadGuardianFirstName = "guardian_first_name"
	EnrollmentPayloadGuardianLastName  = "guardian_last_name"
	EnrollmentPayloadGuardianEmail     = "guardian_email"
	EnrollmentPayloadGuardianPhone     = "guardian_phone"
	EnrollmentPayloadSchoolName        = "school_name"
	EnrollmentPayloadStatusURL         = "status_url"
	EnrollmentPayloadAdminURL          = "admin_url"
	EnrollmentPayloadLogoURL           = "logo_url"
	EnrollmentPayloadMotoLogoURL       = "moto_logo_url"
	EnrollmentPayloadChildNames        = "child_names"
	EnrollmentPayloadRecipientEmail    = "recipient_email"
	EnrollmentPayloadPhaseName         = "phase_name"
	EnrollmentPayloadStatusReason      = "status_reason"
	EnrollmentPayloadRolloverDeadline  = "rollover_deadline"
)

// StatusURL builds the parent-facing status link sent in the submitted,
// decision and rollover emails. It routes to the parents portal.
func StatusURL(parentsURL, token string) string {
	host := parentsURL
	if host == "" {
		host = "http://localhost:3000"
	}
	return fmt.Sprintf("%s/anmeldung/status/%s", host, token)
}

// School is the part of a school the enrollment flows read.
type School struct {
	Name      string
	Subdomain string
	// Settings is the school's raw settings JSON; the e-mail branding reads
	// the login image from it.
	Settings string
	Deleted  bool
}

// SchoolDirectory reads the school a request belongs to. Organisation &
// Tenancy owns the row; the root binds this port to its capability. A school
// that does not exist is (nil, nil).
type SchoolDirectory interface {
	FindSchool(ctx context.Context, id int64) (*School, error)
}

// Delivery outbox kinds of Enrollment's decision mails and the entity they
// relate to. The composition root registers the renderers under the same
// kinds.
const (
	MailKindApproved       = "enrollment_approved"
	MailKindWaitlisted     = "enrollment_waitlisted"
	MailKindRejected       = "enrollment_rejected"
	MailKindDecisionDigest = "enrollment_decision_digest"
	MailKindRolloverOptIn  = "enrollment_rollover_opt_in"
	MailKindRolloverOptOut = "enrollment_rollover_opt_out"
	MailRelatedRequest     = "enrollment_request"
)

// DecisionNotice is one persisted child-state generation of a request whose
// parents may have to hear about it. ImmediateChildIDs holds only the
// children whose new status should produce a standalone email; an empty set
// is still useful for a withdrawal that completes a digest-mode request.
// ParentsURL is the parent-facing base of the status and logo links.
type DecisionNotice struct {
	Request           DecisionRequest
	Children          []DecisionChild
	Phase             *Phase
	ImmediateChildIDs map[int64]struct{}
	ParentsURL        string
}

// DecisionRequest is the part of an enrollment request a decision mail
// needs. NotificationMode is the pinned decision notification mode, nil
// until the first parent-notifiable decision.
type DecisionRequest struct {
	ID                int64
	TenantID          int64
	GuardianFirstName string
	GuardianLastName  string
	GuardianEmail     string
	StatusToken       string
	NotificationMode  *string
}

// DecisionChild is the part of a request child a decision mail needs.
// ReviewedAt identifies the review generation of its status.
type DecisionChild struct {
	ID           int64
	FirstName    string
	LastName     string
	Status       string
	StatusReason *string
	ReviewedAt   *time.Time
}

// ParentResolvedStatus reports whether a child status is a final one the
// parents hear about: approved, rejected, waitlisted or withdrawn.
func ParentResolvedStatus(status string) bool {
	switch status {
	case ChildStatusApproved, ChildStatusRejected, ChildStatusWaitlisted, ChildStatusWithdrawn:
		return true
	default:
		return false
	}
}

// Notifications is Enrollment's side of the parent and admin mails: the
// school branding every mail carries and the decision notifications.
type Notifications interface {
	// SchoolBrand returns the name and logo link of the tenant's school, or
	// empty strings when the school cannot be read.
	SchoolBrand(ctx context.Context, tenantID int64, baseURL string) (schoolName, logoURL string)
	// NotifyDecisions routes one decision generation through the request's
	// pinned notification mode: immediate mails per child, or one digest
	// once every child is resolved. It pins the mode at the first
	// parent-notifiable decision and returns the mode that holds.
	NotifyDecisions(ctx context.Context, notice DecisionNotice) (mode string, err error)
}

// RenderedMail is an Enrollment mail ready for delivery. An empty SenderName
// keeps the platform's default sender; otherwise the mail goes out under the
// school's name from the default address. Content is the template document
// the Delivery renderer executes.
type RenderedMail struct {
	SenderName string
	Recipient  string
	Subject    string
	Template   string
	Content    any
}

// MailRenderer renders one claimed outbox intent of an Enrollment mail kind
// from its JSON payload.
type MailRenderer func(ctx context.Context, kind string, payload json.RawMessage) (*RenderedMail, error)

// MailRenderers are Enrollment's renderers, one per mail template.
type MailRenderers struct {
	Submitted                MailRenderer
	AdminNotification        MailRenderer
	Approved                 MailRenderer
	Waitlisted               MailRenderer
	Rejected                 MailRenderer
	DecisionDigest           MailRenderer
	ChangeRequestSubmitted   MailRenderer
	ChangeRequestQuestion    MailRenderer
	ChangeRequestParentReply MailRenderer
	ChangeRequestApproved    MailRenderer
	ChangeRequestRejected    MailRenderer
	RolloverOptIn            MailRenderer
	RolloverOptOut           MailRenderer
}

// CaptchaVerifier verifies a parent-submitted captcha token against the
// configured provider (today Cloudflare Turnstile). Verify returns nil when
// the captcha is valid, or an error explaining why it failed; it returns nil
// unconditionally when captcha is disabled for the tenant.
type CaptchaVerifier interface {
	IsEnabled(ctx context.Context) (bool, error)
	Verify(ctx context.Context, token, remoteIP string) error
	// SiteKey returns the public site key for the tenant, or "" when unset.
	// Safe to expose on a public endpoint.
	SiteKey(ctx context.Context) (string, error)
}
