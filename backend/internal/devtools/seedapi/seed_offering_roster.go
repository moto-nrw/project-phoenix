package api

import (
	"context"
	"fmt"
	"time"
)

// offeringRosterRequestKey names the approved requests whose child booked
// "OGS Ganztag" (pickup 16:00) on every weekday.
const offeringRosterRequestKey = "approved-5-days"

// seedOfferingRosterStep checks the children of those requests in for today
// and moves the Gehzeit of one of them before 16:00 (#4571). The selection
// "Nach Angebot" of a spontaneous activity then shows booked children
// and leaves out the one who goes home early, on every dev machine. On a
// weekend no booking applies, so the step leaves the day alone.
type seedOfferingRosterStep struct {
	seeder *Seeder
}

func (seedOfferingRosterStep) Name() string { return "Seeding booked children for today" }

func (s seedOfferingRosterStep) Run(_ context.Context, rt *Runtime) error {
	today := todaySeedDate()
	if today.Weekday() == time.Saturday || today.Weekday() == time.Sunday {
		fmt.Println("  Weekend: no booked children checked in")
		return nil
	}
	studentIDs, err := s.offeringRosterStudents(rt)
	if err != nil {
		return err
	}
	for _, studentID := range studentIDs {
		if err := postSchoolAttendance(rt, studentID, "in", "checked_in", ""); err != nil {
			return err
		}
	}
	early := studentIDs[len(studentIDs)-1]
	if _, err := rt.Client.Post(fmt.Sprintf("/api/students/%d/pickup-exceptions", early), map[string]any{
		"exception_date": today.String(),
		"pickup_time":    "14:30",
		"reason":         "Arzttermin",
	}); err != nil {
		return fmt.Errorf("seed early pickup of a booked child: %w", err)
	}
	fmt.Printf("  %d booked children checked in, one goes home at 14:30\n", len(studentIDs))
	return nil
}

func (s seedOfferingRosterStep) offeringRosterStudents(rt *Runtime) ([]int64, error) {
	step := parentEnrollmentSeedStep(s)
	var studentIDs []int64
	for _, request := range rt.Enrollment.Requests {
		if request.Key != offeringRosterRequestKey || request.Status != "approved" {
			continue
		}
		detail, err := step.loadEnrollmentRequestDetail(rt, rt.TenantAuth, request.RequestID)
		if err != nil {
			return nil, err
		}
		if len(detail.CreatedStudentIDs) == 0 {
			return nil, fmt.Errorf("approved enrollment request %d has no created student", request.RequestID)
		}
		studentIDs = append(studentIDs, detail.CreatedStudentIDs[0])
	}
	if len(studentIDs) < 2 {
		return nil, fmt.Errorf("booked children for today: want 2 approved %q requests, got %d", offeringRosterRequestKey, len(studentIDs))
	}
	return studentIDs, nil
}
