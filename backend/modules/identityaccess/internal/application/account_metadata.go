package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (p *AccountProfiles) FindAccountMetadata(ctx context.Context, accountID int64) (identityaccess.AccountMetadata, error) {
	var result domain.AccountMetadata
	err := p.service.run(ctx, p.service.tx.RunPlatform, "find_account_metadata", func(txCtx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, queryErr := p.store.FindAccountMetadata(txCtx, accountID)
		stats.Add(queryStats)
		result = value
		return stateChange(found, queryErr, domain.ErrAccountNotFound)
	})
	if err != nil {
		return identityaccess.AccountMetadata{}, MapError(err)
	}
	return identityaccess.AccountMetadata(result), nil
}

func (p *AccountProfiles) SetAccountUsername(ctx context.Context, accountID int64, username string) (err error) {
	defer func() { err = MapError(err) }()
	return p.service.run(ctx, p.service.tx.RunPlatform, "set_account_username", func(txCtx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := p.store.SetAccountUsername(txCtx, accountID, username)
		stats.Add(queryStats)
		return stateChange(found, err, domain.ErrAccountNotFound)
	})
}

func (p *AccountProfiles) SetAccountAvatar(ctx context.Context, accountID int64, avatar string) (err error) {
	defer func() { err = MapError(err) }()
	return p.service.run(ctx, p.service.tx.RunPlatform, "set_account_avatar", func(txCtx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := p.store.SetAccountAvatar(txCtx, accountID, avatar)
		stats.Add(queryStats)
		return stateChange(found, err, domain.ErrAccountNotFound)
	})
}
