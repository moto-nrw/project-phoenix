package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

type AccountProfiles struct {
	service *Service
	store   ports.AccountProfileStore
}

func NewAccountProfiles(service *Service, store ports.AccountProfileStore) *AccountProfiles {
	return &AccountProfiles{service: service, store: store}
}

func (p *AccountProfiles) FindAccountProfile(ctx context.Context, accountID int64) (result domain.AccountProfile, found bool, err error) {
	err = p.service.run(ctx, p.service.withinSchoolRead, "find_account_profile", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		result, found, queryStats, queryErr = p.store.FindAccountProfile(txCtx, accountID, p.service.tenantOf(txCtx))
		stats.Add(queryStats)
		return queryErr
	})
	return result, found, err
}

func (p *AccountProfiles) SetAccountBio(ctx context.Context, accountID int64, bio string) error {
	return p.service.run(ctx, p.withinSchoolWrite, "set_account_bio", func(txCtx context.Context, stats *domain.OperationStats) error {
		if accountID <= 0 {
			return errors.New("identity access: account ID must be positive")
		}
		queryStats, err := p.store.SetAccountBio(txCtx, accountID, p.service.tenantOf(txCtx), bio)
		stats.Add(queryStats)
		return err
	})
}

func (p *AccountProfiles) withinSchoolWrite(ctx context.Context, fn func(context.Context) error) error {
	if p.service.tenantOf(ctx) <= 0 {
		return domain.ErrTenantRequired
	}
	return p.service.tx.RunWrite(ctx, fn)
}
