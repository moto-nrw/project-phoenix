package organizationtenancy

import (
	"errors"
	"fmt"
)

// The identity steps of operator provisioning — creating a school account,
// inviting a school admin, granting a role — run in Identity & Access. The
// operator routes classify their refusals and may not name that owner, so
// the provisioning capability reports them in its own contract and the
// composition root translates once (#3364). The texts are the ones the
// operator surface renders verbatim.

// ProvisioningIdentityError is the envelope an identity step reports. Op
// names the step, so the routes can treat every refusal of an invitation
// creation as a client error without enumerating its causes.
type ProvisioningIdentityError struct {
	Op  string
	Err error
}

func (e *ProvisioningIdentityError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("identity error during %s", e.Op)
	}
	return fmt.Sprintf("identity error during %s: %v", e.Op, e.Err)
}

func (e *ProvisioningIdentityError) Unwrap() error { return e.Err }

// OpCreateInvitation names the school-admin invitation step.
const OpCreateInvitation = "create invitation"

// The identity refusals the operator provisioning routes classify on.
var (
	// ErrAccountEmailExists reports an address that is already registered.
	ErrAccountEmailExists = errors.New("Diese E-Mail-Adresse ist bereits registriert") //nolint:staticcheck // ST1005: user-facing German message
	// ErrAccountUsernameExists reports a name that is already taken.
	ErrAccountUsernameExists = errors.New("Dieser Benutzername ist bereits vergeben") //nolint:staticcheck // ST1005: user-facing German message
	// ErrAccountNotFound reports an account the operator named that is gone.
	ErrAccountNotFound = errors.New("account not found")
	// ErrInvitationNameRequired reports an acceptance without both names.
	ErrInvitationNameRequired = errors.New("first name and last name are required")
	// ErrPasswordMismatch reports a confirmation that does not match.
	ErrPasswordMismatch = errors.New("passwords don't match")
	// ErrPasswordTooWeak reports a credential the password policy refused.
	ErrPasswordTooWeak = errors.New("password doesn't meet complexity requirements")
)
