package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) UnclaimedGroups(ctx context.Context, date ports.Date) (rows []ports.UnclaimedGroup, err error) {
	err = s.run("unclaimed_groups", func() (ports.Stats, error) {
		var stats ports.Stats
		rows, stats, err = s.store.UnclaimedGroups(ctx, date)
		return stats, err
	})
	return rows, err
}

func (s *Service) ClaimGroup(ctx context.Context, claim ports.GroupClaim) (row ports.ClaimedSupervision, err error) {
	err = s.run("claim_group", func() (ports.Stats, error) {
		if claim.GroupID <= 0 || claim.StaffID <= 0 || claim.Date.IsZero() || claim.Role == "" {
			return ports.Stats{}, errors.New("invalid group claim")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		var stats ports.Stats
		row, stats, err = s.store.ClaimGroup(ctx, claim)
		return stats, err
	})
	return row, err
}
