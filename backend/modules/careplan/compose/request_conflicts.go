package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	careRecordPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func NewRequestConflicts(records careRecordPorts.RequestConflictRecords, people ports.RequestDecisionPeople, plans ports.RequestConflictPlans, decisions carerequests.Decisions, today func() calendar.Date) (carerequests.Conflicts, error) {
	if records == nil || people == nil || plans == nil || decisions == nil || today == nil {
		return nil, errors.New("care request conflicts: records, people, plans, decisions, and clock are required")
	}
	return &application.RequestConflicts{Records: application.RequestConflictRecordsAdapter{RequestConflictRecords: records}, People: people, Plans: plans, Decisions: decisions, Today: today}, nil
}
