package api

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// feedbackHistoryDays is how far the feedback reaches back: two weeks cover
// the Feedbackhistorie's default "Letzte 7 Tage" and its 30-day view, well
// inside the 90 days the school keeps feedback.
const feedbackHistoryDays = 14

// feedbackHistoryEntry is one rating of a child, written through the regular
// feedback batch endpoint like the kiosk's.
type feedbackHistoryEntry struct {
	Value           string `json:"value"`
	Day             string `json:"day"`
	Time            string `json:"time"`
	StudentID       int64  `json:"student_id"`
	IsMensaFeedback bool   `json:"is_mensa_feedback"`
}

// seedFeedbackHistoryStep fills the Feedbackhistorie of the children
// (#5029): on some of the days a child came, it rated its day when it went
// home, and on some it rated lunch in the Mensa. The days come from the same
// plan as the presence history, so a child never rates a day it missed.
type seedFeedbackHistoryStep struct{ seeder *Seeder }

func (seedFeedbackHistoryStep) Name() string { return "Feedbackverlauf wird angelegt" }

func (s seedFeedbackHistoryStep) Run(_ context.Context, rt *Runtime) error {
	if string(s.seeder.definition.Settings[profileSettingFeedbackEnabled].Value) != "true" {
		fmt.Println("  Feedback ist für diese Schule aus, der Feedbackverlauf wird übersprungen")
		return nil
	}
	input, err := pastCareInputFor(rt, s.seeder.definition)
	if err != nil {
		return err
	}
	from, until := input.today.AddDays(-feedbackHistoryDays), input.today.AddDays(-1)
	var existing struct {
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := getSeedJSON(rt, fmt.Sprintf("/api/feedback/date-range?start_date=%s&end_date=%s", from, until), &existing); err != nil {
		return fmt.Errorf("read past feedback: %w", err)
	}
	if len(existing.Data) > 0 {
		// The batch commits as a whole, so a present history is complete.
		fmt.Printf("  %d vergangene Feedbacks sind schon angelegt\n", len(existing.Data))
		return nil
	}
	entries := planFeedbackHistory(planPresenceHistory(input).days, from)
	if len(entries) == 0 {
		return nil
	}
	raw, err := rt.Client.Post("/api/feedback/batch", map[string]any{"entries": entries})
	if err != nil {
		return fmt.Errorf("seed past feedback: %w", err)
	}
	var created struct {
		Data struct {
			Count int `json:"count"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &created); err != nil {
		return fmt.Errorf("read created past feedback: %w", err)
	}
	if created.Data.Count != len(entries) {
		return fmt.Errorf("seed past feedback: created %d of %d entries", created.Data.Count, len(entries))
	}
	mensa := 0
	for _, entry := range entries {
		if entry.IsMensaFeedback {
			mensa++
		}
	}
	fmt.Printf("  %d vergangene Feedbacks, davon %d zum Mittagessen\n", len(entries), mensa)
	return nil
}

// planFeedbackHistory decides the ratings from a fixed hash of child and day.
// About two in five days a child rates its day at check-out; about one in
// four it rates lunch half an hour to an hour after it arrived. Most ratings
// are good, some neutral, few bad.
func planFeedbackHistory(days []PastPresenceDay, from calendar.Date) []feedbackHistoryEntry {
	var entries []feedbackHistoryEntry
	for _, day := range days {
		if day.Date.Before(from) {
			continue
		}
		roll := func(decision string) uint32 { return feedbackRoll(day.StudentID, day.Date, decision) }
		if roll("mensa")%100 < 25 {
			at := day.CheckIn.Add(time.Duration(30+roll("mensa-time")%31) * time.Minute)
			entries = append(entries, feedbackHistoryEntryAt(day, at, roll("mensa-value"), true))
		}
		if roll("day")%100 < 40 {
			entries = append(entries, feedbackHistoryEntryAt(day, day.CheckOut, roll("day-value"), false))
		}
	}
	return entries
}

func feedbackHistoryEntryAt(day PastPresenceDay, at time.Time, valueRoll uint32, mensa bool) feedbackHistoryEntry {
	return feedbackHistoryEntry{
		Value:           feedbackHistoryValue(valueRoll % 100),
		Day:             day.Date.String(),
		Time:            at.In(seedBerlinLocation()).Format("15:04:05"),
		StudentID:       day.StudentID,
		IsMensaFeedback: mensa,
	}
}

func feedbackHistoryValue(roll uint32) string {
	switch {
	case roll < 60:
		return "positive"
	case roll < 88:
		return "neutral"
	default:
		return "negative"
	}
}

func feedbackRoll(studentID int64, day calendar.Date, decision string) uint32 {
	return presenceRoll(int(studentID), day, "feedback-"+decision)
}
