package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	careRecordPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func NewRequestEdits(records careRecordPorts.RequestEditRecords, pickup ports.RequestSubmissionPickup, events ports.RequestEditEvents, today func() calendar.Date) (carerequests.Edits, error) {
	if records == nil || events == nil || today == nil {
		return nil, errors.New("care request edits: records, events, and clock are required")
	}
	return &application.RequestEdits{Records: application.RequestEditRecordsAdapter{RequestEditRecords: records}, Pickup: pickup, Events: events, Today: today}, nil
}
