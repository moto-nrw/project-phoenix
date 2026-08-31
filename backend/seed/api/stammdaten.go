package api

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
	staffPassword    string             // optional: shared password for all staff accounts (from CLI flag)
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
	RoomCount             int
	PersonCount           int
	StaffCount            int
	AccountCount          int
	GroupCount            int
	StudentCount          int
	SickStudentCount      int // Students marked as sick for demo badges
	GuardianCount         int
	PayerCount            int // Children with a guardian marked as payer (#2608)
	GuardianIBANCount     int // Guardians with bank details stored (#2608)
	PickupScheduleCount   int // Students with weekly pickup schedules seeded
	ClassArrivalTimeCount int // Classes with seeded arrival times
	ActivityCount         int
	DeviceCount           int
	StaffCredentials      []StaffCredentials // Login credentials for demo
}

// NewFixedSeeder creates a new fixed data seeder.
// staffPassword is optional: when non-empty, all 20 staff accounts use this
// password instead of the per-account defaults from demoPasswords.
func NewFixedSeeder(client *Client, verbose bool, staffPassword string) *FixedSeeder {
	return &FixedSeeder{
		client:           client,
		verbose:          verbose,
		staffPassword:    staffPassword,
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

	// 3. Create auth accounts and linked persons for staff
	//    (students get persons created automatically via student API)
	if err := s.seedStaffAccounts(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed staff accounts: %w", err)
	}

	// 4. Create staff records
	if err := s.seedStaff(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed staff: %w", err)
	}

	// 5b. Re-login as first OGS-Büro staff member.
	// The admin account (from migration) has no staff record, so endpoints
	// like activities that require a staff context will reject it.
	// OGS-Büro staff have admin role + a staff/person/teacher record.
	if err := s.switchToStaffAccount(); err != nil {
		return nil, fmt.Errorf("failed to switch to staff account: %w", err)
	}

	// 6. Create education groups
	if err := s.seedGroups(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed groups: %w", err)
	}
	if err := s.seedGroupHandover(ctx); err != nil {
		return nil, fmt.Errorf("failed to seed group handover: %w", err)
	}

	// 7. Create students
	if err := s.seedStudents(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed students: %w", err)
	}
	if err := s.seedClassArrivalTimes(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed class arrival times: %w", err)
	}

	// 8. Create guardians and link to students
	if err := s.seedGuardians(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed guardians: %w", err)
	}

	// 8a. Store bank details on the paying guardian and mark who pays per child
	if err := s.seedGuardianPayments(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed guardian payments: %w", err)
	}

	// 8b. Create weekly pickup schedules for students who are picked up
	if err := s.seedPickupSchedules(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed pickup schedules: %w", err)
	}

	// 9. Re-authenticate as a staff member for activity creation
	// The activities API requires a linked staff record (created_by field).
	// The initial admin account may not have a staff record, so we switch
	// to the first demo staff account which was created in steps 3-5.
	if err := s.loginAsStaff(); err != nil {
		return nil, fmt.Errorf("failed to re-authenticate as staff: %w", err)
	}

	// 10. Fetch or create activity categories
	if err := s.fetchCategories(ctx); err != nil {
		return nil, fmt.Errorf("failed to fetch categories: %w", err)
	}

	// 11. Create activities
	if err := s.seedActivities(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed activities: %w", err)
	}

	// 12. Assign supervisors to activities
	if err := s.assignSupervisors(ctx); err != nil {
		return nil, fmt.Errorf("failed to assign supervisors: %w", err)
	}

	// 13. Enroll students in activities
	if err := s.enrollStudents(ctx); err != nil {
		return nil, fmt.Errorf("failed to enroll students: %w", err)
	}

	// 14. Create IoT devices
	if err := s.seedDevices(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed devices: %w", err)
	}

	// Store credentials in result for summary
	result.StaffCredentials = s.staffCredentials

	fmt.Println("✅ Fixed data creation complete!")
	return result, nil
}

func (s *FixedSeeder) seedClassArrivalTimes(_ context.Context, result *FixedResult) error {
	classes := make(map[string]struct{})
	for _, student := range DemoStudents {
		classes[student.Class] = struct{}{}
	}

	classNames := make([]string, 0, len(classes))
	for class := range classes {
		classNames = append(classNames, class)
	}
	sort.Strings(classNames)

	schedules := []map[string]any{
		{"weekday": 1, "expected_arrival": "11:45"},
		{"weekday": 2, "expected_arrival": "11:45"},
		{"weekday": 3, "expected_arrival": "12:45"},
		{"weekday": 4, "expected_arrival": "11:45"},
		{"weekday": 5, "expected_arrival": "11:45"},
	}
	for _, class := range classNames {
		_, err := s.client.Post("/api/students/arrival-schedules/bulk", map[string]any{
			"school_class": class,
			"schedules":    schedules,
		})
		if err != nil {
			return fmt.Errorf("seed class arrival times for %s: %w", class, err)
		}
		result.ClassArrivalTimeCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d class arrival times seeded\n", result.ClassArrivalTimeCount)
	}
	return nil
}

// switchToStaffAccount re-authenticates as the first OGS-Büro staff member.
// This is needed because many endpoints (activities, groups, etc.) require the
// caller to have a staff record, which the migration-created admin account lacks.
func (s *FixedSeeder) switchToStaffAccount() error {
	for _, cred := range s.staffCredentials {
		if cred.Position == "OGS-Büro" {
			fmt.Printf("  Switching to staff account %s (%s)...\n", cred.Name, cred.Email)
			if err := s.client.Login(cred.Email, cred.Password, ""); err != nil {
				return fmt.Errorf("failed to login as %s: %w", cred.Email, err)
			}
			return nil
		}
	}
	return fmt.Errorf("no OGS-Büro staff credential found")
}

// loginAsStaff re-authenticates as the first demo staff account.
// Required because the activities API needs a linked staff record (created_by).
func (s *FixedSeeder) loginAsStaff() error {
	if len(s.staffCredentials) == 0 {
		return fmt.Errorf("no staff credentials available — staff accounts must be created first")
	}

	creds := s.staffCredentials[0]
	fmt.Printf("🔄 Re-authenticating as %s (%s) for activity creation...\n", creds.Name, creds.Email)

	if err := s.client.Login(creds.Email, creds.Password, ""); err != nil {
		return fmt.Errorf("failed to login as %s: %w", creds.Email, err)
	}

	fmt.Println("✓ Authenticated as staff member")
	return nil
}

func (s *FixedSeeder) seedRooms(_ context.Context, result *FixedResult) error {
	// Deliberately outside the status-badge palette: room colors must be
	// tenant-unique and the API rejects colors reserved for presence states.
	roomColors := []string{"#1B5E20", "#2E7D32", "#33691E", "#827717", "#E65100", "#BF360C", "#4A148C", "#6A1B9A", "#283593", "#006064", "#37474F"}
	for index, room := range DemoRooms {
		body := map[string]any{
			"name":     room.Name,
			"capacity": room.Capacity,
			"category": room.Category, // German category name for display
			"color":    roomColors[index],
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

func (s *FixedSeeder) seedStaff(_ context.Context, result *FixedResult) error {
	for _, staff := range DemoStaff {
		personKey := fmt.Sprintf("%s %s", staff.FirstName, staff.LastName)
		personID, ok := s.personIDs[personKey]
		if !ok {
			return fmt.Errorf("person not found for staff %s", personKey)
		}

		// Map position to display role (matches auth role names for clarity)
		displayRole := "Betreuer" // Default for Pädagogische Fachkraft
		switch staff.Position {
		case "OGS-Büro":
			displayRole = "Admin"
		case "Extern":
			displayRole = "Extern"
		}

		body := map[string]any{
			"person_id":   personID,
			"is_teacher":  staff.IsTeacher,
			"staff_notes": fmt.Sprintf("Position: %s", staff.Position),
			"role":        displayRole, // Display role for badge (Admin/Betreuer/Extern)
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
	// Create 10 groups with themed names (typical for German OGS)
	// Each Pädagogische Fachkraft gets exactly one group; Wiesengruppe is
	// additionally assigned to an OGS-Büro teacher.
	// This ensures every Betreuer sees "Meine Gruppe" in the frontend
	// Note: OGS-Büro staff (demo1-demo10) are admins and see ALL groups
	classes := []struct {
		key      string   // lowercase for internal lookup
		name     string   // display name
		teachers []string // teacher names (must match DemoStaff Pädagogische Fachkräfte)
	}{
		{key: "sternengruppe", name: "Sternengruppe", teachers: []string{"Julia Klein"}},
		{key: "bärengruppe", name: "Bärengruppe", teachers: []string{"Markus Wolf"}},
		{key: "sonnengruppe", name: "Sonnengruppe", teachers: []string{"Sandra Schröder"}},
		{key: "mondgruppe", name: "Mondgruppe", teachers: []string{"Christian Neumann"}},
		{key: "regenbogengruppe", name: "Regenbogengruppe", teachers: []string{"Nicole Schwarz"}},
		{key: "blumengruppe", name: "Blumengruppe", teachers: []string{"Frank Zimmermann"}},
		{key: "schmetterlingsgruppe", name: "Schmetterlingsgruppe", teachers: []string{"Birgit Braun"}},
		{key: "waldgruppe", name: "Waldgruppe", teachers: []string{"Jörg Krüger"}},
		{key: "meeresgruppe", name: "Meeresgruppe", teachers: []string{"Heike Hartmann"}},
		{key: "wiesengruppe", name: "Wiesengruppe", teachers: []string{"Anna Müller"}},
	}

	for index, class := range classes {
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
		roomNames := []string{"OGS-Raum 1", "OGS-Raum 2", "OGS-Raum 3", "Kreativraum"}
		if roomID := s.roomIDs[roomNames[index%len(roomNames)]]; roomID > 0 {
			body["room_id"] = roomID
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
		// Groups map to school classes: 1a/1b (born ~2019), 2a/2b (~2018), 3a/3b (~2017), 4a/4b (~2016)
		baseYear := 2019
		switch student.GroupKey {
		case "sternengruppe": // Klasse 1a/1b
			baseYear = 2019
		case "bärengruppe": // Klasse 1b/2a
			baseYear = 2018
		case "sonnengruppe": // Klasse 2a/2b
			baseYear = 2018
		case "mondgruppe": // Klasse 2b/3a
			baseYear = 2017
		case "regenbogengruppe": // Klasse 3a/3b
			baseYear = 2017
		case "blumengruppe": // Klasse 3b/4a
			baseYear = 2016
		case "schmetterlingsgruppe": // Klasse 4a/4b
			baseYear = 2016
		case "waldgruppe": // Klasse 1a/2a
			baseYear = 2019
		case "meeresgruppe": // Klasse 2b/3b
			baseYear = 2017
		case "wiesengruppe": // Klasse 3a/4b
			baseYear = 2016
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
		if i%3 == 0 {
			body["address_street"] = fmt.Sprintf("Schulstraße %d", i+1)
			body["address_postal_code"] = "49074"
			body["address_city"] = "Osnabrück"
		}
		if i%4 == 0 {
			body["photo_consent_given"] = true
		}

		// Add pickup status (rotate through options)
		pickupStatus := pickupStatuses[i%len(pickupStatuses)]
		body["pickup_status"] = pickupStatus

		// Set bus days for some students (every 5th student is a "Buskind").
		// bus_days is the single source of truth (#1582).
		if i%5 == 0 {
			body["bus_days"] = map[string]bool{
				"mon": true, "tue": true, "wed": true, "thu": true, "fri": true,
			}
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
	for index, guardian := range DemoGuardians {
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
		if index%3 == 0 {
			body["address_street"] = fmt.Sprintf("Familienweg %d", index+1)
			body["address_postal_code"] = "49074"
			body["address_city"] = "Osnabrück"
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
		if index >= 6 && index%10 == 0 {
			linkBody["guardian_role"] = "pickup_only"
			linkBody["relationship_type"] = "relative"
			linkBody["is_emergency_contact"] = false
		} else if index >= 6 && index%11 == 0 {
			linkBody["guardian_role"] = "custom"
			linkBody["relationship_type"] = "other"
			linkBody["is_emergency_contact"] = false
			linkBody["can_pickup"] = false
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

// demoIBANBLZ is the Bundesbank test Bankleitzahl block used for the demo
// IBANs. Demo data only — no such account exists.
const demoIBANBLZ = "37040044"

// demoIBAN builds a structurally valid German IBAN with a correct ISO 13616
// mod-97 checksum, so seeded rows survive the same validation the UI applies.
// Deterministic in index: reseeding a machine yields the same list.
func demoIBAN(index int) string {
	account := fmt.Sprintf("%010d", 5320130+index)
	bban := demoIBANBLZ + account
	// Rearranged form for the checksum: BBAN + country code + "00".
	remainder := 0
	for _, r := range bban + "DE00" {
		switch {
		case r >= '0' && r <= '9':
			remainder = (remainder*10 + int(r-'0')) % 97
		default:
			remainder = (remainder*100 + int(r-'A') + 10) % 97
		}
	}
	return fmt.Sprintf("DE%02d%s", 98-remainder, bban)
}

// seedGuardianPayments stores an IBAN on every primary guardian and marks that
// guardian as the payer of their child (#2608).
//
// Two demo gaps are deliberate: every fourth child keeps its payer but gets no
// IBAN, and every seventh child gets no payer at all. The Bankverbindungen
// list exists to show exactly those gaps, and a demo where every row is
// complete never shows what the screen is for.
func (s *FixedSeeder) seedGuardianPayments(_ context.Context, result *FixedResult) error {
	ibanIndex := 0
	for _, guardian := range DemoGuardians {
		if !guardian.IsPrimary {
			continue
		}
		guardianKey := fmt.Sprintf("%s %s", guardian.FirstName, guardian.LastName)
		guardianID, ok := s.guardianIDs[guardianKey]
		if !ok {
			continue
		}
		studentID, ok := s.studentIDByIndex[guardian.StudentIndex]
		if !ok {
			continue
		}

		ibanIndex++
		if guardian.StudentIndex%7 == 6 {
			// No payer assigned: the child still has guardians, nobody was
			// marked. This is the row the list flags as unassigned.
			continue
		}

		payerPath := fmt.Sprintf("/api/guardians/students/%d/payer", studentID)
		if _, err := s.client.Put(payerPath, map[string]any{
			"guardian_id": fmt.Sprintf("%d", guardianID),
		}); err != nil {
			if s.verbose {
				fmt.Printf("    Warning: failed to mark payer for student %d: %v\n", studentID, err)
			}
			continue
		}
		result.PayerCount++

		if guardian.StudentIndex%4 == 3 {
			// Payer known, bank details still missing.
			continue
		}

		body := map[string]any{"iban": demoIBAN(ibanIndex)}
		if guardian.StudentIndex%5 == 2 {
			// A Kontoinhaber that differs from the guardian: the account runs
			// on the partner's name.
			body["account_holder"] = fmt.Sprintf("%s %s", guardian.FirstName, guardian.LastName+"-Berger")
		}
		paymentPath := fmt.Sprintf("/api/guardians/%d/payment", guardianID)
		if _, err := s.client.Put(paymentPath, body); err != nil {
			if s.verbose {
				fmt.Printf("    Warning: failed to store bank details for guardian %s: %v\n", guardianKey, err)
			}
			continue
		}
		result.GuardianIBANCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d payers marked, %d bank details stored\n", result.PayerCount, result.GuardianIBANCount)
	}
	return nil
}

// seedPickupSchedules creates weekly pickup schedules for students with "Wird abgeholt" status.
// Uses a varied but deterministic pattern: different pickup times per weekday to simulate
// realistic family schedules (e.g. earlier on Tuesdays due to extracurricular activities).
func (s *FixedSeeder) seedPickupSchedules(_ context.Context, result *FixedResult) error {
	// Pickup time patterns (varies by student index for realistic diversity)
	// Weekdays: 1=Montag, 2=Dienstag, 3=Mittwoch, 4=Donnerstag, 5=Freitag
	type weekdaySchedule struct {
		weekday    int
		pickupTime string
		notes      string
	}

	schedulePatterns := [][]weekdaySchedule{
		// Pattern 0: Standard full-week pickup at 15:30
		{
			{weekday: 1, pickupTime: "15:30"},
			{weekday: 2, pickupTime: "15:30"},
			{weekday: 3, pickupTime: "15:30"},
			{weekday: 4, pickupTime: "15:30"},
			{weekday: 5, pickupTime: "15:00", notes: "Freitag früher"},
		},
		// Pattern 1: Early Tuesday (Musikunterricht), standard otherwise
		{
			{weekday: 1, pickupTime: "15:30"},
			{weekday: 2, pickupTime: "14:30", notes: "Musikunterricht danach"},
			{weekday: 3, pickupTime: "15:30"},
			{weekday: 4, pickupTime: "15:30"},
			{weekday: 5, pickupTime: "15:00"},
		},
		// Pattern 2: Late pickup Mon/Thu (Eltern arbeiten), early Wed
		{
			{weekday: 1, pickupTime: "16:00"},
			{weekday: 2, pickupTime: "15:30"},
			{weekday: 3, pickupTime: "14:00", notes: "Oma holt ab"},
			{weekday: 4, pickupTime: "16:00"},
			{weekday: 5, pickupTime: "15:00"},
		},
		// Pattern 3: Partial week (e.g. grandparent picks up Mon/Fri only)
		{
			{weekday: 1, pickupTime: "15:30", notes: "Opa holt ab"},
			{weekday: 3, pickupTime: "15:30"},
			{weekday: 5, pickupTime: "14:30", notes: "Früher wegen Sportverein"},
		},
		// Pattern 4: All days, varied times
		{
			{weekday: 1, pickupTime: "15:00"},
			{weekday: 2, pickupTime: "15:30"},
			{weekday: 3, pickupTime: "16:00", notes: "Papa holt ab"},
			{weekday: 4, pickupTime: "15:30"},
			{weekday: 5, pickupTime: "14:00", notes: "Fußballtraining"},
		},
	}

	for i, student := range DemoStudents {
		// Only seed schedules for students being picked up (every other student)
		if i%2 == 0 {
			continue
		}

		studentID, ok := s.studentIDByIndex[i]
		if !ok {
			continue
		}

		pattern := schedulePatterns[i%len(schedulePatterns)]

		// Build schedules array
		schedules := make([]map[string]any, 0, len(pattern))
		for _, day := range pattern {
			entry := map[string]any{
				"weekday":     day.weekday,
				"pickup_time": day.pickupTime,
			}
			if day.notes != "" {
				entry["notes"] = day.notes
			}
			schedules = append(schedules, entry)
		}

		path := fmt.Sprintf("/api/students/%d/pickup-schedules", studentID)
		body := map[string]any{
			"schedules": schedules,
		}

		_, err := s.client.Put(path, body)
		if err != nil {
			// Log warning but continue — pickup schedules are non-critical demo data
			if s.verbose {
				fmt.Printf("    Warning: failed to seed pickup schedule for student %s %s: %v\n",
					student.FirstName, student.LastName, err)
			}
			continue
		}

		result.PickupScheduleCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d pickup schedules seeded\n", result.PickupScheduleCount)
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
			"device_type": "terminal",
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

		// Store device API key for seed state output
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

// seedStaffAccounts creates auth accounts for staff and the matching person
// records (with account_id set on creation, so no separate link step is needed).
func (s *FixedSeeder) seedStaffAccounts(_ context.Context, result *FixedResult) error {
	// Get role IDs for different staff types
	adminRoleID, ok := s.roleIDs["admin"]
	if !ok {
		return fmt.Errorf("admin role not found - available roles: %v", s.roleIDs)
	}
	userRoleID, ok := s.roleIDs["user"]
	if !ok {
		return fmt.Errorf("user role not found - available roles: %v", s.roleIDs)
	}
	guestRoleID, ok := s.roleIDs["guest"]
	if !ok {
		return fmt.Errorf("guest role not found - available roles: %v", s.roleIDs)
	}

	demoPasswords := []string{
		"sdlXK26%", "mQp9Wy3$", "kJt4Nz8!", "hBv7Rx5@", "fGn2Lm6#",
		"pYc8Dq1&", "wZa3Ks9*", "vTe5Hj4%", "xUi6Fo7$", "cRo1Pn2!",
		"bWs4Mv8@", "nLk7Qx3#", "jHd9Zt5&", "gFa2Yc6*", "tEr8Ub1%",
		"qDm3Wp4$", "yKn5Sj7!", "uBx6Gi9@", "iCv1Lh2#", "oAz4Rk8&",
	}

	for i, staff := range DemoStaff {
		personKey := fmt.Sprintf("%s %s", staff.FirstName, staff.LastName)

		// Generate email and credentials for demo accounts.
		// Email: demo{n}@mail.de where n = account number (1-20)
		// Password: per-account defaults, or shared --staff-password when set
		accountNum := i + 1
		email := fmt.Sprintf("demo%d@mail.de", accountNum)
		password := demoPasswords[i]
		if s.staffPassword != "" {
			password = s.staffPassword
		}
		pin := fmt.Sprintf("%04d", 1000+i)

		// Assign role based on position:
		// - OGS-Büro → admin (OGS leadership with full access)
		// - Extern → guest (external helpers with limited access)
		// - Pädagogische Fachkraft → user (standard pedagogical staff / Betreuer)
		var roleID int64
		switch staff.Position {
		case "OGS-Büro":
			roleID = adminRoleID
		case "Extern":
			roleID = guestRoleID
		default:
			roleID = userRoleID
		}

		// Create the account and its person in ONE request. All three roles
		// above are staff tier, and since #2222 /auth/register provisions the
		// person and staff record with the account: it refuses a staff-tier
		// role without a name, and a follow-up POST /api/users carrying the
		// same account_id would hit the partial unique index on
		// (tenant_id, account_id) against the person it just created.
		registerBody := map[string]any{
			"email":            email,
			"username":         fmt.Sprintf("demo%d", accountNum),
			"password":         password,
			"confirm_password": password,
			"role_id":          roleID,
			"first_name":       staff.FirstName,
			"last_name":        staff.LastName,
		}

		registerResp, err := s.client.Post("/auth/register", registerBody)
		if err != nil {
			return fmt.Errorf("failed to create account for %s: %w", personKey, err)
		}

		var account struct {
			Status string `json:"status"`
			Data   struct {
				ID int64 `json:"id"`
				// int64 ids travel as JSON strings on this field.
				SchoolIdentity *struct {
					PersonID int64 `json:"person_id,string"`
				} `json:"school_identity"`
			} `json:"data"`
		}
		if err := json.Unmarshal(registerResp, &account); err != nil {
			return fmt.Errorf("failed to parse account response: %w", err)
		}
		if account.Data.SchoolIdentity == nil || account.Data.SchoolIdentity.PersonID <= 0 {
			return fmt.Errorf("account for %s was created without a person record", personKey)
		}

		// seedStaff later posts this id to /api/staff, which adopts the staff
		// record created here instead of adding a second one.
		s.personIDs[personKey] = account.Data.SchoolIdentity.PersonID
		result.PersonCount++

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
		fmt.Printf("  ✓ %d staff accounts + persons created\n", result.AccountCount)
	}
	return nil
}
