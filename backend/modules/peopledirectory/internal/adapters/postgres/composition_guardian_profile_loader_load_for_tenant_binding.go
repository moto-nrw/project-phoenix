package postgres

import (
	"context"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/uptrace/bun"
)

func GuardianProfileLoaderLoadForTenantBinding(ctx context.Context, accountID int64, tenantID int64, execute0 func(ctx context.Context, _ *bun.DB, rawID int64, fn func(context.Context, bun.Tx) error) error, execute1 func() *bun.DB, execute2 func() peoplePorts.GuardianProfileRepository) (*peopleValues.GuardianProfileWithChildren, error) {
	if accountID <= 0 || tenantID <= 0 {
		return nil, nil
	}

	var result *peopleValues.GuardianProfileWithChildren
	err := execute0(ctx, execute1(), tenantID, func(txCtx context.Context, _ bun.Tx) error {
		loaded, loadErr := execute2().LoadProfileWithChildren(txCtx, accountID)
		if loadErr != nil {
			return loadErr
		}
		result = loaded
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
