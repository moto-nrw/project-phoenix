package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func scaleQuestion(id int64, prompt string) communication.SurveyQuestion {
	return communication.SurveyQuestion{ID: id, Position: int(id), Kind: communication.SurveyQuestionScale, Prompt: prompt, Required: true}
}

func evaluatedSurvey(anonymous bool) *communication.Survey {
	published := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	return &communication.Survey{
		ID: 7, Title: "Team", Anonymous: anonymous, PublishedAt: &published, ClosesAt: published.Add(240 * time.Hour),
		Questions: []communication.SurveyQuestion{
			{ID: 1, Kind: communication.SurveyQuestionSingle, Prompt: "Gern hier?", Options: []communication.SurveyOption{{ID: 11, Label: "Ja"}, {ID: 12, Label: "Nein"}}},
			scaleQuestion(2, "Absprache"),
			scaleQuestion(3, "Pausen"),
			{ID: 4, Kind: communication.SurveyQuestionText, Prompt: "Wünsche"},
		},
	}
}

func evaluatedTally(responses int) *communication.SurveyTally {
	return &communication.SurveyTally{
		ResponseCount: responses,
		AnswerCounts:  map[int64]int{1: responses, 2: responses, 3: responses, 4: 3},
		OptionCounts:  map[int64]int{11: responses - 1, 12: 1},
		ScaleCounts:   map[int64][]int{2: {0, 0, 0, 1, responses - 1}, 3: {responses - 1, 1, 0, 0, 0}},
		Texts:         map[int64][]string{4: {"erster", "zweiter", "dritter"}},
	}
}

// afterClose is a moment after evaluatedSurvey has ended.
var afterClose = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

func TestSurveyResultsHideEverythingBelowTheMinimum(t *testing.T) {
	t.Parallel()
	now := afterClose
	shuffled := false
	results := buildSurveyResults(evaluatedSurvey(true), evaluatedTally(4), now, func([]string) { shuffled = true })
	if results.Available || len(results.Questions) != 0 || len(results.Strengths) != 0 || shuffled {
		t.Fatalf("an anonymous survey with 4 answers shows %+v", results)
	}
	if results.ResponseCount != 4 || results.MinResponses != communication.SurveyMinAnonymousResponses {
		t.Fatalf("counts = %d/%d", results.ResponseCount, results.MinResponses)
	}
	named := buildSurveyResults(evaluatedSurvey(false), evaluatedTally(1), now, func([]string) {})
	if !named.Available || named.MinResponses != 0 {
		t.Fatal("a named survey has no minimum")
	}
}

// An anonymous survey is evaluated once, after its end: two views of a running
// survey would differ by exactly the answers given in between.
func TestAnonymousSurveyResultsWaitForTheEnd(t *testing.T) {
	t.Parallel()
	running := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	results := buildSurveyResults(evaluatedSurvey(true), evaluatedTally(9), running, func([]string) {})
	if results.Available || len(results.Questions) != 0 || len(results.Strengths) != 0 {
		t.Fatalf("a running anonymous survey shows %+v", results)
	}
	if results.ResponseCount != 9 || results.Status != communication.SurveyStatusOpen {
		t.Fatalf("count/status = %d/%s", results.ResponseCount, results.Status)
	}
	named := buildSurveyResults(evaluatedSurvey(false), evaluatedTally(9), running, func([]string) {})
	if !named.Available || len(named.Questions) != 4 {
		t.Fatal("a named survey is evaluated while it runs")
	}
}

// Below the minimum a single question of an anonymous survey shows its count
// only: one optional answer among six would read as one person.
func TestAnonymousSurveyWithholdsQuestionsBelowTheMinimum(t *testing.T) {
	t.Parallel()
	tally := evaluatedTally(6)
	tally.AnswerCounts[3] = 1
	tally.ScaleCounts[3] = []int{1, 0, 0, 0, 0}
	results := buildSurveyResults(evaluatedSurvey(true), tally, afterClose, func([]string) {})
	if !results.Available {
		t.Fatal("six answers are evaluated")
	}
	sparse := results.Questions[2]
	if !sparse.Withheld || sparse.AnswerCount != 1 || sparse.Average != nil || len(sparse.Highest)+len(sparse.Lowest) != 0 || sum(sparse.ScaleCounts) != 0 {
		t.Fatalf("a question with one answer shows %+v", sparse)
	}
	texts := results.Questions[3]
	if !texts.Withheld || len(texts.Texts) != 0 || texts.AnswerCount != 3 {
		t.Fatalf("three free texts show %+v", texts)
	}
	if results.Questions[1].Withheld {
		t.Fatal("a question with six answers is shown")
	}
	for _, h := range append(results.Strengths, results.Improvements...) {
		if h.QuestionID == sparse.QuestionID {
			t.Fatalf("a withheld question is ranked: %+v", h)
		}
	}
	named := buildSurveyResults(evaluatedSurvey(false), tally, afterClose, func([]string) {})
	if named.Questions[2].Withheld || named.Questions[2].Average == nil {
		t.Fatal("a named survey shows every question")
	}
}

