package api

import (
	"context"

	"github.com/go-chi/chi/v5"

	enrollmentOwner "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	projectionStudentsHTTP "github.com/moto-ogs/project-phoenix/modules/presenceprojection/http"
	schoolSearchCompose "github.com/moto-ogs/project-phoenix/modules/schoolsearchview/compose"
	schoolSearchHTTP "github.com/moto-ogs/project-phoenix/modules/schoolsearchview/http"
)

// mountSchoolSearch serves the school search at /api/search (ADR 0229,
// #6954) over Enrollment's registration answers and the caller's staff check.
func mountSchoolSearch(tenant chi.Router, staff schoolSearchHTTP.StaffCheck) error {
	search, err := schoolSearchCompose.New(enrollmentCompose.NewStudentAnswers())
	if err != nil {
		return err
	}
	tenant.Mount("/search", schoolSearchHTTP.NewResource(search, staff).Router(ProtectedTenantRoutes))
	return nil
}

// studentEnrollmentAnswerFilters binds the child list's registration answer
// filter (#6954) to Enrollment. StudentsWithAnswer is Enrollment's own.
type studentEnrollmentAnswerFilters struct{ enrollmentOwner.StudentAnswers }

func newStudentEnrollmentAnswerFilters() studentEnrollmentAnswerFilters {
	return studentEnrollmentAnswerFilters{StudentAnswers: enrollmentCompose.NewStudentAnswers()}
}

// AnswerFilters translates Enrollment's filter questions into the list's
// response shape.
func (f studentEnrollmentAnswerFilters) AnswerFilters(ctx context.Context) ([]projectionStudentsHTTP.EnrollmentAnswerFilterResponse, error) {
	filters, err := f.StudentAnswerFilters(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]projectionStudentsHTTP.EnrollmentAnswerFilterResponse, 0, len(filters))
	for _, filter := range filters {
		options := make([]projectionStudentsHTTP.EnrollmentAnswerOptionResponse, 0, len(filter.Options))
		for _, option := range filter.Options {
			options = append(options, projectionStudentsHTTP.EnrollmentAnswerOptionResponse(option))
		}
		out = append(out, projectionStudentsHTTP.EnrollmentAnswerFilterResponse{
			Key: filter.Key, Label: filter.Label, Type: string(filter.Type), Options: options,
		})
	}
	return out, nil
}
