package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
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

// canonicalTagStore hands the lifecycle flows the same canonical tag lookup:
// a transponder submitted as "04:a1:b2:c3" at registration finds the card
// stored as "04A1B2C3", as it did through the retained card repository.
type CanonicalTagStore struct{ ports.Store }

func (s CanonicalTagStore) FindRFIDCard(ctx context.Context, tag string, tenantID int64) (string, bool, domain.OperationStats, error) {
	return s.Store.FindRFIDCard(ctx, domain.NormalizeTagID(tag), tenantID)
}