func sum(values []int) int {
	total := 0
	for _, v := range values {
		total += v
	}
	return total
}

func TestSurveyResultsShuffleTextsAndRankHighlights(t *testing.T) {
	t.Parallel()
	now := afterClose
	reverse := func(texts []string) {
		for i, j := 0, len(texts)-1; i < j; i, j = i+1, j-1 {
			texts[i], texts[j] = texts[j], texts[i]
		}
	}
	tally := evaluatedTally(6)
	tally.AnswerCounts[4] = 5
	tally.Texts[4] = []string{"erster", "zweiter", "dritter", "vierter", "fünfter"}
	results := buildSurveyResults(evaluatedSurvey(true), tally, now, reverse)
	if !results.Available || len(results.Questions) != 4 {
		t.Fatalf("results = %+v", results)
	}
	if got := results.Questions[3].Texts; len(got) != 5 || got[0] != "fünfter" {
		t.Fatalf("texts must pass through the shuffle, got %v", got)
	}
	if got := results.Questions[0]; got.Highest[0] != "Ja" || got.Lowest[0] != "Nein" {
		t.Fatalf("choice extremes = %v / %v", got.Highest, got.Lowest)
	}
	if got := results.Questions[1]; len(got.Highest) != 1 || got.Highest[0] != "5" || got.Lowest[0] != "4" {
		t.Fatalf("scale range = %v / %v, want the highest and lowest value given", got.Highest, got.Lowest)
	}
	if len(results.Strengths) != 1 || results.Strengths[0].Prompt != "Absprache" {
		t.Fatalf("strengths = %+v", results.Strengths)
	}
	if len(results.Improvements) != 1 || results.Improvements[0].Prompt != "Pausen" {
		t.Fatalf("improvements = %+v", results.Improvements)
	}
}

func TestSurveyAnswersAreCheckedAgainstTheQuestions(t *testing.T) {
	t.Parallel()
	survey := evaluatedSurvey(true)
	four := 4
	zero := 0
	blank := "   "
	cases := []struct {
		name    string
		answers []communication.SurveyAnswerInput
		wantErr bool
		want    int
	}{
		{"complete", []communication.SurveyAnswerInput{{QuestionID: 1, OptionIDs: []int64{11}}, {QuestionID: 2, ScaleValue: &four}, {QuestionID: 3, ScaleValue: &four}}, false, 3},
		{"blank optional text is dropped", []communication.SurveyAnswerInput{{QuestionID: 1, OptionIDs: []int64{11}}, {QuestionID: 2, ScaleValue: &four}, {QuestionID: 3, ScaleValue: &four}, {QuestionID: 4, Text: &blank}}, false, 3},
		{"required scale missing", []communication.SurveyAnswerInput{{QuestionID: 1, OptionIDs: []int64{11}}, {QuestionID: 2, ScaleValue: &four}}, true, 0},
		{"two options on single choice", []communication.SurveyAnswerInput{{QuestionID: 1, OptionIDs: []int64{11, 12}}, {QuestionID: 2, ScaleValue: &four}, {QuestionID: 3, ScaleValue: &four}}, true, 0},
		{"option of another question", []communication.SurveyAnswerInput{{QuestionID: 1, OptionIDs: []int64{99}}, {QuestionID: 2, ScaleValue: &four}, {QuestionID: 3, ScaleValue: &four}}, true, 0},
		{"scale out of range", []communication.SurveyAnswerInput{{QuestionID: 1, OptionIDs: []int64{11}}, {QuestionID: 2, ScaleValue: &zero}, {QuestionID: 3, ScaleValue: &four}}, true, 0},
		{"unknown question", []communication.SurveyAnswerInput{{QuestionID: 42, OptionIDs: []int64{11}}}, true, 0},
		{"same question twice", []communication.SurveyAnswerInput{{QuestionID: 2, ScaleValue: &four}, {QuestionID: 2, ScaleValue: &four}}, true, 0},
	}
	survey.Questions[0].Required = true
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeSurveyAnswers(survey, tc.answers)
			if tc.wantErr {
				if !errors.Is(err, communication.ErrSurveyInvalid) {
					t.Fatalf("err = %v, want ErrSurveyInvalid", err)
				}
				return
			}
			if err != nil || len(got) != tc.want {
				t.Fatalf("got %d answers, err %v", len(got), err)
			}
		})
	}
}

