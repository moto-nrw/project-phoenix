package presenceprojectionhttp

import (
	context "context"
	errors "errors"
	http "net/http"
	strings "strings"

	common "github.com/moto-ogs/project-phoenix/api/common"
	enrollment "github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// EnrollmentAnswerFilters is the consumer-owned port to Enrollment's answers
// of the school's own registration questions (enrollment.StudentAnswers,
// #6954). The root binds it.
type EnrollmentAnswerFilters interface {
	// StudentAnswerFilters lists the yes/no and choice questions with
	// answers.
	StudentAnswerFilters(ctx context.Context) ([]enrollment.StudentAnswerFilter, error)
	// StudentsWithAnswer returns the children whose newest answer to key is
	// value.
	StudentsWithAnswer(ctx context.Context, key, value string) ([]int64, error)
}

// errEnrollmentAnswerFilter refuses an enrollment_answer value that is not
// "<key>=<value>".
var errEnrollmentAnswerFilter = errors.New("enrollment_answer must be <question key>=<answer value>")

// errEnrollmentAnswerForbidden refuses the filter to a caller without
// users:read: the answers are as protected as on the child's page, and a
// filtered count would reveal them.
var errEnrollmentAnswerForbidden = errors.New("the registration answer filter requires users:read")

// enrollmentAnswer is a parsed enrollment_answer filter.
type enrollmentAnswer struct{ key, value string }

// parseEnrollmentAnswer reads "<key>=<value>". Empty means off.
func parseEnrollmentAnswer(raw string) (*enrollmentAnswer, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	key, value, ok := strings.Cut(raw, "=")
	if !ok || strings.TrimSpace(key) == "" || value == "" {
		return nil, errEnrollmentAnswerFilter
	}
	return &enrollmentAnswer{key: strings.TrimSpace(key), value: value}, nil
}

// readEnrollmentAnswerFilter parses the list parameter and checks the
// caller may read the answers.
func readEnrollmentAnswerFilter(r *http.Request, params *studentListParams) error {
	answer, err := parseEnrollmentAnswer(r.URL.Query().Get("enrollment_answer"))
	if err != nil {
		return err
	}
	if answer != nil && !canReadSensitiveStudentData(r.Context()) {
		return errEnrollmentAnswerForbidden
	}
	params.enrollmentAnswer = answer
	return nil
}

// resolveEnrollmentAnswerFilter narrows params.studentIDs to the children
// with the answer. It reports nonEmpty=false when none is left.
func (rs *Resource) resolveEnrollmentAnswerFilter(ctx context.Context, params *studentListParams) (bool, error) {
	if params.enrollmentAnswer == nil {
		return true, nil
	}
	if rs.EnrollmentAnswers == nil {
		return false, errors.New("student list: enrollment answer filters are not configured")
	}
	ids, err := rs.EnrollmentAnswers.StudentsWithAnswer(ctx, params.enrollmentAnswer.key, params.enrollmentAnswer.value)
	if err != nil {
		return false, err
	}
	if len(ids) == 0 {
		return false, nil
	}
	if len(params.studentIDs) == 0 {
		params.studentIDs = ids
		return true, nil
	}
	params.studentIDs = intersectStudentIDs(params.studentIDs, ids)
	return len(params.studentIDs) > 0, nil
}

// listEnrollmentAnswerOptions answers the questions the filter offers. It
// needs users:read, like the answers on the child's page.
func (rs *Resource) listEnrollmentAnswerOptions(w http.ResponseWriter, r *http.Request) {
	if rs.EnrollmentAnswers == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("enrollment answer filters are not configured")))
		return
	}
	filters, err := rs.EnrollmentAnswers.StudentAnswerFilters(r.Context())
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to load registration answer filters", err))
		return
	}
	if filters == nil {
		filters = []enrollment.StudentAnswerFilter{}
	}
	common.Respond(w, r, http.StatusOK, filters, "Registration answer filters retrieved successfully")
}
