package domain

import (
	"errors"
	"net/mail"
	"strings"
	"time"
)

// ErrTenantRequiredForRoleAssignment refuses a registration or link that
// hands out a role without naming the school the role belongs to: an
// account_roles row without a tenant is a role nobody can scope.
var ErrTenantRequiredForRoleAssignment = errors.New("tenant context is required when assigning a role during registration")

var (
	// ErrAccountEmailRequired refuses a registration without an address.
	ErrAccountEmailRequired = errors.New("email is required")
	// ErrAccountEmailInvalid refuses an address that is not one.
	ErrAccountEmailInvalid = errors.New("invalid email format")
)

// NormalizeAccountEmail lower-cases and trims the address and applies the
// required/format rule every account row has carried since the retained
// model validated it on insert.
func NormalizeAccountEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return "", ErrAccountEmailRequired
	}
	if _, err := mail.ParseAddress(normalized); err != nil {
		return "", ErrAccountEmailInvalid
	}
	return normalized, nil
}

// IsSchoolIdentityRequestError reports whether the error is the caller's
// fault rather than the server's: a missing name, a child's record, an
// unknown or conflicting transponder. Handlers render these as 400.
func IsSchoolIdentityRequestError(err error) bool {
	return errors.Is(err, ErrSchoolIdentityNamesRequired) ||
		errors.Is(err, ErrSchoolIdentityPersonIsStudent) ||
		errors.Is(err, ErrSchoolIdentityTagUnknown) ||
		errors.Is(err, ErrSchoolIdentityTagConflict) ||
		errors.Is(err, ErrSchoolIdentityTagTaken)
}

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
// RoleID is optional; without it the account is created and mapped but
// holds no role. Identity is optional too: with it the account is staff at
// the school the moment the transaction commits, without it only the
// account is created and the caller owns the identity chain. Splitting
// those two across separate requests is what leaves accounts that hold a
// role and are not staff (#2222).
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

// NewSchoolAccount is the account row a registration inserts.
type NewSchoolAccount struct {
	Email        string
	Username     string
	PasswordHash string
}

// RegisteredAccount is the account row a registration or a link produced.
// A link fills only what it had to read to authorize itself — the id, the
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

// ProvisionedAccount is the account a registration or link produced,
// together with what it provisioned at the school. Identity is nil for the
// roles that run without a staff record.
type ProvisionedAccount struct {
	Account  RegisteredAccount
	Identity *SchoolIdentity
}
