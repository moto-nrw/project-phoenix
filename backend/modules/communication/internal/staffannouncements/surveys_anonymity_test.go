package announcement_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	announcement "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
	records "github.com/moto-nrw/project-phoenix/modules/communication/internal/staffannouncements"
	configModel "github.com/moto-nrw/project-phoenix/modules/settings"
	deliveryPolicy "github.com/moto-nrw/project-phoenix/roots/communicationtest"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// Surveys (#4781). Protection goal of an anonymous survey: the school cannot
// attribute an answer to a person through the app, its exports or its logs.
// These tests check the storage the goal rests on, the evaluation the school
// sees and the log lines the service writes.

type surveyStaffStub []*communication.AddressableStaffRecord

func (s surveyStaffStub) ListAddressableStaff(context.Context, int64) ([]*communication.AddressableStaffRecord, error) {
	return s, nil
}

type noAttachments struct{}

func (noAttachments) QueueAttachmentCleanupForAnnouncement(context.Context, int64) error { return nil }
func (noAttachments) CountAttachments(context.Context, int64) (int, error)               { return 0, nil }
func (noAttachments) AttachmentDigests(context.Context, int64, func(io.Reader) (string, int64, error)) ([]announcement.AttachmentDigest, error) {
	return nil, nil
}

type surveyFixture struct {
	surveys       communication.SurveyCapability
	announcements communication.ParentAnnouncementCapability
	db            *bun.DB
	logs          *bytes.Buffer
	staff         surveyStaffStub
}

func newSurveyFixture(t *testing.T, staffCount int) *surveyFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	// The school projection is the production wrapper; the survey store has
	// to come through it.
	organizations, err := owners.NewOrganizationTenancy(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	repos := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, db)).WithSchoolProjections(organizations)
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	staff := make(surveyStaffStub, 0, staffCount)
	for i := range staffCount {
		account := testpkg.CreateTestAccount(t, db, "survey-staff-"+strconv.Itoa(i))
		staff = append(staff, &communication.AddressableStaffRecord{AccountID: account.ID, Name: "Kollegin " + strconv.Itoa(i), RoleKind: "staff"})
	}
	svc := announcement.NewService(announcement.ServiceConfig{
		Repo:        records.NewRepository(repos.ParentAnnouncement),
		Settings:    configModel.NewParentAnnouncementValues(noticeSettings()),
		ParentsURL:  "https://parents.example.test",
		Attachments: noAttachments{},
		Logger:      logger,
	}, testpkg.TenantTransactionRuntime{}, deliveryPolicy.NewParentDeliveryPolicy())
	withSurveys, err := announcement.WithSurveys(announcement.NewParentAnnouncementFacade(svc), repos.ParentAnnouncement, staff)
	require.NoError(t, err)
	surveys, err := communication.SurveysOf(withSurveys)
	require.NoError(t, err)
	return &surveyFixture{surveys: surveys, announcements: withSurveys, db: db, logs: logs, staff: staff}
}

