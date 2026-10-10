package api

import (
	parentAPI "github.com/moto-nrw/project-phoenix/modules/careplan/inbound/parent"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
)

// parentEnrollmentForms binds the parents portal's form flow to the
// enrollment routes (#2734); without the intake the port stays unbound and
// the portal answers its enrollment routes as not configured.
func parentEnrollmentForms(requests enrollmentAPI.RequestService) parentAPI.EnrollmentForms {
	if requests == nil {
		return nil
	}
	return enrollmentAPI.ParentForms{Requests: requests}
}
