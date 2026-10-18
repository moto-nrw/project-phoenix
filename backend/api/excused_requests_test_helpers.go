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

// ExcusedRequestTestOptions builds the Care Plan excused-absence workflow
// over a test repository graph. Scope defaults to school-wide; Today to the
// Berlin calendar day.
type ExcusedRequestTestOptions struct {
	CarePlan    careplan.Capability
	Students    peoplePorts.StudentRepository
	Persons     peoplePorts.PersonRepository
	Scope       carePlanCompose.ReviewScopeResolver
	Emitter     *communicationCompose.ParentEventEmitter
	Broadcaster realtime.Broadcaster
	Events      owner0.ParentRequestEvents
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
		scope = schoolWideReviewScope
	}
	return NewExcusedAbsenceRequestService(ExcusedRequestServiceWiring{
		CarePlan: options.CarePlan, Students: options.Students, Persons: options.Persons,
		Scope: scope, Emitter: options.Emitter, Broadcaster: options.Broadcaster, Events: options.Events,
		Logger: options.Logger, Today: options.Today,
	})
}
