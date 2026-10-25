package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// seedParentPollStep creates a full 60-option poll so the staff editor and
// parent portal's long-poll layout have representative demo data (#3861).
type seedParentPollStep struct{}

func (seedParentPollStep) Name() string { return "Seeding parent poll" }

func (seedParentPollStep) Run(_ context.Context, rt *Runtime) error {
	rt.Client.BindAuth(rt.TenantAuth)
	return seedParentPoll(rt)
}

func seedParentPoll(rt *Runtime) error {
	raw, err := rt.Client.Post("/api/parent-announcements/", map[string]any{
		"title":         seedParentPollTitle,
		"body":          "Bitte wählen Sie passende Termine.",
		"priority":      "info",
		"response_type": "multi_choice",
		"options":       seedParentPollOptions(),
		"targets": []map[string]any{
			{"target_type": "school_all"},
		},
	})
	if err != nil {
		return fmt.Errorf("create parent poll: %w", err)
	}
	id, err := parseEnvelopeStringID(raw)
	if err != nil {
		return fmt.Errorf("parse parent poll response: %w", err)
	}
	if _, err := rt.Client.Post(fmt.Sprintf("/api/parent-announcements/%d/publish", id), nil); err != nil {
		return fmt.Errorf("publish parent poll: %w", err)
	}
	fmt.Println("  1 parent poll with 60 appointment options published")
	if len(rt.Parents) > 1 && rt.Adapter != nil {
		answered, err := seedParentPollAnswers(rt)
		if err != nil {
			return err
		}
		fmt.Printf("  %d children answered the parent poll\n", answered)
	}
	return nil
}

const seedParentPollTitle = "Termine für den Elternsprechtag"

type seedParentPollItem struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	PublishedAt string                 `json:"published_at"`
	Options     []seedParentPollOption `json:"options"`
	Children    []struct {
		StudentID string `json:"student_id"`
	} `json:"children"`
}

type seedParentPollOption struct {
	ID string `json:"id"`
}

// seedParentPollAnswers lets the other demo families answer the poll through
// the parent API, so the staff results show answers (#3759). The first parent
// and every child they share stay unanswered: the parent portal shows them
// the open poll.
func seedParentPollAnswers(rt *Runtime) (int, error) {
	defer rt.Client.BindAuth(rt.TenantAuth)
	if len(rt.Parents[0].StudentIDs) == 0 {
		return 0, errors.New("the first demo parent has no children to keep the poll open for")
	}
	firstFamily := map[string]bool{}
	for _, id := range rt.Parents[0].StudentIDs {
		firstFamily[strconv.FormatInt(id, 10)] = true
	}
	answered := 0
	for _, parent := range rt.Parents[1:] {
		count, err := seedParentPollAnswer(rt, parent, firstFamily, answered)
		if err != nil {
			return answered, err
		}
		answered += count
	}
	return answered, nil
}

// seedParentPollAnswer answers the poll as one parent for each of their
// children outside the first family. `previous` counts the answers so far and
// varies the chosen appointments.
func seedParentPollAnswer(rt *Runtime, parent ParentCredentials, firstFamily map[string]bool, previous int) (int, error) {
	auth, err := rt.Adapter.LoginParent(context.Background(), parent.Email, parent.Password)
	if err != nil {
		return 0, fmt.Errorf("login parent to answer the poll: %w", err)
	}
	poll, err := seedParentPollFor(rt, auth)
	if err != nil || len(poll.Options) == 0 {
		return 0, err
	}
	count := 0
	for _, child := range poll.Children {
		if firstFamily[child.StudentID] {
			continue
		}
		if _, err := rt.Client.PostWithAuth(auth, "/parent/me/news/"+poll.ID+"/respond", map[string]any{
			"student_id":   child.StudentID,
			"option_ids":   seedParentPollChoice(poll.Options, previous+count),
			"published_at": poll.PublishedAt,
		}); err != nil {
			return count, fmt.Errorf("answer parent poll: %w", err)
		}
		count++
	}
	return count, nil
}

func seedParentPollFor(rt *Runtime, auth AuthRef) (seedParentPollItem, error) {
	raw, err := rt.Client.GetWithAuth(auth, "/parent/me/news")
	if err != nil {
		return seedParentPollItem{}, fmt.Errorf("load parent feed for the poll: %w", err)
	}
	var feed struct {
		Data []seedParentPollItem `json:"data"`
	}
	if err := json.Unmarshal(raw, &feed); err != nil {
		return seedParentPollItem{}, fmt.Errorf("parse parent feed for the poll: %w", err)
	}
	for _, item := range feed.Data {
		if item.Title == seedParentPollTitle {
			return item, nil
		}
	}
	return seedParentPollItem{}, nil
}

// seedParentPollSpread is how many appointments the answers spread over: the
// first afternoon, the slots the results list shows first.
const seedParentPollSpread = 8

// seedParentPollChoice picks two of the first appointments for the given
// answer, so the families' answers spread over them.
func seedParentPollChoice(options []seedParentPollOption, answer int) []string {
	spread := min(seedParentPollSpread, len(options))
	first := options[(answer*3)%spread].ID
	second := options[(answer*3+1)%spread].ID
	if first == second {
		return []string{first}
	}
	return []string{first, second}
}

func seedParentPollOptions() []string {
	days := []struct {
		weekday string
		date    string
	}{
		{weekday: "Mittwoch", date: "14. Oktober"},
		{weekday: "Donnerstag", date: "15. Oktober"},
		{weekday: "Freitag", date: "16. Oktober"},
	}
	options := make([]string, 0, 60)
	for _, day := range days {
		for slot := range 20 {
			hour := 14 + slot/4
			minute := 15 * (slot % 4)
			options = append(options, fmt.Sprintf("%s, %s, %02d:%02d Uhr", day.weekday, day.date, hour, minute))
		}
	}
	return options
}
