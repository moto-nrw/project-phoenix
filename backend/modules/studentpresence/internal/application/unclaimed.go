package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (s *Service) UnclaimedGroups(ctx context.Context, value string) (rows []studentpresence.UnclaimedGroup, err error) {
	date, err := calendar.ParseDate(value)
	if err != nil {
		return nil, err
	}
	err = s.runWrite(ctx, "unclaimed_groups", func(ctx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		rows, stats, err = s.store.UnclaimedGroups(ctx, date)
		return stats, err
	})
	return listResult(rows, err)
}

// ClaimGroup joins the caller's transaction and holds the group lifecycle lock
// through duplicate detection and insertion. Staff authorization is a caller concern.
func (s *Service) ClaimGroup(ctx context.Context, value studentpresence.GroupClaim) (studentpresence.ClaimedSupervision, error) {
	date, err := calendar.ParseDate(value.Date)
	if err != nil {
		return studentpresence.ClaimedSupervision{}, err
	}
	claim := ports.GroupClaim{GroupID: value.GroupID, StaffID: value.StaffID, Role: value.Role, Date: date}
	var row ports.ClaimedSupervision
	err = s.run("claim_group", func() (ports.Stats, error) {
		if claim.GroupID <= 0 || claim.StaffID <= 0 || claim.Date.IsZero() || claim.Role == "" {
			return ports.Stats{}, errors.New("invalid group claim")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		var stats ports.Stats
		var err error
		row, stats, err = s.store.ClaimGroup(ctx, claim)
		return stats, err
	})
	return studentpresence.ClaimedSupervision{ID: row.ID, TenantID: row.TenantID, GroupID: row.GroupID, StaffID: row.StaffID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Role: row.Role, StartDate: row.StartDate.String()}, err
}
