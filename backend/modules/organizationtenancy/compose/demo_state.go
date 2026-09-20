package compose

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

func NewDemoSchools(db *bun.DB) (*organizationtenancy.DemoSchools, error) {
	if db == nil {
		return nil, fmt.Errorf("demo schools require a database")
	}
	return organizationtenancy.NewDemoSchools(demoStateEngine{store: postgres.NewDemoStateStore(db)}), nil
}

type demoStateEngine struct{ store *postgres.DemoStateStore }

func (e demoStateEngine) LoadDemoSchool(ctx context.Context, name string) (*organizationtenancy.DemoSchoolState, error) {
	state, err := e.store.Load(ctx, name)
	if err != nil || state == nil {
		return nil, err
	}
	return &organizationtenancy.DemoSchoolState{SchoolID: state.TenantID, SeedJSON: []byte(state.SeedJSON)}, nil
}

func (e demoStateEngine) RememberDemoSchool(ctx context.Context, name string, state organizationtenancy.DemoSchoolState) error {
	return e.store.Remember(ctx, name, postgres.DemoState{TenantID: state.SchoolID, SeedJSON: string(state.SeedJSON)})
}

func (e demoStateEngine) WithDemoLease(ctx context.Context, name string, run func(context.Context) error) error {
	acquired, err := e.store.WithLease(ctx, name, run)
	if err != nil {
		return err
	}
	if !acquired {
		return organizationtenancy.ErrDemoAlreadyRunning
	}
	return nil
}
