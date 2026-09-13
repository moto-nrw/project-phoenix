package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (s *Service) FindInvitedPersonIDs(ctx context.Context, email string) (ids []int64, err error) {
	err = s.run(ctx, s.withinSchoolRead, "find_invited_person_ids", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		ids, queryStats, queryErr = s.store.FindInvitedPersonIDs(txCtx, strings.ToLower(strings.TrimSpace(email)), s.tenantOf(txCtx))
		stats.Add(queryStats)
		return queryErr
	})
	return ids, err
}
