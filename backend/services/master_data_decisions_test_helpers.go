package services

import (
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// MasterDataDecisionTestOptions builds the Care Plan Stammdaten decision over
// a test repository graph. Scope defaults to school-wide; Today to the Berlin
// calendar day. Nil effects are skipped.
type MasterDataDecisionTestOptions struct {
	CarePlan    careplan.Capability
	People      peopledirectory.StudentFieldReviewQuery
	Students    usersModels.StudentRepository
	Persons     usersModels.PersonRepository
	Audit       users.StudentChangeRecorder
	Scope       carePlanCompose.ReviewScopeResolver
	Emitter     *parentmessaging.Emitter
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
		scope = repositories.SchoolWideReviewScope
	}
	return newMasterDataDecisions(masterDataDecisionWiring{
		carePlan: options.CarePlan, fields: options.People, students: options.Students, persons: options.Persons,
		audit: options.Audit, scope: scope, emitter: options.Emitter, broadcaster: options.Broadcaster,
		events: options.Events, shares: options.Shares, logger: options.Logger, today: options.Today,
	})
}
