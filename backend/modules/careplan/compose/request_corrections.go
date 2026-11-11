package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	careRecordPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
)

type RequestCorrectionEvent = ports.RequestCorrectionEvent
type RequestCorrectionDependencies struct {
	Records careRecordPorts.RequestCorrectionRecords
	People  ports.RequestCorrectionPeople
	Plans   ports.RequestCorrectionPlans
	Effects ports.RequestCorrectionEffects
	Excusal ports.RequestCorrectionExcusal
}

func NewRequestCorrections(deps RequestCorrectionDependencies) (carerequests.Corrections, error) {
	if deps.Records == nil || deps.People == nil || deps.Plans == nil || deps.Effects == nil || deps.Excusal == nil {
		return nil, errors.New("care request corrections: records, people, plans, effects, and excusal are required")
	}
	return &application.RequestCorrections{Records: application.RequestCorrectionRecordsAdapter{RequestCorrectionRecords: deps.Records}, People: deps.People, Plans: deps.Plans, Effects: deps.Effects, Excusal: deps.Excusal}, nil
}