func (f *surveyFixture) inSchool(t *testing.T, fn func(ctx context.Context) error) error {
	t.Helper()
	ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), f.db)
	return tenant.WithTenantTx(ctx, f.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

func staffSurveyInput(anonymous bool) communication.SurveyInput {
	return communication.SurveyInput{
		Title: "Wie geht es im Team?", Intro: "Bitte nehmen Sie sich fünf Minuten.", Audience: communication.SurveyAudienceStaff,
		Anonymous: anonymous, ClosesAt: time.Now().Add(72 * time.Hour),
		Questions: []communication.SurveyQuestionInput{
			{Kind: communication.SurveyQuestionSingle, Prompt: "Arbeiten Sie gern hier?", Required: true, Options: []string{"Ja", "Nein"}},
			{Kind: communication.SurveyQuestionMultiple, Prompt: "Was hilft Ihnen?", Required: true, Options: []string{"Team", "Räume", "Material"}},
			{Kind: communication.SurveyQuestionScale, Prompt: "Wie gut ist die Absprache?", Required: true},
			{Kind: communication.SurveyQuestionScale, Prompt: "Wie gut sind die Pausen?", Required: true},
			{Kind: communication.SurveyQuestionText, Prompt: "Was wünschen Sie sich?", Required: false},
		},
	}
}

// publishStaffSurvey creates and publishes a staff survey.
func (f *surveyFixture) publishStaffSurvey(t *testing.T, anonymous bool) *communication.Survey {
	t.Helper()
	var survey *communication.Survey
	require.NoError(t, f.inSchool(t, func(ctx context.Context) error {
		draft, err := f.surveys.CreateSurvey(ctx, f.staff[0].AccountID, staffSurveyInput(anonymous))
		if err != nil {
			return err
		}
		survey, err = f.surveys.PublishSurvey(ctx, f.staff[0].AccountID, draft.ID)
		return err
	}))
	return survey
}

// answersFor fills every question; i varies the values so the tally differs
// per person.
func answersFor(survey *communication.Survey, i int) []communication.SurveyAnswerInput {
	q := survey.Questions
	scale := 1 + i%5
	other := 5 - i%5
	text := "Mehr Zeit für Absprachen, Nummer " + strconv.Itoa(i)
	return []communication.SurveyAnswerInput{
		{QuestionID: q[0].ID, OptionIDs: []int64{q[0].Options[i%2].ID}},
		{QuestionID: q[1].ID, OptionIDs: []int64{q[1].Options[0].ID, q[1].Options[1+i%2].ID}},
		{QuestionID: q[2].ID, ScaleValue: &scale},
		{QuestionID: q[3].ID, ScaleValue: &other},
		{QuestionID: q[4].ID, Text: &text},
	}
}

func (f *surveyFixture) answerAll(t *testing.T, survey *communication.Survey, count int) {
	t.Helper()
	for i := range count {
		require.NoError(t, f.inSchool(t, func(ctx context.Context) error {
			return f.surveys.SubmitStaffSurvey(ctx, f.staff[i].AccountID, survey.ID, answersFor(survey, i))
		}))
	}
}

func TestAnonymousSurvey_StorageKeepsParticipationAndAnswersApart(t *testing.T) {
	t.Parallel()
	f := newSurveyFixture(t, 6)
	survey := f.publishStaffSurvey(t, true)
	f.answerAll(t, survey, 6)
	ctx := testpkg.Ctx(t)

	// Participation names the accounts, the answers name nobody.
	var participants int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM users.survey_participations WHERE survey_id = ?`, survey.ID).Scan(ctx, &participants))
	assert.Equal(t, 6, participants)
	var responses []struct {
		ID        string `bun:"id"`
		AccountID *int64 `bun:"account_id"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT id::text AS id, account_id FROM users.survey_responses WHERE survey_id = ?`, survey.ID).Scan(ctx, &responses))
	require.Len(t, responses, 6)
	for _, r := range responses {
		assert.Nil(t, r.AccountID, "an anonymous response names no account")
		require.Len(t, r.ID, 36)
		assert.Equal(t, byte('4'), r.ID[14], "response ids are random UUIDs (version 4), not a sequence")
	}

	// The tables carry no column that could pair a participation with a
	// response: no time, no serial, no shared key.
	var columns []struct {
		Table  string `bun:"table_name"`
		Column string `bun:"column_name"`
		Type   string `bun:"data_type"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT table_name, column_name, data_type FROM information_schema.columns
		WHERE table_schema = 'users' AND table_name IN ('survey_participations', 'survey_responses', 'survey_answers')`).Scan(ctx, &columns))
	byTable := map[string][]string{}
	for _, c := range columns {
		assert.NotContains(t, c.Type, "timestamp", "%s.%s carries a time", c.Table, c.Column)
		assert.NotEqual(t, "date", c.Type, "%s.%s carries a date", c.Table, c.Column)
		byTable[c.Table] = append(byTable[c.Table], c.Column)
	}
	assert.ElementsMatch(t, []string{"tenant_id", "survey_id", "account_id"}, byTable["survey_participations"], "a participation has no id of its own")
	assert.ElementsMatch(t, []string{"id", "tenant_id", "survey_id", "account_id"}, byTable["survey_responses"])
	assert.ElementsMatch(t, []string{"id", "tenant_id", "response_id", "question_id", "option_id", "scale_value", "text_value"}, byTable["survey_answers"])

	// The database refuses an account on an anonymous response, whatever
	// the application sends.
	_, err := f.db.ExecContext(ctx, `INSERT INTO users.survey_responses (tenant_id, survey_id, account_id) VALUES (?, ?, ?)`,
		testpkg.Tenant(t), survey.ID, f.staff[0].AccountID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "anonymous survey stores no account")

	// A published survey keeps its anonymity.
	_, err = f.db.ExecContext(ctx, `UPDATE users.surveys SET anonymous = FALSE WHERE id = ?`, survey.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "keeps its anonymity")
}

func TestAnonymousSurvey_OneAnswerPerAccountAndNoNamedView(t *testing.T) {
	t.Parallel()
	f := newSurveyFixture(t, 1)
	survey := f.publishStaffSurvey(t, true)
	f.answerAll(t, survey, 1)

	err := f.inSchool(t, func(ctx context.Context) error {
		return f.surveys.SubmitStaffSurvey(ctx, f.staff[0].AccountID, survey.ID, answersFor(survey, 0))
	})
	require.ErrorIs(t, err, communication.ErrSurveyAlreadyAnswered)
	var responses int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM users.survey_responses WHERE survey_id = ?`, survey.ID).Scan(testpkg.Ctx(t), &responses))
	assert.Equal(t, 1, responses, "a refused second answer writes nothing")

	err = f.inSchool(t, func(ctx context.Context) error {
		_, err := f.surveys.SurveyRespondents(ctx, survey.ID)
		return err
	})
	require.ErrorIs(t, err, communication.ErrSurveyAnonymous, "no answers per person, whatever the caller may hold")

	// The form says the account took part; it never shows the answers.
	require.NoError(t, f.inSchool(t, func(ctx context.Context) error {
		form, err := f.surveys.StaffSurveyForm(ctx, f.staff[0].AccountID, survey.ID)
		require.NoError(t, err)
		assert.True(t, form.Participated)
		return nil
	}))
}

