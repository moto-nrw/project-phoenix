//go:build testhooks

package api

import (
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
)

// NewParentPortalEnrollments binds the guardian portal's enrollment views to
// an Enrollment owner over the ambient tenant transaction, as the root does,
// for the Care Plan repository suites.
func NewParentPortalEnrollments() owners.ParentPortalEnrollments {
	return owners.ParentPortalEnrollments{Enrollment: enrollmentCompose.New()}
}
