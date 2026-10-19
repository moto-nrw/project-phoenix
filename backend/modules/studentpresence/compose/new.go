// Package compose builds the Student Presence owner over the tenant runtime.
package compose

import (
	"context"
	"errors"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/adapters/postgres"
	application "github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type Observation = ports.Observation

type Dependencies struct {
	DB      *bun.DB
	Observe func(Observation)
	// Now is the clock the module reads the school day from; nil uses time.Now.
	Now func() time.Time
	// The session attendance rules (#2762) resolve their participants through
	// the Timetable owner and decide against the care plan. A composition that
	// serves none of those rules leaves the three ports nil; the rules then
	// fail with the module's configuration errors.
	Roster   studentpresence.PlannedRoster
	CarePlan studentpresence.SessionCarePlanDirectory
	CareDays studentpresence.SessionCareDayLocker
}

func New(deps Dependencies) (*studentpresence.Module, error) {
	if deps.DB == nil || deps.Observe == nil {
		return nil, errors.New("student presence compose: all dependencies are required")
	}
	store := postgres.New(databaseRuntime(deps.DB))
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	service := application.New(store, transaction{}, deps.Observe, sessionAttendancePorts(deps))
	return studentpresence.NewModule(application.NewEngine(service, now)), nil
}

func sessionAttendancePorts(deps Dependencies) ports.AttendanceRulePorts {
	var roster ports.PlannedRoster
	var carePlan ports.CarePlanDirectory
	var careDays ports.CareDayLocker
	if deps.Roster != nil {
		roster = application.NewPlannedRoster(deps.Roster)
	}
	if deps.CarePlan != nil {
		carePlan = application.NewCarePlanDirectory(deps.CarePlan)
	}
	if deps.CareDays != nil {
		careDays = deps.CareDays
	}
	return ports.AttendanceRulePorts{Roster: roster, CarePlan: carePlan, CareDays: careDays}
}

func databaseRuntime(db *bun.DB) postgres.Database { return tenant.CurrentTenantTransaction[bun.IDB] }

// transaction retains the registered runtime entry until its role-changing
// extraction can retire rather than replace that exact command boundary.
type transaction struct{ application.Transaction }

func (transaction) Run(ctx context.Context, fn func(context.Context) error) error {
	return tenant.NewTransactionRunner().RunInTx(ctx, fn)
}