func TestAnonymousSurvey_EvaluationNeedsFiveAnswersAndNamesNobody(t *testing.T) {
	t.Parallel()
	f := newSurveyFixture(t, 6)
	survey := f.publishStaffSurvey(t, true)
	f.answerAll(t, survey, 4)

	results := f.results(t, survey.ID)
	assert.False(t, results.Available, "four answers are not enough")
	assert.Equal(t, 4, results.ResponseCount)
	assert.Empty(t, results.Questions, "below the minimum nothing but the count")
	assert.Empty(t, results.Strengths)

	f.answerAll2(t, survey, 4, 6)
	results = f.results(t, survey.ID)
	assert.False(t, results.Available, "a running anonymous survey waits for its end")
	assert.Equal(t, 6, results.ResponseCount)
	assert.Empty(t, results.Questions, "two views of a running survey would differ by the answers in between")

	require.NoError(t, f.inSchool(t, func(ctx context.Context) error {
		_, err := f.surveys.CloseSurvey(ctx, survey.ID)
		return err
	}))
	results = f.results(t, survey.ID)
	require.True(t, results.Available)
	require.Len(t, results.Questions, 5)
	assert.Equal(t, 6, results.Questions[0].AnswerCount)
	assert.Equal(t, []int{2, 1, 1, 1, 1}, results.Questions[2].ScaleCounts)
	assert.Len(t, results.Questions[4].Texts, 6)
	assert.Len(t, results.Strengths, 1)
	assert.Len(t, results.Improvements, 1)

	// Nothing in the evaluation identifies a person: no account field, no
	// name. (Small account ids would collide with counts, so the field names
	// are what is checked.)
	raw, err := json.Marshal(results)
	require.NoError(t, err)
	assert.NotContains(t, strings.ToLower(string(raw)), "account")
	for _, member := range f.staff {
		assert.NotContains(t, string(raw), member.Name)
	}
}

