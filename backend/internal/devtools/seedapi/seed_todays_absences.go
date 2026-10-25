package api

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

// The staff members of today's absences, as indices into DemoStaff: the third
// is ill, the fifth stands in for her, and the seventh is on vacation. None of
// them is a Pädagogische Fachkraft, so the simulation, which clocks those in,
// never stamps an absent person (#3892).
const (
	sickTodayStaffIndex       = 2
	substituteTodayStaffIndex = 4
	vacationTodayStaffIndex   = 6
)

// seedTodaysAbsencesStep makes today a day with a real substitution case: one
// colleague reports sick, which cancels her Dienst and marks her blocks
// absent (the #1843 cascade), and the Leitung covers the Frühbetreuung and the
// Essensausgabe with the colleague in the Frühdienst. Her afternoon block
// stays an open gap on the Vertretungsplan. Another colleague takes a
// vacation day the Leitung booked, without a Dienst today. It runs after the
// time-tracking history: the absences are priced against the schedules it
// writes, and after this week's shifts, which the sick report cancels.
type seedTodaysAbsencesStep struct{}

func (seedTodaysAbsencesStep) Name() string { return "Seeding today's absences and substitution" }

func (seedTodaysAbsencesStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.FixedSeeder == nil {
		return fmt.Errorf("today's absences prerequisites not available")
	}
	return seedTodaysAbsences(rt, todaySeedDate())
}

func seedTodaysAbsences(rt *Runtime, today seedDate) error {
	staffIDs := orderedSeedStaffIDs(rt.FixedSeeder)
	if len(staffIDs) <= vacationTodayStaffIndex {
		return fmt.Errorf("today's absences need %d staff members, got %d", vacationTodayStaffIndex+1, len(staffIDs))
	}
	if today.Weekday() == time.Saturday || today.Weekday() == time.Sunday {
		fmt.Println("  weekend: no absences today")
		return nil
	}
	currentAuth := rt.Client.auth
	defer rt.Client.BindAuth(currentAuth)
	rt.Client.BindAuth(rt.TenantAuth)

	if err := postStaffAbsence(rt, staffIDs[vacationTodayStaffIndex], today.UTCMidnight(), "vacation", "Urlaub", nil); err != nil {
		return fmt.Errorf("seed today's vacation: %w", err)
	}
	sickID := staffIDs[sickTodayStaffIndex]
	if err := postStaffAbsence(rt, sickID, today.UTCMidnight(), "sick", "Krankmeldung am Morgen telefonisch", nil); err != nil {
		return fmt.Errorf("seed today's sick report: %w", err)
	}
	covered, err := coveredSickBlocks(rt, today, sickID)
	if err != nil {
		return err
	}
	if len(covered) == 0 {
		fmt.Println("  1 vacation and 1 sick report today, no block to cover")
		return nil
	}
	if _, err := rt.Client.Post(fmt.Sprintf("/api/timetable/instances/%d/deviations", covered[0]), map[string]any{
		"substitutions": []map[string]any{{
			"absent_staff_id": sickID, "substitute_staff_id": staffIDs[substituteTodayStaffIndex],
			"reason": "Vertretung wegen Krankheit", "instance_ids": covered,
		}},
	}); err != nil {
		return fmt.Errorf("seed today's substitution: %w", err)
	}
	fmt.Printf("  1 vacation and 1 sick report today, %d blocks covered\n", len(covered))
	return nil
}

// coveredSubstituteBlocks are the blocks of the ill colleague the stand-in
// takes over; both lie in the stand-in's Frühdienst.
var coveredSubstituteBlocks = []string{"Frühbetreuung", "Essensausgabe"}

// coveredSickBlocks returns today's Frühbetreuung and Essensausgabe the ill
// staff member was planned for, in the order the day lists them.
func coveredSickBlocks(rt *Runtime, today seedDate, staffID int64) ([]int64, error) {
	raw, err := rt.Client.Get(fmt.Sprintf("/api/timetable/instances?from=%s&to=%s", today.String(), today.String()))
	if err != nil {
		return nil, fmt.Errorf("list today's planning instances: %w", err)
	}
	var envelope struct {
		Data struct {
			Instances []seedPlanningInstance `json:"instances"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("parse today's planning instances: %w", err)
	}
	var ids []int64
	for _, instance := range envelope.Data.Instances {
		if slices.Contains(coveredSubstituteBlocks, instance.Title) && instancePlansStaff(instance, staffID) {
			ids = append(ids, instance.ID)
		}
	}
	return ids, nil
}

func instancePlansStaff(instance seedPlanningInstance, staffID int64) bool {
	for _, staff := range instance.Staff {
		if staff.StaffID == staffID {
			return true
		}
	}
	return false
}
