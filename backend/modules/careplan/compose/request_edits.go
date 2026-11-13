package compose

import (
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/ports"
	careRecordPorts "github.com/moto-ogs/project-phoenix/modules/careplan/ports"
	calendar "github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

func NewRequestEdits(records careRecordPorts.RequestEditRecords, pickup ports.RequestSubmissionPickup, events ports.RequestEditEvents, baseline ports.RequestWeeklyBaseline, today func() calendar.Date) (carerequests.Edits, error) {
	if records == nil || events == nil || baseline == nil || today == nil {
		return nil, errors.New("care request edits: records, events, weekly baseline, and clock are required")
	}
	return &application.RequestEdits{Records: application.RequestEditRecordsAdapter{RequestEditRecords: records}, Pickup: pickup, Events: events, Baseline: baseline, Today: today}, nil
}