func TestAnonymousSurvey_LogsNameNeitherAccountNorAnswer(t *testing.T) {
	t.Parallel()
	f := newSurveyFixture(t, 2)
	survey := f.publishStaffSurvey(t, true)
	f.logs.Reset()
	f.answerAll(t, survey, 2)

	logged := f.logs.String()
	assert.Contains(t, logged, "survey answer recorded", "the submission is logged")
	for _, line := range strings.Split(strings.TrimSpace(logged), "\n") {
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record))
		for key, value := range record {
			if key == "time" || key == "survey_id" {
				continue
			}
			for i, member := range f.staff {
				assert.NotEqual(t, float64(member.AccountID), value, "no account in the log (%s)", key)
				if text, ok := value.(string); ok {
					assert.NotContains(t, text, "Nummer "+strconv.Itoa(i), "no answer in the log (%s)", key)
				}
			}
		}
		assert.Equal(t, float64(survey.ID), record["survey_id"])
		assert.NotContains(t, record, "response_id")
		assert.NotContains(t, record, "account_id")
	}
}

func TestNamedSurvey_ShowsAnswersPerPerson(t *testing.T) {
	t.Parallel()
	f := newSurveyFixture(t, 2)
	survey := f.publishStaffSurvey(t, false)
	f.answerAll(t, survey, 2)

	results := f.results(t, survey.ID)
	assert.True(t, results.Available, "a named survey has no minimum")
	var respondents []communication.SurveyRespondent
	require.NoError(t, f.inSchool(t, func(ctx context.Context) error {
		var err error
		respondents, err = f.surveys.SurveyRespondents(ctx, survey.ID)
		return err
	}))
	require.Len(t, respondents, 2)
	assert.Equal(t, "Kollegin 0", respondents[0].Name)
	assert.Len(t, respondents[0].Answers, 5)
}

func TestSurvey_PublishedSurveyIsFrozenAndClosedSurveyTakesNoAnswers(t *testing.T) {
	t.Parallel()
	f := newSurveyFixture(t, 2)
	survey := f.publishStaffSurvey(t, true)

	err := f.inSchool(t, func(ctx context.Context) error {
		_, err := f.surveys.UpdateSurvey(ctx, survey.ID, staffSurveyInput(false))
		return err
	})
	require.ErrorIs(t, err, communication.ErrSurveyNotDraft)

	require.NoError(t, f.inSchool(t, func(ctx context.Context) error {
		_, err := f.surveys.CloseSurvey(ctx, survey.ID)
		return err
	}))
	err = f.inSchool(t, func(ctx context.Context) error {
		return f.surveys.SubmitStaffSurvey(ctx, f.staff[1].AccountID, survey.ID, answersFor(survey, 1))
	})
	require.ErrorIs(t, err, communication.ErrSurveyNotOpen)
}

