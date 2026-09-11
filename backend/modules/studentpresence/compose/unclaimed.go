package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (e engine) UnclaimedGroups(ctx context.Context, value string) ([]studentpresence.UnclaimedGroup, error) {
	date, err := timezone.ParseDate(value)
	if err != nil {
		return nil, err
	}
	rows, err := e.Service.UnclaimedGroups(ctx, date)
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.UnclaimedGroup, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentpresence.UnclaimedGroup(row))
	}
	return result, nil
}

func (e engine) ClaimGroup(ctx context.Context, claim studentpresence.GroupClaim) (studentpresence.ClaimedSupervision, error) {
	date, err := timezone.ParseDate(claim.Date)
	if err != nil {
		return studentpresence.ClaimedSupervision{}, err
	}
	row, err := e.Service.ClaimGroup(ctx, ports.GroupClaim{GroupID: claim.GroupID, StaffID: claim.StaffID, Role: claim.Role, Date: date})
	switch {
	case errors.Is(err, ports.ErrGroupNotFound):
		err = studentpresence.ErrGroupNotFound
	case errors.Is(err, ports.ErrGroupEnded):
		err = studentpresence.ErrGroupEnded
	case errors.Is(err, ports.ErrAlreadySupervising):
		err = studentpresence.ErrAlreadySupervising
	}
	return studentpresence.ClaimedSupervision{ID: row.ID, TenantID: row.TenantID, GroupID: row.GroupID, StaffID: row.StaffID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Role: row.Role, StartDate: row.StartDate.String()}, err
}
