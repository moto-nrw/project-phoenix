package announcement

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	announcementService "github.com/moto-nrw/project-phoenix/modules/communication"
)

// The PDF of a survey evaluation (#4781): the same sums and distributions as
// the screen, nothing per person. Below the minimum of an anonymous survey
// the document says how many answers there are and nothing else.

const surveyReportFooter = "Auswertung aus moto. Anonyme Umfragen enthalten keine Zuordnung von Antworten zu Personen."

var surveyScaleLabels = []string{"1 (trifft gar nicht zu)", "2", "3", "4", "5 (trifft voll zu)"}

func surveyReport(r *announcementService.SurveyResults, now time.Time) ReportDocument {
	doc := ReportDocument{
		Title:       "Auswertung: " + r.Title,
		Subtitle:    surveyReportSubtitle(r),
		GeneratedAt: now,
		Footer:      surveyReportFooter,
		Sections:    []ReportSection{},
	}
	if !r.Available {
		doc.Sections = append(doc.Sections, ReportSection{
			Title: "Noch keine Auswertung",
			Cards: []ReportCard{{
				Title:  fmt.Sprintf("Die Auswertung erscheint ab %d Antworten.", r.MinResponses),
				Fields: []ReportField{{Label: "Antworten bisher", Value: strconv.Itoa(r.ResponseCount)}},
			}},
		})
		return doc
	}
	if len(r.Strengths) > 0 {
		doc.Sections = append(doc.Sections, ReportSection{Title: "Überblick", Cards: []ReportCard{
			{Title: "Stärken", Fields: highlightFields(r.Strengths)},
			{Title: "Verbesserungsbedarf", Fields: highlightFields(r.Improvements)},
		}})
	}
	cards := make([]ReportCard, 0, len(r.Questions))
	for _, q := range r.Questions {
		cards = append(cards, questionCard(q))
	}
	doc.Sections = append(doc.Sections, ReportSection{Title: "Fragen", Cards: cards})
	return doc
}

func surveyReportSubtitle(r *announcementService.SurveyResults) string {
	kind := "Nicht anonym"
	if r.Anonymous {
		kind = "Anonym"
	}
	return fmt.Sprintf("%s · %d Antworten", kind, r.ResponseCount)
}

func highlightFields(rows []announcementService.SurveyHighlight) []ReportField {
	out := make([]ReportField, 0, len(rows))
	for _, h := range rows {
		out = append(out, ReportField{Label: h.Prompt, Value: "Durchschnitt " + formatAverage(h.Average)})
	}
	return out
}

func questionCard(q announcementService.SurveyQuestionResult) ReportCard {
	card := ReportCard{
		Title:  fmt.Sprintf("%d. %s", q.Position+1, q.Prompt),
		Fields: []ReportField{{Label: "Antworten", Value: strconv.Itoa(q.AnswerCount)}},
	}
	switch q.Kind {
	case announcementService.SurveyQuestionSingle, announcementService.SurveyQuestionMultiple:
		for _, o := range q.Options {
			card.Fields = append(card.Fields, ReportField{Label: o.Label, Value: countWithShare(o.Count, q.AnswerCount)})
		}
	case announcementService.SurveyQuestionScale:
		for i, count := range q.ScaleCounts {
			if i < len(surveyScaleLabels) {
				card.Fields = append(card.Fields, ReportField{Label: surveyScaleLabels[i], Value: countWithShare(count, q.AnswerCount)})
			}
		}
		if q.Average != nil {
			card.Fields = append(card.Fields, ReportField{Label: "Durchschnitt", Value: formatAverage(*q.Average)})
		}
	case announcementService.SurveyQuestionText:
		texts := ReportBlock{Title: "Antworten (ohne Name und Zeit, zufällige Reihenfolge)"}
		for _, text := range q.Texts {
			texts.Fields = append(texts.Fields, ReportField{Label: "", Value: text})
		}
		card.Blocks = append(card.Blocks, texts)
	}
	highLabel, lowLabel := "Am häufigsten gewählt", "Am seltensten gewählt"
	if q.Kind == announcementService.SurveyQuestionScale {
		highLabel, lowLabel = "Höchster Wert", "Niedrigster Wert"
	}
	if len(q.Highest) > 0 {
		card.Fields = append(card.Fields, ReportField{Label: highLabel, Value: strings.Join(q.Highest, ", ")})
	}
	if len(q.Lowest) > 0 {
		card.Fields = append(card.Fields, ReportField{Label: lowLabel, Value: strings.Join(q.Lowest, ", ")})
	}
	return card
}

func countWithShare(count, total int) string {
	if total == 0 {
		return strconv.Itoa(count)
	}
	return fmt.Sprintf("%d (%d %%)", count, count*100/total)
}

func formatAverage(v float64) string {
	return strings.Replace(strconv.FormatFloat(v, 'f', 1, 64), ".", ",", 1)
}
