package compose

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type RequestDecisionRecords interface {
	FindCareScheduleRequest(context.Context, int64, bool) (carerequests.Request, error)
	DecideCareScheduleRequest(context.Context, careplan.CareScheduleRequestDecision) error
	UpdateCareScheduleRequestSnapshot(context.Context, int64, json.RawMessage) error
}

// RequestDecisionDependencies wires the decisions. Guardians may be nil: a
// graph without parent messaging skips the guardian check.
type RequestDecisionDependencies struct {
	Records   RequestDecisionRecords
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
	return &application.RequestDecisions{Records: requestDecisionRecords{deps.Records}, People: deps.People, Guardians: deps.Guardians,
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

type requestDecisionRecords struct{ RequestDecisionRecords }

func (r requestDecisionRecords) Find(ctx context.Context, id int64, lock bool) (carerequests.Request, error) {
	return r.FindCareScheduleRequest(ctx, id, lock)
}
func (r requestDecisionRecords) Decide(ctx context.Context, id int64, status string, reason *string, actorID int64, applied bool) error {
	return r.DecideCareScheduleRequest(ctx, careplan.CareScheduleRequestDecision{ID: id, Status: status, Reason: reason, ReviewedBy: &actorID, Applied: applied})
}
func (r requestDecisionRecords) StoreSnapshot(ctx context.Context, id int64, snapshot *carerequests.DecisionSnapshot) error {
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	return r.UpdateCareScheduleRequestSnapshot(ctx, id, raw)
}
