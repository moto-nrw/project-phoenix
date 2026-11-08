package announcement_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	compose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	announcement "github.com/moto-nrw/project-phoenix/modules/communication/http/parentannouncements"
	"github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
	deliveryPolicy "github.com/moto-nrw/project-phoenix/roots/communicationtest"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestSurveyPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	database := testpkg.NewDatabase(t, db)
	organizations, err := owners.NewOrganizationTenancy(database)
	require.NoError(t, err)
	repos := api.NewParentMessagingSuiteRepositories(database).WithSchoolProjections(organizations)
	settings, err := owners.NewSettingsTestModule(database, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	objectBackend, err := objects.PrivateUploadsBackend()
	require.NoError(t, err)
	files, _, err := api.NewFileStoreTestModule(database, testpkg.TenantRuntime(t, db), objectBackend)
	require.NoError(t, err)
	withSurveys, err := compose.NewParentAnnouncementsWithAttachments(compose.ParentAnnouncementConfig{Repo: repos.ParentAnnouncement, Settings: settingsValues.NewParentAnnouncementValues(settings.Settings), Logger: slog.Default(), SurveyStaff: repos.StaffMessageRead}, files, owners.CommunicationRequestRuntime{}, deliveryPolicy.NewParentDeliveryPolicy())
	require.NoError(t, err)
	surveys, err := communication.SurveysOf(withSurveys)
	require.NoError(t, err)
	_, viewer := testpkg.CreateTestStaffWithAccount(t, db, "Survey", "Viewer")
	testpkg.EnsureAccountTenant(t, db, viewer.ID, testpkg.Tenant(t))
	token := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(viewer.ID), Sub: viewer.Email, Roles: []string{"user"}, Permissions: []string{"surveys:manage"}, TenantID: testpkg.Tenant(t)})
	router := announcement.NewSurveyResource(surveys, owners.NewRecordReports()).Router(api.ProtectedTenantRoutes)
	counter := testpkg.CaptureQueriesForContext(t, db)
	counter.Stop()
	var lastSurvey *communication.Survey
	for _, n := range []int{3, 8} {
		input := communication.SurveyInput{Title: fmt.Sprintf("Growth-%d", n), Audience: communication.SurveyAudienceStaff, Anonymous: false, ClosesAt: time.Now().Add(72 * time.Hour)}
		for i := 0; i < n; i++ {
			input.Questions = append(input.Questions, communication.SurveyQuestionInput{Kind: communication.SurveyQuestionSingle, Prompt: fmt.Sprintf("Question-%02d", i), Required: true, Options: []string{"Yes", "No"}})
		}
		var survey *communication.Survey
		require.NoError(t, testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context) error {
			draft, e := surveys.CreateSurvey(ctx, viewer.ID, input)
			if e != nil {
				return e
			}
			survey, e = surveys.PublishSurvey(ctx, viewer.ID, draft.ID)
			return e
		}))
		require.Len(t, survey.Questions, n)
		// Grow named respondents independently of question rows. They are real
		// staff memberships selected by the production audience capability.
		for i := 0; i < n; i++ {
			_, account := testpkg.CreateTestStaffWithAccount(t, db, "Respondent", fmt.Sprintf("N%d-%02d", n, i))
			testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
			answers := make([]communication.SurveyAnswerInput, 0, n)
			for _, q := range survey.Questions {
				answers = append(answers, communication.SurveyAnswerInput{QuestionID: q.ID, OptionIDs: []int64{q.Options[0].ID}})
			}
			require.NoError(t, testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context) error { return surveys.SubmitStaffSurvey(ctx, account.ID, survey.ID, answers) }))
		}
		for _, suffix := range []string{"", "/form", "/respondents", "/results/export"} {
			req := httptest.NewRequest("GET", fmt.Sprintf("/%d%s", survey.ID, suffix), nil).WithContext(counter.Context(testpkg.Ctx(t)))
			req.Header.Set("Authorization", "Bearer "+token)
			counter.Reset()
			counter.Start()
			rec := testutil.ExecuteRequestForTest(t, testpkg.MountedQueryBudgetRouter("/api/surveys", router), req)
			counter.Stop()
			require.Equal(t, 200, rec.Code, rec.Body.String())
			name := map[string]string{"": "get", "/form": "form", "/respondents": "respondents", "/results/export": "export"}[suffix]
			t.Logf("MEASURE survey %s N%d=%d", name, n, counter.Total())
			switch suffix {
			case "/results/export":
				require.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
				require.True(t, strings.HasPrefix(rec.Body.String(), "%PDF-"))
				require.Greater(t, rec.Body.Len(), 1000)
			case "/respondents":
				var out struct {
					Data []struct {
						Answers []struct {
							QuestionID string   `json:"question_id"`
							Options    []string `json:"options"`
						} `json:"answers"`
						Name string `json:"name"`
					}
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
				require.Len(t, out.Data, n)
				for i, row := range out.Data {
					require.Equal(t, fmt.Sprintf("Respondent N%d-%02d", n, i), row.Name)
					require.Len(t, row.Answers, n)
					for j, answer := range row.Answers {
						require.Equal(t, fmt.Sprint(survey.Questions[j].ID), answer.QuestionID)
						require.Equal(t, []string{"Yes"}, answer.Options)
					}
				}
			default:
				var out struct{ Data json.RawMessage }
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
				require.Contains(t, string(out.Data), input.Title)
				var payload struct {
					Questions []struct {
						ID      string            `json:"id"`
						Prompt  string            `json:"prompt"`
						Options []json.RawMessage `json:"options"`
					}
				}
				if suffix == "/form" {
					var form struct{ Survey json.RawMessage }
					require.NoError(t, json.Unmarshal(out.Data, &form))
					out.Data = form.Survey
				}
				require.NoError(t, json.Unmarshal(out.Data, &payload))
				require.Len(t, payload.Questions, n)
				for i, q := range payload.Questions {
					require.Equal(t, fmt.Sprintf("Question-%02d", i), q.Prompt)
					require.Len(t, q.Options, 2)
					require.NotEmpty(t, q.ID)
				}
			}
			// The measured budget must hold at both positive cardinalities.
			switch name {
			case "get":
				querybudgets.AssertQueryBudget(t, "api.surveys.get.positive_growth", counter.Measurement(counter.Queries()))
			case "form":
				querybudgets.AssertQueryBudget(t, "api.surveys.form.positive_growth", counter.Measurement(counter.Queries()))
			case "respondents":
				querybudgets.AssertQueryBudget(t, "api.surveys.respondents.positive_growth", counter.Measurement(counter.Queries()))
			case "export":
				querybudgets.AssertQueryBudget(t, "api.surveys.export.positive_growth", counter.Measurement(counter.Queries()))
			}
		}
		lastSurvey = survey
	}
	// A real foreign-school caller cannot load this school's survey or form.
	foreignTenant, _ := testpkg.CreateTestTenant(t, db)
	_, foreign := testpkg.CreateTestStaffWithAccountForTenant(t, db, foreignTenant, "Foreign", "Viewer")
	testpkg.EnsureAccountTenant(t, db, foreign.ID, foreignTenant)
	foreignToken := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(foreign.ID), Sub: foreign.Email, TenantID: foreignTenant, Roles: []string{"user"}, Permissions: []string{"surveys:manage"}})
	for _, suffix := range []string{"", "/form", "/respondents", "/results/export"} {
		req := httptest.NewRequest("GET", fmt.Sprintf("/%d%s", lastSurvey.ID, suffix), nil)
		req.Header.Set("Authorization", "Bearer "+foreignToken)
		rec := testutil.ExecuteRequestForTest(t, testpkg.MountedQueryBudgetRouter("/api/surveys", router), req)
		require.Equal(t, 404, rec.Code, rec.Body.String())
	}
}
