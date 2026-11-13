package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
)

// Admin MFA mutations and their mandatory audit share the admin unit: the
// operator ledger entry and the school's override row commit with the reset
// or not at all (#6989).
func (f *AccountMFAFlows) disableForAdmin(ctx context.Context, actorType string, actorID, schoolID, accountID int64, overridden domain.AuthEvent) error {
	byOperator := actorType == actorTypeOperator
	resetAt := f.now()
	// Both rows are filed at the school that ordered the reset, which the
	// membership check proved holds the account.
	return f.Disable(f.runtime.WithTenantID(ctx, schoolID), accountID, func(txCtx context.Context) error {
		if byOperator {
			if err := f.recordOperatorMFAChange(txCtx, actorID, accountID, domain.OperatorMFAOverrideEvidence{SchoolID: schoolID, Action: "disable", Scope: "tenant"}); err != nil {
				return err
			}
		} else if err := f.requireSchoolHoldsAccount(txCtx, schoolID, accountID); err != nil {
			return err
		}
		if err := f.appendAuthEvent(txCtx, overridden); err != nil {
			return err
		}
		return f.notifyAdminReset(txCtx, accountID, byOperator, resetAt)
	})
}

// requireSchoolHoldsAccount refuses the school administrator's reset of an
// account another school or a Träger also holds (#6862): the reset holds for
// the whole account. It reads in the reset's administrative transaction,
// under the account lock, so a refusal rolls the reset back and a school
// that joins meanwhile waits for it.
func (f *AccountMFAFlows) requireSchoolHoldsAccount(txCtx context.Context, schoolID, accountID int64) error {
	alone, err := f.ownership.HeldBySchoolAlone(txCtx, accountID, schoolID)
	if err != nil {
		return fmt.Errorf("verify account ownership: %w", err)
	}
	if !alone {
		return identityaccess.ErrAccountSharedAcrossSchools
	}
	return nil
}

// notifyAdminReset queues the holder's notice of an administrator's reset
// (#6862) in the reset's administrative transaction, as a global identity
// mail like the reset itself: without the notice nothing is reset. An account without an address receives none.
func (f *AccountMFAFlows) notifyAdminReset(txCtx context.Context, accountID int64, byOperator bool, resetAt time.Time) error {
	if f.securityNotice == nil {
		return nil
	}
	account, found, err := f.records.FindAccountIdentity(txCtx, accountID)
	if err != nil {
		return fmt.Errorf("load account for mfa reset notice: %w", err)
	}
	if !found || account.Email == "" {
		return nil
	}
	return f.securityNotice.NotifyMFAResetByAdmin(txCtx, identityaccess.MFAResetByAdminMail{
		Recipient: account.Email, ReferenceID: accountID,
		ResetAt: resetAt.Format("02.01.2006 15:04"), ByOperator: byOperator,
	}, fmt.Sprintf("%d:%d", accountID, resetAt.UnixNano()))
}

func (f *AccountMFAFlows) recordOperatorMFAChange(ctx context.Context, operatorID, targetID int64, evidence domain.OperatorMFAOverrideEvidence) error {
	if err := f.audit.RecordOperatorAction(ctx, domain.OperatorAuditEntry{IPAddress: identityaccess.OperatorAuditClientAddress(ctx), OperatorID: operatorID, Action: domain.OperatorAuditActionMFAAdminOverride, ResourceType: domain.OperatorAuditResourceAccount, ResourceID: &targetID, MFA: &domain.OperatorMFAEvidence{Override: &evidence}}); err != nil {
		f.logger.Error("operator audit unavailable", slog.String("action", evidence.Action))
		return errors.Join(identityaccess.ErrOperatorAuditUnavailable, err)
	}
	return nil
}
