package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// CareExitApplicationLinks returns links for the selected students. An empty
// selection supplies the school's links for the scheduled expiry evaluation.
func (m *Service) CareExitApplicationLinks(ctx context.Context, studentIDs []int64) ([]enrollment.CareExitApplicationLink, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.CareExitApplicationLinks(ctx, studentIDs)
}
