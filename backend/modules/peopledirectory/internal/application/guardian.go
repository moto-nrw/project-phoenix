package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

// GuardianService serves the row-level guardian reads over the same
// transaction and observation seams as the person service.
type GuardianService struct {
	store   ports.GuardianStore
	tx      ports.Transaction
	observe ports.Observer
}

func NewGuardians(store ports.GuardianStore, tx ports.Transaction, observe ports.Observer) *GuardianService {
	if store == nil || tx == nil || observe == nil {
		panic("people directory application: all guardian dependencies are required")
	}
	return &GuardianService{store: store, tx: tx, observe: observe}
}

func (s *GuardianService) ListLinksByAccount(ctx context.Context, accountID int64) (result []domain.GuardianLink, err error) {
	err = observeRun(ctx, s.observe, "list_guardian_links_by_account", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListLinksByAccount(txCtx, accountID)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *GuardianService) ListByAccounts(ctx context.Context, accountIDs []int64) (result []domain.Guardian, err error) {
	err = observeRun(ctx, s.observe, "list_guardians_by_account", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListByAccounts(txCtx, accountIDs)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *GuardianService) ListByIDs(ctx context.Context, ids []int64) (result []domain.Guardian, err error) {
	err = observeRun(ctx, s.observe, "list_guardians_by_id", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListByIDs(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *GuardianService) CountLinks(ctx context.Context, guardianIDs []int64) (result map[int64]int, err error) {
	err = observeRun(ctx, s.observe, "count_guardian_links", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CountLinks(txCtx, guardianIDs)
		stats.Add(queryStats)
		return err
	})
	return result, err
}
