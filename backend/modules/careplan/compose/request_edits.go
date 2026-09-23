package compose

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type RequestEditRecords interface {
	FindCareScheduleRequest(context.Context, int64, bool) (carerequests.Request, error)
	UpdatePendingCareScheduleRequest(context.Context, int64, json.RawMessage) error
}

type RequestEditEvents = ports.RequestEditEvents

func NewRequestEdits(records RequestEditRecords, pickup RequestSubmissionPickup, events RequestEditEvents, today func() calendar.Date) (carerequests.Edits, error) {
	if records == nil || events == nil || today == nil {
		return nil, errors.New("care request edits: records, events, and clock are required")
	}
	return &application.RequestEdits{Records: requestEditRecords{records}, Pickup: pickup, Events: events, Today: today}, nil
}

type requestEditRecords struct{ RequestEditRecords }

func (r requestEditRecords) Find(ctx context.Context, id int64, lock bool) (carerequests.Request, error) {
	return r.FindCareScheduleRequest(ctx, id, lock)
}
func (r requestEditRecords) UpdatePending(ctx context.Context, id int64, payload json.RawMessage) error {
	return r.UpdatePendingCareScheduleRequest(ctx, id, payload)
}
