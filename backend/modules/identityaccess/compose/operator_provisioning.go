package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The operator invitation and e-mail change flows (#3332) need three seams
// the root binds: the two mail transports and the People Directory rule for
// a routable address, which stays where it is owned instead of becoming a
// second copy of the pattern here.

// OperatorLinkDelivery mails both operator links.
type OperatorLinkDelivery interface {
	identityPorts.OperatorInvitationDelivery
	identityPorts.OperatorEmailChangeDelivery
}

// OperatorProvisioningDependencies compose the invitation and e-mail change
// flows. They require the operator dependencies: the flows reuse the
// operator ledger and the credential policy.
type OperatorProvisioningDependencies struct {
	Invites identityPorts.OperatorInvitationDelivery
	Changes identityPorts.OperatorEmailChangeDelivery
	// DeliveryFor builds both transports over the recorder of their outcome;
	// the composition calls it once. It replaces Invites and Changes, which
	// must be unset.
	DeliveryFor       func(identityaccess.OperatorLinkDeliveryRecorder) OperatorLinkDelivery
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
	invites, changes := deps.Invites, deps.Changes
	if deps.DeliveryFor != nil && invites == nil && changes == nil {
		// The recorder answers like the module: its engine holds the tokens alone.
		delivery := deps.DeliveryFor(identityaccess.NewModule(application.NewEngine(application.EngineParts{Tokens: tokens}), nil))
		invites, changes = delivery, delivery
	} else if deps.DeliveryFor != nil {
		return nil, errors.New("identity access compose: the operator link transports and their factory are exclusive")
	}
	if invites == nil || changes == nil || deps.Format == nil {
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
		Invites:   application.OperatorInvitationDelivery{Source: invites, Expiry: deps.InvitationExpiry},
		Changes:   application.OperatorEmailChangeDelivery{Source: changes, Expiry: deps.EmailChangeExpiry},
		Runtime:   tenantRuntime{attach: attach, runner: tenant.NewTransactionRunner()},
		Expiry: application.OperatorProvisioningExpiry{
			Invitation: deps.InvitationExpiry, EmailChange: deps.EmailChangeExpiry,
		},
		Logger: operators.Logger,
	})
}

// --- port adapters ---------------------------------------------------------

// --- engine methods --------------------------------------------------------
