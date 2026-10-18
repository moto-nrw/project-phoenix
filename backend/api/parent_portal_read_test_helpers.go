//go:build testhooks

package api

import (
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// ParentChildReads binds this fixture's retained rows through the owner's
// public capability, with the same security policy as the production root.
func (f *CarePlanSuiteRepositories) ParentChildReads() parentportal.ChildReads {
	if f.ParentChild == nil {
		return nil
	}
	return NewParentChildReads(careplanCompose.NewParentChildQueries(f.ParentChild, securityruntime.GuardianPermissionGranted))
}
func (f *CarePlanSuiteRepositories) ParentPhaseReads() parentportal.EnrollablePhaseReads {
	if f.ParentEnrollablePhase == nil {
		return nil
	}
	return NewParentPhaseReads(careplanCompose.NewParentPhaseQueries(f.ParentEnrollablePhase))
}
func (f *CarePlanSuiteRepositories) ParentEnrollmentReads() parentportal.EnrollmentRequestReads {
	if f.ParentEnrollmentRequest == nil {
		return nil
	}
	return NewParentEnrollmentReads(careplanCompose.NewParentEnrollmentQueries(f.ParentEnrollmentRequest))
}
func (r ParentRouteTestRepositories) ParentChildReads() parentportal.ChildReads {
	if r.ParentChild == nil {
		return nil
	}
	return NewParentChildReads(careplanCompose.NewParentChildQueries(r.ParentChild, securityruntime.GuardianPermissionGranted))
}
