package active

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

type PresenceQueries interface {
	FindVisit(context.Context, int64) (*studentpresence.Visit, error)
	ListVisits(context.Context, studentpresence.VisitFilter) ([]studentpresence.Visit, error)
}

func (rs *Resource) findPresenceVisit(ctx context.Context, id int64) (*studentpresence.Visit, error) {
	visit, err := rs.Presence.FindVisit(ctx, id)
	if err == nil && visit == nil {
		err = studentpresence.ErrVisitNotFound
	}
	return visit, presenceQueryError("GetVisit", err)
}

func (rs *Resource) currentPresenceVisit(ctx context.Context, studentID int64) (*studentpresence.Visit, error) {
	visits, err := rs.Presence.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{studentID}, OpenOnly: true, NewestFirst: true, Limit: 1})
	if err != nil {
		return nil, presenceQueryError("GetStudentCurrentVisit", err)
	}
	if len(visits) == 0 {
		return nil, presenceQueryError("GetStudentCurrentVisit", studentpresence.ErrVisitNotFound)
	}
	return &visits[0], nil
}

func presenceQueryError(operation string, err error) error {
	if err == nil {
		return nil
	}
	cause := activeService.ErrDatabaseOperation
	if errors.Is(err, studentpresence.ErrVisitNotFound) {
		cause = activeService.ErrVisitNotFound
	}
	return &activeService.ActiveError{Op: operation, Err: cause}
}

func newPresenceVisitResponse(visit studentpresence.Visit) VisitResponse {
	return VisitResponse{
		ID: visit.ID, StudentID: visit.StudentID, ActiveGroupID: visit.ActiveGroupID,
		CheckInTime: visit.EntryTime, CheckOutTime: visit.ExitTime, IsActive: visit.ExitTime == nil,
		CreatedAt: visit.CreatedAt, UpdatedAt: visit.UpdatedAt,
	}
}
