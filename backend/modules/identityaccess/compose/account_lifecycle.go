package compose

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// The account lifecycle flows (#3225) and the role administration (#3314)
// need facts other owners hold and seams the composition root binds: persons,
// staff and caregiver profiles, guardian profiles and relationships, the
// audit evidence, the password hasher, the retained role storage and
// account management and the retained guardian invitation delivery. The
// seams below are expressed in public values; this package adapts them to the
// consumer-owned ports.

// PasswordPolicy validates and hashes parent account passwords.
type PasswordPolicy interface {
	ValidatePasswordStrength(password string) error
	HashPassword(password string) (string, error)
}

// FinancialAudit records the removal of a child's payer.
type FinancialAudit interface {
	RecordPayerRemoved(ctx context.Context, guardianProfileID, studentID, actorAccountID int64) error
}

// LifecycleDependencies are the seams the account lifecycle flows need
// beyond the database and the session dependencies.
type LifecycleDependencies struct {
	Staff     identityPorts.StaffDirectory
	Audit     identityPorts.PreviewAudit
	Passwords PasswordPolicy
	Guardians identityPorts.GuardianDirectory
	Delivery  identityPorts.GuardianInvitationDelivery
	// Enrollments is optional: without it an acceptance claims no
	// pre-account enrollment requests.
	Enrollments identityPorts.GuardianEnrollments
	Financial   FinancialAudit
	Logger      *slog.Logger
}

// newAccountLifecycle composes the lifecycle flows and the role
// administration: offboarding removes roles through the administration, and
// an assignment completes the school identity through the lifecycle.
func newAccountLifecycle(service *application.Service, auth *application.AccountAuthentication, store *postgres.Store, sessions *SessionDependencies, deps *LifecycleDependencies, administration *application.AccountAdministration) (*application.AccountLifecycle, *application.RoleAdministration, error) {
	if deps == nil {
		return nil, nil, nil
	}
	if auth == nil || sessions == nil {
		return nil, nil, errors.New("identity access compose: the lifecycle flows require the session dependencies")
	}
	switch {
	case deps.Staff == nil, deps.Audit == nil,
		deps.Passwords == nil, deps.Guardians == nil, deps.Delivery == nil, deps.Financial == nil:
		return nil, nil, errors.New("identity access compose: every lifecycle dependency is required")
	case administration == nil:
		return nil, nil, errors.New("identity access compose: the account administration is required")
	}
	runtime := tenantRuntime{attach: sessions.TenantRuntime, runner: newTransactionRunner()}
	var lifecycle *application.AccountLifecycle
	roles, err := newRoleAdministration(auth, runtime, postgres.NewRoleStore(store), administration, deps, func() *application.AccountLifecycle { return lifecycle })
	if err != nil {
		return nil, nil, err
	}
	lifecycle, err = application.NewAccountLifecycle(service, auth, application.AccountLifecycleDependencies{
		Store: store, Logins: store, RFID: application.CanonicalTagStore{Store: store},
		Staff:       application.StaffDirectory{Source: deps.Staff},
		Profiles:    application.StaffDirectory{Source: deps.Staff},
		Roles:       application.RolePolicy{},
		Audit:       application.PreviewAudit{Source: deps.Audit},
		Codec:       application.TokenCodec{Source: sessions.Codec},
		Admin:       application.AccountAdministrationSeam{Roles: roles, Accounts: administration},
		Passwords:   deps.Passwords,
		Guardians:   application.GuardianDirectory{Source: deps.Guardians},
		Invitations: store,
		Delivery:    application.GuardianInvitationDelivery{Source: deps.Delivery},
		Enrollments: guardianEnrollments(deps.Enrollments),
		Schools:     application.SchoolDirectory{Source: sessions.Schools},
		Financial:   deps.Financial,
		Runtime:     runtime,
		Logger:      deps.Logger,
	})
	if err != nil {
		return nil, nil, err
	}
	return lifecycle, roles, nil
}

// --- port adapters ---------------------------------------------------------

// guardianEnrollments keeps the optional enrollment claim out of the flows:
// a composition without it reports nothing to claim.
func guardianEnrollments(source identityPorts.GuardianEnrollments) ports.GuardianEnrollments {
	if source == nil {
		return nil
	}
	return application.GuardianEnrollmentClaims{Source: source}
}

// --- engine methods -------------------------------------------------------

// The guardian invitation lifecycle answers with the invitation sentinels
// the school invitation flows share; invitationError falls through to the
// lifecycle mapping for the rest.
