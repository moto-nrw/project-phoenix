package services

import (
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// ExcusedRequestTestOptions builds the Care Plan excused-absence workflow
// over a test repository graph. Scope defaults to school-wide; Today to the
// Berlin calendar day.
type ExcusedRequestTestOptions struct {
	CarePlan    careplan.Capability
	Students    usersModels.StudentRepository
	Persons     usersModels.PersonRepository
	Scope       carePlanCompose.ReviewScopeResolver
	Emitter     *parentmessaging.Emitter
	Broadcaster realtime.Broadcaster
	Events      users.ParentRequestEventRecorder
	Logger      *slog.Logger
	Today       func() timezone.Date
}

// NewTestExcusedAbsenceRequests composes the workflow for behavior tests of
// the legacy consumers that still build their own repository graph.
func NewTestExcusedAbsenceRequests(options ExcusedRequestTestOptions) (*carePlanCompose.ExcusedAbsenceRequests, error) {
	if options.CarePlan == nil || options.Students == nil || options.Persons == nil {
		return nil, errors.New("test excused requests: care plan capability, student and person repositories are required")
	}
	scope := options.Scope
	if scope == nil {
		scope = repositories.SchoolWideReviewScope
	}
	return newExcusedAbsenceRequests(excusedRequestWiring{
		carePlan: options.CarePlan, students: options.Students, persons: options.Persons,
		scope: scope, emitter: options.Emitter, broadcaster: options.Broadcaster, events: options.Events,
		logger: options.Logger, today: options.Today,
	})
}
