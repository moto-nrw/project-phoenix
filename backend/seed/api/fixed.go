package api

import (
	"context"
	"encoding/json"
	"fmt"
)

// FixedSeeder seeds fixed demo data via API calls
type FixedSeeder struct {
	client          *Client
	verbose         bool
	roomIDs         map[string]int64  // room name -> id
	personIDs       map[string]int64  // "firstName lastName" -> id (staff only)
	staffIDs        map[string]int64  // "firstName lastName" -> id
	studentIDs      map[string]int64  // "firstName lastName" -> id (student IDs for enrollment)
	studentRFID     map[int64]string  // student ID -> RFID tag
	groupIDs        map[string]int64  // class name -> id
	activityIDs     map[string]int64  // activity name -> id
	activityRoomIDs map[int64]int64   // activity ID -> room ID (for runtime seeder)
	categoryIDs     map[string]int64  // category name -> id
	deviceKeys      map[string]string // device ID -> API key
}

// FixedResult contains counts of created entities
type FixedResult struct {
	RoomCount     int
	PersonCount   int
	StaffCount    int
	GroupCount    int
	StudentCount  int
	ActivityCount int
	DeviceCount   int
}

// NewFixedSeeder creates a new fixed data seeder
func NewFixedSeeder(client *Client, verbose bool) *FixedSeeder {
	return &FixedSeeder{
		client:          client,
		verbose:         verbose,
		roomIDs:         make(map[string]int64),
		personIDs:       make(map[string]int64),
		staffIDs:        make(map[string]int64),
		studentIDs:      make(map[string]int64),
		studentRFID:     make(map[int64]string),
		groupIDs:        make(map[string]int64),
		activityIDs:     make(map[string]int64),
		activityRoomIDs: make(map[int64]int64),
		categoryIDs:     make(map[string]int64),
		deviceKeys:      make(map[string]string),
	}
}

// Seed creates all fixed demo data
func (s *FixedSeeder) Seed(ctx context.Context) (*FixedResult, error) {
	result := &FixedResult{}

	fmt.Println("📦 Creating Fixed Data...")

	// 1. Create rooms
	if err := s.seedRooms(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed rooms: %w", err)
	}

	// 2. Create persons (staff + students)
	if err := s.seedPersons(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed persons: %w", err)
	}

	// 3. Create staff records
	if err := s.seedStaff(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed staff: %w", err)
	}

	// 4. Create education groups
	if err := s.seedGroups(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed groups: %w", err)
	}

	// 5. Create students
	if err := s.seedStudents(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed students: %w", err)
	}

	// 6. Fetch or create activity categories
	if err := s.fetchCategories(ctx); err != nil {
		return nil, fmt.Errorf("failed to fetch categories: %w", err)
	}

	// 7. Create activities
	if err := s.seedActivities(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed activities: %w", err)
	}

	// 8. Assign supervisors to activities
	if err := s.assignSupervisors(ctx); err != nil {
		return nil, fmt.Errorf("failed to assign supervisors: %w", err)
	}

	// 9. Enroll students in activities
	if err := s.enrollStudents(ctx); err != nil {
		return nil, fmt.Errorf("failed to enroll students: %w", err)
	}

	// 10. Create IoT devices
	if err := s.seedDevices(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed devices: %w", err)
	}

	fmt.Println("✅ Fixed data creation complete!")
	return result, nil
}

func (s *FixedSeeder) seedRooms(ctx context.Context, result *FixedResult) error {
	for _, room := range DemoRooms {
		// Note: The Room API expects 'category' not 'type'
		// We'll map the type to category
		body := map[string]interface{}{
			"name":     room.Name,
			"capacity": room.Capacity,
			"category": room.Type, // Map type to category
		}

		respBody, err := s.client.Post("/api/rooms", body)
		if err != nil {
			return fmt.Errorf("failed to create room %s: %w", room.Name, err)
		}

		// Parse response to extract ID
		var resp struct {
			Status string `json:"status"`
			Data   struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse room response: %w", err)
		}

		s.roomIDs[room.Name] = resp.Data.ID
		result.RoomCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d rooms created\n", result.RoomCount)
	}
	return nil
}

func (s *FixedSeeder) seedPersons(ctx context.Context, result *FixedResult) error {
	// Create persons for staff only
	// (Students will have persons created automatically via student API)
	for _, staff := range DemoStaff {
		personKey := fmt.Sprintf("%s %s", staff.FirstName, staff.LastName)
		body := map[string]string{
			"first_name": staff.FirstName,
			"last_name":  staff.LastName,
		}

		respBody, err := s.client.Post("/api/users", body)
		if err != nil {
			return fmt.Errorf("failed to create person %s: %w", personKey, err)
		}

		var resp struct {
			Status string `json:"status"`
			Data   struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse person response: %w", err)
		}

		s.personIDs[personKey] = resp.Data.ID
		result.PersonCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d persons created (staff)\n", result.PersonCount)
	}
	return nil
}

