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
	staffIDs         map[string]int64   // "firstName lastName" -> staff id
	teacherIDs       map[string]int64   // "firstName lastName" -> teacher id (for group assignment)
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
	SickStudentCount int // Students marked as sick for demo badges
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
		teacherIDs:       make(map[string]int64),
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

		// Add building if specified
		if room.Building != "" {
			body["building"] = room.Building
		}

		// Add floor if specified (can be 0, so check for nil)
		if room.Floor != nil {
			body["floor"] = *room.Floor
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
				ID        int64 `json:"id"`
				TeacherID int64 `json:"teacher_id,omitempty"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse staff response: %w", err)
		}

		s.staffIDs[personKey] = resp.Data.ID
		// Store teacher ID if this is a teacher (for group assignment)
		if resp.Data.TeacherID > 0 {
			s.teacherIDs[personKey] = resp.Data.TeacherID
		}
		result.StaffCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d staff created (%d teachers)\n", result.StaffCount, len(s.teacherIDs))
	}
	return nil
}

func (s *FixedSeeder) seedGroups(_ context.Context, result *FixedResult) error {
	// Create groups with themed names (typical for German OGS)
	// Each teacher gets at least one group so they see "Meine Gruppe" in frontend
	// Teacher distribution:
	//   Sternengruppe: Anna Müller, Thomas Weber, Sarah Schmidt (3 teachers)
	//   Bärengruppe: Michael Hoffmann, Lisa Wagner (2 teachers)
	//   Sonnengruppe: Jan Becker, Maria Fischer (2 teachers)
	classes := []struct {
		key      string   // lowercase for internal lookup
		name     string   // display name
		teachers []string // teacher names (must match DemoStaff)
	}{
		{key: "sternengruppe", name: "Sternengruppe", teachers: []string{"Anna Müller", "Thomas Weber", "Sarah Schmidt"}},
		{key: "bärengruppe", name: "Bärengruppe", teachers: []string{"Michael Hoffmann", "Lisa Wagner"}},
		{key: "sonnengruppe", name: "Sonnengruppe", teachers: []string{"Jan Becker", "Maria Fischer"}},
	}

	for _, class := range classes {
		// Collect teacher IDs for this group
		teacherIDsForGroup := []int64{}
		for _, teacherName := range class.teachers {
			if teacherID, ok := s.teacherIDs[teacherName]; ok {
				teacherIDsForGroup = append(teacherIDsForGroup, teacherID)
			}
		}

		body := map[string]any{
			"name":        class.name,
			"teacher_ids": teacherIDsForGroup,
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
		fmt.Printf("  ✓ %d education groups created (with teacher assignments)\n", result.GroupCount)
	}
	return nil
}

func (s *FixedSeeder) seedStudents(_ context.Context, result *FixedResult) error {
	// Pickup status options - MUST match frontend dropdown values exactly!
	// Frontend: student-personal-info-form.tsx defines these options
	pickupStatuses := []string{"Geht alleine nach Hause", "Wird abgeholt"}

	// Health info samples - every student gets health information for demo
	healthInfoSamples := []string{
		"Keine bekannten Allergien",
		"Laktoseintoleranz",
		"Asthma - Notfallspray in Tasche",
		"Nussallergie (Epipen vorhanden)",
		"Diabetes Typ 1 - Insulinpumpe",
		"Glutenunverträglichkeit",
		"Heuschnupfen (saisonal)",
		"Neurodermitis - Creme in Tasche",
		"Keine bekannten Allergien",
		"Leichte Pollenallergie",
		"Keine Medikamente notwendig",
		"Brille zum Lesen erforderlich",
		"Hörgerät links",
		"Keine bekannten Allergien",
		"Eierallergie - bitte bei Essen beachten",
	}

	// Supervisor notes samples - every student gets supervisor notes for demo
	supervisorNotesSamples := []string{
		"Sehr sportlich und aktiv",
		"Braucht manchmal etwas mehr Zeit bei Übergängen",
		"Freut sich besonders auf Bastelaktivitäten",
		"Hat Schwierigkeiten mit lauten Geräuschen",
		"Sehr hilfsbereit bei jüngeren Kindern",
		"Muss um 15:30 Uhr abgeholt werden (Musikunterricht)",
		"Liebt Bücher und liest gerne vor",
		"Spielt gerne Fußball in der Pause",
		"Braucht klare Strukturen und Ansagen",
		"Ist sehr kreativ und malt gerne",
		"Hilft gerne beim Aufräumen",
		"Braucht regelmäßige Bewegungspausen",
		"Arbeitet gut in kleinen Gruppen",
		"Mag Musik und singt gerne",
		"Ist manchmal schüchtern bei neuen Aktivitäten",
	}

	// Extra info samples - every student gets parent notes for demo
	extraInfoSamples := []string{
		"Vegetarische Ernährung",
		"Geschwisterkind in Klasse 2b",
		"Neu an der Schule seit September",
		"Förderunterricht Deutsch",
		"Eltern arbeiten beide, Oma holt manchmal ab",
		"Hat einen jüngeren Bruder im Kindergarten",
		"Nimmt Klavierunterricht donnerstags",
		"Spielt im Fußballverein",
		"Familie spricht zuhause Türkisch",
		"Geht zum Schwimmunterricht mittwochs",
		"Hat eine ältere Schwester in Klasse 4",
		"Eltern sind geschieden, wechselnde Abholung",
		"Nimmt an der Theatergruppe teil",
		"Liebt Tiere, hat einen Hund zuhause",
		"Familie ist kürzlich umgezogen",
	}

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
		pickupStatus := pickupStatuses[i%len(pickupStatuses)]
		body["pickup_status"] = pickupStatus

		// Set bus flag for some students (every 5th student is a "Buskind")
		if i%5 == 0 {
			body["bus"] = true
		}

		// Add health info for some students (about 1/3)
		healthInfo := healthInfoSamples[i%len(healthInfoSamples)]
		if healthInfo != "" {
			body["health_info"] = healthInfo
		}

		// Add supervisor notes for some students
		supervisorNotes := supervisorNotesSamples[i%len(supervisorNotesSamples)]
		if supervisorNotes != "" {
			body["supervisor_notes"] = supervisorNotes
		}

		// Add extra info for some students
		extraInfo := extraInfoSamples[i%len(extraInfoSamples)]
		if extraInfo != "" {
			body["extra_info"] = extraInfo
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
	// Get role IDs for different staff types
	adminRoleID, ok := s.roleIDs["admin"]
	if !ok {
		return fmt.Errorf("admin role not found - available roles: %v", s.roleIDs)
	}
	teacherRoleID, ok := s.roleIDs["teacher"]
	if !ok {
		return fmt.Errorf("teacher role not found - available roles: %v", s.roleIDs)
	}
	guestRoleID, ok := s.roleIDs["guest"]
	if !ok {
		return fmt.Errorf("guest role not found - available roles: %v", s.roleIDs)
	}

	for i, staff := range DemoStaff {
		personKey := fmt.Sprintf("%s %s", staff.FirstName, staff.LastName)
		personID, ok := s.personIDs[personKey]
		if !ok {
			return fmt.Errorf("person not found for staff account %s", personKey)
		}

		// Generate email and credentials for demo accounts
		// Email: demo{n}@mail.de where n = account number (1-20)
		// Password: hardcoded unique passwords so accounts survive cronjob resets
		demoPasswords := []string{
			"sdlXK26%", "mQp9Wy3$", "kJt4Nz8!", "hBv7Rx5@", "fGn2Lm6#",
			"pYc8Dq1&", "wZa3Ks9*", "vTe5Hj4%", "xUi6Fo7$", "cRo1Pn2!",
			"bWs4Mv8@", "nLk7Qx3#", "jHd9Zt5&", "gFa2Yc6*", "tEr8Ub1%",
			"qDm3Wp4$", "yKn5Sj7!", "uBx6Gi9@", "iCv1Lh2#", "oAz4Rk8&",
		}
		accountNum := i + 1
		email := fmt.Sprintf("demo%d@mail.de", accountNum)
		password := demoPasswords[i]
		pin := fmt.Sprintf("%04d", 1000+i)

		// Assign role based on position:
		// - OGS-Büro → admin (OGS leadership with full access)
		// - Extern → guest (external helpers with limited access)
		// - Pädagogische Fachkraft → teacher (standard pedagogical staff)
		var roleID int64
		switch staff.Position {
		case "OGS-Büro":
			roleID = adminRoleID
		case "Extern":
			roleID = guestRoleID
		default:
			roleID = teacherRoleID
		}

		// Create account via /register with role_id
		registerBody := map[string]any{
			"email":            email,
			"username":         fmt.Sprintf("demo%d", accountNum),
			"password":         password,
			"confirm_password": password,
			"role_id":          roleID,
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
