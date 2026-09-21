package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type RequestSubmissionRecords interface {
	CreateCareScheduleRequest(context.Context, carerequests.Request) (carerequests.Request, error)
}

type RequestSubmissionPickup = ports.RequestSubmissionPickup
type RequestSubmissionEffects = ports.RequestSubmissionEffects

func NewRequestSubmissions(records RequestSubmissionRecords, pickup RequestSubmissionPickup, effects RequestSubmissionEffects, today func() calendar.Date) (carerequests.Submissions, error) {
	if records == nil || effects == nil || today == nil {
		return nil, errors.New("care request submissions: records, effects, and clock are required")
	}
	return &application.RequestSubmissions{Records: requestSubmissionRecords{records}, Pickup: pickup, Effects: effects, Today: today}, nil
}

type requestSubmissionRecords struct{ RequestSubmissionRecords }

func (r requestSubmissionRecords) Create(ctx context.Context, request carerequests.Request) (carerequests.Request, error) {
	return r.CreateCareScheduleRequest(ctx, request)
}

func (requestSubmissionRecords) IsPendingConflict(err error) bool {
	return postgres.IsPendingCareScheduleConflict(err)
}
