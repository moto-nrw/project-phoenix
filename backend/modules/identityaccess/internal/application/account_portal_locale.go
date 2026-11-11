package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// These commands participate in the portal workflow's administrative unit.
// The store rejects calls without an ambient transaction.
func (p *AccountProfiles) FindAccountPortalLocale(ctx context.Context, accountID int64) (*string, error) {
	locale, found, _, err := p.store.FindAccountPortalLocale(ctx, accountID)
	return locale, MapError(stateChange(found, err, domain.ErrAccountNotFound))
}

func (p *AccountProfiles) SetAccountPortalLocale(ctx context.Context, accountID int64, locale string) error {
	if accountID <= 0 || locale == "" {
		return errors.New("identity access: account ID and portal locale are required")
	}
	found, _, err := p.store.SetAccountPortalLocale(ctx, accountID, locale)
	return MapError(stateChange(found, err, domain.ErrAccountNotFound))
}
