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
	// ListGuardianInvitations returns every invitation of the contact.
	ListGuardianInvitations(ctx context.Context, guardianProfileID int64) ([]GuardianInvitation, error)
	// ListOpenGuardianInvitations returns the invitations of those contacts
	// that are redeemable or awaiting a staff decision.
	ListOpenGuardianInvitations(ctx context.Context, guardianProfileIDs []int64) ([]GuardianInvitation, error)
	// ListRedeemableGuardianInvitations returns the invitations of the school
	// in context whose link can still be spent.
	ListRedeemableGuardianInvitations(ctx context.Context) ([]GuardianInvitation, error)
	// GuardianInvitationSchoolSlug resolves the school of an invitation; it
	// is best-effort and answers "" when it cannot.
	GuardianInvitationSchoolSlug(ctx context.Context, token string) string
}

func (m *Module) CreateGuardianInvitation(ctx context.Context, request GuardianInvitationRequest) (GuardianInvitation, error) {
	return m.engine.CreateGuardianInvitation(ctx, request)
}

func (m *Module) ValidateGuardianInvitation(ctx context.Context, token string) (GuardianInvitationPreview, error) {
	return m.engine.ValidateGuardianInvitation(ctx, token)
}

func (m *Module) AcceptGuardianInvitation(ctx context.Context, token string, registration GuardianRegistration) (Account, error) {
	return m.engine.AcceptGuardianInvitation(ctx, token, registration)
}

func (m *Module) ResendGuardianInvitation(ctx context.Context, invitationID, actorAccountID int64) error {
	return m.engine.ResendGuardianInvitation(ctx, invitationID, actorAccountID)
}

func (m *Module) ListGuardianInvitations(ctx context.Context, guardianProfileID int64) ([]GuardianInvitation, error) {
	return m.engine.ListGuardianInvitations(ctx, guardianProfileID)
}

func (m *Module) ListOpenGuardianInvitations(ctx context.Context, guardianProfileIDs []int64) ([]GuardianInvitation, error) {
	return m.engine.ListOpenGuardianInvitations(ctx, guardianProfileIDs)
}

func (m *Module) ListRedeemableGuardianInvitations(ctx context.Context) ([]GuardianInvitation, error) {
	return m.engine.ListRedeemableGuardianInvitations(ctx)
}

func (m *Module) GuardianInvitationSchoolSlug(ctx context.Context, token string) string {
	return m.engine.GuardianInvitationSchoolSlug(ctx, token)
}
