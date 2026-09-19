package application

import (
	"context"
	"slices"

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

// StudentsWithPortalGuardian reports which of the children can be reached
// through the parents portal: at least one guardian holds an account AND
// that guardian's link to this child grants portal access. An account alone
// is not enough, because a pickup-only contact may have one for a sibling and
// still sees nothing of this child. Children without such a guardian are
// absent from the result.
//
// Whether the account's school membership is active belongs to Identity &
// Access and is not judged here.
func (s *GuardianService) StudentsWithPortalGuardian(ctx context.Context, studentIDs []int64) (map[int64]bool, error) {
	result := map[int64]bool{}
	err := observeRun(ctx, s.observe, "students_with_portal_guardian", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		links, queryStats, err := s.store.ListAccountLinksByStudents(txCtx, studentIDs)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		for _, link := range links {
			if slices.Contains(link.Permissions, domain.GuardianPermissionPortalAccess) {
				result[link.StudentID] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
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
