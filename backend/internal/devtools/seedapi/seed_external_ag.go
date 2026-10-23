package api

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// seedExternalAGLeader leads the seeded AG: an external caregiver with an
// account in the system role „AG-Leitung“.
const seedExternalAGLeader = "Jonas Becker"

// seedExternalAGStep plans a weekly AG as a Regeltermin with an external
// caregiver as its only and responsible staff member (#4573), so the planner
// shows an external in a regular event and the external's own „Heute“ has a
// block to run. It runs right after seedExternalCaregiversStep, which records
// the AG's leader.
type seedExternalAGStep struct{}

func (seedExternalAGStep) Name() string { return "Seeding an AG led by an external caregiver" }

func (seedExternalAGStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.FixedSeeder == nil || rt.Client == nil {
		return fmt.Errorf("external AG prerequisites not available")
	}
	fs := rt.FixedSeeder
	staffID := seededExternalCaregiverIDs(rt)[seedExternalAGLeader]
	roomID := fs.roomIDs["Musikraum"]
	categoryID := fs.categoryIDs["Musik"]
	studentIDs := orderedSeedStudentIDs(fs)
	if staffID == 0 || roomID == 0 || categoryID == 0 || len(studentIDs) < seedExternalAGStudents {
		return fmt.Errorf("external AG references not available")
	}
	body := seedExternalAGBody(todaySeedDate(), roomID, categoryID, staffID, studentIDs)
	if _, err := rt.Client.Post("/api/timetable/templates", body); err != nil {
		return fmt.Errorf("create external AG: %w", err)
	}
	fmt.Println("  AG Musik planned with an external caregiver")
	return nil
}

// seedExternalAGStudents is the size of the AG: the last children of the
// demo school, who sit in no other seeded afternoon block.
const seedExternalAGStudents = 8

// seedExternalAGBody is a weekly AG on the weekday of the seed (Monday on a
// weekend), so the external's „Heute“ shows it on the day the stack is
// seeded.
func seedExternalAGBody(today seedDate, roomID, categoryID, staffID int64, studentIDs []int64) map[string]any {
	day := seedWeekdayOnOrAfter(today)
	return map[string]any{
		"name":              "AG Musik",
		"type":              "activity",
		"list_kind":         "activity",
		"target_group_type": "none",
		"weekdays":          []int{calendar.ISOWeekday(calendar.NewDate(day.Year(), day.Month(), day.Day()))},
		"start_time":        "14:00",
		"end_time":          "15:00",
		"room_id":           roomID,
		"category_id":       categoryID,
		"week_pattern":      0,
		"student_ids":       studentIDs[len(studentIDs)-seedExternalAGStudents:],
		"staff_ids":         []int64{staffID},
		"primary_staff_id":  staffID,
		"materialize_from":  today.String(),
		"materialize_to":    today.AddDays(6).String(),
	}
}
