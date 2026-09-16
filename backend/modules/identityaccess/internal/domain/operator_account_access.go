package domain

import (
	"errors"
	"slices"
	"strings"
	"time"
)

// Operator-led school access of an account (#3252, issue #1021): which
// schools an existing account may reach and with which role. The data model
// has always allowed one account to hold different roles at several schools;
// these flows create, change and revoke those mappings.
var (
	// ErrAccountTenantAccessNotFound reports an account without an active
	// mapping to the school an operation targets.
	ErrAccountTenantAccessNotFound = errors.New("account has no active access to this school")
	// ErrAccountTenantAccessExists refuses a grant for a school the account
	// already reaches.
	ErrAccountTenantAccessExists = errors.New("account already has access to this school")
	ErrSchoolNotFound            = errors.New("school not found")
	ErrSchoolDeleted             = errors.New("school is soft-deleted")
)

// Base roles and the retired system role the access rules recognise.
const (
	BaseRoleGuardian      = "guardian"
	BaseRoleUser          = "user"
	LegacyTeacherRoleName = "teacher"
)

// AccessAuditIP is the address recorded on tenant-visible auth events an
// operator triggers: the tenant-side request address is not the meaningful
// actor address.
const AccessAuditIP = "0.0.0.0"

// Tenant-visible auth event types the access flows record.
const (
	AuthEventTenantAccessGranted = "tenant_access_granted"
	AuthEventTenantRoleChanged   = "tenant_role_changed"
	AuthEventTenantAccessRevoked = "tenant_access_revoked"
)

// Revocation reasons the access flows hand the session owner.
const (
	RevocationReasonRoleChanged         = "role_changed"
	RevocationReasonTenantAccessRevoked = "tenant_access_revoked"
	RevocationReasonAccountDeactivated  = "account_deactivated"
)

// ManagedAccount is the platform account row the access commands lock and
// decide on.
type ManagedAccount struct {
	ID     int64
	Email  string
	Active bool
}

// TenantMapping is one auth.account_tenants row of an account.
type TenantMapping struct {
	TenantID      int64
	Status        string
	ActivatedAt   *time.Time
	DeactivatedAt *time.Time
}

// RoleFact is the auth.roles row the access rules decide on.
type RoleFact struct {
	ID       int64
	Name     string
	IsSystem bool
	BaseRole *string
	TenantID *int64
}

// AccountTenantRole is one role an account holds at one school.
type AccountTenantRole struct {
	ID       int64
	Name     string
	IsSystem bool
	BaseRole *string
}

// AccountRoleAssignment is one role assignment with the school it applies to.
type AccountRoleAssignment struct {
	TenantID int64
	Role     AccountTenantRole
}

// AccountTenantAccess is one school an account has (or had) access to,
// including the roles it holds there.
type AccountTenantAccess struct {
	TenantID         int64
	SchoolName       string
	SchoolSlug       string
	SchoolActive     bool
	OrganizationID   int64
	OrganizationName string
	Status           string
	ActivatedAt      *time.Time
	DeactivatedAt    *time.Time
	HasPerson        bool
	HasStaff         bool
	Roles            []AccountTenantRole
}

// OrganizationName is the organisation fact the listing shows.
type OrganizationName struct {
	ID   int64
	Name string
}

// AccountIdentityFact reports whether a person and a staff record back the
// account at one school.
type AccountIdentityFact struct {
	TenantID  int64
	HasPerson bool
	HasStaff  bool
}

// AccountPersonIdentity is one person row that carries the account, at any
// school, with the facts the name resolution decides on.
type AccountPersonIdentity struct {
	ID        int64
	TenantID  int64
	FirstName string
	LastName  string
	Deleted   bool
	IsStudent bool
}

// GrantAccountTenantAccess carries a grant: the school, the role and the
// optional person data used when the account has no person record anywhere
// yet.
type GrantAccountTenantAccess struct {
	AccountID  int64
	SchoolID   int64
	RoleID     int64
	FirstName  string
	LastName   string
	Position   string
	OperatorID int64
	ClientIP   string
}

// SchoolIdentityRequest describes the identity chain a school access
// requires: person, staff and (for caregiver roles) teacher rows.
type SchoolIdentityRequest struct {
	AccountID    int64
	TenantID     int64
	Role         RoleFact
	FirstName    string
	LastName     string
	Position     string
	CreatePerson bool
}

