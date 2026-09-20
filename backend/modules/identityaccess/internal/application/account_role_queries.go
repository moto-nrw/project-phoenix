package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

type AccountRoleQueries struct {
	*RoleCatalog
	service *Service
	store   ports.AccountRoleQueryStore
}

func NewAccountRoleQueries(service *Service, store ports.AccountRoleQueryStore, catalog ports.RoleCatalogStore) *AccountRoleQueries {
	return &AccountRoleQueries{RoleCatalog: NewRoleCatalog(catalog), service: service, store: store}
}

func (q *AccountRoleQueries) ListActiveAccountSchoolIDs(ctx context.Context, accountID int64) (ids []int64, err error) {
	err = q.service.run(ctx, q.service.tx.RunPlatform, "list_active_account_school_ids", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		ids, queryStats, queryErr = q.store.ListActiveTenantIDs(txCtx, accountID)
		stats.Add(queryStats)
		return queryErr
	})
	return ids, err
}

func (q *AccountRoleQueries) ListSchoolAccountRoleNames(ctx context.Context, accountID int64) (names []string, err error) {
	err = q.service.run(ctx, q.service.withinSchoolRead, "list_school_account_role_names", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		names, queryStats, queryErr = q.store.ListSchoolAccountRoleNames(txCtx, accountID, q.service.tenantOf(txCtx))
		stats.Add(queryStats)
		return queryErr
	})
	return names, err
}

func (q *AccountRoleQueries) FindSystemRoleID(ctx context.Context, name string) (id int64, found bool, err error) {
	err = q.service.run(ctx, q.service.tx.RunPlatform, "find_system_role_id", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		id, found, queryStats, queryErr = q.store.FindSystemRoleID(txCtx, name)
		stats.Add(queryStats)
		return queryErr
	})
	return id, found, err
}
