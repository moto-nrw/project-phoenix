package api

import (
	"context"
	"fmt"
	"slices"
	"strconv"
)

// regularLunchDaysKey holds the regular lunch weekdays (1 = Monday) of every
// child the families and the school register in the meal plan, by student ID.
// seedRegularBookingsStep books exactly these days (#5023).
const regularLunchDaysKey = "regular-lunch-days"

// enrollmentPhaseBodyKey holds the body the demo phase was created with:
// a phase update replaces every field, so changing its audience needs it.
const enrollmentPhaseBodyKey = "enrollment-phase-body"

// regularBookingShortCareEvery makes every third booked child a
// Kurzbetreuung child and the others Ganztag children.
const regularBookingShortCareEvery = 3

// regularBookingCare is the care of the booked child at position.
func regularBookingCare(position int) string {
	if position%regularBookingShortCareEvery == regularBookingShortCareEvery-1 {
		return "ogs-kurz"
	}
	return "ogs-ganztag"
}

var seedWeekdayKeys = map[int]string{1: "mon", 2: "tue", 3: "wed", 4: "thu", 5: "fri"}

// recordRegularLunchDays notes a child's regular lunch days for the
// bookings that follow.
func recordRegularLunchDays(rt *Runtime, studentID int64, weekdays []int) {
	if rt.Values == nil {
		rt.Values = map[string]any{}
	}
	days, _ := rt.Values[regularLunchDaysKey].(map[int64][]int)
	if days == nil {
		days = map[int64][]int{}
		rt.Values[regularLunchDaysKey] = days
	}
	days[studentID] = slices.Clone(weekdays)
}

// careExitDemoStudentIndexes are the two children from the tail of the
// cohort whose care ends soon (seedCareExitsStep).
func careExitDemoStudentIndexes() []int {
	return []int{len(DemoStudents) - 1, len(DemoStudents) - 2}
}

// bookableDemoStudentIndex returns the DemoStudents index of a child the
// school may give a regular booking: a child of the master data whose care
// does not end soon. An approval renews the child's enrollment window, so it
// would undo a planned care exit; children from the families' applications
// already carry their own bookings.
func bookableDemoStudentIndex(rt *Runtime, studentID int64) (int, bool) {
	if rt.FixedSeeder == nil {
		return 0, false
	}
	for index, id := range rt.FixedSeeder.studentIDByIndex {
		if id != studentID || index < 0 || index >= len(DemoStudents) {
			continue
		}
		return index, !slices.Contains(careExitDemoStudentIndexes(), index)
	}
	return 0, false
}

// seedRegularBookingsStep gives every child of the master data who eats
// regularly in the meal plan an approved booking in the running enrollment
// phase (#5023): Ganztag or Kurzbetreuung plus Mittagessen on exactly its
// lunch days. Before, only the families' applications booked lunch, about
// twenty children against the sixty the meal plan orders for, and
// "Belegt" under Betreuungsangebote looked empty.
//
// The school enters each booking as a manual, approved enrollment, the way
// it records a care contract on paper. While the phase admits only existing
// children, such an enrollment is pinned to the child the school already has
// instead of creating a second one. The demo parent's child already has its
// booking from seedVisitorChildEnrollment and stays out.
//
// The other way round, the children of the approved applications whose
// booking includes lunch eat on their booked days from the day after the
// fixed order day on, so every lunch booking shows in the meal plan too.
type seedRegularBookingsStep struct{}

func (seedRegularBookingsStep) Name() string { return "Seeding regular care bookings" }

func (s seedRegularBookingsStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.FixedSeeder == nil || rt.Enrollment.PhaseID == 0 {
		return fmt.Errorf("regular booking prerequisites not available")
	}
	phaseBody, ok := rt.Values[enrollmentPhaseBodyKey].(map[string]any)
	if !ok {
		return fmt.Errorf("enrollment phase body not available")
	}
	lunchDays, _ := rt.Values[regularLunchDaysKey].(map[int64][]int)
	studentIDs := regularBookingStudents(rt, lunchDays)
	auth := rt.TenantAuth
	shortCare := len(studentIDs) / regularBookingShortCareEvery
	// The bookings of different children are independent, so the school
	// enters them in parallel; the position alone picks the care.
	err := withPhaseAudience(rt, auth, rt.Enrollment.PhaseID, phaseBody, "existing_students", func() error {
		return forEachParallel(rt, studentIDs, func(worker *Runtime, position int, studentID int64) error {
			return s.book(worker, auth, studentID, regularBookingCare(position), lunchDays[studentID])
		})
	})
	if err != nil {
		return err
	}
	eating, err := registerBookedApplicationLunch(rt, auth, lunchDays)
	if err != nil {
		return err
	}
	fmt.Printf("  %d children of the master data booked with lunch: %d Ganztag, %d Kurzbetreuung; %d approved applications eat from %s\n",
		len(studentIDs), len(studentIDs)-shortCare, shortCare, eating, bookedApplicationLunchFrom(todaySeedDate()))
	return nil
}

