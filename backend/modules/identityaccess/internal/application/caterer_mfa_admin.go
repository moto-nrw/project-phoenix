package application

import (
	"context"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// minCatererResetReasonLength mirrors the operator's reset of a Träger-Büro
// member: the audit needs a reason someone can read.
const minCatererResetReasonLength = 3

// ListCaterersForOperator lists every caterer account and whether its
// second factor is on, for the operator's reset (#4497). The operator route
// carries the platform gate; the read runs administratively, as the
// caterers belong to no school.
func (c *CatererAccess) ListCaterersForOperator(ctx context.Context) ([]identityaccess.CatererSecuritySummary, error) {
	rows, err := c.listCatererAccountSecurity(c.tenantless(ctx))
	if err != nil {
		return nil, catererMFAError(failed("list caterers for operator", err))
	}
	result := make([]identityaccess.CatererSecuritySummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, identityaccess.CatererSecuritySummary{
			ID: row.CatererID, AccountID: row.AccountID, Name: row.Name, Email: row.Email, MFAEnabled: row.MFAEnabled,
		})
	}
	return result, nil
}

func (c *CatererAccess) listCatererAccountSecurity(ctx context.Context) ([]domain.CatererAccountSecurity, error) {
	var rows []domain.CatererAccountSecurity
	err := globalIdentityRead(ctx, c.runtime, func(txCtx context.Context) error {
		var err error
		rows, _, err = c.store.ListCatererAccountSecurity(txCtx)
		return err
	})
	return rows, err
}

// OperatorResetCatererMFA resets the second factor of one account of the
// caterer (#4497): the recovery for a caterer that lost access to it. The
// reset, its operator audit with the reason and its notice commit together.
// An account without the factor is refused: there is nothing to reset and
// nothing to tell. School admins have no such reset: a caterer supplies
// several schools.
func (c *CatererAccess) OperatorResetCatererMFA(ctx context.Context, operatorID, catererID, accountID int64, reason string) error {
	return catererMFAError(c.operatorResetMFA(c.tenantless(ctx), operatorID, catererID, accountID, reason))
}

func (c *CatererAccess) operatorResetMFA(ctx context.Context, operatorID, catererID, accountID int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) < minCatererResetReasonLength {
		return identityaccess.ErrCatererMFAResetReasonRequired
	}
	if c.mfa == nil {
		return errCatererMFAUnavailable
	}
	recipient, err := c.catererAccountAddress(ctx, catererID, accountID)
	if err != nil {
		return err
	}
	enrolled, err := c.mfa.HasEnrollment(ctx, accountID)
	if err != nil {
		return err
	}
	if !enrolled {
		return identityaccess.ErrCatererMFANotEnabled
	}
	if err := c.mfa.operatorResetCatererMFA(ctx, operatorID, accountID, recipient, reason); err != nil {
		return failed("reset caterer mfa", err)
	}
	c.logger.Info("caterer_mfa_reset",
		slog.Int64("caterer_id", catererID),
		slog.Int64("account_id", accountID),
		slog.Int64("operator_id", operatorID))
	return nil
}

// catererAccountAddress proves the account belongs to the caterer and names
// the address its notice goes to.
func (c *CatererAccess) catererAccountAddress(ctx context.Context, catererID, accountID int64) (string, error) {
	var recipient string
	err := globalIdentityRead(ctx, c.runtime, func(readCtx context.Context) error {
		caterer, found, _, err := c.store.FindCatererOfAccount(readCtx, accountID)
		if err != nil {
			return err
		}
		if !found || caterer.ID != catererID {
			return identityaccess.ErrCatererNotFound
		}
		account, found, _, err := c.auth.store.FindLoginAccount(readCtx, accountID, false)
		if err != nil {
			return err
		}
		if !found {
			return identityaccess.ErrCatererNotFound
		}
		recipient = account.Email
		return nil
	})
	return recipient, err
}
