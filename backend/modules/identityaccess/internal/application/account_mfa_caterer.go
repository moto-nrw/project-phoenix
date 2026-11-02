package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// operatorResetCatererMFA is the operator's recovery of a caterer account
// (#4497), after OperatorAdminDisable: the e-mail factor, the remembered
// devices and the sessions' proof go, the operator audit with its reason
// and the notice to the caterer commit in the same administrative
// transaction. Without the audit or the notice nothing is reset. A caterer
// supplies several schools, so the reset names none.
func (f *AccountMFAFlows) operatorResetCatererMFA(ctx context.Context, operatorID, accountID int64, recipient, reason string) error {
	if f.resetNotice == nil {
		return identityaccess.ErrMFAStatusUnavailable
	}
	resetAt := f.now()
	return f.Disable(ctx, accountID, func(txCtx context.Context) error {
		if err := f.recordOperatorMFAChange(txCtx, operatorID, accountID, domain.OperatorMFAOverrideEvidence{
			Action: "disable", Scope: domain.MFAChallengeScopeCaterer, Reason: reason,
		}); err != nil {
			return err
		}
		return f.resetNotice.NotifyCatererMFAReset(txCtx, domain.CatererMFAResetMail{
			Recipient: recipient, ReferenceID: accountID, ResetAt: resetAt.Format("02.01.2006 15:04"),
		}, fmt.Sprintf("%d:%d", accountID, resetAt.UnixNano()))
	})
}
