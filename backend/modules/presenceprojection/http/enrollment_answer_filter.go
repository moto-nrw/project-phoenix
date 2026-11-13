package presenceprojectionhttp

import (
	context "context"
	errors "errors"
	http "net/http"
	strings "strings"

	common "github.com/moto-ogs/project-phoenix/api/common"
	sessiontoken "github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	peopleModule "github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	securityruntime "github.com/moto-ogs/project-phoenix/modules/securityruntime"
)

// EnrollmentAnswerFilters is the consumer-owned port to Enrollment's answers
// of the school's own registration questions (#6954). The root binds it.
type EnrollmentAnswerFilters interface {
	// AnswerFilters lists the yes/no and choice questions with answers and,
	// per answer, every linked child who gave it (StudentIDs).
	AnswerFilters(ctx context.Context) ([]EnrollmentAnswerFilterResponse, error)
	// StudentsWithAnswer returns the children whose newest answer to key is
	// value.
	StudentsWithAnswer(ctx context.Context, key, value string) ([]int64, error)
}

// EnrollmentAnswerFilterResponse is one question of the filter: what the form
// asked and the answers given, with how many children gave them.
type EnrollmentAnswerFilterResponse struct {
	Key     string                           `json:"key"`
	Label   string                           `json:"label"`
	Type    string                           `json:"type"`
	Options []EnrollmentAnswerOptionResponse `json:"options"`
}

// EnrollmentAnswerOptionResponse is one answer of a filter question. Count is
// the number of those children the list shows; StudentIDs, every linked child
// with the answer, stays on the server.
type EnrollmentAnswerOptionResponse struct {
	Value      string  `json:"value"`
	Label      string  `json:"label"`
	Count      int     `json:"count"`
	StudentIDs []int64 `json:"-"`
}

// errEnrollmentAnswerFilter refuses an enrollment_answer value that is not
// "<key>=<value>".
var errEnrollmentAnswerFilter = errors.New("enrollment_answer must be <question key>=<answer value>")

// errEnrollmentAnswerForbidden refuses the filter, its options and a filtered
// export to a caller who may not read the answers on the child's page: a
// filtered list or count would reveal them.
var errEnrollmentAnswerForbidden = errors.New("the registration answer filter requires users:read and a staff record of the school")

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

// canReadEnrollmentAnswers is the gate of the answers on the child's page
// (GET /students/{id}/enrollment-extra-fields): users:read, held by an
// administrator or by staff of this school.
func (rs *Resource) canReadEnrollmentAnswers(ctx context.Context) bool {
	return canReadSensitiveStudentData(ctx) &&
		securityruntime.ResolveStudentReadScope(ctx, sessiontoken.PermissionsFromCtx(ctx), rs.UserContextService)
}

// readEnrollmentAnswerFilter parses the list parameter and checks the
// caller may read the answers.
func (rs *Resource) readEnrollmentAnswerFilter(r *http.Request, params *studentListParams) error {
	answer, err := parseEnrollmentAnswer(r.URL.Query().Get("enrollment_answer"))
	if err != nil {
		return err
	}
	if answer != nil && !rs.canReadEnrollmentAnswers(r.Context()) {
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

// listEnrollmentAnswerOptions answers the questions the filter offers, with
// the number of children the list shows for each answer. It needs the access
// of the answers on the child's page.
func (rs *Resource) listEnrollmentAnswerOptions(w http.ResponseWriter, r *http.Request) {
	if !rs.canReadEnrollmentAnswers(r.Context()) {
		renderError(w, r, common.ErrorForbidden(errEnrollmentAnswerForbidden))
		return
	}
	if rs.EnrollmentAnswers == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("enrollment answer filters are not configured")))
		return
	}
	filters, err := rs.EnrollmentAnswers.AnswerFilters(r.Context())
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to load registration answer filters", err))
		return
	}
	listed, err := rs.listedAmong(r, answeredStudents(filters))
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to count registration answers", err))
		return
	}
	common.Respond(w, r, http.StatusOK, countListedAnswers(filters, listed), "Registration answer filters retrieved successfully")
}

// answeredStudents is every child with an answer to one of the questions.
func answeredStudents(filters []EnrollmentAnswerFilterResponse) []int64 {
	seen := map[int64]bool{}
	ids := []int64{}
	for _, filter := range filters {
		for _, option := range filter.Options {
			for _, id := range option.StudentIDs {
				if !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
		}
	}
	return ids
}

// listedAmong reports which of ids the child list shows without further
// filters, through the list's own path: the running care of today, then the
// directory, which leaves out children who left the school.
func (rs *Resource) listedAmong(r *http.Request, ids []int64) (map[int64]bool, error) {
	listed := make(map[int64]bool, len(ids))
	if len(ids) == 0 {
		return listed, nil
	}
	params := parseStudentListParams(r)
	params.careStatus = CareStatusRunning
	params.studentIDs = ids
	params.fetchAll = true
	if nonEmpty, err := rs.resolveOperationalParticipantFilter(r.Context(), params); err != nil || !nonEmpty {
		return listed, err
	}
	records, err := rs.PeopleDirectory.ListStudentDirectory(r.Context(), params.buildDirectoryFilter())
	if err != nil {
		return nil, err
	}
	for _, student := range peopleModule.StudentStatesFromRecords(records) {
		listed[student.ID] = true
	}
	return listed, nil
}

// countListedAnswers sets each answer's count to the listed children who gave
// it. A choice nobody listed gave goes, as does a question left without any
// listed answer; yes and no both stay while one of them has children.
func countListedAnswers(filters []EnrollmentAnswerFilterResponse, listed map[int64]bool) []EnrollmentAnswerFilterResponse {
	out := make([]EnrollmentAnswerFilterResponse, 0, len(filters))
	for _, filter := range filters {
		options := make([]EnrollmentAnswerOptionResponse, 0, len(filter.Options))
		total := 0
		for _, option := range filter.Options {
			option.Count = 0
			for _, id := range option.StudentIDs {
				if listed[id] {
					option.Count++
				}
			}
			total += option.Count
			if option.Count > 0 || filter.Type == "boolean" {
				options = append(options, option)
			}
		}
		if total > 0 {
			filter.Options = options
			out = append(out, filter)
		}
	}
	return out
}