func TestSurveyInputRules(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	valid := communication.SurveyInput{
		Title: " Team ", Audience: communication.SurveyAudienceStaff, Anonymous: true, SendEmail: true, ClosesAt: now.Add(time.Hour),
		Questions: []communication.SurveyQuestionInput{{Kind: communication.SurveyQuestionScale, Prompt: "Wie gut?"}},
	}
	got, err := normalizeSurveyInput(valid, now)
	if err != nil || got.Title != "Team" || got.SendEmail {
		t.Fatalf("staff survey = %+v, %v; a staff survey sends no parent e-mail", got, err)
	}
	broken := []func(*communication.SurveyInput){
		func(in *communication.SurveyInput) { in.ClosesAt = now.Add(-time.Minute) },
		func(in *communication.SurveyInput) { in.ClosesAt = now.Add(400 * 24 * time.Hour) },
		func(in *communication.SurveyInput) { in.Audience = "everyone" },
		func(in *communication.SurveyInput) { in.Audience = communication.SurveyAudienceParents },
		func(in *communication.SurveyInput) {
			in.Targets = []communication.SurveyTarget{{TargetType: "school_all"}}
		},
		func(in *communication.SurveyInput) { in.Questions = nil },
		func(in *communication.SurveyInput) {
			in.Questions = []communication.SurveyQuestionInput{{Kind: communication.SurveyQuestionSingle, Prompt: "Eine?", Options: []string{"Nur eine"}}}
		},
		func(in *communication.SurveyInput) {
			in.Questions = []communication.SurveyQuestionInput{{Kind: communication.SurveyQuestionSingle, Prompt: "Doppelt?", Options: []string{"Ja", "ja"}}}
		},
	}
	for i, breakIt := range broken {
		in := valid
		breakIt(&in)
		if _, err := normalizeSurveyInput(in, now); !errors.Is(err, communication.ErrSurveyInvalid) {
			t.Errorf("case %d: err = %v, want ErrSurveyInvalid", i, err)
		}
	}
}

func TestPublishSurveyCarrierSkipsEmailsWhenDisabled(t *testing.T) {
	t.Parallel()
	repo := &fakeAnnouncementRepo{recipients: []*StaffAnnouncementRecipient{{AccountID: 1, Email: "parent@example.test"}}}
	outbox := &fakeOutbox{}
	svc := newTestService(repo, outbox).(*service)
	closesAt := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	survey := &communication.Survey{
		Title: "Rückmeldung", Audience: communication.SurveyAudienceParents, SendEmail: false, ClosesAt: closesAt,
		Targets: []communication.SurveyTarget{{TargetType: "school_all"}},
	}

	if _, err := svc.publishSurveyCarrier(context.Background(), survey, 1, closesAt.Add(-time.Hour)); err != nil {
		t.Fatalf("publish survey carrier: %v", err)
	}
	if repo.announcement.SendEmail {
		t.Fatal("carrier must preserve the disabled e-mail option")
	}
	if len(outbox.requests) != 0 {
		t.Fatalf("disabled survey e-mail queued %d message(s)", len(outbox.requests))
	}
}

func TestSurveyRetentionCutoffCountsBerlinDays(t *testing.T) {
	t.Parallel()
	// The day after daylight saving time ended (25.10.2026): 30 days back
	// is still summer time, the cutoff is that day's Berlin midnight.
	now := time.Date(2026, 10, 26, 0, 30, 0, 0, calendar.Berlin)
	cutoff := surveyRetentionCutoff(now.UTC(), 30)
	if want := time.Date(2026, 9, 26, 0, 0, 0, 0, calendar.Berlin); !cutoff.Equal(want) {
		t.Fatalf("cutoff = %s, want %s", cutoff, want)
	}
	if got := cutoff.UTC().Format(time.RFC3339); got != "2026-09-25T22:00:00Z" {
		t.Fatalf("cutoff in UTC = %s, want 2026-09-25T22:00:00Z", got)
	}
}
