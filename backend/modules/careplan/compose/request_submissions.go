package compose

import (
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/ports"
	careRecordPorts "github.com/moto-ogs/project-phoenix/modules/careplan/ports"
	calendar "github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

func NewRequestSubmissions(records careRecordPorts.RequestSubmissionRecords, pickup ports.RequestSubmissionPickup, effects ports.RequestSubmissionEffects, baseline ports.RequestWeeklyBaseline, today func() calendar.Date) (carerequests.Submissions, error) {
	if records == nil || effects == nil || baseline == nil || today == nil {
		return nil, errors.New("care request submissions: records, effects, weekly baseline, and clock are required")
	}
	return &application.RequestSubmissions{Records: retainedRequestSubmissionRecordsAdapter{RequestSubmissionRecords: records}, Pickup: pickup, Effects: effects, Baseline: baseline, Today: today}, nil
}
