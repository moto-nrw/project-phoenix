package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (s *Store) FindRFIDCard(ctx context.Context, tag string, tenantID int64) (string, bool, domain.OperationStats, error) {
	card, found, stats, err := s.LookupRFIDCard(ctx, tag, tenantID)
	return card.ID, found, stats, err
}
