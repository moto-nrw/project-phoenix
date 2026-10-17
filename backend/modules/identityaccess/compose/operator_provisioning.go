package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The operator invitation and e-mail change flows (#3332) need three seams
// the root binds: the two mail transports and the People Directory rule for
// a routable address, which stays where it is owned instead of becoming a
// second copy of the pattern here.

// OperatorProvisioningDependencies compose the invitation and e-mail change
// flows. They require the operator dependencies: the flows reuse the
// operator ledger and the credential policy.
type OperatorProvisioningDependencies struct {
	Invites           identityPorts.OperatorInvitationDelivery
	Changes           identityPorts.OperatorEmailChangeDelivery
	Format            identityPorts.EmailFormat
	InvitationExpiry  time.Duration
	EmailChangeExpiry time.Duration
}

func newOperatorProvisioning(
	service *application.Service,
	tokens *application.OperatorTokens,
	sessions *SessionDependencies,
	operators *OperatorDependencies,
	deps *OperatorProvisioningDependencies,
) (*application.OperatorProvisioning, error) {
	if deps == nil {
		return nil, nil
	}
	if sessions == nil || operators == nil {
		return nil, errors.New("identity access compose: the provisioning flows require the session and operator dependencies")
	}
	if deps.Invites == nil || deps.Changes == nil || deps.Format == nil {
		return nil, errors.New("identity access compose: every operator provisioning dependency is required")
	}
	attach := sessions.TenantRuntime
	if attach == nil {
		attach = func(ctx context.Context) context.Context { return ctx }
	}
	return application.NewOperatorProvisioning(service, application.OperatorProvisioningDependencies{
		Tokens:    tokens,
		Passwords: sessions.Passwords,
		Hasher:    operators.Passwords,
		Format:    application.EmailFormat{Source: deps.Format},
		Audit:     application.OperatorAudit{Source: operators.Audit},
		Invites:   application.OperatorInvitationDelivery{Source: deps.Invites, Expiry: deps.InvitationExpiry},
		Changes:   application.OperatorEmailChangeDelivery{Source: deps.Changes, Expiry: deps.EmailChangeExpiry},
		Runtime:   tenantRuntime{attach: attach, runner: tenant.NewTransactionRunner()},
		Expiry: application.OperatorProvisioningExpiry{
			Invitation: deps.InvitationExpiry, EmailChange: deps.EmailChangeExpiry,
		},
		Logger: operators.Logger,
	})
}

// --- port adapters ---------------------------------------------------------

// --- engine methods --------------------------------------------------------
