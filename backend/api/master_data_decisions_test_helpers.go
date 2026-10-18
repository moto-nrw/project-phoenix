//go:build testhooks

package api

import (
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/realtime"
	usersModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
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
	Events      usersModels.ParentRequestEventRepository
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
	return NewMasterDataDecisions(
		options.CarePlan, options.People, options.Students, options.Persons, options.Audit, scope,
		options.Emitter, options.Broadcaster, options.Events, options.Shares, options.Logger, options.Today,
	)
}
