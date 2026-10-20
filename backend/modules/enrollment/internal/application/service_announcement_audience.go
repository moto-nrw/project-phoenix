package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// PendingAnnouncementApplicantsForSchools serves a parent feed's explicit school
// scope. Cross-school callers supply their authorized admin transaction.
func (m *Service) PendingAnnouncementApplicantsForSchools(ctx context.Context, schoolIDs []int64) ([]enrollment.PendingAnnouncementApplicant, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.PendingAnnouncementApplicantsForSchools(ctx, schoolIDs)
}

func (m *Service) PendingAnnouncementApplicants(ctx context.Context) ([]enrollment.PendingAnnouncementApplicant, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.PendingAnnouncementApplicants(ctx)
}
