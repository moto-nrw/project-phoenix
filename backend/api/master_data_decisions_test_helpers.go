//go:build testhooks

package api

import (
	"errors"
	"log/slog"

	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"

	owner0 "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/realtime"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// MasterDataDecisionTestOptions are the Care Plan inputs of the Stammdaten
// decision over a test repository graph. Scope defaults to school-wide;
// Today to the Berlin calendar day.
type MasterDataDecisionTestOptions struct {
	CarePlan careplan.Capability
	Scope    carePlanCompose.ReviewScopeResolver
	Shares   carePlanCompose.ShareVisibility
	Logger   *slog.Logger
	Today    func() timezone.Date
}

// MasterDataDecisionTestPeople are the People Directory rows and recorders
// the decision binds. Nil effects are skipped.
type MasterDataDecisionTestPeople struct {
	People   MasterDataPeople
	Students peoplePorts.StudentRepository
	Persons  peoplePorts.PersonRepository
	Audit    peoplePorts.StudentChangeRecorder
	Events   owner0.ParentRequestEvents
}

// NewTestMasterDataDecisions composes the decision for the behaviour suites
// that bind the People Directory rows through the retained repositories. A
// nil emitter or broadcaster is skipped.
func NewTestMasterDataDecisions(
	options MasterDataDecisionTestOptions, people MasterDataDecisionTestPeople,
	emitter eventContract.Emitter, broadcaster realtime.Broadcaster,
) (*carePlanCompose.MasterDataDecisions, error) {
	if options.CarePlan == nil || people.People == nil || people.Students == nil || people.Persons == nil {
		return nil, errors.New("test master data decisions: care plan capability, field review, student and person repositories are required")
	}
	scope := options.Scope
	if scope == nil {
		scope = schoolWideReviewScope
	}
	return NewMasterDataDecisions(
		options.CarePlan, people.People, people.Students, people.Persons, people.Audit, scope,
		emitter, broadcaster, people.Events, options.Shares, options.Logger, options.Today,
	)
}
