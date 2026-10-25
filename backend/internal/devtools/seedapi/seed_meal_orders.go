package api

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"
)

// mealOrderChildren is how many children eat on the fixed order day before
// the late changes, the size of a busy OGS lunch (#3873): about two thirds
// of the eighty to ninety children in care on a full day.
const mealOrderChildren = 60

// mealOrderLateAdds and mealOrderLateCancels are the school's changes after
// the deadline: the Nachmeldungen the order week shows on the fixed day.
const (
	mealOrderLateAdds    = 2
	mealOrderLateCancels = 1
)

// mealOrderWeekdays are the regular lunch days of the children the school
// registers; a pattern without the order day's weekday falls back to the
// whole week, so every one of them eats on that day.
var mealOrderWeekdays = [][]int{{1, 2, 3, 4, 5}, {1, 2, 3, 4}, {2, 3, 4, 5}, {1, 2, 4, 5}, {1, 3, 4, 5}}

// seedMealOrdersStep fills the order week through the school's API the way
// a busy school does (#3873). The families registered about twenty children
// before; the school registers more until the first open weekday has
// mealOrderChildren, spread over the classes. Every other day of the week
// keeps its own deadline and shows its current count. A past day is never
// ordered after the fact, so the first open weekday is the one fixed: the
// school moves the deadline back until it is due, its first late change
// fixes the order, and two late registrations and a cancellation follow as
// Nachmeldungen. The deadline is restored even when a write fails. Then the
// caterer the school invited earlier (seedMealCatererInvitationStep) accepts
// the invitation and confirms that order.
type seedMealOrdersStep struct {
	seeder *Seeder
}

func (seedMealOrdersStep) Name() string { return "Seeding meal orders" }

func (s seedMealOrdersStep) Run(ctx context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || len(rt.Parents) == 0 || len(rt.Parents[0].StudentIDs) == 0 {
		return fmt.Errorf("meal order prerequisites not available")
	}
	rt.Client.BindAuth(rt.TenantAuth)
	var readMailLink MailLinkReader
	if s.seeder != nil {
		readMailLink = s.seeder.options.ReadMailLink
	}
	return seedMealOrders(ctx, rt, readMailLink, todaySeedDate())
}

func seedMealOrders(ctx context.Context, rt *Runtime, readMailLink MailLinkReader, today seedDate) error {
	day, leadDays := mealOrderSeedDay(today)
	list, err := readMealDay(rt, day)
	if err != nil {
		return err
	}
	added, err := registerMealChildren(rt, day, &list, mealOrderChildren-len(list.participants))
	if err != nil {
		return err
	}
	if err := fixMealOrderDay(rt, day, leadDays, list, rt.Parents[0].StudentIDs[0]); err != nil {
		return err
	}
	fmt.Printf("  %d children registered by the school; order of %s fixed with %d meals, +%d/-%d after the deadline\n",
		added, day, len(list.participants)+mealOrderLateAdds-mealOrderLateCancels, mealOrderLateAdds, mealOrderLateCancels)
	return seedMealCaterer(ctx, rt, readMailLink, day)
}

// mealOrderSeedDay is the first weekday from today on and the number of
// lead days that put its deadline before today, whatever the time of day.
func mealOrderSeedDay(today seedDate) (seedDate, int) {
	day := seedWeekdayOnOrAfter(today)
	return day, int(day.Sub(today.Time).Hours()/24) + 1
}

// mealChild is a child of the school's day list.
type mealChild struct {
	id          int64
	schoolClass string
}

// mealDay is who eats on a day and who else is in care that day.
type mealDay struct {
	participants []mealChild
	others       []mealChild
}

