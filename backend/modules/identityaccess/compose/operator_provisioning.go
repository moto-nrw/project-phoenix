package compose

import (
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
	// DeliveryFor builds both transports over the recorder of their outcome;
	// the composition calls it once.
	DeliveryFor       func(identityaccess.OperatorLinkDeliveryRecorder) OperatorLinkDelivery
	Format            identityPorts.EmailFormat
	InvitationExpiry  time.Duration
	EmailChangeExpiry time.Duration
}

func newOperatorProvisioning(
	service *application.Service,
	tokens *application.OperatorTokens,
	sessions *identityPorts.SessionDependencies,
	operators *OperatorDependencies,
	deps *OperatorProvisioningDependencies,
) (*application.OperatorProvisioning, error) {
	if deps == nil {
		return nil, nil
	}
	if sessions == nil || operators == nil {
		return nil, errors.New("identity access compose: the provisioning flows require the session and operator dependencies")
	}
	var delivery OperatorLinkDelivery
	if deps.DeliveryFor != nil {
		delivery = deps.DeliveryFor(tokens)
	}
	if delivery == nil || deps.Format == nil {
		return nil, errors.New("identity access compose: every operator provisioning dependency is required")
	}
	return application.NewOperatorProvisioning(service, application.OperatorProvisioningDependencies{
		Tokens:    tokens,
		Passwords: sessions.Passwords,
		Hasher:    operators.Passwords,
		Format:    application.EmailFormat{Source: deps.Format},
		Audit:     application.OperatorAudit{Source: operators.Audit},
		Invites:   application.OperatorInvitationDelivery{Source: delivery, Expiry: deps.InvitationExpiry},
		Changes:   application.OperatorEmailChangeDelivery{Source: delivery, Expiry: deps.EmailChangeExpiry},
		Runtime:   tenantRuntime{attach: sessions.TenantRuntime, runner: tenant.NewTransactionRunner()},
		Attach:    sessions.TenantRuntime,
		Expiry: application.OperatorProvisioningExpiry{
			Invitation: deps.InvitationExpiry, EmailChange: deps.EmailChangeExpiry,
		},
		Logger: operators.Logger,
	})
}
