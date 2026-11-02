package organizationtenancy

import (
	"context"
	"errors"
	"net"
	"time"
)

// Träger membership (ADR 0084, #2811, #935): the account reaches the
// Träger-Portal of an organization. Its lifecycle mirrors the school mapping:
// pending, active, inactive. The Träger roles belong to Identity & Access.

// Träger membership states.
const (
	TraegerMembershipPending  = "pending"
	TraegerMembershipActive   = "active"
	TraegerMembershipInactive = "inactive"
)

// ErrOrganizationInactive reports a Träger that is deactivated or deleted.
var ErrOrganizationInactive = errors.New("organization is inactive")

// TraegerOrganization is a Träger an account may sign in to: active,
// undeleted, with an active membership of the account.
type TraegerOrganization struct {
	ID   int64
	Name string
	// TestSystem marks a test Träger (#4658).
	TestSystem bool
}

// TraegerMembership is one account's membership at a Träger.
type TraegerMembership struct {
	OrganizationID int64
	AccountID      int64
	Status         string
	ActivatedAt    *time.Time
	CreatedAt      time.Time
}

// TraegerMembershipQuery reads the Träger memberships and the schools a
// Träger session reaches. Every read joins the caller's transaction.
type TraegerMembershipQuery interface {
	// ListAccountTraegerOrganizations returns the active, undeleted Träger
	// where the account's membership is active, ordered by name and ID.
	ListAccountTraegerOrganizations(ctx context.Context, accountID int64) ([]TraegerOrganization, error)
	// ListTraegerSchoolIDs returns the undeleted schools of the Träger: the
	// tenant set the Träger projection filters on (ADR 0084 section 1).
	ListTraegerSchoolIDs(ctx context.Context, organizationID int64) ([]int64, error)
	// ListTraegerMemberships returns every membership of the Träger.
	ListTraegerMemberships(ctx context.Context, organizationID int64) ([]TraegerMembership, error)
}

// TraegerMembershipCommand writes Träger memberships.
type TraegerMembershipCommand interface {
	// ActivateTraegerMembership creates or reactivates the account's
	// membership at an active, undeleted Träger on the caller's
	// administrative transaction.
	ActivateTraegerMembership(ctx context.Context, organizationID, accountID int64) (TraegerMembership, error)
	// DeactivateTraegerMembership ends the account's active membership on
	// the caller's administrative transaction and reports whether one was
	// active (#4650). The Träger roles and sessions are Identity & Access's.
	DeactivateTraegerMembership(ctx context.Context, organizationID, accountID int64) (bool, error)
}

// ErrTraegerAccountNotFound reports an e-mail address without an account:
// the person needs an account before the operator assigns them to a Träger.
var ErrTraegerAccountNotFound = errors.New("no account with this e-mail address")

// ErrTraegerMemberNotFound reports an account that is no member of the
// Träger-Büro the operator acts on.
var ErrTraegerMemberNotFound = errors.New("account is no member of this Träger-Büro")

// ErrInvalidTraegerRole reports a role outside the Träger roles or an empty
// role set.
var ErrInvalidTraegerRole = errors.New("invalid Träger role")

// ErrTraegerAlreadyMember reports an invitation for a person who already
// belongs to the Träger-Büro (#5144).
var ErrTraegerAlreadyMember = errors.New("the person already belongs to the Träger-Büro")

// ErrTraegerInvalidEmail reports an invitation address that cannot receive
// mail (#5144).
var ErrTraegerInvalidEmail = errors.New("invalid e-mail address")

// TraegerAccount is an account at a Träger with its Träger roles, as Identity
// & Access reports it to the operator listing.
type TraegerAccount struct {
	AccountID int64
	Email     string
	Roles     []string
}

// TraegerMember is a person of the Träger-Büro in the operator listing.
type TraegerMember struct {
	AccountID   int64      `json:"account_id"`
	Email       string     `json:"email"`
	Status      string     `json:"status"`
	Roles       []string   `json:"roles"`
	ActivatedAt *time.Time `json:"activated_at"`
}

// AssignTraegerMember is the operator's grant of a Träger membership with
// one or more Träger roles (spec #4611 user stories 1 and 2, #5144).
type AssignTraegerMember struct {
	OrganizationID int64
	Email          string
	Roles          []string
	OperatorID     int64
	ClientIP       net.IP
}

// InviteTraegerMember is the operator's invitation into a Träger-Büro (spec
// #4611 user story 2, #5144): the person gets the Träger-Büro's invitation
// mail and sets a password at the acceptance, or confirms an existing moto
// account. No school account is needed.
type InviteTraegerMember struct {
	OrganizationID int64
	Email          string
	Roles          []string
	OperatorID     int64
	ClientIP       net.IP
}

// TraegerInvitation is an open invitation into a Träger-Büro as the operator
// sees it after inviting.
type TraegerInvitation struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Roles     []string  `json:"roles"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ResetTraegerMemberMFA is the operator's reset of the second factor of a
// Träger-Büro member (#4651): the person sets it up again at the next
// sign-in. Reason is recorded in the operator audit.
type ResetTraegerMemberMFA struct {
	OrganizationID int64
	AccountID      int64
	Reason         string
	OperatorID     int64
	ClientIP       net.IP
}

// TraegerMemberProvisioning lets the operator see, assign and invite the
// people of a Träger-Büro and reset their second factor. Each write is
// recorded in the operator audit.
type TraegerMemberProvisioning interface {
	ListTraegerMembers(ctx context.Context, organizationID int64) ([]TraegerMember, error)
	AssignTraegerMember(ctx context.Context, input AssignTraegerMember) (TraegerMember, error)
	InviteTraegerMember(ctx context.Context, input InviteTraegerMember) (TraegerInvitation, error)
	ResetTraegerMemberMFA(ctx context.Context, input ResetTraegerMemberMFA) error
}
