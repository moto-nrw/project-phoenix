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

// ExcusedRequestTestOptions are the Care Plan inputs of the excused-absence
// workflow over a test repository graph. Scope defaults to school-wide; Today
// to the Berlin calendar day.
type ExcusedRequestTestOptions struct {
	CarePlan careplan.Capability
	Scope    carePlanCompose.ReviewScopeResolver
	Logger   *slog.Logger
	Today    func() timezone.Date
}

// ExcusedRequestTestPeople are the People Directory rows the workflow binds.
type ExcusedRequestTestPeople struct {
	Students peoplePorts.StudentRepository
	Persons  peoplePorts.PersonRepository
	Events   owner0.ParentRequestEvents
}

// NewTestExcusedAbsenceRequests composes the workflow for behavior tests of
// the legacy consumers that still build their own repository graph. A nil
// emitter or broadcaster is skipped.
func NewTestExcusedAbsenceRequests(
	options ExcusedRequestTestOptions, people ExcusedRequestTestPeople,
	emitter eventContract.Emitter, broadcaster realtime.Broadcaster,
) (*carePlanCompose.ExcusedAbsenceRequests, error) {
	if options.CarePlan == nil || people.Students == nil || people.Persons == nil {
		return nil, errors.New("test excused requests: care plan capability, student and person repositories are required")
	}
	scope := options.Scope
	if scope == nil {
		scope = schoolWideReviewScope
	}
	return NewExcusedAbsenceRequestService(
		options.CarePlan, people.Students, people.Persons, scope, emitter, broadcaster,
		people.Events, nil, nil, nil, options.Logger, options.Today,
	)
}
