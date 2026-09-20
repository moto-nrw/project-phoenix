package organizationtenancy

import (
	"context"
	"errors"
	"fmt"
)

var ErrDemoAlreadyRunning = errors.New("demo process already running")

// DemoSchoolState retains the complete synthetic seed contract. It is never
// exposed through a tenant or operator HTTP endpoint: it includes credentials.
type DemoSchoolState struct {
	SchoolID int64
	SeedJSON []byte
}

type DemoStateEngine interface {
	LoadDemoSchool(context.Context, string) (*DemoSchoolState, error)
	RememberDemoSchool(context.Context, string, DemoSchoolState) error
	WithDemoLease(context.Context, string, func(context.Context) error) error
}

// DemoSchools is the provisioning-state capability for the isolated demo
// process, separate from the request-scoped school administration capability.
type DemoSchools struct{ engine DemoStateEngine }

func NewDemoSchools(engine DemoStateEngine) *DemoSchools {
	if engine == nil {
		panic("demo schools require persistence")
	}
	return &DemoSchools{engine: engine}
}

func (d *DemoSchools) LoadDemoSchool(ctx context.Context, name string) (*DemoSchoolState, error) {
	if name == "" {
		return nil, fmt.Errorf("demo school name is required")
	}
	return d.engine.LoadDemoSchool(ctx, name)
}

func (d *DemoSchools) RememberDemoSchool(ctx context.Context, name string, state DemoSchoolState) error {
	if name == "" || state.SchoolID <= 0 || len(state.SeedJSON) == 0 {
		return fmt.Errorf("demo school name, school ID and seed state are required")
	}
	return d.engine.RememberDemoSchool(ctx, name, state)
}

func (d *DemoSchools) WithDemoLease(ctx context.Context, name string, run func(context.Context) error) error {
	if name == "" || run == nil {
		return fmt.Errorf("demo lease name and callback are required")
	}
	return d.engine.WithDemoLease(ctx, name, run)
}
