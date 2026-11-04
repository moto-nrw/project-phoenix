package identityaccess

import (
	"context"
	"errors"
	"time"
)

// Träger-Büro account management (ADR 0084 section 4, #4650): a person with
// the right "Konten verwalten" invites colleagues by mail, gives them Träger
// roles, changes and withdraws them. The acceptance activates the Träger
// membership of Organisation & Tenancy with the roles the invitation names.
// Every step, and every read of the member list, is written to the Träger
// account ledger of the Audit Platform. The Träger always comes from the
// session of the acting person, never from the request.

var (
	// ErrTraegerAccountsUnavailable reports a module composed without the
	// Träger account management.
	ErrTraegerAccountsUnavailable = errors.New("the Träger account management is not composed")
	// ErrTraegerAccountsForbidden reports an acting person whose current
	// roles no longer grant "Konten verwalten".
	ErrTraegerAccountsForbidden = errors.New("the Träger roles do not grant account management")
	// ErrTraegerAlreadyMember reports an invitation for a person who already
	// belongs to the Träger-Büro.
	ErrTraegerAlreadyMember = errors.New("the person already belongs to the Träger-Büro")
	// ErrTraegerMemberNotFound reports an account without an active
	// membership at the Träger of the session.
	ErrTraegerMemberNotFound = errors.New("the person does not belong to the Träger-Büro")
	// ErrTraegerLastLeader refuses a change that would leave the
	// Träger-Büro without a usable person holding the role Leitung (#5471).
	ErrTraegerLastLeader = errors.New("the Träger-Büro needs at least one person with the role Leitung")
	// ErrTraegerInvalidEmail reports an invitation address that cannot
	// receive mail.
	ErrTraegerInvalidEmail = errors.New("invalid e-mail address")
)

// TraegerActor is the person acting in the Träger-Portal: the account and
// the Träger of the session.
type TraegerActor struct {
	OrganizationID int64
	AccountID      int64
}

// TraegerMember is an active person of the Träger-Büro with its roles.
type TraegerMember struct {
	AccountID   int64
	Email       string
	Roles       []string
	ActivatedAt *time.Time
}

// TraegerInvitation is an open invitation into the Träger-Büro. Token is set
// only on the invitation the creating call returns, for the mail link.
type TraegerInvitation struct {
	ID             int64
	OrganizationID int64
	Email          string
	Roles          []string
	Token          string
	ExpiresAt      time.Time
	CreatedAt      time.Time
}

// TraegerBuero is the member list of a Träger-Büro: the active people and
// the open invitations.
type TraegerBuero struct {
	Members     []TraegerMember
	Invitations []TraegerInvitation
}

// TraegerInvitationPreview is the public-safe view of a redeemable
// invitation. RequiresExistingPassword marks an address that already has a
// moto account: the invitee confirms with that account's password instead of
// choosing a new one.
type TraegerInvitationPreview struct {
	OrganizationName         string
	Email                    string
	Roles                    []string
	RequiresExistingPassword bool
	ExpiresAt                time.Time
}

// TraegerInvitationAcceptance is what the invitee supplies: the password of
// the existing account, or a new password twice. IPAddress and UserAgent
// reach the login's record of a refused password.
type TraegerInvitationAcceptance struct {
	Password        string
	ConfirmPassword string
	IPAddress       string
	UserAgent       string
}

// TraegerAccountChange is one entry of the Träger account ledger.
type TraegerAccountChange struct {
	OrganizationID  int64
	ActorAccountID  int64
	Action          string
	TargetAccountID *int64
	TargetEmail     *string
	RolesBefore     []string
	RolesAfter      []string
}

// TraegerAccountHistoryEntry is one change of the Träger-Büro as its history
// shows it (#5474): who did what, when, to whom, with the roles before and
// after. ActorEmail is empty for an account erased since.
type TraegerAccountHistoryEntry struct {
	ID              int64
	OccurredAt      time.Time
	Action          string
	ActorAccountID  int64
	ActorEmail      string
	TargetAccountID *int64
	TargetEmail     string
	RolesBefore     []string
	RolesAfter      []string
}

