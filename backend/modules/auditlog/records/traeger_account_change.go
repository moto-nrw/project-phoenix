package records

import (
	"errors"
	"time"
)

// The actions of the Träger-Büro account ledger (#4650).
const (
	TraegerAccountInvitationCreated  = "invitation_created"
	TraegerAccountInvitationRevoked  = "invitation_revoked"
	TraegerAccountInvitationAccepted = "invitation_accepted"
	TraegerAccountRolesChanged       = "roles_changed"
	TraegerAccountMemberRemoved      = "member_removed"
	TraegerAccountMembersListed      = "members_listed"
)

// TraegerAccountChange is one step of the Träger-Büro account management in
// the Audit-owned audit.traeger_account_changes ledger: who invited, revoked,
// accepted, changed roles, removed a person or read the member list. A Träger
// is no tenant, so the entry carries its Träger and no school.
type TraegerAccountChange struct {
	ID              int64     `bun:"id,pk,autoincrement" json:"id"`
	OrganizationID  int64     `bun:"organization_id,notnull" json:"organization_id"`
	ActorAccountID  int64     `bun:"actor_account_id,notnull" json:"actor_account_id"`
	Action          string    `bun:"action,notnull" json:"action"`
	TargetAccountID *int64    `bun:"target_account_id" json:"target_account_id,omitempty"`
	TargetEmail     *string   `bun:"target_email" json:"target_email,omitempty"`
	RolesBefore     []string  `bun:"roles_before,array,notnull" json:"roles_before"`
	RolesAfter      []string  `bun:"roles_after,array,notnull" json:"roles_after"`
	CreatedAt       time.Time `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
}

// PlatformScoped marks the entry as tenantless: the Audit adapter appends it
// without the tenant handshake every tenant-scoped ledger requires.
func (*TraegerAccountChange) PlatformScoped() bool { return true }

// Validate requires the Träger, the acting account and a known action.
func (c *TraegerAccountChange) Validate() error {
	switch {
	case c.OrganizationID <= 0:
		return errors.New("the Träger account change requires the Träger")
	case c.ActorAccountID <= 0:
		return errors.New("the Träger account change requires the acting account")
	}
	switch c.Action {
	case TraegerAccountInvitationCreated, TraegerAccountInvitationRevoked, TraegerAccountInvitationAccepted,
		TraegerAccountRolesChanged, TraegerAccountMemberRemoved, TraegerAccountMembersListed:
	default:
		return errors.New("unknown Träger account change action")
	}
	if c.RolesBefore == nil {
		c.RolesBefore = []string{}
	}
	if c.RolesAfter == nil {
		c.RolesAfter = []string{}
	}
	return nil
}
