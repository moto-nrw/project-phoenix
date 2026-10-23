package application

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Operator MFA mutations and their mandatory audit share the admin unit.
func (f *AccountMFAFlows) disableForAdmin(ctx context.Context, actorType string, actorID, schoolID, accountID int64) error {
	if actorType != actorTypeOperator {
		return f.Disable(ctx, accountID)
	}
	return f.Disable(ctx, accountID, func(txCtx context.Context) error {
		return f.recordOperatorMFAChange(txCtx, actorID, accountID, domain.OperatorMFAOverrideEvidence{SchoolID: schoolID, Action: "disable", Scope: "tenant"})
	})
}

func (f *AccountMFAFlows) recordOperatorMFAChange(ctx context.Context, operatorID, targetID int64, evidence domain.OperatorMFAOverrideEvidence) error {
	if err := f.audit.RecordOperatorAction(ctx, domain.OperatorAuditEntry{IPAddress: identityaccess.OperatorAuditClientAddress(ctx), OperatorID: operatorID, Action: domain.OperatorAuditActionMFAAdminOverride, ResourceType: domain.OperatorAuditResourceAccount, ResourceID: &targetID, MFA: &domain.OperatorMFAEvidence{Override: &evidence}}); err != nil {
		f.logger.Error("operator audit unavailable", slog.String("action", evidence.Action))
		return errors.Join(identityaccess.ErrOperatorAuditUnavailable, err)
	}
	return nil
}