func readMealDay(rt *Runtime, day seedDate) (mealDay, error) {
	raw, err := rt.Client.Get("/api/meal-plan/participants?include=all&date=" + day.String())
	if err != nil {
		return mealDay{}, fmt.Errorf("read the meal list of %s: %w", day, err)
	}
	type child struct {
		StudentID   string `json:"student_id"`
		SchoolClass string `json:"school_class"`
	}
	var response struct {
		Data struct {
			Participants []child `json:"participants"`
			Others       []child `json:"others"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &response); err != nil {
		return mealDay{}, fmt.Errorf("parse the meal list of %s: %w", day, err)
	}
	convert := func(children []child) ([]mealChild, error) {
		result := make([]mealChild, 0, len(children))
		for _, c := range children {
			id, err := strconv.ParseInt(c.StudentID, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parse meal list child %q: %w", c.StudentID, err)
			}
			result = append(result, mealChild{id: id, schoolClass: c.SchoolClass})
		}
		return result, nil
	}
	participants, err := convert(response.Data.Participants)
	if err != nil {
		return mealDay{}, err
	}
	others, err := convert(response.Data.Others)
	return mealDay{participants: participants, others: others}, err
}

// registerMealChildren gives up to count children of other classes regular
// lunch days from day on, one class after the other, and moves them onto
// the day's list. It keeps enough others back for the late registrations.
func registerMealChildren(rt *Runtime, day seedDate, list *mealDay, count int) (int, error) {
	count = min(count, len(list.others)-mealOrderLateAdds)
	if count <= 0 {
		return 0, nil
	}
	picked := pickAcrossClasses(list.others, count)
	for index, child := range picked {
		if _, err := rt.Client.Put(fmt.Sprintf("/api/meal-plan/participation/%d/schedule", child.id), map[string]any{
			"weekdays":       mealWeekdaysWith(index, day.Weekday()),
			"effective_from": day.String(),
		}); err != nil {
			return index, fmt.Errorf("register child %d for lunch: %w", child.id, err)
		}
	}
	list.participants = append(list.participants, picked...)
	list.others = slices.DeleteFunc(list.others, func(child mealChild) bool { return slices.Contains(picked, child) })
	return len(picked), nil
}

// mealWeekdaysWith is the index-th lunch pattern, or the whole week when
// that pattern leaves out weekday.
func mealWeekdaysWith(index int, weekday time.Weekday) []int {
	pattern := mealOrderWeekdays[index%len(mealOrderWeekdays)]
	if !slices.Contains(pattern, int(weekday)) {
		return mealOrderWeekdays[0]
	}
	return pattern
}

// pickAcrossClasses takes count children, the first of every class before
// the second of any, so the order counts spread over the classes.
func pickAcrossClasses(children []mealChild, count int) []mealChild {
	var classes []string
	byClass := map[string][]mealChild{}
	for _, child := range children {
		if _, seen := byClass[child.schoolClass]; !seen {
			classes = append(classes, child.schoolClass)
		}
		byClass[child.schoolClass] = append(byClass[child.schoolClass], child)
	}
	picked := make([]mealChild, 0, count)
	for round := 0; len(picked) < count; round++ {
		before := len(picked)
		for _, class := range classes {
			if round < len(byClass[class]) && len(picked) < count {
				picked = append(picked, byClass[class][round])
			}
		}
		if len(picked) == before {
			break
		}
	}
	return picked
}

// fixMealOrderDay makes the day due, records the late changes (the first
// one fixes the order) and restores the deadline, also after a failure.
// The kept child is never cancelled: the parents portal shows the first
// parent's child with lunch.
func fixMealOrderDay(rt *Runtime, day seedDate, leadDays int, list mealDay, kept int64) (err error) {
	if err := setMealLeadDays(rt, leadDays); err != nil {
		return err
	}
	defer func() {
		if restoreErr := setMealLeadDays(rt, 0); err == nil {
			err = restoreErr
		}
	}()
	cancelled := 0
	for _, child := range list.participants {
		if cancelled < mealOrderLateCancels && child.id != kept {
			if err := setMealDay(rt, child.id, day, false); err != nil {
				return err
			}
			cancelled++
		}
	}
	for _, child := range pickAcrossClasses(list.others, mealOrderLateAdds) {
		if err := setMealDay(rt, child.id, day, true); err != nil {
			return err
		}
	}
	return nil
}

func setMealDay(rt *Runtime, studentID int64, day seedDate, participating bool) error {
	path := fmt.Sprintf("/api/meal-plan/participation/%d/days/%s", studentID, day)
	if _, err := rt.Client.Put(path, map[string]any{"participating": participating}); err != nil {
		return fmt.Errorf("change child %d's lunch on %s after the deadline: %w", studentID, day, err)
	}
	return nil
}

func setMealLeadDays(rt *Runtime, days int) error {
	if _, err := rt.Client.Put("/api/settings/values/operations.meal_registration_lead_days", map[string]any{"value": days}); err != nil {
		return fmt.Errorf("set meal registration lead days to %d: %w", days, err)
	}
	return nil
}
