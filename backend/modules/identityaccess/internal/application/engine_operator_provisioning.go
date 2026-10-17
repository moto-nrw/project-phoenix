package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

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
	invitations, err := e.tokens.ListRedeemableInvitations(e.attach(ctx))
	if err != nil {
		return nil, operatorProvisioningError(err)
	}
	result := make([]identityaccess.OperatorInvitation, 0, len(invitations))
	for _, invitation := range invitations {
		result = append(result, PublicOperatorInvitation(invitation))
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

func (e engine) ChangeOperatorStatus(ctx context.Context, change identityaccess.OperatorStatusChange) (identityaccess.Operator, error) {
	if e.operatorProvisioning == nil {
		return identityaccess.Operator{}, errOperatorProvisioningUnavailable
	}
	operator, err := e.operatorProvisioning.ChangeOperatorStatus(e.attach(ctx), domain.OperatorStatusChange(change))
	return identityaccess.Operator(operator), operatorProvisioningError(err)
}

func (e engine) DeleteDeactivatedOperators(ctx context.Context) (int, error) {
	if e.operatorProvisioning == nil {
		return 0, errOperatorProvisioningUnavailable
	}
	deleted, err := e.operatorProvisioning.DeleteDeactivatedOperators(e.attach(ctx))
	return deleted, operatorProvisioningError(err)
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
	{domain.ErrOperatorSelfDeactivation, identityaccess.ErrOperatorSelfDeactivation},
	// The link sentinels: the operator mapping does not carry them, because
	// the storage capability reports them through MapError instead.
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
