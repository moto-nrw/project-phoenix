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
		roster = plannedRoster{query: deps.Roster}
	}
	if deps.CarePlan != nil {
		carePlan = carePlanDirectory{query: deps.CarePlan}
	}
	if deps.CareDays != nil {
		careDays = deps.CareDays
	}
	return ports.AttendanceRulePorts{Roster: roster, CarePlan: carePlan, CareDays: careDays}
}

func databaseRuntime(db *bun.DB) postgres.Database { return tenant.CurrentTenantTransaction[bun.IDB] }

type transaction struct{}

func (transaction) Run(ctx context.Context, fn func(context.Context) error) error {
	return tenant.NewTransactionRunner().RunInTx(ctx, fn)
}

func (transaction) Require(ctx context.Context) error {
	if _, err := tenant.TenantFromContext(ctx); err != nil {
		return err
	}
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return errors.New("student presence: transaction is required")
	}
	return nil
}
