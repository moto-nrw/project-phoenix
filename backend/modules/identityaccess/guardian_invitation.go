package identityaccess

import (
	"context"
	"errors"
	"time"
)

// ErrTransactionUnusable reports a seam that left the caller's transaction
// in a state nothing may commit on top of: the acceptance gives up its
// writes instead of committing half of them.
var ErrTransactionUnusable = errors.New("transaction state is unusable")

// Guardian invitation approval states reported by the owner capability.
const (
	GuardianInvitationApprovalNotRequired = "not_required"
	GuardianInvitationApprovalPending     = "pending"
	GuardianInvitationApprovalApproved    = "approved"
	GuardianInvitationApprovalRejected    = "rejected"
)

// Delivery outbox kinds of the guardian portal-access and welcome mails and
// the entities they relate to. The composition root enqueues the mails and
// registers their renderers under the same kinds.
const (
	MailKindGuardianInvitation    = "guardian_invitation"
	MailKindGuardianWelcome       = "guardian_welcome"
	MailRelatedGuardianInvitation = "guardian_invitation"
	MailRelatedGuardianProfile    = "guardian_profile"
)

// Guardian invitations (#2722): the one-time link a guardian redeems for
// their parents-portal account. A link is never stored already expired or
// spent, a resend never revives one, and accepting it writes the account,
// its school mapping, the guardian role, the profile link and the spent
// invitation in one transaction. The relative access flows
// (GuardianRelativeAccess) issue the same links for one child.

// GuardianInvitation is one auth.guardian_invitations row as the owner
// reports it.
type GuardianInvitation struct {
	ID                          int64
	TenantID                    int64
	Token                       string
	GuardianProfileID           int64
	CreatedBy                   int64
	ExpiresAt                   time.Time
	AcceptedAt                  *time.Time
	EmailSentAt                 *time.Time
	EmailError                  *string
	StudentID                   *int64
	RequestedByAccountID        *int64
	ApprovalStatus              string
	ApprovedBy                  *int64
	ApprovedAt                  *time.Time
	ProfileCreatedForInvitation bool
	RoleUpgrade                 bool
	CreatedAt                   time.Time
}

// ApprovalAllowsRedemption reports whether the invitation's approval state
// lets the guardian redeem it: it needed no approval, or the approval was
// given. A pending or rejected request is not redeemable.
func (i GuardianInvitation) ApprovalAllowsRedemption() bool {
	return i.ApprovalStatus == GuardianInvitationApprovalNotRequired || i.ApprovalStatus == GuardianInvitationApprovalApproved
}

// IsRejected reports whether the school rejected the invitation request.
func (i GuardianInvitation) IsRejected() bool {
	return i.ApprovalStatus == GuardianInvitationApprovalRejected
}

// GuardianInvitationRequest issues an invitation for a guardian contact that
// has an address on file and no account yet. The school comes from the
// tenant in context.
type GuardianInvitationRequest struct {
	GuardianProfileID int64
	CreatedBy         int64
}

// GuardianInvitationPreview is the public-safe view of a redeemable
// invitation: enough to pre-fill and brand the accept page, and no internal
// identifier.
type GuardianInvitationPreview struct {
	Email         string
	FirstName     string
	LastName      string
	ExpiresAt     time.Time
	SchoolName    string
	SchoolSlug    string
	SchoolLogoURL string
}

// GuardianRegistration is what a guardian supplies when accepting. An
// address that already owns an account keeps its credential; the fields are
// then ignored.
type GuardianRegistration struct {
	Password        string
	ConfirmPassword string
}

// GuardianInvitations is the capability the public guardian invitation
// routes and the enrollment decisions consume. Flow errors arrive in the
// AuthenticationError envelope.
type GuardianInvitations interface {
	CreateGuardianInvitation(ctx context.Context, request GuardianInvitationRequest) (GuardianInvitation, error)
	ValidateGuardianInvitation(ctx context.Context, token string) (GuardianInvitationPreview, error)
	// AcceptGuardianInvitation returns the account the guardian signs in with.
	AcceptGuardianInvitation(ctx context.Context, token string, registration GuardianRegistration) (Account, error)
	ResendGuardianInvitation(ctx context.Context, invitationID, actorAccountID int64) error
	GuardianInvitationReads
	// GuardianInvitationSchoolSlug resolves the school of an invitation; it
	// is best-effort and answers "" when it cannot.
	GuardianInvitationSchoolSlug(ctx context.Context, token string) string
}

// GuardianInvitationReads are the invitation lists the People Directory
// guardian list and the parents portal read. They need the database alone,
// so a root can compose them before the module (#4080).
type GuardianInvitationReads interface {
	// ListGuardianInvitations returns every invitation of the contact.
	ListGuardianInvitations(ctx context.Context, guardianProfileID int64) ([]GuardianInvitation, error)
	// ListOpenGuardianInvitations returns the invitations of those contacts
	// that are redeemable or awaiting a staff decision.
	ListOpenGuardianInvitations(ctx context.Context, guardianProfileIDs []int64) ([]GuardianInvitation, error)
	// ListRedeemableGuardianInvitations returns the invitations of the school
	// in context whose link can still be spent.
	ListRedeemableGuardianInvitations(ctx context.Context) ([]GuardianInvitation, error)
}
