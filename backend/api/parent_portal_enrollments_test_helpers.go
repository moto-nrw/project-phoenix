package api

import enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"

// NewParentPortalEnrollments binds the guardian portal's enrollment views to
// an Enrollment owner over the ambient tenant transaction, as the root does,
// for the Care Plan repository suites.
func NewParentPortalEnrollments() ParentPortalEnrollments {
	return ParentPortalEnrollments{enrollment: enrollmentCompose.New()}
}