func (s *FixedSeeder) seedStaff(ctx context.Context, result *FixedResult) error {
	for _, staff := range DemoStaff {
		personKey := fmt.Sprintf("%s %s", staff.FirstName, staff.LastName)
		personID, ok := s.personIDs[personKey]
		if !ok {
			return fmt.Errorf("person not found for staff %s", personKey)
		}

		body := map[string]interface{}{
			"person_id":   personID,
			"is_teacher":  true,
			"staff_notes": fmt.Sprintf("Role: %s", staff.Role),
		}

		respBody, err := s.client.Post("/api/staff", body)
		if err != nil {
			return fmt.Errorf("failed to create staff %s: %w", personKey, err)
		}

		var resp struct {
			Status string `json:"status"`
			Data   struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse staff response: %w", err)
		}

		s.staffIDs[personKey] = resp.Data.ID
		result.StaffCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d staff created\n", result.StaffCount)
	}
	return nil
}

func (s *FixedSeeder) seedGroups(ctx context.Context, result *FixedResult) error {
	// Create groups for the 3 classes
	classes := []string{"1a", "2b", "3c"}

	for _, class := range classes {
		groupName := fmt.Sprintf("Klasse %s", class)
		body := map[string]string{
			"name": groupName,
		}

		respBody, err := s.client.Post("/api/groups", body)
		if err != nil {
			return fmt.Errorf("failed to create group %s: %w", groupName, err)
		}

		var resp struct {
			Status string `json:"status"`
			Data   struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse group response: %w", err)
		}

		s.groupIDs[class] = resp.Data.ID
		result.GroupCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d education groups created\n", result.GroupCount)
	}
	return nil
}

func (s *FixedSeeder) seedStudents(ctx context.Context, result *FixedResult) error {
	for _, student := range DemoStudents {
		studentKey := fmt.Sprintf("%s %s", student.FirstName, student.LastName)

		groupID, ok := s.groupIDs[student.Class]
		if !ok {
			return fmt.Errorf("group not found for class %s", student.Class)
		}

		body := map[string]interface{}{
			"first_name":   student.FirstName,
			"last_name":    student.LastName,
			"school_class": student.Class,
			"group_id":     groupID,
		}

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
		result.StudentCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d students created (persons created automatically)\n", result.StudentCount)
	}
	return nil
}

func (s *FixedSeeder) fetchCategories(ctx context.Context) error {
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

func (s *FixedSeeder) seedActivities(ctx context.Context, result *FixedResult) error {
	// Map activity names to reasonable category names
	activityCategoryMap := map[string]string{
		"Hausaufgaben": "Hausaufgabenhilfe",
		"Fußball":      "Sport",
		"Basteln":      "Kunst & Basteln",
		"Kochen":       "Kunst & Basteln",
		"Lesen":        "Lesen",
		"Musik":        "Musik",
		"Tanzen":       "Sport",
		"Schach":       "Spiele",
		"Garten":       "Natur & Forschen",
		"Freispiel":    "Schulhof",
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

		body := map[string]interface{}{
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

func (s *FixedSeeder) assignSupervisors(ctx context.Context) error {
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
		body := map[string]interface{}{
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

func (s *FixedSeeder) enrollStudents(ctx context.Context) error {
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

func (s *FixedSeeder) seedDevices(ctx context.Context, result *FixedResult) error {
	for _, device := range DemoDevices {
		body := map[string]interface{}{
			"device_id":   device.DeviceID,
			"name":        device.Name,
			"device_type": "rfid_scanner",
			"status":      "active",
		}

		// Device CRUD routes are at /api/iot/ (not /api/iot/devices)
		// The devices router is mounted at "/" within the IoT router
		respBody, err := s.client.Post("/api/iot/", body)
		if err != nil {
			return fmt.Errorf("failed to create device %s: %w", device.DeviceID, err)
		}

		// Parse response to extract API key
		var resp struct {
			Status string `json:"status"`
			Data   struct {
				ID     int64  `json:"id"`
				APIKey string `json:"api_key"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse device response: %w", err)
		}

		// Store device API key for later use in RuntimeSeeder
		s.deviceKeys[device.DeviceID] = resp.Data.APIKey

		result.DeviceCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d IoT devices created\n", result.DeviceCount)
	}
	return nil
}
