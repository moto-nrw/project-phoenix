package postgres

import (
	context "context"

	organizationtenancy "github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

type DemoStateEngine struct{ Store *DemoStateStore }

func (e DemoStateEngine) LoadDemoSchool(ctx context.Context, name string) (*organizationtenancy.DemoSchoolState, error) {
	state, err := e.Store.Load(ctx, name)
	if err != nil || state == nil {
		return nil, err
	}
	return &organizationtenancy.DemoSchoolState{SchoolID: state.TenantID, SeedJSON: []byte(state.SeedJSON), VisitorAccountID: state.VisitorAccountID, VisitorParentAccountID: state.VisitorParentAccountID}, nil
}
func (e DemoStateEngine) ReserveDemoSchool(ctx context.Context, name string, schoolID, visitorAccountID, visitorParentAccountID int64) error {
	return e.Store.Reserve(ctx, name, schoolID, visitorAccountID, visitorParentAccountID)
}
func (e DemoStateEngine) RememberDemoSchool(ctx context.Context, name string, state organizationtenancy.DemoSchoolState) error {
	return e.Store.Remember(ctx, name, DemoState{TenantID: state.SchoolID, SeedJSON: string(state.SeedJSON), VisitorAccountID: state.VisitorAccountID, VisitorParentAccountID: state.VisitorParentAccountID})
}
func (e DemoStateEngine) UpdateDemoSchool(ctx context.Context, name string, state organizationtenancy.DemoSchoolState) error {
	return e.Store.Update(ctx, name, DemoState{TenantID: state.SchoolID, SeedJSON: string(state.SeedJSON), VisitorAccountID: state.VisitorAccountID, VisitorParentAccountID: state.VisitorParentAccountID})
}
func (e DemoStateEngine) WithDemoLease(ctx context.Context, name string, run func(context.Context) error) error {
	acquired, err := e.Store.WithLease(ctx, name, run)
	if err != nil {
		return err
	}
	if !acquired {
		return organizationtenancy.ErrDemoAlreadyRunning
	}
	return nil
}
