package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	careRecordPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func NewRequestSubmissions(records careRecordPorts.RequestSubmissionRecords, pickup ports.RequestSubmissionPickup, effects ports.RequestSubmissionEffects, today func() calendar.Date) (carerequests.Submissions, error) {
	if records == nil || effects == nil || today == nil {
		return nil, errors.New("care request submissions: records, effects, and clock are required")
	}
	return &application.RequestSubmissions{Records: retainedRequestSubmissionRecordsAdapter{RequestSubmissionRecords: records}, Pickup: pickup, Effects: effects, Today: today}, nil
}
