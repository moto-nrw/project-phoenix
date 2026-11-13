package api

import (
	"context"

	"github.com/go-chi/chi/v5"

	enrollmentOwner "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	projectionStudentsHTTP "github.com/moto-ogs/project-phoenix/modules/presenceprojection/http"
	"github.com/moto-ogs/project-phoenix/modules/schoolsearchview"
	schoolSearchCompose "github.com/moto-ogs/project-phoenix/modules/schoolsearchview/compose"
	schoolSearchHTTP "github.com/moto-ogs/project-phoenix/modules/schoolsearchview/http"
)

// mountSchoolSearch serves the school search at /api/search (ADR 0227,
// #6954) over Enrollment's registration answers and the caller's staff check.
func mountSchoolSearch(tenant chi.Router, staff schoolSearchHTTP.StaffCheck) error {
	search, err := schoolSearchCompose.New(schoolSearchAnswers{answers: enrollmentCompose.NewStudentAnswers()})
	if err != nil {
		return err
	}
	tenant.Mount("/search", schoolSearchHTTP.NewResource(search, staff).Router(ProtectedTenantRoutes))
	return nil
}

// schoolSearchAnswers binds the search's answer port to Enrollment.
type schoolSearchAnswers struct {
	answers enrollmentOwner.StudentAnswers
}

func (a schoolSearchAnswers) SearchStudentAnswers(ctx context.Context, text string, limit int) ([]schoolsearchview.StudentAnswerHit, error) {
	matches, err := a.answers.SearchStudentAnswers(ctx, text, limit)
	if err != nil {
		return nil, err
	}
	hits := make([]schoolsearchview.StudentAnswerHit, 0, len(matches))
	for _, match := range matches {
		hits = append(hits, schoolsearchview.StudentAnswerHit{StudentID: match.StudentID, Label: match.Label, Answer: match.Answer})
	}
	return hits, nil
}

// studentEnrollmentAnswerFilters binds the child list's registration answer
// filter (#6954) to Enrollment.
type studentEnrollmentAnswerFilters struct {
	answers enrollmentOwner.StudentAnswers
}

func newStudentEnrollmentAnswerFilters() studentEnrollmentAnswerFilters {
	return studentEnrollmentAnswerFilters{answers: enrollmentCompose.NewStudentAnswers()}
}

func (f studentEnrollmentAnswerFilters) AnswerFilters(ctx context.Context) ([]projectionStudentsHTTP.EnrollmentAnswerFilterResponse, error) {
	filters, err := f.answers.StudentAnswerFilters(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]projectionStudentsHTTP.EnrollmentAnswerFilterResponse, 0, len(filters))
	for _, filter := range filters {
		options := make([]projectionStudentsHTTP.EnrollmentAnswerOptionResponse, 0, len(filter.Options))
		for _, option := range filter.Options {
			options = append(options, projectionStudentsHTTP.EnrollmentAnswerOptionResponse(option))
		}
		out = append(out, projectionStudentsHTTP.EnrollmentAnswerFilterResponse{Key: filter.Key, Label: filter.Label, Type: string(filter.Type), Options: options})
	}
	return out, nil
}

func (f studentEnrollmentAnswerFilters) StudentsWithAnswer(ctx context.Context, key, value string) ([]int64, error) {
	return f.answers.StudentsWithAnswer(ctx, key, value)
}