// RoleOwnedByOtherFeature reports whether a role change must leave the
// assignment alone: guardian-tier roles carry parent-portal access, the
// user and legacy teacher system roles are managed through the caregiver
// flows.
func RoleOwnedByOtherFeature(role AccountTenantRole) bool {
	if role.BaseRole != nil && strings.EqualFold(strings.TrimSpace(*role.BaseRole), BaseRoleGuardian) {
		return true
	}
	if !role.IsSystem {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(role.Name)) {
	case BaseRoleGuardian, BaseRoleUser, LegacyTeacherRoleName:
		return true
	default:
		return false
	}
}

// RoleBlocksAccessRevocation reports whether school access holding this role
// must be removed through its dedicated flow instead.
func RoleBlocksAccessRevocation(role AccountTenantRole) bool {
	if role.BaseRole != nil && strings.EqualFold(strings.TrimSpace(*role.BaseRole), BaseRoleGuardian) {
		return true
	}
	return role.IsSystem && (strings.EqualFold(role.Name, BaseRoleGuardian) || strings.EqualFold(role.Name, BaseRoleUser) || strings.EqualFold(role.Name, LegacyTeacherRoleName))
}

// RolesAtTenant projects the assignments held at one school.
func RolesAtTenant(assignments []AccountRoleAssignment, tenantID int64) []AccountTenantRole {
	var roles []AccountTenantRole
	for _, assignment := range assignments {
		if assignment.TenantID == tenantID {
			roles = append(roles, assignment.Role)
		}
	}
	return roles
}

// UnambiguousPersonIdentity returns the identity the account already carries
// elsewhere, but only when there is one answer to return.
//
// What comes back becomes a person, and therefore a staff member, at the
// target school, so this is not a convenience lookup. A child's record is
// never the account holder's identity, and only a name every candidate
// agrees on is taken: two different names is an ambiguity this code is no
// more entitled to resolve than the login is. Revoked mappings deliberately
// keep the person row, so the school need not still be actively mapped.
//
// Returns found=false when there is no such identity, whether because none
// exists or because the candidates disagree.
func UnambiguousPersonIdentity(persons []AccountPersonIdentity) (AccountPersonIdentity, bool) {
	var selected *AccountPersonIdentity
	for i := range persons {
		person := persons[i]
		if person.Deleted || person.IsStudent {
			continue
		}
		firstName, lastName := strings.TrimSpace(person.FirstName), strings.TrimSpace(person.LastName)
		if firstName == "" || lastName == "" {
			continue
		}
		if selected == nil {
			selected = &person
			continue
		}
		if firstName != strings.TrimSpace(selected.FirstName) || lastName != strings.TrimSpace(selected.LastName) {
			return AccountPersonIdentity{}, false
		}
		// Same name: pick a stable representative so the outcome does not
		// depend on row order.
		if person.TenantID < selected.TenantID || (person.TenantID == selected.TenantID && person.ID < selected.ID) {
			selected = &person
		}
	}
	if selected == nil {
		return AccountPersonIdentity{}, false
	}
	return *selected, true
}

// SortTenantAccessByOrganization orders the entries by the organisation's
// position in organizations (equal names share a position) and keeps the
// incoming order within one organisation; entries whose organisation is
// unknown are reported as false.
func SortTenantAccessByOrganization(entries []AccountTenantAccess, organizations []OrganizationName) bool {
	names := make(map[int64]string, len(organizations))
	ranks := make(map[int64]int, len(organizations))
	rank := -1
	previous := ""
	for index, organization := range organizations {
		if index == 0 || organization.Name != previous {
			rank++
			previous = organization.Name
		}
		names[organization.ID] = organization.Name
		ranks[organization.ID] = rank
	}
	for index := range entries {
		name, found := names[entries[index].OrganizationID]
		if !found {
			return false
		}
		entries[index].OrganizationName = name
	}
	slices.SortStableFunc(entries, func(left, right AccountTenantAccess) int {
		return ranks[left.OrganizationID] - ranks[right.OrganizationID]
	})
	return true
}

// SortTenantAccessBySchoolName orders the entries by school name, keeping
// the incoming order for equal names.
func SortTenantAccessBySchoolName(entries []AccountTenantAccess) {
	slices.SortStableFunc(entries, func(left, right AccountTenantAccess) int {
		return strings.Compare(left.SchoolName, right.SchoolName)
	})
}
