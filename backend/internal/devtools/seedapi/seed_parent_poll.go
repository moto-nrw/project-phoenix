package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// seedParentPollStep creates the appointment poll for the Elternsprechtag.
// Its 24 options are a believable school's choice and still a long poll:
// from 11 options on, the parent portal shows compact rows and "x von y
// gewählt" (#3861). Until round 3 of the product screenshots the seed offered
// the maximum of 60, but the answers only reached the first twelve, so the
// results showed 48 empty rows.
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
	fmt.Printf("  1 parent poll with %d appointment options published\n", seedParentPollSlotsPerDay*len(seedParentPollDays(todaySeedDate())))
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
//
// The families sign in and answer in parallel. Which family answers for a
// child and which appointments it picks are planned in between, in family
// order, so they stay the same as when the families answered one by one.
func seedParentPollAnswers(rt *Runtime) (int, error) {
	defer rt.Client.BindAuth(rt.TenantAuth)
	if len(rt.Parents[0].StudentIDs) == 0 {
		return 0, errors.New("the first demo parent has no children to keep the poll open for")
	}
	families := respondingFamilies(rt)
	views := make([]seedParentPollView, len(families))
	if err := forEachParallel(rt, families, func(worker *Runtime, index int, parent ParentCredentials) error {
		view, err := seedParentPollViewOf(worker, parent)
		views[index] = view
		return err
	}); err != nil {
		return 0, err
	}
	answers := planSeedParentPollAnswers(views, firstFamilyChildren(rt))
	if err := forEachParallel(rt, views, func(worker *Runtime, index int, view seedParentPollView) error {
		for _, answer := range answers[index] {
			if _, err := worker.Client.PostWithAuth(view.auth, "/parent/me/news/"+view.poll.ID+"/respond", answer); err != nil {
				return fmt.Errorf("answer parent poll: %w", err)
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}
	answered := 0
	for _, family := range answers {
		answered += len(family)
	}
	return answered, nil
}

// seedParentPollView is the poll as one responding parent sees it.
type seedParentPollView struct {
	auth AuthRef
	poll seedParentPollItem
}

func seedParentPollViewOf(rt *Runtime, parent ParentCredentials) (seedParentPollView, error) {
	auth, err := rt.Adapter.LoginParent(context.Background(), parent.Email, parent.Password)
	if err != nil {
		return seedParentPollView{}, fmt.Errorf("login parent to answer the poll: %w", err)
	}
	poll, err := seedParentPollFor(rt, auth)
	return seedParentPollView{auth: auth, poll: poll}, err
}

// planSeedParentPollAnswers lets each parent answer for each of their
// children outside the first family that nobody answered yet, in family
// order. The number of answers before one varies its appointments.
func planSeedParentPollAnswers(views []seedParentPollView, firstFamily map[string]bool) [][]map[string]any {
	answers := make([][]map[string]any, len(views))
	answeredChild := map[string]bool{}
	previous := 0
	for index, view := range views {
		if len(view.poll.Options) == 0 {
			continue
		}
		for _, child := range view.poll.Children {
			if firstFamily[child.StudentID] || answeredChild[child.StudentID] || len(child.SelectedOptions) > 0 {
				continue
			}
			answeredChild[child.StudentID] = true
			answers[index] = append(answers[index], map[string]any{
				"student_id":   child.StudentID,
				"option_ids":   seedParentPollChoice(view.poll.Options, previous),
				"published_at": view.poll.PublishedAt,
			})
			previous++
		}
	}
	return answers
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

// seedParentPollSlotsPerDay are the appointments of one afternoon: a talk
// of 15 minutes every half hour from 14:00 to 17:30, so the teacher has time
// to note what was agreed.
const seedParentPollSlotsPerDay = 8

// seedParentPollSlotWeights is how popular the appointments of an afternoon
// are: few families make it at two, most come after work.
var seedParentPollSlotWeights = [seedParentPollSlotsPerDay]int{1, 1, 2, 3, 4, 4, 3, 2}

// seedParentPollFirstSlots lists every appointment as often as its weight,
// on each poll day, in an order that looks random but repeats on every run.
// Taking them in turn, the first sixty answers start at every appointment
// at least once, and the popular ones more often.
var seedParentPollFirstSlots = func() []int {
	slots := make([]int, 0, 60)
	for day := range 3 {
		for slot, weight := range seedParentPollSlotWeights {
			for range weight {
				slots = append(slots, day*seedParentPollSlotsPerDay+slot)
			}
		}
	}
	for i := len(slots) - 1; i > 0; i-- {
		j := int(seedSpread(i) % uint64(i+1))
		slots[i], slots[j] = slots[j], slots[i]
	}
	return slots
}()

// seedParentPollChoice picks one to three adjacent appointments on one
// afternoon for the given answer, so the results show a believable spread
// over every appointment. The same answer number always picks the same
// appointments.
func seedParentPollChoice(options []seedParentPollOption, answer int) []string {
	if len(options) == 0 {
		return nil
	}
	first := seedParentPollFirstSlots[answer%len(seedParentPollFirstSlots)] % len(options)
	dayEnd := min((first/seedParentPollSlotsPerDay+1)*seedParentPollSlotsPerDay, len(options))
	count := 1 + int((seedSpread(answer)>>20)%3)
	ids := make([]string, 0, count)
	for slot := first; slot < first+count && slot < dayEnd; slot++ {
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

// seedParentPollOptions offers the appointments of every poll day:
// "Dienstag, 27. Oktober, 14:00 Uhr".
func seedParentPollOptions(today seedDate) []string {
	days := seedParentPollDays(today)
	options := make([]string, 0, len(days)*seedParentPollSlotsPerDay)
	for _, day := range days {
		for slot := range seedParentPollSlotsPerDay {
			options = append(options, fmt.Sprintf("%s, %02d:%02d Uhr", germanDate(day), 14+slot/2, 30*(slot%2)))
		}
	}
	return options
}
