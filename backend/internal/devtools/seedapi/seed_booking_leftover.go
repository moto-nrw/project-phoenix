package api

import (
	"context"
	"fmt"
)

// bookingLeftoverRequestKey names the approved requests that booked four
// days; the first of them is Hannah Schmitz with "OGS Ganztag" (pickup
// 16:00) on Monday, Tuesday, Wednesday and Friday.
const bookingLeftoverRequestKey = "approved-4-days"

// seedBookingLeftoverStep reproduces the case of #5102 on every dev machine:
// staff put Hannah by hand on the Regeltermin "Hausaufgabenbetreuung"
// (Monday to Friday until 16:00), then a Korrektur takes her Wednesday
// away and nobody removes her. The Betreuungsplan and the home page show
// the hint "Ohne passende Buchung eingeplant" for her Wednesday.
type seedBookingLeftoverStep struct {
	seeder *Seeder
}

func (seedBookingLeftoverStep) Name() string {
	return "Seeding a Regeltermin left behind by a Korrektur"
}

func (s seedBookingLeftoverStep) Run(_ context.Context, rt *Runtime) error {
	request, studentID, err := s.leftoverChild(rt)
	if err != nil {
		return err
	}
	if err := seedHandTemplate(rt, studentID); err != nil {
		return err
	}
	ganztag, lunch := rt.Enrollment.Offerings["ogs-ganztag"], rt.Enrollment.Offerings["mittagessen"]
	if ganztag == 0 || lunch == 0 {
		return fmt.Errorf("booking leftover: offerings ogs-ganztag and mittagessen not seeded")
	}
	days := []string{"mon", "tue", "fri"}
	path := fmt.Sprintf("/api/enrollment/admin/requests/%d/children/%d/offerings", request.RequestID, request.ChildIDs[0])
	if _, err := rt.Client.PutWithAuth(rt.TenantAuth, path, map[string]any{
		"offerings": []map[string]any{
			{"offering_id": fmt.Sprint(ganztag), "selected_days": days},
			{"offering_id": fmt.Sprint(lunch), "selected_days": days},
		},
		"reason":         "Mittwochs geht Hannah zur Oma",
		"effective_from": todaySeedDate().String(),
	}); err != nil {
		return fmt.Errorf("booking leftover: drop Wednesday: %w", err)
	}
	fmt.Println("  Hannah stays planned by hand on Wednesdays after her Korrektur")
	return nil
}

func (s seedBookingLeftoverStep) leftoverChild(rt *Runtime) (SeedEnrollmentRequest, int64, error) {
	step := parentEnrollmentSeedStep(s)
	for _, request := range rt.Enrollment.Requests {
		if request.Key != bookingLeftoverRequestKey || request.Status != "approved" || len(request.ChildIDs) == 0 {
			continue
		}
		detail, err := step.loadEnrollmentRequestDetail(rt, rt.TenantAuth, request.RequestID)
		if err != nil {
			return SeedEnrollmentRequest{}, 0, err
		}
		if len(detail.CreatedStudentIDs) == 0 {
			return SeedEnrollmentRequest{}, 0, fmt.Errorf("approved enrollment request %d has no created student", request.RequestID)
		}
		return request, detail.CreatedStudentIDs[0], nil
	}
	return SeedEnrollmentRequest{}, 0, fmt.Errorf("booking leftover: no approved %q request", bookingLeftoverRequestKey)
}

// seedHandTemplate creates the Regeltermin whose children staff pick by
// hand: no Klasse, no offering as source.
func seedHandTemplate(rt *Runtime, studentID int64) error {
	if rt.FixedSeeder == nil {
		return fmt.Errorf("booking leftover: fixed seed data not available")
	}
	roomID := rt.FixedSeeder.roomIDs["OGS-Raum 1"]
	categoryID := rt.FixedSeeder.categoryIDs["Hausaufgaben"]
	if categoryID == 0 {
		categoryID = seedAnyCategoryID(rt)
	}
	staffIDs := orderedSeedStaffIDs(rt.FixedSeeder)
	if roomID == 0 || categoryID == 0 || len(staffIDs) == 0 {
		return fmt.Errorf("booking leftover: template references not available (room %d, category %d, %d staff)", roomID, categoryID, len(staffIDs))
	}
	if _, err := rt.Client.Post("/api/timetable/templates", map[string]any{
		"name": "Hausaufgabenbetreuung", "type": "care", "target_group_type": "none",
		"weekdays": []int{1, 2, 3, 4, 5}, "start_time": "14:00", "end_time": "16:00",
		"room_id": roomID, "category_id": categoryID, "week_pattern": 0,
		"student_ids": []int64{studentID}, "staff_ids": staffIDs[:1], "primary_staff_id": staffIDs[0],
	}); err != nil {
		return fmt.Errorf("booking leftover: create hand template: %w", err)
	}
	return nil
}
