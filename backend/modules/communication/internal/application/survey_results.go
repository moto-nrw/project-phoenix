package application

import (
	"math/rand/v2"
	"sort"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// maxSurveyHighlights is how many scale questions the overall strengths and
// improvement lists name.
const maxSurveyHighlights = 3

// buildSurveyResults turns the counted answers into the evaluation. An
// anonymous survey is evaluated once, after its end, and only from the
// minimum on, overall and per question: two views of a running survey differ
// by exactly the answers given in between, and with two or three answers a
// distribution already reads as single persons. Until then it reports its
// count and nothing else. The free texts are shuffled once more with shuffle,
// so their order follows neither the insertion nor the database.
func buildSurveyResults(survey *communication.Survey, tally *communication.SurveyTally, now time.Time, shuffle func([]string)) *communication.SurveyResults {
	results := &communication.SurveyResults{
		SurveyID: survey.ID, Title: survey.Title, Anonymous: survey.Anonymous, Status: survey.Status(now),
		ResponseCount: tally.ResponseCount, MinResponses: communication.SurveyMinAnonymousResponses,
		Questions: []communication.SurveyQuestionResult{}, Strengths: []communication.SurveyHighlight{}, Improvements: []communication.SurveyHighlight{},
	}
	if !survey.Anonymous {
		results.MinResponses = 0
	}
	results.Available = tally.ResponseCount >= results.MinResponses &&
		(!survey.Anonymous || results.Status == communication.SurveyStatusClosed)
	if !results.Available {
		return results
	}
	for _, q := range survey.Questions {
		if tally.AnswerCounts[q.ID] < results.MinResponses {
			results.Questions = append(results.Questions, withheldQuestion(q, tally))
			continue
		}
		results.Questions = append(results.Questions, questionResult(q, tally, shuffle))
	}
	results.Strengths, results.Improvements = surveyHighlights(results.Questions)
	return results
}

// withheldQuestion is a question of an anonymous survey answered by fewer
// people than the minimum: its count, without a value or a text.
func withheldQuestion(q communication.SurveyQuestion, tally *communication.SurveyTally) communication.SurveyQuestionResult {
	result := emptyQuestionResult(q, tally)
	result.Withheld = true
	return result
}

func emptyQuestionResult(q communication.SurveyQuestion, tally *communication.SurveyTally) communication.SurveyQuestionResult {
	return communication.SurveyQuestionResult{
		QuestionID: q.ID, Position: q.Position, Kind: q.Kind, Prompt: q.Prompt, AnswerCount: tally.AnswerCounts[q.ID],
		Options: []communication.SurveyOptionCount{}, ScaleCounts: []int{}, Texts: []string{}, Highest: []string{}, Lowest: []string{},
	}
}

func questionResult(q communication.SurveyQuestion, tally *communication.SurveyTally, shuffle func([]string)) communication.SurveyQuestionResult {
	result := emptyQuestionResult(q, tally)
	switch q.Kind {
	case communication.SurveyQuestionSingle, communication.SurveyQuestionMultiple:
		labels := make([]string, 0, len(q.Options))
		counts := make([]int, 0, len(q.Options))
		for _, o := range q.Options {
			count := tally.OptionCounts[o.ID]
			result.Options = append(result.Options, communication.SurveyOptionCount{OptionID: o.ID, Label: o.Label, Count: count})
			labels = append(labels, o.Label)
			counts = append(counts, count)
		}
		result.Highest, result.Lowest = extremes(labels, counts, result.AnswerCount)
	case communication.SurveyQuestionScale:
		result.ScaleCounts = scaleCounts(tally.ScaleCounts[q.ID])
		result.Average = scaleAverage(result.ScaleCounts)
		result.Highest, result.Lowest = scaleRange(result.ScaleCounts)
	case communication.SurveyQuestionText:
		texts := append([]string(nil), tally.Texts[q.ID]...)
		shuffle(texts)
		if texts == nil {
			texts = []string{}
		}
		result.Texts = texts
	}
	return result
}

// scaleRange names the highest and the lowest value anyone gave. One value
// given by everyone has no separate lowest.
func scaleRange(counts []int) ([]string, []string) {
	lowest, highest := 0, 0
	for i, count := range counts {
		if count == 0 {
			continue
		}
		if lowest == 0 {
			lowest = i + 1
		}
		highest = i + 1
	}
	if highest == 0 {
		return []string{}, []string{}
	}
	if lowest == highest {
		return []string{strconv.Itoa(highest)}, []string{}
	}
	return []string{strconv.Itoa(highest)}, []string{strconv.Itoa(lowest)}
}

func scaleCounts(counts []int) []int {
	out := make([]int, communication.SurveyScaleMax)
	copy(out, counts)
	return out
}

func scaleAverage(counts []int) *float64 {
	total, sum := 0, 0
	for i, count := range counts {
		total += count
		sum += (i + 1) * count
	}
	if total == 0 {
		return nil
	}
	average := float64(sum) / float64(total)
	return &average
}

// extremes names the most and least frequent entries. Without answers there
// is neither; when every entry is equally frequent there is no lowest.
func extremes(labels []string, counts []int, answered int) ([]string, []string) {
	highest, lowest := []string{}, []string{}
	if answered == 0 || len(counts) == 0 {
		return highest, lowest
	}
	maxCount, minCount := counts[0], counts[0]
	for _, c := range counts {
		maxCount = max(maxCount, c)
		minCount = min(minCount, c)
	}
	for i, c := range counts {
		if c == maxCount {
			highest = append(highest, labels[i])
		}
		if c == minCount && minCount != maxCount {
			lowest = append(lowest, labels[i])
		}
	}
	return highest, lowest
}

// surveyHighlights ranks the answered scale questions by their average: the
// best are the strengths, the worst the improvement areas. With a single
// scale question there is nothing to compare, and a question never appears in
// both lists.
func surveyHighlights(questions []communication.SurveyQuestionResult) ([]communication.SurveyHighlight, []communication.SurveyHighlight) {
	rated := make([]communication.SurveyHighlight, 0, len(questions))
	for _, q := range questions {
		if q.Kind == communication.SurveyQuestionScale && q.Average != nil {
			rated = append(rated, communication.SurveyHighlight{QuestionID: q.QuestionID, Prompt: q.Prompt, Average: *q.Average})
		}
	}
	strengths, improvements := []communication.SurveyHighlight{}, []communication.SurveyHighlight{}
	if len(rated) < 2 {
		return strengths, improvements
	}
	sort.SliceStable(rated, func(i, j int) bool { return rated[i].Average > rated[j].Average })
	n := min(maxSurveyHighlights, len(rated)/2)
	strengths = append(strengths, rated[:n]...)
	for i := len(rated) - 1; i >= len(rated)-n; i-- {
		improvements = append(improvements, rated[i])
	}
	return strengths, improvements
}

// shuffleTexts is the production shuffle of the free texts.
func shuffleTexts(texts []string) {
	rand.Shuffle(len(texts), func(i, j int) { texts[i], texts[j] = texts[j], texts[i] })
}
