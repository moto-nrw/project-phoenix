package application

import (
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
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

func TestSurveyResultsHideEverythingBelowTheMinimum(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
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

func TestSurveyResultsShuffleTextsAndRankHighlights(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	reverse := func(texts []string) {
		for i, j := 0, len(texts)-1; i < j; i, j = i+1, j-1 {
			texts[i], texts[j] = texts[j], texts[i]
		}
	}
	results := buildSurveyResults(evaluatedSurvey(true), evaluatedTally(6), now, reverse)
	if !results.Available || len(results.Questions) != 4 {
		t.Fatalf("results = %+v", results)
	}
	if got := results.Questions[3].Texts; len(got) != 3 || got[0] != "dritter" {
		t.Fatalf("texts must pass through the shuffle, got %v", got)
	}
	if got := results.Questions[0]; got.Highest[0] != "Ja" || got.Lowest[0] != "Nein" {
		t.Fatalf("choice extremes = %v / %v", got.Highest, got.Lowest)
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
