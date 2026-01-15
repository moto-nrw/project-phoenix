package api

import (
	"encoding/json"
	"fmt"
)

// fetchCategories fetches existing activity categories from the API
func (s *FixedSeeder) fetchCategories() error {
	// Fetch existing categories
	respBody, err := s.client.Get("/api/activities/categories")
	if err != nil {
		return fmt.Errorf("failed to fetch categories: %w", err)
	}

	var resp struct {
		Status string `json:"status"`
		Data   []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("failed to parse categories response: %w", err)
	}

	// Build category map
	for _, cat := range resp.Data {
		s.categoryIDs[cat.Name] = cat.ID
	}

	// For demo, we'll use the first available category for all activities
	// Or create a default "Sport" category if none exist
	if len(s.categoryIDs) == 0 {
		return fmt.Errorf("no categories found - please seed categories first")
	}

	if s.verbose {
		fmt.Printf("  ✓ %d categories found\n", len(s.categoryIDs))
	}
	return nil
}

// seedActivities creates activity records via API
func (s *FixedSeeder) seedActivities(result *FixedResult) error {
	// Map activity names to category names that exist in the database
	// Available categories: Draußen, Gruppenraum, Hausaufgaben, Kreativ, Lernen, Mensa, Musik, Spiele, Sport
	activityCategoryMap := map[string]string{
		"Hausaufgaben": "Hausaufgaben",
		"Fußball":      "Sport",
		"Basteln":      "Kreativ",
		"Kochen":       "Mensa",
		"Lesen":        "Lernen",
		"Musik":        "Musik",
		"Tanzen":       "Sport",
		"Schach":       "Spiele",
		"Garten":       "Draußen",
		"Freispiel":    "Draußen",
	}

	for _, activity := range DemoActivities {
		roomID, ok := s.roomIDs[activity.DefaultRoom]
		if !ok {
			return fmt.Errorf("room not found: %s", activity.DefaultRoom)
		}

		// Get category ID (fallback to first available)
		categoryName := activityCategoryMap[activity.Name]
		categoryID, ok := s.categoryIDs[categoryName]
		if !ok {
			// Use first available category
			for _, id := range s.categoryIDs {
				categoryID = id
				break
			}
		}

		body := map[string]any{
			"name":             activity.Name,
			"max_participants": 20,
			"is_open":          true,
			"category_id":      categoryID,
			"planned_room_id":  roomID,
		}

		respBody, err := s.client.Post("/api/activities", body)
		if err != nil {
			return fmt.Errorf("failed to create activity %s: %w", activity.Name, err)
		}

		var resp struct {
			Status string `json:"status"`
			Data   struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse activity response: %w", err)
		}

		s.activityIDs[activity.Name] = resp.Data.ID
		s.activityRoomIDs[resp.Data.ID] = roomID // Store activity → room mapping for runtime seeder
		result.ActivityCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d activities created\n", result.ActivityCount)
	}
	return nil
}

// assignSupervisors assigns staff members as supervisors to activities
func (s *FixedSeeder) assignSupervisors() error {
	// Assign first staff member as supervisor to each activity
	if len(DemoStaff) == 0 || len(s.staffIDs) == 0 {
		return fmt.Errorf("no staff available for supervisor assignment")
	}

	// Get first staff ID
	var firstStaffID int64
	firstStaffKey := fmt.Sprintf("%s %s", DemoStaff[0].FirstName, DemoStaff[0].LastName)
	firstStaffID = s.staffIDs[firstStaffKey]

	// Assign to each activity
	for activityName, activityID := range s.activityIDs {
		path := fmt.Sprintf("/api/activities/%d/supervisors", activityID)
		body := map[string]any{
			"staff_id":   firstStaffID,
			"is_primary": true,
		}

		_, err := s.client.Post(path, body)
		if err != nil {
			return fmt.Errorf("failed to assign supervisor to activity %s: %w", activityName, err)
		}
	}

	if s.verbose {
		fmt.Printf("  ✓ Supervisors assigned to activities\n")
	}
	return nil
}

// enrollStudents enrolls students in activities
func (s *FixedSeeder) enrollStudents() error {
	// Enroll first 5 students in each activity
	maxEnrollmentsPerActivity := 5
	studentCount := 0

	for activityName, activityID := range s.activityIDs {
		enrolled := 0
		for _, student := range DemoStudents {
			if enrolled >= maxEnrollmentsPerActivity {
				break
			}

			studentKey := fmt.Sprintf("%s %s", student.FirstName, student.LastName)
			studentID, ok := s.studentIDs[studentKey]
			if !ok {
				if s.verbose {
					fmt.Printf("    Warning: student ID not found for %s\n", studentKey)
				}
				continue
			}

			path := fmt.Sprintf("/api/activities/%d/students/%d", activityID, studentID)
			_, err := s.client.Post(path, nil)
			if err != nil {
				// Log but continue on enrollment errors
				if s.verbose {
					fmt.Printf("    Warning: failed to enroll student in %s: %v\n", activityName, err)
				}
				continue
			}

			enrolled++
			studentCount++
		}
	}

	if s.verbose {
		fmt.Printf("  ✓ %d student enrollments created\n", studentCount)
	}
	return nil
}
