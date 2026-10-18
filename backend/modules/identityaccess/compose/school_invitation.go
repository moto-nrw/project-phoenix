package compose

import (
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// The school invitation flows (#2722) need three seams the root binds: the
// role-grant decision of Security Runtime, the access token that proves an
// invitee owns the invited account, and the mail.

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
	Grants identityPorts.InvitationGrantPolicy
	Owners InvitationOwnerTokens
	// DeliveryFor builds the delivery over the recorder of its outcome; the
	// composition calls it once.
	DeliveryFor func(identityaccess.SchoolInvitationDeliveryRecorder) identityPorts.SchoolInvitationDelivery
	Passwords   PasswordPolicy
	Expiry      time.Duration
	Logger      *slog.Logger
}

func newSchoolInvitation(
	store *postgres.Store,
	roles *application.RoleAdministration,
	lifecycle *application.AccountLifecycle,
	sessions *SessionDependencies,
	lifecycleDeps *LifecycleDependencies,
	deps *SchoolInvitationDependencies,
) (*application.SchoolInvitation, error) {
	if deps == nil {
		return nil, nil
	}
	if sessions == nil || lifecycleDeps == nil || roles == nil || lifecycle == nil {
		return nil, errors.New("identity access compose: the invitation flows require the session and lifecycle dependencies")
	}
	attach := sessions.TenantRuntime
	var delivery identityPorts.SchoolInvitationDelivery
	if deps.DeliveryFor != nil {
		delivery = deps.DeliveryFor(application.NewSchoolInvitationDeliveryRecords(store, attach))
	}
	if deps.Grants == nil || deps.Owners == nil || delivery == nil || deps.Passwords == nil {
		return nil, errors.New("identity access compose: every invitation dependency is required")
	}
	return application.NewSchoolInvitation(application.SchoolInvitationDependencies{
		Store: store, Logins: store, Roles: postgres.NewRoleStore(store),
		Policy:    application.RoleAssignmentPolicy{},
		Grants:    application.InvitationGrantPolicy{Source: deps.Grants},
		Identity:  lifecycle,
		Schools:   application.SchoolDirectory{Source: sessions.Schools},
		Owners:    deps.Owners,
		Passwords: deps.Passwords,
		Delivery:  application.SchoolInvitationDelivery{Source: delivery},
		Runtime:   tenantRuntime{attach: attach, runner: newTransactionRunner()},
		Expiry:    deps.Expiry,
		Logger:    deps.Logger,
	})
}

// --- engine methods --------------------------------------------------------

// invitationLogger is the logger the always-composed invitation maintenance
// uses; compositions without the invitation seams fall back to the default.
func invitationLogger(deps *SchoolInvitationDependencies) *slog.Logger {
	if deps == nil {
		return slog.Default()
	}
	return deps.Logger
}
