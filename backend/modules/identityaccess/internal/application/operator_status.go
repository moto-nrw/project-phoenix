package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The accounts of team members who left (#4047): another operator
// deactivates the account in the dashboard, which ends its sessions at once,
// and the hourly cleanup deletes it once the operator ledger has lost the
// entries written before the deactivation. A reactivation clears the
// deactivation, so the deletion window starts over.

// operatorDeletionMargin reads the year an hour early: right after New Year
// the account cutoff stays a year older than the ledger cutoff, so the
// ledger has already lost the account's entries when the account goes.
const operatorDeletionMargin = time.Hour

// operatorDeletionCutoff is 1 January of last year (Berlin), the cutoff of
// the operator ledger: an account deactivated before it has no ledger entry
// left.
func operatorDeletionCutoff(now time.Time) time.Time {
	year := now.In(calendar.Berlin).Add(-operatorDeletionMargin).Year()
	return time.Date(year-1, time.January, 1, 0, 0, 0, 0, calendar.Berlin)
}

// ChangeOperatorStatus deactivates or reactivates another operator's account
// and records the change in the operator ledger.
func (p *OperatorProvisioning) ChangeOperatorStatus(ctx context.Context, change domain.OperatorStatusChange) (domain.Operator, error) {
	if change.Active {
		return p.reactivateOperator(ctx, change)
	}
	return p.deactivateOperator(ctx, change)
}

// deactivateOperator locks another operator's account and ends its sessions.
// Deactivating an inactive account keeps its original deactivation time.
func (p *OperatorProvisioning) deactivateOperator(ctx context.Context, change domain.OperatorStatusChange) (domain.Operator, error) {
	if change.OperatorID == change.ActorID {
		return domain.Operator{}, domain.ErrOperatorSelfDeactivation
	}
	operator, err := p.changeOperatorStatus(ctx, change.OperatorID, func(txCtx context.Context, operator *domain.Operator) error {
		if operator.Active {
			deactivatedAt := p.now()
			operator.Active = false
			operator.DeactivatedAt = &deactivatedAt
		}
		_, revokeErr := p.operators.RevokeOperatorSessions(txCtx, operator.ID)
		return revokeErr
	})
	if err != nil {
		return domain.Operator{}, err
	}
	p.recordStatusChange(ctx, change, domain.OperatorAuditActionDeactivated)
	return operator, nil
}

// reactivateOperator lets a deactivated operator sign in again.
func (p *OperatorProvisioning) reactivateOperator(ctx context.Context, change domain.OperatorStatusChange) (domain.Operator, error) {
	operator, err := p.changeOperatorStatus(ctx, change.OperatorID, func(_ context.Context, operator *domain.Operator) error {
		operator.Active = true
		operator.DeactivatedAt = nil
		return nil
	})
	if err != nil {
		return domain.Operator{}, err
	}
	p.recordStatusChange(ctx, change, domain.OperatorAuditActionReactivated)
	return operator, nil
}

// DeleteDeactivatedOperators removes the accounts deactivated before the
// operator ledger cutoff and reports how many went. Platform rows go only in
// an administrative transaction; the hourly cleanup hands its own in.
func (p *OperatorProvisioning) DeleteDeactivatedOperators(ctx context.Context) (deleted int, err error) {
	err = p.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		deleted, err = p.operators.deleteDeactivatedOperators(txCtx, operatorDeletionCutoff(p.now()))
		return err
	})
	return deleted, err
}

// changeOperatorStatus locks the operator's row, applies change and stores
// the result in one administrative transaction.
func (p *OperatorProvisioning) changeOperatorStatus(ctx context.Context, operatorID int64, change func(context.Context, *domain.Operator) error) (domain.Operator, error) {
	var result domain.Operator
	err := p.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		operator, err := p.operators.FindOperatorForUpdate(txCtx, operatorID)
		if err != nil {
			return err
		}
		if err := change(txCtx, &operator); err != nil {
			return err
		}
		updated, err := p.operators.UpdateOperator(txCtx, operator)
		if err != nil {
			return err
		}
		result = updated
		return nil
	})
	return result, err
}

func (p *OperatorProvisioning) recordStatusChange(ctx context.Context, change domain.OperatorStatusChange, action string) {
	operatorID := change.OperatorID
	p.record(ctx, domain.OperatorAuditEntry{
		OperatorID: change.ActorID, Action: action,
		ResourceType: domain.OperatorAuditResourceOperator, ResourceID: &operatorID, IPAddress: change.IPAddress,
	}, "operator status change")
}
