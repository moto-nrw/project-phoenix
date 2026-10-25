package simulate

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

// todayPlan is what the school plans for the simulated day, read once at the
// start: the staff shifts and the timetable blocks. The waves of the day act
// on it instead of inventing sessions and supervisors of their own.
type todayPlan struct {
	Shifts []plannedShift
	Blocks []dayBlock
}

// dayBlock is the part of a timetable instance the simulation needs.
type dayBlock struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	Status       string `json:"status"`
	ActivityType string `json:"activity_type"`
	Spontaneous  bool   `json:"is_spontaneous"`
	RoomID       int64  `json:"room_id"`
	Staff        []struct {
		StaffID  int64 `json:"staff_id"`
		IsAbsent bool  `json:"is_absent"`
	} `json:"staff"`
	StudentIDs []int64 `json:"student_ids"`
}

// startable reports a planned block with children: no duty, no external
// appointment, nothing cancelled or already running.
func (b dayBlock) startable() bool {
	return b.Status == "planned" && !b.Spontaneous && b.ActivityType != "duty" && b.ActivityType != "external"
}

// runsAt reports whether the block's planned window holds the time of day.
func (b dayBlock) runsAt(c wallClock) bool {
	start, startOK := parseWallClock(b.StartTime)
	end, endOK := parseWallClock(b.EndTime)
	return startOK && endOK && start <= c && c < end
}

// overlaps reports whether the block's planned window meets [from, to).
func (b dayBlock) overlaps(from, to wallClock) bool {
	start, startOK := parseWallClock(b.StartTime)
	end, endOK := parseWallClock(b.EndTime)
	return startOK && endOK && start < to && from < end
}

// presentStaff lists the staff planned on the block who are not marked absent
// for it; a substitute is one of them.
func (b dayBlock) presentStaff() []int64 {
	ids := make([]int64, 0, len(b.Staff))
	for _, staff := range b.Staff {
		if !staff.IsAbsent {
			ids = append(ids, staff.StaffID)
		}
	}
	return ids
}

// parseWallClock reads "HH:MM" or "HH:MM:SS".
func parseWallClock(value string) (wallClock, bool) {
	parsed, err := time.Parse("15:04", value[:min(5, len(value))])
	if err != nil {
		return 0, false
	}
	return at(parsed.Hour(), parsed.Minute()), true
}

// todayChild is one child of the simulated day with what today's plan says
// about it.
type todayChild struct {
	SeedStudent
	// Absent: reported sick, excused, on a class trip, or not coming today
	// according to the care plan. The child does not arrive.
	Absent bool
	// ArrivalException is today's arrival time of an exception (a class
	// whose lessons are cancelled, a doctor's appointment), 0 without one.
	ArrivalException wallClock
}

// apiChild is one row of GET /api/students.
type apiChild struct {
	ID                 json.Number `json:"id"`
	FirstName          string      `json:"first_name"`
	LastName           string      `json:"last_name"`
	SchoolClass        string      `json:"school_class"`
	Sick               bool        `json:"sick"`
	Excused            bool        `json:"excused"`
	ClassTrip          bool        `json:"class_trip"`
	DayPlanningStatus  string      `json:"day_planning_status"`
	ArrivalTime        *string     `json:"arrival_time"`
	ArrivalIsException bool        `json:"arrival_is_exception"`
}

func (c apiChild) absent() bool {
	return c.Sick || c.Excused || c.ClassTrip || c.DayPlanningStatus == "not_coming_today"
}

func (c apiChild) arrivalException() wallClock {
	if !c.ArrivalIsException || c.ArrivalTime == nil {
		return 0
	}
	clock, ok := parseWallClock(*c.ArrivalTime)
	if !ok {
		return 0
	}
	return clock
}

type loadTodayAction struct{}

func (loadTodayAction) Name() string { return "read today's plan" }