func TestSurvey_ClosingWhileSubmissionWaitsRejectsTheSubmission(t *testing.T) {
	t.Parallel()
	f := newSurveyFixture(t, 1)
	survey := f.publishStaffSurvey(t, true)

	closing := make(chan int, 1)
	commitClose := make(chan struct{})
	closed := make(chan error, 1)
	go func() {
		closed <- testpkg.WithTenantTx(t, testpkg.Ctx(t), f.db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
			var pid int
			if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &pid); err != nil {
				return err
			}
			if _, err := f.surveys.CloseSurvey(ctx, survey.ID); err != nil {
				return err
			}
			closing <- pid
			<-commitClose
			return nil
		})
	}()
	closePID := <-closing

	submitted := make(chan error, 1)
	go func() {
		submitted <- f.inSchool(t, func(ctx context.Context) error {
			return f.surveys.SubmitStaffSurvey(ctx, f.staff[0].AccountID, survey.ID, answersFor(survey, 0))
		})
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := f.db.NewRaw(`SELECT EXISTS (
			SELECT 1 FROM pg_locks l
			WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid))
		)`, closePID).Scan(testpkg.Ctx(t), &waiting)
		return err == nil && waiting
	}, 5*time.Second, 10*time.Millisecond, "submission must wait for the closing survey row lock")

	close(commitClose)
	require.NoError(t, <-closed)
	require.ErrorIs(t, <-submitted, communication.ErrSurveyNotOpen)

	var responses int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM users.survey_responses WHERE survey_id = ?`, survey.ID).Scan(testpkg.Ctx(t), &responses))
	assert.Zero(t, responses, "a submission released after closing commits writes no response")
}

func TestSurvey_StaffOutsideTheAudienceCannotAnswer(t *testing.T) {
	t.Parallel()
	f := newSurveyFixture(t, 1)
	survey := f.publishStaffSurvey(t, true)
	outsider := testpkg.CreateTestAccount(t, f.db, "survey-outsider")
	err := f.inSchool(t, func(ctx context.Context) error {
		return f.surveys.SubmitStaffSurvey(ctx, outsider.ID, survey.ID, answersFor(survey, 0))
	})
	require.ErrorIs(t, err, communication.ErrSurveyNotFound)
	require.NoError(t, f.inSchool(t, func(ctx context.Context) error {
		open, err := f.surveys.StaffSurveysToAnswer(ctx, outsider.ID)
		assert.Empty(t, open)
		return err
	}))
}

func TestParentSurvey_OneAnswerPerGuardianAccountAndCarrierStaysOutOfAnnouncements(t *testing.T) {
	t.Parallel()
	f := newSurveyFixture(t, 1)
	chain := testpkg.CreateTestParentGuardianChain(t, f.db)
	in := staffSurveyInput(true)
	in.Audience = communication.SurveyAudienceParents
	in.Targets = []communication.SurveyTarget{{TargetType: "school_all"}}

	var survey *communication.Survey
	require.NoError(t, f.inSchool(t, func(ctx context.Context) error {
		draft, err := f.surveys.CreateSurvey(ctx, f.staff[0].AccountID, in)
		if err != nil {
			return err
		}
		survey, err = f.surveys.PublishSurvey(ctx, chain.AccountID, draft.ID)
		return err
	}))
	require.NotNil(t, survey.AnnouncementID, "a parent survey is carried by an announcement")
	var kind string
	require.NoError(t, f.db.NewRaw(`SELECT system_kind FROM users.parent_announcements WHERE id = ?`, *survey.AnnouncementID).Scan(testpkg.Ctx(t), &kind))
	assert.Equal(t, "survey", kind)

	tenantID := testpkg.Tenant(t)
	require.NoError(t, f.inSchool(t, func(ctx context.Context) error {
		summaries, err := f.surveys.PortalSurveySummaries(ctx, chain.AccountID, []int64{*survey.AnnouncementID})
		require.NoError(t, err)
		require.Contains(t, summaries, *survey.AnnouncementID)
		assert.True(t, summaries[*survey.AnnouncementID].Anonymous)
		assert.False(t, summaries[*survey.AnnouncementID].Participated)
		return f.surveys.SubmitPortalSurvey(ctx, tenantID, *survey.AnnouncementID, chain.AccountID, answersFor(survey, 0))
	}))
	err := f.inSchool(t, func(ctx context.Context) error {
		return f.surveys.SubmitPortalSurvey(ctx, tenantID, *survey.AnnouncementID, chain.AccountID, answersFor(survey, 1))
	})
	require.ErrorIs(t, err, communication.ErrSurveyAlreadyAnswered, "one answer per guardian account, not per child")

	var closed *communication.Survey
	require.NoError(t, f.inSchool(t, func(ctx context.Context) error {
		closed, err = f.surveys.CloseSurvey(ctx, survey.ID)
		return err
	}))
	var carrierExpiresAt time.Time
	require.NoError(t, f.db.NewRaw(`SELECT expires_at FROM users.parent_announcements WHERE id = ?`, *survey.AnnouncementID).Scan(testpkg.Ctx(t), &carrierExpiresAt))
	assert.Equal(t, closed.ClosesAt, carrierExpiresAt, "closing a parent survey expires its feed carrier")
	assert.True(t, carrierExpiresAt.Before(survey.ClosesAt), "the carrier no longer stays visible until the original deadline")

	// The carrier is none of the announcement routes: its read times per
	// guardian would line up with the anonymous answers.
	require.NoError(t, f.inSchool(t, func(ctx context.Context) error {
		_, err := f.announcements.GetParentAnnouncement(ctx, *survey.AnnouncementID)
		assert.ErrorIs(t, err, communication.ErrParentAnnouncementNotFound, "get")
		_, err = f.announcements.ParentAnnouncementStats(ctx, *survey.AnnouncementID)
		assert.ErrorIs(t, err, communication.ErrParentAnnouncementNotFound, "stats")
		_, err = f.announcements.ParentAnnouncementRecipients(ctx, *survey.AnnouncementID)
		assert.ErrorIs(t, err, communication.ErrParentAnnouncementNotFound, "recipients")
		return nil
	}))

	// Deleting the survey removes its carrier with it.
	require.NoError(t, f.inSchool(t, func(ctx context.Context) error {
		return f.surveys.DeleteSurvey(ctx, survey.ID)
	}))
	var carriers int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM users.parent_announcements WHERE id = ?`, *survey.AnnouncementID).Scan(testpkg.Ctx(t), &carriers))
	assert.Zero(t, carriers)
}

