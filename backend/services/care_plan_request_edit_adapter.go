package services

import (
	"context"
	"errors"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

type requestEditAdapter struct{ requestSubmissionAdapter }

func (a requestEditAdapter) RecordGuardianEdit(ctx context.Context, request *carerequests.Request, actorID int64) error {
	row := request
	return a.s.recordCareRequestEvent(ctx, row, usersModels.ParentRequestEventGuardianEdit, actorID, nil)
}

func legacyEditError(err error) error {
	switch {
	case errors.Is(err, careplan.ErrParentRequestStale):
		return usersService.ErrParentRequestStale
	case errors.Is(err, careplan.ErrParentRequestReasonRequired):
		return usersService.ErrParentRequestReasonRequired
	case errors.Is(err, careplan.ErrParentRequestNotPast):
		return usersService.ErrParentRequestNotPast
	default:
		return err
	}
}
