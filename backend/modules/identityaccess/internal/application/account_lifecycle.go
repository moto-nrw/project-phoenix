package application

import (
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// AccountLifecycle runs the account lifecycle flows (#3225): staff PIN
// verification and lockout, the admin staff-view preview, staff offboarding,
// the school identity chain a personnel role requires, parent accounts and
// guardian relative access. Identity-owned rows are read and written through
// the module's store; the facts of other owners (persons, staff, guardian
// profiles, relationships, audit evidence, the password and PIN hashers, the
// retained role management and invitation delivery) arrive through the
// consumer-owned ports the composition binds.
type AccountLifecycle struct {
	sessions    *Service
	auth        *AccountAuthentication
	store       ports.AccountLifecycleStore
	logins      ports.AccountLoginStore
	staff       ports.StaffDirectory
	profiles    ports.CaregiverProfiles
	roles       ports.RolePolicy
	pins        ports.PINHasher
	lockout     ports.LockoutPolicy
	audit       ports.PreviewAudit
	codec       ports.AccessTokenCodec
	admin       ports.AccountAdministration
	passwords   ports.PasswordPolicy
	guardians   ports.GuardianDirectory
	invitations ports.GuardianInvitationStore
	delivery    ports.GuardianInvitationDelivery
	enrollments ports.GuardianEnrollments
	schools     ports.SchoolDirectory
	financial   ports.FinancialAudit
	runtime     ports.Runtime
	rfid        ports.Store
	logger      *slog.Logger
}

// AccountLifecycleDependencies are the ports the lifecycle flows consume.
type AccountLifecycleDependencies struct {
	Store       ports.AccountLifecycleStore
	Logins      ports.AccountLoginStore
	RFID        ports.Store
	Staff       ports.StaffDirectory
	Profiles    ports.CaregiverProfiles
	Roles       ports.RolePolicy
	PINs        ports.PINHasher
	Lockout     ports.LockoutPolicy
	Audit       ports.PreviewAudit
	Codec       ports.AccessTokenCodec
	Admin       ports.AccountAdministration
	Passwords   ports.PasswordPolicy
	Guardians   ports.GuardianDirectory
	Invitations ports.GuardianInvitationStore
	Delivery    ports.GuardianInvitationDelivery
	Enrollments ports.GuardianEnrollments
	Schools     ports.SchoolDirectory
	Financial   ports.FinancialAudit
	Runtime     ports.Runtime
	Logger      *slog.Logger
}

// NewAccountLifecycle composes the flows over the session service and the
// account-authentication flows the preview and offboarding borrow claims and
// session revocation from.
func NewAccountLifecycle(sessions *Service, auth *AccountAuthentication, deps AccountLifecycleDependencies) (*AccountLifecycle, error) {
	switch {
	case sessions == nil:
		return nil, fmt.Errorf("identity access account lifecycle: session service is required")
	case auth == nil:
		return nil, fmt.Errorf("identity access account lifecycle: account authentication is required")
	case deps.Store == nil, deps.Logins == nil, deps.RFID == nil:
		return nil, fmt.Errorf("identity access account lifecycle: stores are required")
	case deps.Staff == nil, deps.Profiles == nil, deps.Roles == nil:
		return nil, fmt.Errorf("identity access account lifecycle: staff directory, caregiver profiles and role policy are required")
	case deps.PINs == nil, deps.Lockout == nil:
		return nil, fmt.Errorf("identity access account lifecycle: pin hasher and lockout policy are required")
	case deps.Audit == nil, deps.Codec == nil:
		return nil, fmt.Errorf("identity access account lifecycle: preview audit and token codec are required")
	case deps.Admin == nil, deps.Passwords == nil:
		return nil, fmt.Errorf("identity access account lifecycle: account administration and password policy are required")
	case deps.Guardians == nil, deps.Invitations == nil, deps.Delivery == nil, deps.Financial == nil:
		return nil, fmt.Errorf("identity access account lifecycle: guardian directory, invitation store, delivery and financial audit are required")
	case deps.Schools == nil:
		return nil, fmt.Errorf("identity access account lifecycle: school directory is required")
	case deps.Runtime == nil:
		return nil, fmt.Errorf("identity access account lifecycle: tenant runtime is required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &AccountLifecycle{
		sessions: sessions, auth: auth, store: deps.Store, logins: deps.Logins, rfid: deps.RFID, staff: deps.Staff,
		profiles: deps.Profiles, roles: deps.Roles, pins: deps.PINs, lockout: deps.Lockout, audit: deps.Audit, codec: deps.Codec, admin: deps.Admin,
		passwords: deps.Passwords, guardians: deps.Guardians, invitations: deps.Invitations, delivery: deps.Delivery,
		enrollments: deps.Enrollments, schools: deps.Schools,
		financial: deps.Financial, runtime: deps.Runtime, logger: logger,
	}, nil
}