// Run reads today's shifts, timetable blocks and children with their day
// status, so the waves follow the plan the screens show next to them.
func (loadTodayAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 1b: Reading today's plan...")
	today := rt.Options.now().In(simulationBerlin).Format(isoDate)
	shifts, err := todaysShifts(rt, today)
	if err != nil {
		return err
	}
	blocks, err := todaysBlocks(rt, today)
	if err != nil {
		return err
	}
	children, err := todaysChildren(rt)
	if err != nil {
		return err
	}
	rt.Today = todayPlan{Shifts: shifts, Blocks: blocks}
	rt.Children = mergeChildren(rt.State.Students, children)
	extra := make([]SeedStudent, 0)
	for _, child := range rt.Children[len(rt.State.Students):] {
		extra = append(extra, child.SeedStudent)
	}
	if err := rt.assignTags(extra); err != nil {
		return err
	}
	absent := 0
	for _, child := range rt.Children {
		if child.Absent {
			absent++
		}
	}
	fmt.Printf("  %d shifts, %d blocks, %d children (%d not coming today)\n", len(shifts), len(blocks), len(rt.Children), absent)
	return nil
}

func todaysBlocks(rt *Runtime, today string) ([]dayBlock, error) {
	raw, err := rt.Client.Get(fmt.Sprintf("/api/timetable/instances?from=%s&to=%s", today, today))
	if err != nil {
		return nil, fmt.Errorf("list today's blocks: %w", err)
	}
	var envelope struct {
		Data struct {
			Instances []dayBlock `json:"instances"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode today's blocks: %w", err)
	}
	return envelope.Data.Instances, nil
}

// todaysChildrenPageSize keeps a school in one or two pages.
const todaysChildrenPageSize = 500

// todaysChildren lists every child with today's day planning and arrival.
func todaysChildren(rt *Runtime) ([]apiChild, error) {
	var all []apiChild
	for page := 1; ; page++ {
		raw, err := rt.Client.Get(fmt.Sprintf("/api/students?page=%d&page_size=%d&include_arrival_times=true", page, todaysChildrenPageSize))
		if err != nil {
			return nil, fmt.Errorf("list today's children: %w", err)
		}
		var envelope struct {
			Data       []apiChild `json:"data"`
			Pagination struct {
				TotalPages int `json:"total_pages"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, fmt.Errorf("decode today's children: %w", err)
		}
		all = append(all, envelope.Data...)
		if len(envelope.Data) == 0 || page >= envelope.Pagination.TotalPages {
			return all, nil
		}
	}
}

// mergeChildren keeps the seed state's order and adds the children only the
// API knows, by ID. A seeded child the API does not list keeps coming.
func mergeChildren(seeded []SeedStudent, listed []apiChild) []todayChild {
	byID := make(map[int64]apiChild, len(listed))
	for _, child := range listed {
		if id, err := child.ID.Int64(); err == nil {
			byID[id] = child
		}
	}
	children := make([]todayChild, 0, max(len(seeded), len(byID)))
	for _, student := range seeded {
		child := todayChild{SeedStudent: student}
		if listed, ok := byID[student.ID]; ok {
			child.Absent, child.ArrivalException = listed.absent(), listed.arrivalException()
			delete(byID, student.ID)
		}
		children = append(children, child)
	}
	extraIDs := make([]int64, 0, len(byID))
	for id := range byID {
		extraIDs = append(extraIDs, id)
	}
	slices.Sort(extraIDs)
	for _, id := range extraIDs {
		listed := byID[id]
		children = append(children, todayChild{
			SeedStudent: SeedStudent{ID: id, FirstName: listed.FirstName, LastName: listed.LastName, Class: listed.SchoolClass},
			Absent:      listed.absent(), ArrivalException: listed.arrivalException(),
		})
	}
	return children
}

// child returns the child of the day with the ID.
func (rt *Runtime) child(id int64) (todayChild, bool) {
	i := slices.IndexFunc(rt.Children, func(child todayChild) bool { return child.ID == id })
	if i < 0 {
		return todayChild{}, false
	}
	return rt.Children[i], true
}

// markAbsent records a child reported sick during the run.
func (rt *Runtime) markAbsent(id int64) {
	for i := range rt.Children {
		if rt.Children[i].ID == id {
			rt.Children[i].Absent = true
		}
	}
}

// firstName is how a message names the child: the first name only.
func (rt *Runtime) firstName(id int64) string {
	child, ok := rt.child(id)
	if !ok || strings.TrimSpace(child.FirstName) == "" {
		return "Ihr Kind"
	}
	return child.FirstName
}
