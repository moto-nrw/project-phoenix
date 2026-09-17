package domain

import (
	"errors"
	"strings"
	"time"
)

// School invitations (#2722): the one-time link that grants an invitee
// access to a school with a role. A link is redeemable while it is unused
// and unexpired; accepting it creates or reuses the account, maps it to the
// school, assigns the role and completes the identity chain the role needs.
var (
	ErrInvitationNotFound      = errors.New("invitation not found")
	ErrInvitationExpired       = errors.New("invitation has expired")
	ErrInvitationUsed          = errors.New("invitation has already been used")
	ErrInvitationTenantDeleted = errors.New("the school for this invitation has been deleted")
	ErrInvitationNameRequired  = errors.New("first name and last name are required")
	// ErrInvitationOwnerRequired reports an acceptance for an address that
	// already has an account without a session of that account.
	ErrInvitationOwnerRequired = errors.New("sign in to the invited account before accepting")
	ErrInvitationOwnerMismatch = errors.New("the signed-in account does not own this invitation")
	// ErrAccountAlreadyHasTenantAccess reports an invitation to a school the
	// account can already sign in to.
	ErrAccountAlreadyHasTenantAccess = errors.New("account already has access to tenant")
	// ErrRoleGrantNotPermitted reports an inviter handing out a role beyond
	// its own authority.
	ErrRoleGrantNotPermitted = errors.New("role grant is not permitted")
	// ErrLehrkraftNoCaregiver reports the refused combination of the
	// Lehrkraft role with the caregiver upgrade (#1772).
	ErrLehrkraftNoCaregiver = errors.New("the Lehrkraft role cannot be combined with caregiver rights")
	// ErrPasswordMismatch reports an acceptance whose two password fields
	// differ.
	ErrPasswordMismatch = errors.New("passwords don't match")
)

// SchoolInvitation is one auth.invitation_tokens row.
type SchoolInvitation struct {
	ID               int64
	TenantID         int64
	Email            string
	Token            string
	RoleID           int64
	RoleName         string
	ExpiresAt        time.Time
	UsedAt           *time.Time
	CreatedBy        *int64
	CreatorEmail     string
	FirstName        *string
	LastName         *string
	Position         *string
	CaregiverEnabled bool
	PersonID         *int64
	Delivery         TokenDelivery
	CreatedAt        time.Time
}

// Redeemable reports whether the invitation can still be accepted at now.
func (i SchoolInvitation) Redeemable(now time.Time) bool {
	return i.UsedAt == nil && i.ExpiresAt.After(now)
}

// Validate rejects an invitation that cannot be stored. A link is never
// stored already expired or used, so no delivery carries a dead link.
func (i *SchoolInvitation) Validate(now time.Time) error {
	switch {
	case i.Email == "":
		return errors.New("email is required")
	case i.Token == "":
		return errors.New("token value is required")
	case i.RoleID <= 0:
		return errors.New("role id is required")
	case i.UsedAt != nil:
		return ErrInvitationUsed
	case !i.ExpiresAt.After(now):
		return ErrInvitationExpired
	}
	return nil
}

// SchoolInvitationRequest describes the invitation to create.
type SchoolInvitationRequest struct {
	Email            string
	RoleID           int64
	TenantID         int64
	FirstName        *string
	LastName         *string
	Position         *string
	CaregiverEnabled bool
	// PersonID is the existing, account-less person the invitee becomes on
	// acceptance; the staff import sets it (#2600).
	PersonID   *int64
	CreatedBy  int64
	SchoolName string
	// ActorPermissions are the inviting account's permissions. The empty set
	// grants nothing beyond the user tier, so a caller that forgets it fails
	// closed.
	ActorPermissions []string
	// OperatorGrant marks an invitation issued from the operator portal,
	// whose authority does not come from tenant permissions.
	OperatorGrant bool
}

// InvitationRegistration is the data an invitee supplies when accepting.
type InvitationRegistration struct {
	// OwnerAccessToken is a backend-signed session, never a client-supplied
	// account id.
	OwnerAccessToken string
	FirstName        string
	LastName         string
	Password         string
	ConfirmPassword  string
}

// Names resolves the invitee's name from the registration, falling back to
// the names the invitation carries.
func (r InvitationRegistration) Names(invitation SchoolInvitation) (firstName, lastName string, err error) {
	firstName = strings.TrimSpace(r.FirstName)
	lastName = strings.TrimSpace(r.LastName)
	if firstName == "" && invitation.FirstName != nil {
		firstName = strings.TrimSpace(*invitation.FirstName)
	}
	if lastName == "" && invitation.LastName != nil {
		lastName = strings.TrimSpace(*invitation.LastName)
	}
	if firstName == "" || lastName == "" {
		return "", "", ErrInvitationNameRequired
	}
	return firstName, lastName, nil
}

// InvitationPortal names where an invitation is accepted. School-portal
// roles accept on the school portal, because that is where their login is
// (#2207).
type InvitationPortal string

const (
	InvitationPortalTenant InvitationPortal = "tenant"
	InvitationPortalSchool InvitationPortal = "school"
)

// InvitationPreview is the public-safe view of a redeemable invitation.
type InvitationPreview struct {
	Portal               InvitationPortal
	RequiresAccountLogin bool
	Email                string
	RoleName             string
	FirstName            *string
	LastName             *string
	Position             *string
	CaregiverEnabled     bool
	ExpiresAt            time.Time
}
