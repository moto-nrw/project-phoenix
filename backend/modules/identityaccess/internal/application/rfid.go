package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// FindRFIDCard receives the canonical tag from composition. It never performs
// a tenantless lookup, including when the caller has an admin connection.
func (s *Service) FindRFIDCard(ctx context.Context, tag string) (id string, found bool, err error) {
	err = s.run(ctx, s.withinSchoolRead, "find_rfid_card", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		id, found, queryStats, queryErr = s.store.FindRFIDCard(txCtx, tag, s.tenantOf(txCtx))
		stats.Add(queryStats)
		return queryErr
	})
	return id, found, err
}
