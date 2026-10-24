package api

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

// seedCourseParticipationStep records past occurrences of three demo courses
// with a decided attendance row per child, so the Statistik section "Kurse"
// (#2891) is not empty on a fresh demo tenant.
//
// Each demo course gets its own Betreuungsplan template first, and the past
// dates hang off that template: only templates carry course identity, so an
// occurrence booked on the plain Angebots-Gruppe would be an operational date
// and never reach the section. The dates themselves are created through the
// ordinary instance endpoint — a real course date, only outside the
// materialization cycle, which does not reach into the past.
//
// The template deliberately gets no roster. A roster written today would start
// its enrollment today, and an enrollment never covers a date before it began,
// so every seeded date would drop out of the child view. The children come in
// per occurrence instead, which is what a walk-in course looks like anyway.
//
// One date is cancelled and one child is left undecided on purpose — the
// screen has to show that cancelled dates count nowhere and that an unfinished
// block does not drag the quota down.
type seedCourseParticipationStep struct{}

func (seedCourseParticipationStep) Name() string { return "Seeding course participation" }

// courseDemoWeeks is how many past occurrences each demo course gets.
const courseDemoWeeks = 4

// courseDemoChildren is how many children take part per course; the fixed
// seeder enrolls the first five demo children in every activity.
const courseDemoChildren = 5

func (seedCourseParticipationStep) Run(_ context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil {
		return fmt.Errorf("fixed seeder not available")
	}
	dates := pastWeekdays(courseDemoWeeks)
	instances, cancelled, err := seedCourseParticipation(rt, dates)
	if err != nil {
		return err
	}
	fmt.Printf("  %d course occurrences seeded (%d cancelled)\n", instances, cancelled)
	return nil
}

func seedCourseParticipation(rt *Runtime, dates []seedDate) (int, int, error) {
	instances, cancelled := 0, 0
	for courseIndex, activity := range DemoActivities[:min(3, len(DemoActivities))] {
		courseInstances, courseCancelled, err := seedCourseActivity(rt, courseIndex, activity.Name, dates)
		if err != nil {
			return 0, 0, err
		}
		instances += courseInstances
		cancelled += courseCancelled
	}
	return instances, cancelled, nil
}

func seedCourseActivity(rt *Runtime, courseIndex int, name string, dates []seedDate) (int, int, error) {
	roomID := rt.FixedSeeder.activityRoomIDs[rt.FixedSeeder.activityIDs[name]]
	if roomID == 0 {
		return 0, 0, fmt.Errorf("room not available for course %s", name)
	}
	activityID, err := createCourseTemplate(rt, name, roomID, dates)
	if err != nil {
		return 0, 0, err
	}
	studentIDs := seedCourseStudentIDs(rt)
	if len(studentIDs) == 0 {
		return 0, 0, fmt.Errorf("no students available for course %s", name)
	}
	return seedCourseOccurrences(rt, courseIndex, name, dates, roomID, activityID, studentIDs)
}

func seedCourseStudentIDs(rt *Runtime) []int64 {
	studentIDs := make([]int64, 0, courseDemoChildren)
	for i := range courseDemoChildren {
		if id, ok := rt.FixedSeeder.studentIDByIndex[i]; ok {
			studentIDs = append(studentIDs, id)
		}
	}
	return studentIDs
}

func seedCourseOccurrences(rt *Runtime, courseIndex int, name string, dates []seedDate, roomID, activityID int64, studentIDs []int64) (int, int, error) {
	instances, cancelled := 0, 0
	for dateIndex, date := range dates {
		wasCancelled, err := seedCourseOccurrence(rt, courseIndex, dateIndex, name, date, roomID, activityID, studentIDs, len(dates))
		if err != nil {
			return 0, 0, err
		}
		instances++
		if wasCancelled {
			cancelled++
		}
	}
	return instances, cancelled, nil
}

func seedCourseOccurrence(rt *Runtime, courseIndex, dateIndex int, name string, date seedDate, roomID, activityID int64, studentIDs []int64, dateCount int) (bool, error) {
	instanceID, err := createCourseOccurrence(rt, name, date, roomID, activityID, studentIDs)
	if err != nil {
		return false, err
	}
	if courseIndex == 0 && dateIndex == 0 {
		if _, err := rt.Client.Post(fmt.Sprintf("/api/timetable/instances/%d/cancel", instanceID), map[string]any{"reason": "Raum war belegt"}); err != nil {
			return false, fmt.Errorf("cancel course occurrence: %w", err)
		}
		return true, nil
	}
	return false, seedCourseAttendance(rt, courseIndex, dateIndex, dateCount, instanceID, studentIDs)
}

