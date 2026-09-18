package identityaccess

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrTenantRequiredForRoleAssignment reports a registration or link that
	// hands out a role without naming the school the role belongs to.
	ErrTenantRequiredForRoleAssignment = errors.New("tenant context is required when assigning a role during registration")

	// ErrAccountProvisioningUnavailable reports a composition without the
	// account lifecycle dependencies the provisioning flows need.
	ErrAccountProvisioningUnavailable = errors.New("account provisioning is not composed")
)

// SchoolAccountIdentity carries the person fields needed to make an account
// staff at a school. There is no other source for them: an account holds an
// email and a username, never a person's name.
type SchoolAccountIdentity struct {
	FirstName string
	LastName  string
	TagID     *string
}

// SchoolAccountRegistration creates one account at one school.
//
// RoleID is optional; without it the account is created and mapped but holds
// no role, and without a TenantID only the account is created at all.
// Identity is optional too: with it the account is staff at the school the
// moment the transaction commits, without it the caller owns the identity
// chain. Splitting those two across separate requests is what leaves
// accounts that hold a role and are not staff (#2222).
type SchoolAccountRegistration struct {
	TenantID int64
	Email    string
	Username string
	Password string
	RoleID   *int64
	Identity *SchoolAccountIdentity
}

// SchoolAccountLink gives an existing account access to one school. The
// password is never touched — the account keeps its credential.
type SchoolAccountLink struct {
	TenantID int64
	Email    string
	RoleID   *int64
	Identity *SchoolAccountIdentity
}

// RegisteredAccount is the account row a registration or a link produced. A
// link fills only what it had to read to authorize itself — the id, the
// address, the name and the active flag — because that is all its caller
// reports back.
type RegisteredAccount struct {
	ID        int64
	Email     string
	Username  string
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
	LastLogin *time.Time
}

// ProvisionedAccount is what a registration or a link produced: the account
// and what it provisioned at the school. Identity is nil for the roles that
// run without a staff record and whenever the caller supplied none.
type ProvisionedAccount struct {
	Account  RegisteredAccount
	Identity *SchoolIdentity
}

// AccountProvisioning creates accounts at a school and gives existing
// accounts access to one (#3332): the admin registration and link routes,
// the operator-led school account creation and the seeder.
//
// Both commands write the account, its school mapping, its role and its
// identity chain in one transaction. Refusals arrive wrapped in an
// AuthenticationError whose Op names the flow, carrying
// ErrEmailAlreadyExists, ErrUsernameAlreadyExists, ErrPasswordTooWeak,
// ErrTenantRequiredForRoleAssignment, ErrAccountNotFound,
// ErrAccountInactive, ErrRoleLehrkraftCaregiverProfile, one of the role
// policy sentinels or one of the school identity request errors.
type AccountProvisioning interface {
	RegisterSchoolAccount(ctx context.Context, request SchoolAccountRegistration) (ProvisionedAccount, error)
	LinkSchoolAccount(ctx context.Context, request SchoolAccountLink) (ProvisionedAccount, error)
}

func (m *Module) RegisterSchoolAccount(ctx context.Context, request SchoolAccountRegistration) (ProvisionedAccount, error) {
	return m.engine.RegisterSchoolAccount(ctx, request)
}

func (m *Module) LinkSchoolAccount(ctx context.Context, request SchoolAccountLink) (ProvisionedAccount, error) {
	return m.engine.LinkSchoolAccount(ctx, request)
}