// bookedApplicationLunchFrom is the first day the approved applications eat:
// the day after the fixed order day, so the order and its Nachmeldungen stay
// as seedMealOrdersStep left them.
func bookedApplicationLunchFrom(today seedDate) seedDate {
	day, _ := mealOrderSeedDay(today)
	return day.AddDays(1)
}

// registerBookedApplicationLunch registers in the meal plan every child of an
// approved application whose booking includes lunch but who does not eat
// yet, on its booked lunch days. Without it the families' applications booked
// lunch for about fifteen children the meal plan never listed (#5023).
func registerBookedApplicationLunch(rt *Runtime, auth AuthRef, lunchDays map[int64][]int) (int, error) {
	lunchOffering := strconv.FormatInt(rt.Enrollment.Offerings["mittagessen"], 10)
	from := bookedApplicationLunchFrom(todaySeedDate())
	registered := 0
	for _, request := range rt.Enrollment.Requests {
		if request.Status != "approved" {
			continue
		}
		children, err := approvedApplicationLunch(rt, auth, request.RequestID, lunchOffering)
		if err != nil {
			return registered, err
		}
		for studentID, weekdays := range children {
			if _, eats := lunchDays[studentID]; eats || len(weekdays) == 0 {
				continue
			}
			if _, err := rt.Client.PutWithAuth(auth, fmt.Sprintf("/api/meal-plan/participation/%d/schedule", studentID), map[string]any{
				"weekdays":       weekdays,
				"effective_from": from.String(),
			}); err != nil {
				return registered, fmt.Errorf("register booked lunch of child %d: %w", studentID, err)
			}
			recordRegularLunchDays(rt, studentID, weekdays)
			registered++
		}
	}
	return registered, nil
}