func TestSurveyRetention_DeletesClosedSurveysAfterTheWindowOnly(t *testing.T) {
	t.Parallel()
	f := newSurveyFixture(t, 1)
	old := f.publishStaffSurvey(t, true)
	open := f.publishStaffSurvey(t, true)
	ctx := testpkg.Ctx(t)
	_, err := f.db.ExecContext(ctx, `UPDATE users.surveys SET closes_at = now() - interval '40 days' WHERE id = ?`, old.ID)
	require.NoError(t, err)

	var deleted int
	require.NoError(t, f.inSchool(t, func(ctx context.Context) error {
		deleted, err = f.surveys.DeleteExpiredSurveys(ctx, communication.SurveyAudienceStaff, 30)
		return err
	}))
	assert.Equal(t, 1, deleted)
	var left []int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM users.surveys WHERE id IN (?, ?)`, old.ID, open.ID).Scan(ctx, &left))
	assert.Equal(t, []int64{open.ID}, left)
}

func (f *surveyFixture) results(t *testing.T, id int64) *communication.SurveyResults {
	t.Helper()
	var results *communication.SurveyResults
	require.NoError(t, f.inSchool(t, func(ctx context.Context) error {
		var err error
		results, err = f.surveys.SurveyResults(ctx, id)
		return err
	}))
	return results
}

func (f *surveyFixture) answerAll2(t *testing.T, survey *communication.Survey, from, to int) {
	t.Helper()
	for i := from; i < to; i++ {
		require.NoError(t, f.inSchool(t, func(ctx context.Context) error {
			return f.surveys.SubmitStaffSurvey(ctx, f.staff[i].AccountID, survey.ID, answersFor(survey, i))
		}))
	}
}

// TestSurveyListsQueryBudgetStaysFlat: the survey lists and the evaluation
// read in a fixed number of statements, however many surveys, questions or
// answers there are.
func TestSurveyListsQueryBudgetStaysFlat(t *testing.T) {
	t.Parallel()
	f := newSurveyFixture(t, 3)
	var surveys []*communication.Survey
	for range 3 {
		surveys = append(surveys, f.publishStaffSurvey(t, true))
	}
	f.answerAll(t, surveys[0], 3)
	ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), f.db)

	counter := testpkg.CaptureQueriesForContext(t, f.db)
	rows, err := testpkg.MeasuredTenantQuery0(t, counter.Context(ctx), f.db, counter, f.surveys.ListSurveys)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, 3, rows[2].ParticipantCount+rows[1].ParticipantCount+rows[0].ParticipantCount)
	querybudgets.AssertQueryBudget(t, "modules.communication.surveys.list", counter.Queries())

	open := testpkg.CaptureQueriesForContext(t, f.db)
	toAnswer, err := testpkg.MeasuredTenantQuery1(t, open.Context(ctx), f.db, open, f.surveys.StaffSurveysToAnswer, f.staff[1].AccountID)
	require.NoError(t, err)
	require.Len(t, toAnswer, 3)
	querybudgets.AssertQueryBudget(t, "modules.communication.surveys.to_answer", open.Queries())

	evaluation := testpkg.CaptureQueriesForContext(t, f.db)
	results, err := testpkg.MeasuredTenantQuery1(t, evaluation.Context(ctx), f.db, evaluation, f.surveys.SurveyResults, surveys[0].ID)
	require.NoError(t, err)
	require.Len(t, results.Questions, 0, "three answers stay below the minimum")
	querybudgets.AssertQueryBudget(t, "modules.communication.surveys.results", evaluation.Queries())
}
