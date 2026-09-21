package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type RequestConflictRecords interface {
	FindCareScheduleRequest(context.Context, int64, bool) (carerequests.Request, error)
}
type RequestConflictPlans = ports.RequestConflictPlans

func NewRequestConflicts(records RequestConflictRecords, plans RequestConflictPlans, decisions carerequests.Decisions, today func() calendar.Date) (carerequests.Conflicts, error) {
	if records == nil || plans == nil || decisions == nil || today == nil {
		return nil, errors.New("care request conflicts: records, plans, decisions, and clock are required")
	}
	return &application.RequestConflicts{Records: requestConflictRecords{records}, Plans: plans, Decisions: decisions, Today: today}, nil
}

type requestConflictRecords struct{ RequestConflictRecords }

func (r requestConflictRecords) Find(ctx context.Context, id int64, lock bool) (carerequests.Request, error) {
	return r.FindCareScheduleRequest(ctx, id, lock)
}
