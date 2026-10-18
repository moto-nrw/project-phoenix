//go:build testhooks

package api

import (
	"errors"
	"log/slog"

	owner0 "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	"github.com/moto-nrw/project-phoenix/realtime"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// MasterDataDecisionTestOptions builds the Care Plan Stammdaten decision over
// a test repository graph. Scope defaults to school-wide; Today to the Berlin
// calendar day. Nil effects are skipped.
type MasterDataDecisionTestOptions struct {
	CarePlan    careplan.Capability
	People      MasterDataPeople
	Students    peoplePorts.StudentRepository
	Persons     peoplePorts.PersonRepository
	Audit       peoplePorts.StudentChangeRecorder
	Scope       carePlanCompose.ReviewScopeResolver
	Emitter     *communicationCompose.ParentEventEmitter
	Broadcaster realtime.Broadcaster
	Events      owner0.ParentRequestEvents
	Shares      carePlanCompose.ShareVisibility
	Logger      *slog.Logger
	Today       func() timezone.Date
}

// NewTestMasterDataDecisions composes the decision for the behaviour suites
// that bind the People Directory rows through the retained repositories.
func NewTestMasterDataDecisions(options MasterDataDecisionTestOptions) (*carePlanCompose.MasterDataDecisions, error) {
	if options.CarePlan == nil || options.People == nil || options.Students == nil || options.Persons == nil {
		return nil, errors.New("test master data decisions: care plan capability, field review, student and person repositories are required")
	}
	scope := options.Scope
	if scope == nil {
		scope = schoolWideReviewScope
	}
	return NewMasterDataDecisions(MasterDataDecisionWiring{
		CarePlan: options.CarePlan, Fields: options.People, Students: options.Students, Persons: options.Persons,
		Audit: options.Audit, Scope: scope, Emitter: options.Emitter, Broadcaster: options.Broadcaster,
		Events: options.Events, Shares: options.Shares, Logger: options.Logger, Today: options.Today,
	})
}
