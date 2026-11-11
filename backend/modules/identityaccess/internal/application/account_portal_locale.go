package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// These commands participate in the portal workflow's administrative unit.
// The store rejects calls without an ambient transaction.
func (p *AccountProfiles) FindAccountPortalLocale(ctx context.Context, accountID int64) (locale *string, err error) {
	err = p.service.run(ctx, accountProfileUnit, "find_account_portal_locale", func(txCtx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, queryErr := p.store.FindAccountPortalLocale(txCtx, accountID)
		stats.Add(queryStats)
		locale = value
		return stateChange(found, queryErr, domain.ErrAccountNotFound)
	})
	return locale, MapError(err)
}

func (p *AccountProfiles) SetAccountPortalLocale(ctx context.Context, accountID int64, locale string) error {
	if accountID <= 0 || locale == "" {
		return errors.New("identity access: account ID and portal locale are required")
	}
	err := p.service.run(ctx, accountProfileUnit, "set_account_portal_locale", func(txCtx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := p.store.SetAccountPortalLocale(txCtx, accountID, locale)
		stats.Add(queryStats)
		return stateChange(found, err, domain.ErrAccountNotFound)
	})
	return MapError(err)
}

// accountProfileUnit retains the workflow's unit while the owner records its
// operation metrics. It does not acquire a school scope or open a transaction.
func accountProfileUnit(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
