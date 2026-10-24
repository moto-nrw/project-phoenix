package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// The accounts of team members who left (#4047): another operator
// deactivates the account in the dashboard, which ends its sessions at once,
// and the hourly cleanup deletes it once the operator ledger has lost the
// entries written before the deactivation. A reactivation clears the
// deactivation, so the deletion window starts over.

// ChangeOperatorStatus deactivates or reactivates another operator's account
// and records the change in the operator ledger.
func (p *OperatorProvisioning) ChangeOperatorStatus(ctx context.Context, change identityaccess.OperatorStatusChange) (identityaccess.Operator, error) {
	if p == nil {
		return identityaccess.Operator{}, errOperatorProvisioningUnavailable
	}
	ctx = bindRuntime(p.attach, ctx)
	apply := p.deactivateOperator
	if change.Active {
		apply = p.reactivateOperator
	}
	operator, err := apply(ctx, change)
	return identityaccess.Operator(operator), operatorProvisioningError(err)
}

// deactivateOperator locks another operator's account and ends its sessions.
// Deactivating an inactive account keeps its original deactivation time.
func (p *OperatorProvisioning) deactivateOperator(ctx context.Context, change identityaccess.OperatorStatusChange) (domain.Operator, error) {
	if change.OperatorID == change.ActorID {
		return domain.Operator{}, identityaccess.ErrOperatorSelfDeactivation
	}
	return p.changeOperatorStatus(ctx, change, domain.OperatorAuditActionDeactivated, func(txCtx context.Context, operator *domain.Operator) error {
		if operator.Active {
			operator.Active = false
		}
		_, revokeErr := p.operators.RevokeOperatorSessions(txCtx, operator.ID)
		return revokeErr
	})
}

// reactivateOperator lets a deactivated operator sign in again.
func (p *OperatorProvisioning) reactivateOperator(ctx context.Context, change identityaccess.OperatorStatusChange) (domain.Operator, error) {
	return p.changeOperatorStatus(ctx, change, domain.OperatorAuditActionReactivated, func(_ context.Context, operator *domain.Operator) error {
		operator.Active = true
		operator.DeactivatedAt = nil
		return nil
	})
}

// DeleteDeactivatedOperators removes the accounts deactivated before the
// operator ledger cutoff and reports how many went. Platform rows go only in
// an administrative transaction; the hourly cleanup hands its own in.
func (p *OperatorProvisioning) DeleteDeactivatedOperators(ctx context.Context) (deleted int, err error) {
	if p == nil {
		return 0, errOperatorProvisioningUnavailable
	}
	defer func() { err = operatorProvisioningError(err) }()
	ctx = bindRuntime(p.attach, ctx)
	err = p.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		deleted, err = p.operators.deleteDeactivatedOperators(txCtx)
		return err
	})
	return deleted, err
}

// changeOperatorStatus locks the operator's row, applies and records the
// change, then stores the result in one administrative transaction.
func (p *OperatorProvisioning) changeOperatorStatus(ctx context.Context, status identityaccess.OperatorStatusChange, action string, change func(context.Context, *domain.Operator) error) (domain.Operator, error) {
	var result domain.Operator
	err := p.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		operator, err := p.operators.findOperatorForUpdate(txCtx, status.OperatorID)
		if err != nil {
			return err
		}
		if err := change(txCtx, &operator); err != nil {
			return err
		}
		if _, err := p.operators.UpdateOperator(txCtx, identityaccess.Operator(operator)); err != nil {
			return err
		}
		if err := p.recordStatusChange(txCtx, status, action); err != nil {
			return fmt.Errorf("record operator status change: %w", err)
		}
		updated, err := p.operators.findOperatorByID(txCtx, operator.ID)
		if err != nil {
			return err
		}
		result = updated
		return nil
	})
	return result, err
}

func (p *OperatorProvisioning) recordStatusChange(ctx context.Context, change identityaccess.OperatorStatusChange, action string) error {
	operatorID := change.OperatorID
	return p.audit.RecordOperatorAction(ctx, domain.OperatorAuditEntry{
		OperatorID: change.ActorID, Action: action,
		ResourceType: domain.OperatorAuditResourceOperator, ResourceID: &operatorID, IPAddress: change.IPAddress,
	})
}