// approvedApplicationLunch reads an application's approved children with a
// current lunch booking and their lunch weekdays (1 = Monday), by student ID.
func approvedApplicationLunch(rt *Runtime, auth AuthRef, requestID int64, lunchOffering string) (map[int64][]int, error) {
	raw, err := rt.Client.GetWithAuth(auth, fmt.Sprintf("/api/enrollment/admin/requests/%d", requestID))
	if err != nil {
		return nil, fmt.Errorf("load enrollment request %d: %w", requestID, err)
	}
	var response struct {
		Data struct {
			Children []struct {
				Status           string `json:"status"`
				CreatedStudentID string `json:"created_student_id"`
				Offerings        []struct {
					OfferingID    string   `json:"offering_id"`
					SelectedDays  []string `json:"selected_days"`
					AvailableDays []string `json:"available_days"`
				} `json:"offerings"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &response); err != nil {
		return nil, fmt.Errorf("parse enrollment request %d: %w", requestID, err)
	}
	lunch := map[int64][]int{}
	for _, child := range response.Data.Children {
		studentID, err := strconv.ParseInt(child.CreatedStudentID, 10, 64)
		if child.Status != "approved" || err != nil || studentID <= 0 {
			continue
		}
		for _, offering := range child.Offerings {
			if offering.OfferingID != lunchOffering {
				continue
			}
			days := offering.SelectedDays
			if len(days) == 0 {
				days = offering.AvailableDays
			}
			lunch[studentID] = seedWeekdayNumbers(days)
		}
	}
	return lunch, nil
}

// seedWeekdayNumbers turns weekday keys ("mon") into numbers (1 = Monday).
func seedWeekdayNumbers(keys []string) []int {
	numbers := make([]int, 0, len(keys))
	for number := 1; number <= 5; number++ {
		if slices.Contains(keys, seedWeekdayKeys[number]) {
			numbers = append(numbers, number)
		}
	}
	return numbers
}

// regularBookingStudents are the bookable children with regular lunch days,
// in DemoStudents order so the care offerings spread over the classes.
func regularBookingStudents(rt *Runtime, lunchDays map[int64][]int) []int64 {
	var visitorChild int64
	if len(rt.Parents) > 0 && len(rt.Parents[0].StudentIDs) > 0 {
		visitorChild = rt.Parents[0].StudentIDs[0]
	}
	byIndex := map[int]int64{}
	for studentID, days := range lunchDays {
		index, bookable := bookableDemoStudentIndex(rt, studentID)
		if bookable && studentID != visitorChild && len(days) > 0 {
			byIndex[index] = studentID
		}
	}
	indexes := make([]int, 0, len(byIndex))
	for index := range byIndex {
		indexes = append(indexes, index)
	}
	slices.Sort(indexes)
	studentIDs := make([]int64, 0, len(indexes))
	for _, index := range indexes {
		studentIDs = append(studentIDs, byIndex[index])
	}
	return studentIDs
}

// book enters one child's approved booking and checks it landed on that
// child: an unmatched name and birthday would have created a new one.
func (s seedRegularBookingsStep) book(rt *Runtime, auth AuthRef, studentID int64, careKey string, weekdays []int) error {
	index, _ := bookableDemoStudentIndex(rt, studentID)
	child := DemoStudents[index]
	grade, err := demoClassGrade(child.Class)
	if err != nil {
		return fmt.Errorf("demo child %d has no grade in class %q", studentID, child.Class)
	}
	guardian, err := primaryGuardianOf(rt, auth, studentID)
	if err != nil {
		return err
	}
	days := make([]string, 0, len(weekdays))
	for _, weekday := range weekdays {
		days = append(days, seedWeekdayKeys[weekday])
	}
	offerings := rt.Enrollment.Offerings
	body := map[string]any{
		"phase_id":            rt.Enrollment.PhaseID,
		"guardian_first_name": guardian.FirstName,
		"guardian_last_name":  guardian.LastName,
		"guardian_email":      guardian.Email,
		// Only the consent the school confirms; an absent key leaves the
		// child's photo consent as it is.
		"consent_flags": map[string]any{"data_processing": true},
		"children": []map[string]any{{
			"first_name":         child.FirstName,
			"last_name":          child.LastName,
			"date_of_birth":      demoStudentBirthday(index, child, rt.FixedSeeder.seedDay),
			"target_grade_level": grade,
			"offering_ids":       []int64{offerings[careKey], offerings["mittagessen"]},
			"offering_days": []map[string]any{
				{"offering_id": offerings[careKey], "selected_days": days},
				{"offering_id": offerings["mittagessen"], "selected_days": days},
			},
			"custom_data": seedEnrollmentChildAnswers(child.FirstName, child.LastName),
		}},
		"reason":                     "Betreuungsvertrag aus dem laufenden Schuljahr",
		"external_consent_confirmed": true,
		"send_notification":          false,
	}
	raw, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/api/enrollment/phases/%d/manual-approved-enrollments", rt.Enrollment.PhaseID), body)
	if err != nil {
		return fmt.Errorf("book care and lunch for child %d: %w", studentID, err)
	}
	var response struct {
		Data struct {
			StudentID string `json:"student_id"`
			Status    string `json:"status"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &response); err != nil {
		return fmt.Errorf("parse booking of child %d: %w", studentID, err)
	}
	if response.Data.StudentID != strconv.FormatInt(studentID, 10) || response.Data.Status != "approved" {
		return fmt.Errorf("booking of child %d landed on student %q with status %q", studentID, response.Data.StudentID, response.Data.Status)
	}
	return restoreGuardianRelationship(rt, auth, guardian)
}

// seedPrimaryGuardian is a child's primary guardian link as the school
// shows it.
type seedPrimaryGuardian struct {
	FirstName, LastName, Email string
	RelationshipID             int64
	RelationshipType           string
}

// primaryGuardianOf reads the child's primary guardian. The booking names
// that guardian, so the approval links no second one.
func primaryGuardianOf(rt *Runtime, auth AuthRef, studentID int64) (seedPrimaryGuardian, error) {
	raw, err := rt.Client.GetWithAuth(auth, fmt.Sprintf("/api/guardians/students/%d/guardians", studentID))
	if err != nil {
		return seedPrimaryGuardian{}, fmt.Errorf("load guardians of child %d: %w", studentID, err)
	}
	var response struct {
		Data []struct {
			Guardian struct {
				FirstName string  `json:"first_name"`
				LastName  string  `json:"last_name"`
				Email     *string `json:"email"`
			} `json:"guardian"`
			RelationshipID   int64  `json:"relationship_id"`
			RelationshipType string `json:"relationship_type"`
			IsPrimary        bool   `json:"is_primary"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &response); err != nil {
		return seedPrimaryGuardian{}, fmt.Errorf("parse guardians of child %d: %w", studentID, err)
	}
	for _, row := range response.Data {
		if row.IsPrimary && row.Guardian.Email != nil && *row.Guardian.Email != "" {
			return seedPrimaryGuardian{
				FirstName: row.Guardian.FirstName, LastName: row.Guardian.LastName, Email: *row.Guardian.Email,
				RelationshipID: row.RelationshipID, RelationshipType: row.RelationshipType,
			}, nil
		}
	}
	return seedPrimaryGuardian{}, fmt.Errorf("child %d has no primary guardian with an e-mail address", studentID)
}

// restoreGuardianRelationship sets the relationship type back, which the
// approval sets to "guardian" for the primary guardian it confirms; the
// master data keeps "parent".
func restoreGuardianRelationship(rt *Runtime, auth AuthRef, guardian seedPrimaryGuardian) error {
	if guardian.RelationshipType == "" || guardian.RelationshipType == "guardian" {
		return nil
	}
	if _, err := rt.Client.PutWithAuth(auth, fmt.Sprintf("/api/guardians/relationships/%d", guardian.RelationshipID), map[string]any{
		"relationship_type": guardian.RelationshipType,
	}); err != nil {
		return fmt.Errorf("restore guardian relationship %d: %w", guardian.RelationshipID, err)
	}
	return nil
}
