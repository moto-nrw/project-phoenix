package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// fakePortalModules books every portal unless a school is listed as
// without its module (#4895).
type fakePortalModules struct {
	withoutParentPortal map[int64]bool
	withoutSchoolPortal map[int64]bool
	err                 error
}

func (f *fakePortalModules) BookedParentPortalSchools(_ context.Context, tenantIDs []int64) ([]int64, error) {
	if f.err != nil {
		return nil, f.err
	}
	booked := []int64{}
	for _, tenantID := range tenantIDs {
		if !f.withoutParentPortal[tenantID] {
			booked = append(booked, tenantID)
		}
	}
	return booked, nil
}

func (f *fakePortalModules) RequireParentPortal(_ context.Context, tenantID int64) error {
	if f.err != nil {
		return f.err
	}
	if f.withoutParentPortal[tenantID] {
		return &identityaccess.PortalNotBookedError{Module: "eltern_app", ModuleLabel: "Eltern-App"}
	}
	return nil
}

func (f *fakePortalModules) RequireSchoolPortal(_ context.Context, tenantID int64) error {
	if f.err != nil {
		return f.err
	}
	if f.withoutSchoolPortal[tenantID] {
		return &identityaccess.PortalNotBookedError{Module: "unterricht_vormittag", ModuleLabel: "Unterricht am Vormittag"}
	}
	return nil
}
