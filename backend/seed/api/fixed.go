package api

import (
	"context"
	"encoding/json"
	"fmt"
)

// StaffCredentials stores login credentials for a staff member
type StaffCredentials struct {
	Email    string
	Password string
	PIN      string
	Name     string
	Position string
}

// FixedSeeder seeds fixed demo data via API calls
type FixedSeeder struct {
	client           *Client
	verbose          bool
	roomIDs          map[string]int64   // room name -> id
	personIDs        map[string]int64   // "firstName lastName" -> id (staff only)
	staffIDs         map[string]int64   // "firstName lastName" -> id
	studentIDs       map[string]int64   // "firstName lastName" -> id (student IDs for enrollment)
	studentIDByIndex map[int]int64      // student index -> student ID (for guardian linking)
	studentRFID      map[int64]string   // student ID -> RFID tag
	groupIDs         map[string]int64   // class name -> id
	activityIDs      map[string]int64   // activity name -> id
	activityRoomIDs  map[int64]int64    // activity ID -> room ID (for runtime seeder)
	categoryIDs      map[string]int64   // category name -> id
	deviceKeys       map[string]string  // device ID -> API key
	roleIDs          map[string]int64   // role name -> id
	guardianIDs      map[string]int64   // guardian "firstName lastName" -> id
	staffCredentials []StaffCredentials // created staff credentials for summary
}

// FixedResult contains counts of created entities
type FixedResult struct {
	RoomCount        int
	PersonCount      int
	StaffCount       int
	AccountCount     int
	GroupCount       int
	StudentCount     int
	GuardianCount    int
	ActivityCount    int
	DeviceCount      int
	StaffCredentials []StaffCredentials // Login credentials for demo
}

// NewFixedSeeder creates a new fixed data seeder
func NewFixedSeeder(client *Client, verbose bool) *FixedSeeder {
	return &FixedSeeder{
		client:           client,
		verbose:          verbose,
		roomIDs:          make(map[string]int64),
		personIDs:        make(map[string]int64),
		staffIDs:         make(map[string]int64),
		studentIDs:       make(map[string]int64),
		studentIDByIndex: make(map[int]int64),
		studentRFID:      make(map[int64]string),
		groupIDs:         make(map[string]int64),
		activityIDs:      make(map[string]int64),
		activityRoomIDs:  make(map[int64]int64),
		categoryIDs:      make(map[string]int64),
		deviceKeys:       make(map[string]string),
		roleIDs:          make(map[string]int64),
		guardianIDs:      make(map[string]int64),
		staffCredentials: make([]StaffCredentials, 0),
	}
}

// Seed creates all fixed demo data
func (s *FixedSeeder) Seed(ctx context.Context) (*FixedResult, error) {
	result := &FixedResult{}

	fmt.Println("📦 Creating Fixed Data...")

	// 1. Fetch available roles (needed for account creation)
	if err := s.fetchRoles(ctx); err != nil {
		return nil, fmt.Errorf("failed to fetch roles: %w", err)
	}

	// 2. Create rooms
	if err := s.seedRooms(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed rooms: %w", err)
	}

	// 3. Create persons (staff only, students created via student API)
	if err := s.seedPersons(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed persons: %w", err)
	}

	// 4. Create auth accounts for staff and link to persons
	if err := s.seedStaffAccounts(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed staff accounts: %w", err)
	}

	// 5. Create staff records
	if err := s.seedStaff(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed staff: %w", err)
	}

	// 6. Create education groups
	if err := s.seedGroups(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed groups: %w", err)
	}

	// 7. Create students
	if err := s.seedStudents(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed students: %w", err)
	}

	// 8. Create guardians and link to students
	if err := s.seedGuardians(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed guardians: %w", err)
	}

	// 9. Fetch or create activity categories
	if err := s.fetchCategories(ctx); err != nil {
		return nil, fmt.Errorf("failed to fetch categories: %w", err)
	}

	// 10. Create activities
	if err := s.seedActivities(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed activities: %w", err)
	}

	// 11. Assign supervisors to activities
	if err := s.assignSupervisors(ctx); err != nil {
		return nil, fmt.Errorf("failed to assign supervisors: %w", err)
	}

	// 12. Enroll students in activities
	if err := s.enrollStudents(ctx); err != nil {
		return nil, fmt.Errorf("failed to enroll students: %w", err)
	}

	// 13. Create IoT devices
	if err := s.seedDevices(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed devices: %w", err)
	}

	// Store credentials in result for summary
	result.StaffCredentials = s.staffCredentials

	fmt.Println("✅ Fixed data creation complete!")
	return result, nil
}