// TraegerAccountHistory is one page of the history, newest first. HasMore
// reports older entries; the next page starts before the last entry.
type TraegerAccountHistory struct {
	Entries []TraegerAccountHistoryEntry
	HasMore bool
}

// TraegerAccountHistoryPageSize is the number of entries on one page.
const TraegerAccountHistoryPageSize = 25

// TraegerAccountChangeNotice is the mail to the person whose roles another
// person changed or who was removed from the Träger-Büro (#5474). It goes
// also when a person changes their own roles: a taken-over account would do
// exactly that.
type TraegerAccountChangeNotice struct {
	OrganizationID  int64
	TraegerName     string
	Action          string
	TargetAccountID int64
	TargetEmail     string
	ActorAccountID  int64
	ActorEmail      string
	RolesBefore     []string
	RolesAfter      []string
	OccurredAt      time.Time
}

// The actions of the Träger account ledger.
const (
	TraegerAccountInvitationCreated  = "invitation_created"
	TraegerAccountInvitationRevoked  = "invitation_revoked"
	TraegerAccountInvitationAccepted = "invitation_accepted"
	TraegerAccountRolesChanged       = "roles_changed"
	TraegerAccountMemberRemoved      = "member_removed"
	TraegerAccountMembersListed      = "members_listed"
)

// TraegerAccountManagement serves the Träger-Büro account routes. Role sets
// are validated against the Träger roles; an empty set is invalid. A change
// that would leave the Träger-Büro without a person holding the role
// Leitung is refused with ErrTraegerLastLeader; only a person with
// Leitung grants, withdraws or removes Leitung (#5471). Changed roles reach
// a session at its next refresh; a removal ends the person's Träger sessions
// at once.
type TraegerAccountManagement interface {
	ListTraegerBuero(ctx context.Context, actor TraegerActor) (TraegerBuero, error)
	InviteTraegerAccount(ctx context.Context, actor TraegerActor, email string, roles []string) (TraegerInvitation, error)
	RevokeTraegerInvitation(ctx context.Context, actor TraegerActor, invitationID int64) error
	ChangeTraegerRoles(ctx context.Context, actor TraegerActor, accountID int64, roles []string) (TraegerMember, error)
	RemoveTraegerMember(ctx context.Context, actor TraegerActor, accountID int64) error
	// ListTraegerAccountHistory lists the changes of the Träger-Büro older
	// than the entry beforeID, newest first; beforeID 0 starts at the newest.
	ListTraegerAccountHistory(ctx context.Context, actor TraegerActor, beforeID int64) (TraegerAccountHistory, error)
	PreviewTraegerInvitation(ctx context.Context, token string) (TraegerInvitationPreview, error)
	// AcceptTraegerInvitation returns the address the invitee signs in with.
	AcceptTraegerInvitation(ctx context.Context, token string, acceptance TraegerInvitationAcceptance) (string, error)
}

// TraegerOperatorInvitation lets the moto team invite a person into a
// Träger-Büro from the operator portal (spec #4611 user story 2, #5144),
// also the first one: the same invitation, mail and acceptance as the
// Träger-Büro's own, so the Träger-Büro sees it among its open invitations.
// It participates in the caller's administrative transaction and refuses
// without one (ErrTraegerTransactionRequired); the operator audit of the
// caller records the step. A Träger that is not live is
// ErrNoTraegerMembership. While the Träger has no usable person with
// Leitung, the invitation adds Leitung to the chosen roles: the first person
// the moto team invites can manage the Träger-Büro (#5471).
type TraegerOperatorInvitation interface {
	InviteTraegerAccountForOperator(ctx context.Context, organizationID int64, email string, roles []string) (TraegerInvitation, error)
}
