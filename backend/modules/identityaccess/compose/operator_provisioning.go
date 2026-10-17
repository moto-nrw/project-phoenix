package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The operator invitation and e-mail change flows (#3332) need three seams
// the root binds: the two mail transports and the People Directory rule for
// a routable address, which stays where it is owned instead of becoming a
// second copy of the pattern here.

// OperatorInvitationDelivery mails the invitation link. The module decides
// what the link is worth and when it goes out; the root owns the transport,
// the portal host and the template, and records the outcome back through
// the module.
type OperatorInvitationDelivery interface {
	DispatchOperatorInvitation(ctx context.Context, invitation identityaccess.OperatorInvitation, inviterName string, expiry time.Duration)
}

// OperatorEmailChangeDelivery mails the three messages an e-mail change
// sends: the confirmation link to the new address, the heads-up to the
// current one and the notice to the address that was replaced.
type OperatorEmailChangeDelivery interface {
	DispatchOperatorEmailChangeVerification(ctx context.Context, change identityaccess.OperatorEmailChange, expiry time.Duration)
	DispatchOperatorEmailChangeRequested(ctx context.Context, operator identityaccess.Operator, maskedNewEmail string)
	DispatchOperatorEmailChangeConfirmed(ctx context.Context, oldEmail, displayName string)
}

// EmailFormat is the People Directory contact rule for a routable address.
type EmailFormat interface {
	IsRoutable(address string) bool
}

// OperatorProvisioningDependencies compose the invitation and e-mail change
// flows. They require the operator dependencies: the flows reuse the
// operator ledger and the credential policy.
type OperatorProvisioningDependencies struct {
	Invites           OperatorInvitationDelivery
	Changes           OperatorEmailChangeDelivery
	Format            EmailFormat
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
		Format:    emailFormat{deps.Format},
		Audit:     operatorAudit{operators.Audit},
		Invites:   operatorInvitationDelivery{source: deps.Invites, expiry: deps.InvitationExpiry},
		Changes:   operatorEmailChangeDelivery{source: deps.Changes, expiry: deps.EmailChangeExpiry},
		Runtime:   tenantRuntime{attach: attach, runner: tenant.NewTransactionRunner()},
		Expiry: application.OperatorProvisioningExpiry{
			Invitation: deps.InvitationExpiry, EmailChange: deps.EmailChangeExpiry,
		},
		Logger: operators.Logger,
	})
}

// --- port adapters ---------------------------------------------------------

type emailFormat struct{ source EmailFormat }

func (f emailFormat) IsRoutable(address string) bool { return f.source.IsRoutable(address) }

type operatorInvitationDelivery struct {
	source OperatorInvitationDelivery
	expiry time.Duration
}

func (d operatorInvitationDelivery) DispatchOperatorInvitation(ctx context.Context, invitation domain.OperatorInvitation, inviterName string) {
	d.source.DispatchOperatorInvitation(ctx, application.PublicOperatorInvitation(invitation), inviterName, d.expiry)
}

type operatorEmailChangeDelivery struct {
	source OperatorEmailChangeDelivery
	expiry time.Duration
}

func (d operatorEmailChangeDelivery) DispatchOperatorEmailChangeVerification(ctx context.Context, change domain.OperatorEmailChange) {
	d.source.DispatchOperatorEmailChangeVerification(ctx, application.PublicOperatorEmailChange(change), d.expiry)
}

func (d operatorEmailChangeDelivery) DispatchOperatorEmailChangeRequested(ctx context.Context, operator domain.Operator, maskedNewEmail string) {
	d.source.DispatchOperatorEmailChangeRequested(ctx, identityaccess.Operator(operator), maskedNewEmail)
}

func (d operatorEmailChangeDelivery) DispatchOperatorEmailChangeConfirmed(ctx context.Context, oldEmail, displayName string) {
	d.source.DispatchOperatorEmailChangeConfirmed(ctx, oldEmail, displayName)
}

// --- engine methods --------------------------------------------------------
