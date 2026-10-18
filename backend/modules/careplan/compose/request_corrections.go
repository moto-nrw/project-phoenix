package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

type RequestCorrectionRecords interface {
	FindCareScheduleRequest(context.Context, int64, bool) (carerequests.Request, error)
	RedecideCareScheduleRequest(context.Context, careplan.CareScheduleRequestDecision) error
	FindPickupException(context.Context, int64, bool) (careplan.PickupException, error)
	DeletePickupException(context.Context, int64) error
	ListDepartureExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.DepartureException, error)
	DeleteDepartureException(context.Context, int64) error
}

type RequestCorrectionEvent = ports.RequestCorrectionEvent

type RequestCorrectionDependencies struct {
	Records RequestCorrectionRecords
	People  ports.RequestCorrectionPeople
	Plans   ports.RequestCorrectionPlans
	Effects ports.RequestCorrectionEffects
}

func NewRequestCorrections(deps RequestCorrectionDependencies) (carerequests.Corrections, error) {
	if deps.Records == nil || deps.People == nil || deps.Plans == nil || deps.Effects == nil {
		return nil, errors.New("care request corrections: records, people, plans, and effects are required")
	}
	return &application.RequestCorrections{Records: requestCorrectionRecords{deps.Records}, People: deps.People, Plans: deps.Plans, Effects: deps.Effects}, nil
}

type requestCorrectionRecords struct{ RequestCorrectionRecords }

func (r requestCorrectionRecords) Find(ctx context.Context, id int64, lock bool) (carerequests.Request, error) {
	return r.FindCareScheduleRequest(ctx, id, lock)
}
func (r requestCorrectionRecords) Redecide(ctx context.Context, id int64, status string, reason *string, actorID int64, applied bool) error {
	return r.RedecideCareScheduleRequest(ctx, careplan.CareScheduleRequestDecision{ID: id, Status: status, Reason: reason, ReviewedBy: &actorID, Applied: applied})
}
func (r requestCorrectionRecords) FindException(ctx context.Context, id int64) (*careplan.PickupException, error) {
	row, err := r.FindPickupException(ctx, id, true)
	if err != nil {
		return nil, err
	}
	return &row, nil
}
func (r requestCorrectionRecords) DeleteException(ctx context.Context, id int64) error {
	return r.DeletePickupException(ctx, id)
}
func (r requestCorrectionRecords) FindDepartureException(ctx context.Context, id int64) (*careplan.DepartureException, error) {
	rows, err := r.ListDepartureExceptions(ctx, careplan.StudentScheduleFilter{IDs: []int64{id}, LockForUpdate: true})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}
