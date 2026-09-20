package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

type GuardianSchools struct {
	service *Service
	store   ports.GuardianSchoolStore
}

func NewGuardianSchools(service *Service, store ports.GuardianSchoolStore) *GuardianSchools {
	return &GuardianSchools{service: service, store: store}
}

func (s *GuardianSchools) ListGuardianSchoolIDs(ctx context.Context, accountID int64) (ids []int64, err error) {
	err = s.service.run(ctx, s.service.tx.RunPlatform, "list_guardian_schools", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		ids, queryStats, queryErr = s.store.ListGuardianSchoolIDs(txCtx, accountID)
		stats.Add(queryStats)
		return queryErr
	})
	return ids, err
}
