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

// OperatorResetCatererMFA resets the second factor of every account of the
// caterer (#4497): the recovery for a caterer that lost access to it. Each
// account's reset, its operator audit with the reason and its notice
// commit together. School admins have no such reset: a caterer supplies
// several schools.
func (c *CatererAccess) OperatorResetCatererMFA(ctx context.Context, operatorID, catererID int64, reason string) error {
	return catererMFAError(c.operatorResetMFA(c.tenantless(ctx), operatorID, catererID, reason))
}

func (c *CatererAccess) operatorResetMFA(ctx context.Context, operatorID, catererID int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) < minCatererResetReasonLength {
		return identityaccess.ErrCatererMFAResetReasonRequired
	}
	if c.mfa == nil {
		return errCatererMFAUnavailable
	}
	rows, err := c.listCatererAccountSecurity(ctx)
	if err != nil {
		return failed("find caterer accounts", err)
	}
	reset := 0
	for _, row := range rows {
		if row.CatererID != catererID {
			continue
		}
		if err := c.mfa.operatorResetCatererMFA(ctx, operatorID, row.AccountID, row.Email, reason); err != nil {
			return failed("reset caterer mfa", err)
		}
		reset++
		c.logger.Info("caterer_mfa_reset",
			slog.Int64("caterer_id", catererID),
			slog.Int64("account_id", row.AccountID),
			slog.Int64("operator_id", operatorID))
	}
	if reset == 0 {
		return identityaccess.ErrCatererNotFound
	}
	return nil
}
