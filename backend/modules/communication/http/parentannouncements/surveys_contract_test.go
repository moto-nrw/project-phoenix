package announcement_test

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rootAPI "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	announcement "github.com/moto-nrw/project-phoenix/modules/communication/http/parentannouncements"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Only the survey capability is replaced (#4781); the routes, the
// permission gate and the tenant transaction are the registered ones.
type surveyContract struct {
	communication.SurveyCapability
	t           *testing.T
	survey      communication.Survey
	calls       []string
	answers     []communication.SurveyAnswerInput
	answeredBy  int64
	respondents error
}

func (s *surveyContract) record(name string) { s.calls = append(s.calls, name) }

func (s *surveyContract) ListSurveys(context.Context) ([]communication.Survey, error) {
	s.record("list")
	return []communication.Survey{s.survey}, nil
}
func (s *surveyContract) SurveyResults(context.Context, int64) (*communication.SurveyResults, error) {
	s.record("results")
	average := 4.5
	return &communication.SurveyResults{
		SurveyID: s.survey.ID, Title: s.survey.Title, Anonymous: true, Status: communication.SurveyStatusOpen,
		ResponseCount: 6, MinResponses: 5, Available: true,
		Questions: []communication.SurveyQuestionResult{{
			QuestionID: 1, Kind: communication.SurveyQuestionScale, Prompt: "Wie gut?", AnswerCount: 6,
			ScaleCounts: []int{0, 0, 0, 3, 3}, Average: &average, Options: []communication.SurveyOptionCount{}, Texts: []string{"mehr Zeit"},
			Highest: []string{"4", "5"}, Lowest: []string{"1", "2", "3"},
		}},
		Strengths: []communication.SurveyHighlight{}, Improvements: []communication.SurveyHighlight{},
	}, nil
}
func (s *surveyContract) SurveyRespondents(context.Context, int64) ([]communication.SurveyRespondent, error) {
	s.record("respondents")
	return nil, s.respondents
}
func (s *surveyContract) SubmitStaffSurvey(_ context.Context, accountID, _ int64, answers []communication.SurveyAnswerInput) error {
	s.record("answer")
	s.answeredBy, s.answers = accountID, answers
	return nil
}
func (s *surveyContract) StaffSurveysToAnswer(context.Context, int64) ([]communication.SurveyToAnswer, error) {
	s.record("to-answer")
	return []communication.SurveyToAnswer{{ID: s.survey.ID, Title: s.survey.Title, Anonymous: true, ClosesAt: s.survey.ClosesAt}}, nil
}

func surveyRoute(t *testing.T, perms ...string) (*surveyContract, *announcement.SurveyResource, string, int64) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	staff := testpkg.CreateTestStaff(t, db, "Survey", "Contract")
	claims := routetest.DefaultTestClaims()
	claims.IsAdmin = false
	claims.Permissions = perms
	s := &surveyContract{t: t, survey: communication.Survey{
		ID: staff.ID, Title: "Team", Audience: communication.SurveyAudienceStaff, Anonymous: true,
		ClosesAt: time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC), Targets: []communication.SurveyTarget{}, Questions: []communication.SurveyQuestion{},
	}}
	return s, announcement.NewSurveyResource(s, nil), routetest.MintTestJWT(t, claims), int64(claims.ID)
}

func surveyRequest(t *testing.T, r *announcement.SurveyResource, token, method, path string, body any) (int, string) {
	t.Helper()
	req := routetest.NewAuthenticatedRequest(t, method, path, body, routetest.WithJWTBearer(token))
	out := routetest.ExecuteRequest(r.Router(rootAPI.ProtectedTenantRoutes), req)
	return out.Code, out.Body.String()
}

func TestSurveyManagementNeedsTheSurveyPermission(t *testing.T) {
	t.Parallel()
	s, r, token, _ := surveyRoute(t, permissions.UsersRead)
	id := strconv.FormatInt(s.survey.ID, 10)
	for _, tc := range []struct{ method, path string }{
		{"GET", "/"}, {"POST", "/"}, {"GET", "/" + id}, {"PUT", "/" + id}, {"DELETE", "/" + id},
		{"POST", "/" + id + "/publish"}, {"POST", "/" + id + "/close"}, {"POST", "/" + id + "/remind"},
		{"GET", "/" + id + "/results"}, {"GET", "/" + id + "/results/export"}, {"GET", "/" + id + "/respondents"},
	} {
		status, body := surveyRequest(t, r, token, tc.method, tc.path, nil)
		require.Equal(t, 403, status, "%s %s: %s", tc.method, tc.path, body)
	}
	require.Empty(t, s.calls, "no management call without surveys:manage")

	// Answering needs no permission: the audience decides in the service.
	status, body := surveyRequest(t, r, token, "GET", "/to-answer", nil)
	require.Equal(t, 200, status, body)
	status, body = surveyRequest(t, r, token, "POST", "/"+id+"/answers", map[string]any{"answers": []map[string]any{{"question_id": "3", "scale_value": 4}}})
	require.Equal(t, 200, status, body)
	require.Equal(t, []string{"to-answer", "answer"}, s.calls)
}

func TestSurveyAnswerRouteHandsOnlyTheCallerAndTheAnswers(t *testing.T) {
	t.Parallel()
	s, r, token, caller := surveyRoute(t)
	id := strconv.FormatInt(s.survey.ID, 10)
	status, body := surveyRequest(t, r, token, "POST", "/"+id+"/answers", map[string]any{"answers": []map[string]any{
		{"question_id": "3", "option_ids": []string{"8", "9"}}, {"question_id": "4", "text": "mehr Zeit"},
	}})
	require.Equal(t, 200, status, body)
	assert.Equal(t, `{"status":"success","data":{"submitted":true},"message":"Survey answered"}`, strings.TrimSpace(body), "the answer is never echoed back")
	assert.Equal(t, caller, s.answeredBy)
	require.Len(t, s.answers, 2)
	assert.Equal(t, []int64{8, 9}, s.answers[0].OptionIDs)

	status, body = surveyRequest(t, r, token, "POST", "/"+id+"/answers", map[string]any{"answers": []map[string]any{{"question_id": "x"}}})
	require.Equal(t, 400, status, body)
}

func TestSurveyEvaluationWireCarriesNoPerson(t *testing.T) {
	t.Parallel()
	s, r, token, _ := surveyRoute(t, permissions.SurveysManage)
	id := strconv.FormatInt(s.survey.ID, 10)
	status, body := surveyRequest(t, r, token, "GET", "/"+id+"/results", nil)
	require.Equal(t, 200, status, body)
	assert.Contains(t, body, `"available":true`)
	assert.Contains(t, body, `"texts":["mehr Zeit"]`)
	for _, field := range []string{"account", "name", "created_at", "answered_at", "response_id"} {
		assert.NotContains(t, body, `"`+field, "the evaluation names no %s", field)
	}

	s.respondents = communication.ErrSurveyAnonymous
	status, body = surveyRequest(t, r, token, "GET", "/"+id+"/respondents", nil)
	require.Equal(t, 409, status, body)
	assert.Contains(t, body, "communication.survey_anonymous")
}
