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
	d.source.DispatchOperatorInvitation(ctx, publicOperatorInvitation(invitation), inviterName, d.expiry)
}

type operatorEmailChangeDelivery struct {
	source OperatorEmailChangeDelivery
	expiry time.Duration
}

func (d operatorEmailChangeDelivery) DispatchOperatorEmailChangeVerification(ctx context.Context, change domain.OperatorEmailChange) {
	d.source.DispatchOperatorEmailChangeVerification(ctx, publicOperatorEmailChange(change), d.expiry)
}

func (d operatorEmailChangeDelivery) DispatchOperatorEmailChangeRequested(ctx context.Context, operator domain.Operator, maskedNewEmail string) {
	d.source.DispatchOperatorEmailChangeRequested(ctx, identityaccess.Operator(operator), maskedNewEmail)
}

func (d operatorEmailChangeDelivery) DispatchOperatorEmailChangeConfirmed(ctx context.Context, oldEmail, displayName string) {
	d.source.DispatchOperatorEmailChangeConfirmed(ctx, oldEmail, displayName)
}

// --- engine methods --------------------------------------------------------

var errOperatorProvisioningUnavailable = identityaccess.ErrOperatorProvisioningUnavailable

func (e engine) InviteOperator(ctx context.Context, request identityaccess.OperatorInvitationRequest) error {
	if e.operatorProvisioning == nil {
		return errOperatorProvisioningUnavailable
	}
	return operatorProvisioningError(e.operatorProvisioning.InviteOperator(e.attach(ctx), domain.OperatorInvitationRequest{
		Email: request.Email, DisplayName: request.DisplayName, CreatedBy: request.CreatedBy, IPAddress: request.IPAddress,
	}))
}

func (e engine) ValidateOperatorInvitation(ctx context.Context, token string) (identityaccess.OperatorInvitationPreview, error) {
	if e.operatorProvisioning == nil {
		return identityaccess.OperatorInvitationPreview{}, errOperatorProvisioningUnavailable
	}
	preview, err := e.operatorProvisioning.ValidateOperatorInvitation(e.attach(ctx), token)
	return identityaccess.OperatorInvitationPreview(preview), operatorProvisioningError(err)
}

func (e engine) AcceptOperatorInvitation(ctx context.Context, acceptance identityaccess.OperatorInvitationAcceptance) (identityaccess.Operator, error) {
	if e.operatorProvisioning == nil {
		return identityaccess.Operator{}, errOperatorProvisioningUnavailable
	}
	operator, err := e.operatorProvisioning.AcceptOperatorInvitation(e.attach(ctx), domain.OperatorInvitationAcceptance{
		Token: acceptance.Token, DisplayName: acceptance.DisplayName,
		Password: acceptance.Password, IPAddress: acceptance.IPAddress,
	})
	return identityaccess.Operator(operator), operatorProvisioningError(err)
}

func (e engine) ListPendingOperatorInvitations(ctx context.Context) ([]identityaccess.OperatorInvitation, error) {
	if e.operatorProvisioning == nil {
		return nil, errOperatorProvisioningUnavailable
	}
	invitations, err := e.operatorProvisioning.ListPendingOperatorInvitations(e.attach(ctx))
	if err != nil {
		return nil, operatorProvisioningError(err)
	}
	result := make([]identityaccess.OperatorInvitation, 0, len(invitations))
	for _, invitation := range invitations {
		result = append(result, publicOperatorInvitation(invitation))
	}
	return result, nil
}

func (e engine) RevokeOperatorInvitationByActor(ctx context.Context, invitationID, actorID int64, ipAddress string) error {
	if e.operatorProvisioning == nil {
		return errOperatorProvisioningUnavailable
	}
	return operatorProvisioningError(e.operatorProvisioning.RevokeOperatorInvitation(e.attach(ctx), invitationID, actorID, ipAddress))
}

func (e engine) ResendOperatorInvitationByActor(ctx context.Context, invitationID, actorID int64, ipAddress string) error {
	if e.operatorProvisioning == nil {
		return errOperatorProvisioningUnavailable
	}
	return operatorProvisioningError(e.operatorProvisioning.ResendOperatorInvitation(e.attach(ctx), invitationID, actorID, ipAddress))
}

func (e engine) InitiateOperatorEmailChange(ctx context.Context, request identityaccess.OperatorEmailChangeRequest) error {
	if e.operatorProvisioning == nil {
		return errOperatorProvisioningUnavailable
	}
	return operatorProvisioningError(e.operatorProvisioning.InitiateOperatorEmailChange(e.attach(ctx), domain.OperatorEmailChangeRequest{
		OperatorID: request.OperatorID, NewEmail: request.NewEmail,
		CurrentPassword: request.CurrentPassword, IPAddress: request.IPAddress,
	}))
}

func (e engine) ConfirmOperatorEmailChange(ctx context.Context, token, ipAddress string) (identityaccess.OperatorEmailChangeResult, error) {
	if e.operatorProvisioning == nil {
		return identityaccess.OperatorEmailChangeResult{}, errOperatorProvisioningUnavailable
	}
	result, err := e.operatorProvisioning.ConfirmOperatorEmailChange(e.attach(ctx), token, ipAddress)
	return identityaccess.OperatorEmailChangeResult(result), operatorProvisioningError(err)
}

func (e engine) CleanupOperatorEmailChanges(ctx context.Context) (int, error) {
	if e.operatorProvisioning == nil {
		return 0, errOperatorProvisioningUnavailable
	}
	cleaned, err := e.operatorProvisioning.CleanupOperatorEmailChanges(e.attach(ctx))
	return cleaned, operatorProvisioningError(err)
}

var operatorProvisioningSentinels = []struct {
	internal error
	public   error
}{
	{domain.ErrOperatorEmailExists, identityaccess.ErrOperatorEmailExists},
	{domain.ErrOperatorInvitationRateLimited, identityaccess.ErrOperatorInvitationRateLimited},
	{domain.ErrOperatorEmailChangeRateLimited, identityaccess.ErrOperatorEmailChangeRateLimited},
	{domain.ErrOperatorEmailChangeSameEmail, identityaccess.ErrOperatorEmailChangeSameEmail},
	{domain.ErrOperatorEmailInUse, identityaccess.ErrOperatorEmailInUse},
	// The link sentinels: the operator mapping does not carry them, because
	// the storage capability reports them through mapError instead.
	{domain.ErrOperatorInvitationNotFound, identityaccess.ErrOperatorInvitationNotFound},
	{domain.ErrOperatorEmailChangeNotFound, identityaccess.ErrOperatorEmailChangeNotFound},
}

// operatorProvisioningError translates a flow error to the public contract.
// The invitation and e-mail change sentinels gain their public twin;
// everything else falls through to the operator mapping, which carries the
// operator, credential and link sentinels these flows share.
func operatorProvisioningError(err error) error {
	if err == nil {
		return nil
	}
	for _, sentinel := range operatorProvisioningSentinels {
		if !errors.Is(err, sentinel.internal) {
			continue
		}
		if err == sentinel.internal {
			return sentinel.public
		}
		return &translatedError{text: err.Error(), public: sentinel.public, cause: err}
	}
	return operatorError(err)
}
