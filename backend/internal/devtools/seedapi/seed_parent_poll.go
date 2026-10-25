package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
		"body":          seedParentPollBody,
		"priority":      "info",
		"response_type": "multi_choice",
		"options":       seedParentPollOptions(todaySeedDate()),
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

const seedParentPollBody = "Liebe Eltern, beim Elternsprechtag sprechen wir 15 Minuten mit Ihnen über Ihr Kind in der OGS. " +
	"Bitte kreuzen Sie alle Termine an, die Ihnen passen. Wir schicken Ihnen danach Ihren festen Termin."

type seedParentPollItem struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	PublishedAt string                 `json:"published_at"`
	Options     []seedParentPollOption `json:"options"`
	Children    []struct {
		StudentID       string   `json:"student_id"`
		SelectedOptions []string `json:"selected_options"`
	} `json:"children"`
}

type seedParentPollOption struct {
	ID string `json:"id"`
}

// seedParentPollAnswers lets the other demo families answer the poll through
// the parent API, so the staff results show answers (#3759): the scripted
// parents and three of four family app accounts. The first parent and every
// child they share stay unanswered: the parent portal shows them the open
// poll.
func seedParentPollAnswers(rt *Runtime) (int, error) {
	defer rt.Client.BindAuth(rt.TenantAuth)
	if len(rt.Parents[0].StudentIDs) == 0 {
		return 0, errors.New("the first demo parent has no children to keep the poll open for")
	}
	firstFamily := firstFamilyChildren(rt)
	answered := 0
	for _, parent := range respondingFamilies(rt) {
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
		if firstFamily[child.StudentID] || len(child.SelectedOptions) > 0 {
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

// seedParentPollSlotWeights is how popular the first twelve appointments of
// the first afternoon (14:00 to 16:45) are: few families make it at two,
// most come after work.
var seedParentPollSlotWeights = []int{1, 1, 2, 2, 3, 3, 4, 5, 5, 4, 3, 2}

// seedParentPollChoice picks one to three adjacent appointments among the
// first ones for the given answer, weighted by seedParentPollSlotWeights,
// so the results show a believable spread. The same answer number always
// picks the same appointments.
func seedParentPollChoice(options []seedParentPollOption, answer int) []string {
	weights := seedParentPollSlotWeights[:min(len(seedParentPollSlotWeights), len(options))]
	total := 0
	for _, weight := range weights {
		total += weight
	}
	spread := seedSpread(answer)
	pick := int(spread % uint64(total))
	first := 0
	for first < len(weights)-1 && pick >= weights[first] {
		pick -= weights[first]
		first++
	}
	count := 1 + int((spread>>20)%3)
	ids := make([]string, 0, count)
	for slot := first; slot < first+count && slot < len(weights); slot++ {
		ids = append(ids, options[slot].ID)
	}
	return ids
}

// seedSpread mixes a counter into well-spread bits (SplitMix64), for choices
// that look random but repeat on every run.
func seedSpread(n int) uint64 {
	z := uint64(n) + 0x9e3779b97f4a7c15
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// seedParentPollDays are Tuesday to Thursday of next week: always ahead of
// the seed day, and close enough that families plan them now.
func seedParentPollDays(today seedDate) []seedDate {
	tuesday := seedWeekMonday(today).AddDays(8)
	return []seedDate{tuesday, tuesday.AddDays(1), tuesday.AddDays(2)}
}

// seedParentPollOptions offers twenty quarter hours from 14:00 on each poll
// day: "Dienstag, 27. Oktober, 14:00 Uhr".
func seedParentPollOptions(today seedDate) []string {
	options := make([]string, 0, 60)
	for _, day := range seedParentPollDays(today) {
		for slot := range 20 {
			hour := 14 + slot/4
			minute := 15 * (slot % 4)
			options = append(options, fmt.Sprintf("%s, %02d:%02d Uhr", germanDate(day), hour, minute))
		}
	}
	return options
}
