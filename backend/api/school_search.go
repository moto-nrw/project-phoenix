package api

import (
	"github.com/go-chi/chi/v5"

	enrollmentCompose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	schoolSearchCompose "github.com/moto-ogs/project-phoenix/modules/schoolsearchview/compose"
	schoolSearchHTTP "github.com/moto-ogs/project-phoenix/modules/schoolsearchview/http"
)

// mountSchoolSearch serves the school search at /api/search (ADR 0227,
// #6954) over Enrollment's registration answers and the caller's staff check.
func mountSchoolSearch(tenant chi.Router, staff schoolSearchHTTP.StaffCheck) error {
	search, err := schoolSearchCompose.New(enrollmentCompose.NewStudentAnswers())
	if err != nil {
		return err
	}
	tenant.Mount("/search", schoolSearchHTTP.NewResource(search, staff).Router(ProtectedTenantRoutes))
	return nil
}