func seedCourseAttendance(rt *Runtime, courseIndex, dateIndex, dateCount int, instanceID int64, studentIDs []int64) error {
	for studentIndex, studentID := range studentIDs {
		status := courseAttendanceStatus(courseIndex, dateIndex, studentIndex, dateCount)
		if status == "" {
			continue // left undecided: the block was never finished
		}
		path := fmt.Sprintf("/api/timetable/instances/%d/students/%d", instanceID, studentID)
		if _, err := rt.Client.Patch(path, map[string]any{"status": status}); err != nil {
			return fmt.Errorf("record course attendance: %w", err)
		}
	}
	return nil
}

// createCourseTemplate creates the Betreuungsplan template that carries the
// course identity and returns its group id. The materialization window is left
// out: the demo dates lie in the past, where materialization never reaches, and
// a future occurrence would show up as a date that has not happened yet.
func createCourseTemplate(rt *Runtime, name string, roomID int64, dates []seedDate) (int64, error) {
	categoryID := seedAnyCategoryID(rt)
	if categoryID == 0 {
		return 0, fmt.Errorf("no category available for course template %s", name)
	}
	weekday := int(dates[0].Weekday())
	if weekday == 0 {
		weekday = 7 // ISO: Sunday is 7, and pastWeekdays never returns one
	}
	raw, err := rt.Client.Post("/api/timetable/templates", map[string]any{
		"name":             name,
		"type":             "activity",
		"list_kind":        "activity",
		"weekdays":         []int{weekday},
		"start_time":       "14:00",
		"end_time":         "15:00",
		"room_id":          roomID,
		"category_id":      categoryID,
		"max_participants": 20,
		"week_pattern":     0,
	})
	if err != nil {
		return 0, fmt.Errorf("create course template %s: %w", name, err)
	}
	var resp struct {
		Data struct {
			TemplateID json.Number `json:"template_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return 0, fmt.Errorf("parse course template response: %w", err)
	}
	id, err := resp.Data.TemplateID.Int64()
	if err != nil || id == 0 {
		return 0, fmt.Errorf("course template response carries no id")
	}
	return id, nil
}

// seedAnyCategoryID picks a stable activity category: the sport one the demo
// courses belong to, otherwise the alphabetically first so the choice does not
// change between runs.
func seedAnyCategoryID(rt *Runtime) int64 {
	if id := rt.FixedSeeder.categoryIDs["Sport"]; id > 0 {
		return id
	}
	names := make([]string, 0, len(rt.FixedSeeder.categoryIDs))
	for name := range rt.FixedSeeder.categoryIDs {
		names = append(names, name)
	}
	slices.Sort(names)
	if len(names) == 0 {
		return 0
	}
	return rt.FixedSeeder.categoryIDs[names[0]]
}

// createCourseOccurrence books one past date of a course with its roster and
// returns the new instance ID.
func createCourseOccurrence(rt *Runtime, title string, date seedDate, roomID, activityID int64, studentIDs []int64) (int64, error) {
	body := map[string]any{
		"date":              date.String(),
		"start_time":        "14:00",
		"end_time":          "15:00",
		"title":             title,
		"room_id":           roomID,
		"activity_group_id": activityID,
		"student_ids":       studentIDs,
	}
	respBody, err := rt.Client.Post("/api/timetable/instances", body)
	if err != nil {
		return 0, fmt.Errorf("create course occurrence %s on %s: %w", title, date, err)
	}
	var resp struct {
		Data struct {
			ID json.Number `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return 0, fmt.Errorf("parse course occurrence response: %w", err)
	}
	id, err := resp.Data.ID.Int64()
	if err != nil || id == 0 {
		return 0, fmt.Errorf("course occurrence response carries no id")
	}
	return id, nil
}

// courseAttendanceStatus spreads a plausible pattern over the demo courses:
// mostly present, a recurring absence for one child, and the newest date of
// the last course left undecided so the "Offen" column is not always zero.
// An empty result means "write nothing".
func courseAttendanceStatus(courseIndex, dateIndex, studentIndex, dateCount int) string {
	if courseIndex == 2 && dateIndex == dateCount-1 {
		return ""
	}
	if studentIndex == courseIndex+1 && dateIndex%2 == 0 {
		return "absent"
	}
	return "present"
}

// pastWeekdays returns the last n weekdays before today, oldest first, one
// per week so the occurrences read as a weekly course.
func pastWeekdays(n int) []seedDate {
	dates := make([]seedDate, 0, n)
	day := todaySeedDate().AddDays(-1)
	for len(dates) < n {
		switch day.Weekday() {
		case time.Saturday, time.Sunday:
			day = day.AddDays(-1)
			continue
		default:
		}
		dates = append(dates, day)
		day = day.AddDays(-7)
	}
	// Collected newest first; the demo reads better oldest first.
	for i, j := 0, len(dates)-1; i < j; i, j = i+1, j-1 {
		dates[i], dates[j] = dates[j], dates[i]
	}
	return dates
}
