package announcement

import (
	"strings"
	"testing"
	"time"

	announcementService "github.com/moto-nrw/project-phoenix/modules/communication"
)

// The PDF holds back what the screen holds back (#4781): a running anonymous
// survey says when its evaluation comes, a withheld question only its count.
func TestSurveyReportHoldsBackLikeTheScreen(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

	running := surveyReport(&announcementService.SurveyResults{
		Title: "Team", Anonymous: true, Status: announcementService.SurveyStatusOpen, ResponseCount: 9, MinResponses: 5,
	}, now)
	if got := running.Sections[0].Cards[0].Title; !strings.Contains(got, "nach dem Ende der Umfrage") {
		t.Fatalf("running anonymous survey says %q", got)
	}

	average := 1.0
	closed := surveyReport(&announcementService.SurveyResults{
		Title: "Team", Anonymous: true, Status: announcementService.SurveyStatusClosed, ResponseCount: 6, MinResponses: 5, Available: true,
		Questions: []announcementService.SurveyQuestionResult{{
			Kind: announcementService.SurveyQuestionScale, Prompt: "Pausen", AnswerCount: 1, Withheld: true,
		}, {
			Kind: announcementService.SurveyQuestionScale, Prompt: "Absprache", AnswerCount: 6,
			ScaleCounts: []int{6, 0, 0, 0, 0}, Average: &average, Highest: []string{"1"},
		}},
	}, now)
	cards := closed.Sections[len(closed.Sections)-1].Cards
	withheld := cards[0]
	if len(withheld.Fields) != 2 || withheld.Fields[1].Label != "Auswertung" {
		t.Fatalf("withheld question card = %+v", withheld)
	}
	if len(cards[1].Fields) < 7 {
		t.Fatalf("a shown question lists its distribution, got %+v", cards[1])
	}
}