func (s *FixedSeeder) seedRooms(_ context.Context, result *FixedResult) error {
	for _, room := range DemoRooms {
		body := map[string]any{
			"name":     room.Name,
			"capacity": room.Capacity,
			"category": room.Category, // German category name for display
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

func (s *FixedSeeder) seedPersons(_ context.Context, result *FixedResult) error {
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

func (s *FixedSeeder) seedStaff(_ context.Context, result *FixedResult) error {
	for _, staff := range DemoStaff {
		personKey := fmt.Sprintf("%s %s", staff.FirstName, staff.LastName)
		personID, ok := s.personIDs[personKey]
		if !ok {
			return fmt.Errorf("person not found for staff %s", personKey)
		}

		body := map[string]any{
			"person_id":   personID,
			"is_teacher":  staff.IsTeacher,
			"staff_notes": fmt.Sprintf("Position: %s", staff.Position),
			"role":        staff.Position, // Role field for teacher record
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

func (s *FixedSeeder) seedGroups(_ context.Context, result *FixedResult) error {
	// Create groups for the 3 classes
	// Note: Group names use uppercase format (1A, 2B, 3C) matching frontend expectations
	classes := []struct {
		key  string // lowercase for internal lookup
		name string // display name
	}{
		{key: "1a", name: "1A"},
		{key: "2b", name: "2B"},
		{key: "3c", name: "3C"},
	}

	for _, class := range classes {
		body := map[string]string{
			"name": class.name,
		}

		respBody, err := s.client.Post("/api/groups", body)
		if err != nil {
			return fmt.Errorf("failed to create group %s: %w", class.name, err)
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

		s.groupIDs[class.key] = resp.Data.ID
		result.GroupCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d education groups created\n", result.GroupCount)
	}
	return nil
}

func (s *FixedSeeder) seedStudents(_ context.Context, result *FixedResult) error {
	// Pickup status options for variety
	pickupStatuses := []string{"Selbständig", "Abholung", "Bus", "Hort"}

	// Health info samples (only some students have health info)
	healthInfoSamples := []string{
		"", // Most students have no health issues
		"",
		"",
		"Laktoseintoleranz",
		"",
		"",
		"Asthma - Notfallspray in Tasche",
		"",
		"",
		"Nussallergie (Epipen vorhanden)",
		"",
		"",
		"",
		"",
		"Diabetes Typ 1 - Insulinpumpe",
	}

	for i, student := range DemoStudents {
		studentKey := fmt.Sprintf("%s %s", student.FirstName, student.LastName)

		groupID, ok := s.groupIDs[student.Class]
		if !ok {
			return fmt.Errorf("group not found for class %s", student.Class)
		}

		// Generate birthday based on class (1a = 6-7 years old, 2b = 7-8, 3c = 8-9)
		baseYear := 2019 // For 6-year-olds
		switch student.Class {
		case "1a":
			baseYear = 2019
		case "2b":
			baseYear = 2018
		case "3c":
			baseYear = 2017
		}
		// Spread birthdays across the year
		month := (i % 12) + 1
		day := (i%28) + 1
		birthday := fmt.Sprintf("%d-%02d-%02d", baseYear, month, day)

		body := map[string]any{
			"first_name":   student.FirstName,
			"last_name":    student.LastName,
			"school_class": student.Class,
			"group_id":     groupID,
			"birthday":     birthday,
		}

		// Add pickup status (rotate through options)
		pickupStatus := pickupStatuses[i%len(pickupStatuses)]
		body["pickup_status"] = pickupStatus

		// Set bus flag for students with "Bus" pickup status
		if pickupStatus == "Bus" {
			bus := true
			body["bus"] = bus
		}

		// Add health info for some students (about 1/3)
		healthInfo := healthInfoSamples[i%len(healthInfoSamples)]
		if healthInfo != "" {
			body["health_info"] = healthInfo
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
		s.studentIDByIndex[i] = resp.Data.ID
		result.StudentCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d students created (with birthday, health info, pickup status)\n", result.StudentCount)
	}
	return nil
}

func (s *FixedSeeder) seedGuardians(_ context.Context, result *FixedResult) error {
	for _, guardian := range DemoGuardians {
		guardianKey := fmt.Sprintf("%s %s", guardian.FirstName, guardian.LastName)

		// 1. Create guardian profile
		body := map[string]any{
			"first_name":               guardian.FirstName,
			"last_name":                guardian.LastName,
			"preferred_contact_method": "email",
			"language_preference":      "de",
		}

		// Add contact methods
		if guardian.Email != "" {
			body["email"] = guardian.Email
		}
		if guardian.Phone != "" {
			body["phone"] = guardian.Phone
		}
		if guardian.MobilePhone != "" {
			body["mobile_phone"] = guardian.MobilePhone
		}

		respBody, err := s.client.Post("/api/guardians", body)
		if err != nil {
			return fmt.Errorf("failed to create guardian %s: %w", guardianKey, err)
		}

		// Parse response to extract guardian ID
		var resp struct {
			Status string `json:"status"`
			Data   struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse guardian response: %w", err)
		}

		guardianID := resp.Data.ID
		s.guardianIDs[guardianKey] = guardianID

		// 2. Link guardian to student
		studentID, ok := s.studentIDByIndex[guardian.StudentIndex]
		if !ok {
			if s.verbose {
				fmt.Printf("    Warning: student index %d not found for guardian %s\n", guardian.StudentIndex, guardianKey)
			}
			continue
		}

		linkPath := fmt.Sprintf("/api/guardians/students/%d/guardians", studentID)
		linkBody := map[string]any{
			"guardian_profile_id":  guardianID,
			"relationship_type":    guardian.Relationship,
			"is_primary":           guardian.IsPrimary,
			"is_emergency_contact": true,
			"can_pickup":           true,
			"emergency_priority":   1,
		}
		if !guardian.IsPrimary {
			linkBody["emergency_priority"] = 2
		}

		_, err = s.client.Post(linkPath, linkBody)
		if err != nil {
			if s.verbose {
				fmt.Printf("    Warning: failed to link guardian %s to student: %v\n", guardianKey, err)
			}
			continue
		}

		result.GuardianCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d guardians created and linked to students\n", result.GuardianCount)
	}
	return nil
}

func (s *FixedSeeder) fetchCategories(_ context.Context) error {
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

func (s *FixedSeeder) seedActivities(_ context.Context, result *FixedResult) error {
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

func (s *FixedSeeder) assignSupervisors(_ context.Context) error {
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

func (s *FixedSeeder) enrollStudents(_ context.Context) error {
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

func (s *FixedSeeder) seedDevices(_ context.Context, result *FixedResult) error {
	for _, device := range DemoDevices {
		body := map[string]any{
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

// fetchRoles retrieves available roles from the API
func (s *FixedSeeder) fetchRoles(_ context.Context) error {
	respBody, err := s.client.Get("/auth/roles")
	if err != nil {
		return fmt.Errorf("failed to fetch roles: %w", err)
	}

	var resp struct {
		Status string `json:"status"`
		Data   []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("failed to parse roles response: %w", err)
	}

	for _, role := range resp.Data {
		s.roleIDs[role.Name] = role.ID
	}

	if s.verbose {
		fmt.Printf("  ✓ %d roles found\n", len(s.roleIDs))
	}
	return nil
}

// seedStaffAccounts creates auth accounts for staff and links them to persons
func (s *FixedSeeder) seedStaffAccounts(_ context.Context, result *FixedResult) error {
	// Get user role ID for staff accounts
	// Available roles: admin, user, guest - staff need "user" role for permissions
	userRoleID, ok := s.roleIDs["user"]
	if !ok {
		return fmt.Errorf("user role not found - available roles: %v", s.roleIDs)
	}

	for i, staff := range DemoStaff {
		personKey := fmt.Sprintf("%s %s", staff.FirstName, staff.LastName)
		personID, ok := s.personIDs[personKey]
		if !ok {
			return fmt.Errorf("person not found for staff account %s", personKey)
		}

		// Generate email and credentials
		email := fmt.Sprintf("%s.%s@example.com",
			normalizeForEmail(staff.FirstName),
			normalizeForEmail(staff.LastName))
		password := "Test1234%"
		pin := fmt.Sprintf("%04d", 1000+i)

		// Create account via /register with role_id
		registerBody := map[string]any{
			"email":            email,
			"username":         fmt.Sprintf("%s.%s", normalizeForEmail(staff.FirstName), normalizeForEmail(staff.LastName)),
			"password":         password,
			"confirm_password": password,
			"role_id":          userRoleID,
		}

		respBody, err := s.client.Post("/auth/register", registerBody)
		if err != nil {
			return fmt.Errorf("failed to create account for %s: %w", personKey, err)
		}

		// Parse response to get account ID
		var resp struct {
			Status string `json:"status"`
			Data   struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse account response: %w", err)
		}

		// Link account to person
		linkPath := fmt.Sprintf("/api/users/%d/account", personID)
		linkBody := map[string]any{
			"account_id": resp.Data.ID,
		}
		_, err = s.client.Put(linkPath, linkBody)
		if err != nil {
			return fmt.Errorf("failed to link account to person %s: %w", personKey, err)
		}

		// Store credentials for summary
		s.staffCredentials = append(s.staffCredentials, StaffCredentials{
			Email:    email,
			Password: password,
			PIN:      pin,
			Name:     personKey,
			Position: staff.Position,
		})

		result.AccountCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d staff accounts created and linked\n", result.AccountCount)
	}
	return nil
}

// normalizeForEmail converts a name to a valid email component
func normalizeForEmail(name string) string {
	// Convert to lowercase
	result := []rune{}
	for _, r := range name {
		switch r {
		case 'ä', 'Ä':
			result = append(result, 'a', 'e')
		case 'ö', 'Ö':
			result = append(result, 'o', 'e')
		case 'ü', 'Ü':
			result = append(result, 'u', 'e')
		case 'ß':
			result = append(result, 's', 's')
		default:
			if r >= 'A' && r <= 'Z' {
				result = append(result, r+32) // lowercase
			} else if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				result = append(result, r)
			}
		}
	}
	return string(result)
}
