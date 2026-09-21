package application

import (
	"context"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (s *GuardianService) ListPortalContacts(ctx context.Context, guardianIDs, studentIDs []int64) (result []domain.GuardianPortalContact, err error) {
	err = observeRun(ctx, s.observe, "list_guardian_portal_contacts", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		candidates, queryStats, queryErr := s.store.ListPortalContacts(txCtx, guardianIDs, studentIDs)
		stats.Add(queryStats)
		if queryErr != nil {
			return queryErr
		}
		result = []domain.GuardianPortalContact{}
		if len(candidates) == 0 {
			return nil
		}
		accountIDs := portalContactAccountIDs(candidates)
		memberships, queryErr := s.store.FindPortalMemberships(txCtx, accountIDs)
		if queryErr != nil {
			return queryErr
		}
		for _, candidate := range candidates {
			if slices.Contains(memberships[candidate.AccountID], candidate.TenantID) {
				result = append(result, candidate)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func portalContactAccountIDs(contacts []domain.GuardianPortalContact) []int64 {
	ids := make([]int64, 0, len(contacts))
	seen := make(map[int64]bool)
	for _, contact := range contacts {
		if !seen[contact.AccountID] {
			ids = append(ids, contact.AccountID)
			seen[contact.AccountID] = true
		}
	}
	return ids
}
