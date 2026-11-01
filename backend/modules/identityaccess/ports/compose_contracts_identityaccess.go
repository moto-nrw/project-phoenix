package ports

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// PersonDirectory reads the person name of an account in the tenant the
// context carries.
type PersonDirectory interface {
	FindPersonName(ctx context.Context, accountID int64) (firstName, lastName string, found bool, err error)
}

// PasswordVerifier checks a password against its stored hash.
type PasswordVerifier interface {
	VerifyPassword(password, hash string) (bool, error)
}

// MFAPolicyLock pins a school's MFA mode for the caller's transaction.
type MFAPolicyLock interface {
	LockMFAPolicySharedForTenant(ctx context.Context, tenantID int64) error
}

// PushSubscriptionCleanup removes the Delivery platform's push rows a
// revocation orphans. portal is "staff", "school" or "parent".
type PushSubscriptionCleanup interface {
	DeleteStaffByAccount(ctx context.Context, accountID int64) error
	DeleteSchoolByAccount(ctx context.Context, accountID int64) error
	DeleteParentByAccount(ctx context.Context, accountID int64) error
	DeleteByTokenFamily(ctx context.Context, accountID int64, familyID string) error
	DeleteUnboundByAccount(ctx context.Context, accountID, tenantID int64, portal string) error
	DeleteOrphaned(ctx context.Context) error
}

// SessionDependencies are the seams the account-authentication flows need
// beyond the database. TenantRuntime attaches the unit of work the flows
// open their transactions under; nil leaves the context as is.
type SessionDependencies struct {
	// PasswordLoginPolicy resolves installation-wide registry defaults. School overrides cannot weaken a global identity lock.
	PasswordLoginPolicy func() (int, time.Duration, error)
	Schools             SchoolDirectory
	// Traeger reads the Träger memberships of Organisation & Tenancy the
	// Träger-Portal sign-in and refresh re-check (ADR 0084).
	Traeger TraegerMemberships
	// PortalModules locks the parents and school portals of a school that
	// has not booked their module (#4895).
	PortalModules PortalModules
	Persons       PersonDirectory
	Passwords     PasswordVerifier
	Codec         TokenCodec
	MFALock       MFAPolicyLock
	Audit         AuthAudit
	Push          PushSubscriptionCleanup
	TenantRuntime func(context.Context) context.Context
	Logger        *slog.Logger
}

// PasswordPolicy validates and hashes parent account passwords.
type PasswordPolicy interface {
	ValidatePasswordStrength(password string) error
	HashPassword(password string) (string, error)
}

// CallerMemo is the request-scoped slot the caller context memoizes the
// identity chain in (#2099): Entry returns the value kept for (tenant,
// account), creating it on first use, and Evict drops it.
type CallerMemo interface {
	Entry(tenantID, accountID int64, create func() any) any
	Evict(tenantID, accountID int64)
}

// DemoDependencies are the Organisation & Tenancy facts the demo flow needs
// when the Identity & Access module is composed for the demo environment.
type DemoDependencies struct {
	Schools     DemoSchools
	Mail        identityaccess.DemoAccessMail
	NewToken    func() (raw, fingerprint string, err error)
	Fingerprint func(raw string) string
	// OperatorWithoutSecondFactor lets the operator sign in with the password
	// alone (#3460). The demo's mail lock drops the sign-in code, so the demo
	// process that seeds every demo school could never finish the second
	// factor; the demo host keeps the operator surface off the internet.
	OperatorWithoutSecondFactor bool
}

// MFASettings resolves the school settings that parameterise the account
// gate. Each answers the school's override for a positive tenant and the
// registry default otherwise.
type MFASettings interface {
	MFAMode(ctx context.Context, tenantID int64) (string, error)
	// MFAModeInTx reads on the caller's transaction, past the
	// request-scoped memo cache.
	MFAModeInTx(ctx context.Context, tenantID int64) (string, error)
	TrustedDeviceEnabled(ctx context.Context, tenantID int64) (bool, error)
	TrustedDeviceDays(ctx context.Context, tenantID int64) (int, error)
	LockoutThreshold(ctx context.Context, tenantID int64) (int, error)
	LockoutDuration(ctx context.Context, tenantID int64) (time.Duration, error)
}

// ShortCodeHasher hashes and verifies the e-mail codes through the
// project-wide Argon2id helper.
type ShortCodeHasher interface {
	HashShortCode(plain string) (string, error)
	VerifyShortCode(plain, encodedHash string) (bool, error)
}

// PasskeyDependencies are the relying-party facts the ceremonies need. A
// composition without them serves the MFA flows and reports
// ErrPasskeyFlowsUnavailable for every ceremony.
type PasskeyDependencies struct {
	// RPID is the configured relying party. A localhost value follows the
	// request's own host, so every school subdomain works in development.
	RPID   string
	RPName string
	// TenantDomain is the base domain the school subdomains live under.
	TenantDomain string
	// OperatorFrontendURL is the operator portal the operator ceremonies are
	// pinned to.
	OperatorFrontendURL string
	// CatererFrontendURL is the caterer portal the caterer's ceremonies are
	// pinned to (#4497); empty leaves them unavailable.
	CatererFrontendURL string
}

// MFADependencies compose the account and operator MFA flows (#3331). They
// require Sessions, whose audit ledger the account events append to;
// compositions without them report ErrAccountMFAUnavailable and
// ErrOperatorMFAUnavailable.
type MFADependencies struct {
	Settings MFASettings
	Codec    MFAChallengeCodec
	Codes    ShortCodeHasher
	Mail     MFAMail
	// ResetNotice mails the caterer the operator's reset of its second
	// factor (#4497); nil leaves the reset unavailable.
	ResetNotice   CatererMFAResetNotice
	OperatorAudit OperatorActionLog
	// JWTSecret derives the trusted-device HMAC key, so the cookie signature
	// and the JWT signing key stay independent.
	JWTSecret string
	Passkeys  *PasskeyDependencies
	Logger    *slog.Logger
	// Capability lets a composition serve the account second factor itself
	// instead of composing it over native records. The service root leaves it nil;
	// a root that already holds the capability — a behaviour test driving
	// the login branches, a future in-process fake — supplies it here, and
	// the login gate, the MFA routes and the passkey registration all reach
	// that one implementation.
	Capability identityaccess.AccountMFA
}

// InvitationOwnerTokens verifies that the caller holds a live session of the
// invited account.
type InvitationOwnerTokens interface {
	// AccountOfAccessToken returns the account a usable access token proves,
	// or zero when the token proves nothing.
	AccountOfAccessToken(token string) (int64, error)
}

// SchoolInvitationDependencies compose the invitation flows. They require
// the session and lifecycle dependencies: the flows reuse the school
// directory, the role storage and the identity chain.
type SchoolInvitationDependencies struct {
	EmploymentReset *EmploymentResetCommands
	Grants          InvitationGrantPolicy
	Owners          InvitationOwnerTokens
	// DeliveryFor builds the delivery over the recorder of its outcome; the
	// composition calls it once.
	DeliveryFor func(identityaccess.SchoolInvitationDeliveryRecorder) SchoolInvitationDelivery
	Passwords   PasswordPolicy
	Expiry      time.Duration
	Logger      *slog.Logger
}
