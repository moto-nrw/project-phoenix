package api

import (
	"context"
	"encoding/json"
	"fmt"
)

func (s *FixedSeeder) seedStudents(_ context.Context, result *FixedResult) error {
	for i, student := range DemoStudents {
		studentKey := fmt.Sprintf("%s %s", student.FirstName, student.LastName)

		groupID, ok := s.groupIDs[student.GroupKey]
		if !ok {
			return fmt.Errorf("group not found for group key %s", student.GroupKey)
		}

		// Generate birthday based on group (varied ages within each group)
		baseYear := 2019 // For 6-year-olds
		switch student.GroupKey {
		case "sternengruppe":
			baseYear = 2019
		case "bärengruppe":
			baseYear = 2018
		case "sonnengruppe":
			baseYear = 2017
		}
		// Spread birthdays across the year
		month := (i % 12) + 1
		day := (i % 28) + 1
		birthday := fmt.Sprintf("%d-%02d-%02d", baseYear, month, day)

		body := map[string]any{
			"first_name":   student.FirstName,
			"last_name":    student.LastName,
			"school_class": student.Class,
			"group_id":     groupID,
			"birthday":     birthday,
		}

		// Add pickup status (rotate through options)
		pickupStatus := DemoPickupStatuses[i%len(DemoPickupStatuses)]
		body["pickup_status"] = pickupStatus

		// Set bus flag for some students (every 5th student is a "Buskind")
		if i%5 == 0 {
			body["bus"] = true
		}

		// Add health info for demo
		healthInfo := DemoHealthInfoSamples[i%len(DemoHealthInfoSamples)]
		body["health_info"] = healthInfo

		// Add supervisor notes for demo
		supervisorNotes := DemoSupervisorNotesSamples[i%len(DemoSupervisorNotesSamples)]
		body["supervisor_notes"] = supervisorNotes

		// Add extra info for demo
		extraInfo := DemoExtraInfoSamples[i%len(DemoExtraInfoSamples)]
		body["extra_info"] = extraInfo

		respBody, err := s.client.Post("/api/students", body)
		if err != nil {
			return fmt.Errorf("failed to create student %s: %w", studentKey, err)
		}

		// Parse response to extract student ID for enrollment
		var resp struct {
			Status string `json:"status"`
			Data   struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse student response: %w", err)
		}

		s.studentIDs[studentKey] = resp.Data.ID
		s.studentIDByIndex[i] = resp.Data.ID
		result.StudentCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d students created (with birthday, health info, pickup status, supervisor notes, extra info)\n", result.StudentCount)
	}
	return nil
}

// MarkStudentsSick marks students as sick for demo badges
// Per group: 1 checked-in student (sick at school) + 1 not checked-in (sick at home)
// This should be called AFTER runtime seeding to avoid auto-clear on check-in
func (s *FixedSeeder) MarkStudentsSick(_ context.Context, result *FixedResult) error {
	// Get set of checked-in student IDs
	checkedInIDs, err := s.getCheckedInStudentIDs()
	if err != nil {
		return fmt.Errorf("failed to get checked-in students: %w", err)
	}

	// Track per group: need 1 checked-in sick, 1 not-checked-in sick
	groupCheckedInSick := make(map[string]bool)    // groupKey -> has checked-in sick student
	groupNotCheckedInSick := make(map[string]bool) // groupKey -> has not-checked-in sick student

	for i, student := range DemoStudents {
		studentID, ok := s.studentIDByIndex[i]
		if !ok {
			continue
		}

		isCheckedIn := checkedInIDs[studentID]

		// Check if we need this type of sick student for this group
		needCheckedInSick := !groupCheckedInSick[student.GroupKey] && isCheckedIn
		needNotCheckedInSick := !groupNotCheckedInSick[student.GroupKey] && !isCheckedIn

		if !needCheckedInSick && !needNotCheckedInSick {
			continue
		}

		// Mark student as sick
		path := fmt.Sprintf("/api/students/%d", studentID)
		body := map[string]any{
			"sick": true,
		}

		_, err := s.client.Put(path, body)
		if err != nil {
			return fmt.Errorf("failed to mark student %s %s as sick: %w",
				student.FirstName, student.LastName, err)
		}

		if isCheckedIn {
			groupCheckedInSick[student.GroupKey] = true
		} else {
			groupNotCheckedInSick[student.GroupKey] = true
		}
		result.SickStudentCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d students marked as sick (demo badges)\n", result.SickStudentCount)
	}
	return nil
}

// getCheckedInStudentIDs returns a set of student IDs that are currently checked in
func (s *FixedSeeder) getCheckedInStudentIDs() (map[int64]bool, error) {
	checkedIn := make(map[int64]bool)

	// Query active visits to find checked-in students
	respBody, err := s.client.Get("/api/active/visits")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch active visits: %w", err)
	}

	var resp struct {
		Status string `json:"status"`
		Data   []struct {
			StudentID int64 `json:"student_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse visits response: %w", err)
	}

	for _, visit := range resp.Data {
		checkedIn[visit.StudentID] = true
	}

	return checkedIn, nil
}
