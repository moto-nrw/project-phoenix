package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	careRecordPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// RequestDecisionDependencies wires the decisions. Guardians may be nil: a
// graph without parent messaging skips the guardian check.
type RequestDecisionDependencies struct {
	Records   careRecordPorts.RequestDecisionRecords
	People    ports.RequestDecisionPeople
	Guardians ports.GuardianAccess
	Plans     ports.RequestDecisionPlans
	Effects   ports.RequestDecisionEffects
	Today     func() calendar.Date
}

func NewRequestDecisions(deps RequestDecisionDependencies) (carerequests.Decisions, error) {
	if deps.Records == nil || deps.People == nil || deps.Plans == nil || deps.Effects == nil || deps.Today == nil {
		return nil, errors.New("care request decisions: records, people, plans, effects, and clock are required")
	}
	return &application.RequestDecisions{Records: application.RequestDecisionRecordsAdapter{RequestDecisionRecords: deps.Records}, People: deps.People, Guardians: deps.Guardians,
		Plans: deps.Plans, Effects: deps.Effects, Today: deps.Today, Transaction: tenant.NewTransactionRunner()}, nil
}

// CareRequestEffects tells the families and the staff tabs about a care
// request transition; every request command takes it as its effects.
type CareRequestEffects interface {
	ports.RequestSubmissionEffects
	ports.RequestEditEvents
	ports.RequestDecisionEffects
	ports.RequestCorrectionEffects
}

// CareRequestEffectPorts binds the owners a care request reports to.
type CareRequestEffectPorts application.CareRequestEffectPorts

func NewCareRequestEffects(deps CareRequestEffectPorts) (CareRequestEffects, error) {
	return application.NewCareRequestEffects(application.CareRequestEffectPorts(deps), TenantHooks{})
}
