package cmd

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/spf13/cobra"
	"github.com/uptrace/bun"
)

// seedCmd represents the seed command
var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed the database with German test data",
	Long: `Seed the database with German test data for testing purposes.
This command creates a complete OGS (Offene Ganztagsschule) dataset:
- 24 rooms (classrooms, labs, gym, library, etc.)
- 25 education groups (grade classes)
- 150 persons (30 staff/teachers, 120 students)
- 8 activity categories (Sport, Kunst & Basteln, Musik, etc.)
- 19 activity groups (Fußball-AG, Computer-Grundlagen, etc.)
- 6 OGS timeframes (12:00-17:00 afternoon supervision)
- Activity schedules linking groups to weekdays and times
- Supervisor assignments for all activities
- Student enrollments in activities
- 7 IoT devices (RFID readers, sensors)
- Privacy consents (GDPR compliance)

Usage:
  go run main.go seed
  go run main.go seed --reset (to clear existing data first)`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		reset, _ := cmd.Flags().GetBool("reset")
		if reset {
			fmt.Println("WARNING: --reset flag is set. This will delete existing data!")
			fmt.Print("Are you sure you want to continue? (y/N): ")
			var response string
			_, err := fmt.Scanln(&response)
			if err != nil || (response != "y" && response != "Y") {
				fmt.Println("Seed operation cancelled.")
				return
			}
		}
		seedDatabase(ctx, reset)
	},
}

func init() {
	RootCmd.AddCommand(seedCmd)
	seedCmd.Flags().Bool("reset", false, "Reset all data before seeding")
}

func seedDatabase(ctx context.Context, reset bool) {
	// Initialize database connection
	db, err := database.DBConn()
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Failed to close database connection: %v", err)
		}
	}()

	// Initialize random number generator
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Reset data if requested
	if reset {
		fmt.Println("Resetting existing data...")
		if err := resetData(ctx, db); err != nil {
			log.Fatal("Failed to reset data:", err)
		}
		fmt.Println("Data reset completed.")
	}

	fmt.Println("Starting database seeding...")

	// Use transaction for all operations
	err = db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {

		// 1. Create Rooms first (no dependencies)
		fmt.Println("Creating rooms...")
		roomIDs, err := seedRooms(ctx, tx)
		if err != nil {
			return fmt.Errorf("failed to seed rooms: %w", err)
		}
		fmt.Printf("Created %d rooms\n", len(roomIDs))

		// 2. Create Groups (depends on rooms)
		fmt.Println("Creating groups...")
		groupIDs, err := seedGroups(ctx, tx, roomIDs)
		if err != nil {
			return fmt.Errorf("failed to seed groups: %w", err)
		}
		fmt.Printf("Created %d groups\n", len(groupIDs))

		// 3. Create RFID cards first (needed for persons)
		fmt.Println("Creating RFID cards...")
		rfidIDs, err := seedRFIDCards(ctx, tx, rng)
		if err != nil {
			return fmt.Errorf("failed to seed RFID cards: %w", err)
		}
		fmt.Printf("Created %d RFID cards\n", len(rfidIDs))

		// 4. Create Persons (base for all users)
		fmt.Println("Creating persons...")
		personIDs, err := seedPersons(ctx, tx, rfidIDs, rng)
		if err != nil {
			return fmt.Errorf("failed to seed persons: %w", err)
		}
		fmt.Printf("Created %d persons\n", len(personIDs))

		// 4. Create Staff members from some persons
		fmt.Println("Creating staff members...")
		staffIDs, err := seedStaff(ctx, tx, personIDs[:30]) // First 30 persons are staff
		if err != nil {
			return fmt.Errorf("failed to seed staff: %w", err)
		}
		fmt.Printf("Created %d staff members\n", len(staffIDs))

		// 5. Create Teachers from some staff members
		fmt.Println("Creating teachers...")
		teacherIDs, err := seedTeachers(ctx, tx, staffIDs[:20]) // First 20 staff are teachers
		if err != nil {
			return fmt.Errorf("failed to seed teachers: %w", err)
		}
		fmt.Printf("Created %d teachers\n", len(teacherIDs))

		// 6. Create Students from remaining persons
		fmt.Println("Creating students...")
		studentPersonIDs := personIDs[30:]                                             // Remaining persons are students
		studentIDs, err := seedStudents(ctx, tx, studentPersonIDs, groupIDs[:10], rng) // First 10 groups are grade classes
		if err != nil {
			return fmt.Errorf("failed to seed students: %w", err)
		}
		fmt.Printf("Created %d students\n", len(studentIDs))

		// 6.5. Assign Teachers to Education Groups
		fmt.Println("Assigning teachers to education groups...")
		err = seedGroupTeachers(ctx, tx, groupIDs[:10], teacherIDs, rng) // First 10 groups are grade classes
		if err != nil {
			return fmt.Errorf("failed to assign teachers to groups: %w", err)
		}
		fmt.Printf("Assigned teachers to education groups\n")

		// 7. Create Activity Categories (no dependencies)
		fmt.Println("Creating activity categories...")
		categoryIDs, err := seedActivityCategories(ctx, tx)
		if err != nil {
			return fmt.Errorf("failed to seed activity categories: %w", err)
		}
		fmt.Printf("Created %d activity categories\n", len(categoryIDs))

		// 8. Create Schedule Timeframes (no dependencies)
		fmt.Println("Creating schedule timeframes...")
		timeframeIDs, err := seedScheduleTimeframes(ctx, tx)
		if err != nil {
			return fmt.Errorf("failed to seed schedule timeframes: %w", err)
		}
		fmt.Printf("Created %d schedule timeframes\n", len(timeframeIDs))

		// 9. Create Activity Groups (depends on categories and rooms)
		fmt.Println("Creating activity groups...")
		activityGroupIDs, err := seedActivityGroups(ctx, tx, categoryIDs, roomIDs, rng)
		if err != nil {
			return fmt.Errorf("failed to seed activity groups: %w", err)
		}
		fmt.Printf("Created %d activity groups\n", len(activityGroupIDs))

		// 10. Create Activity Schedules (depends on activity groups and timeframes)
		fmt.Println("Creating activity schedules...")
		scheduleIDs, err := seedActivitySchedules(ctx, tx, activityGroupIDs, timeframeIDs, rng)
		if err != nil {
			return fmt.Errorf("failed to seed activity schedules: %w", err)
		}
		fmt.Printf("Created %d activity schedules\n", len(scheduleIDs))

		// 11. Create Activity Supervisors (depends on activity groups and staff)
		fmt.Println("Creating activity supervisors...")
		supervisorIDs, err := seedActivitySupervisors(ctx, tx, activityGroupIDs, staffIDs, rng)
		if err != nil {
			return fmt.Errorf("failed to seed activity supervisors: %w", err)
		}
		fmt.Printf("Created %d activity supervisors\n", len(supervisorIDs))

		// 12. Create Student Enrollments (depends on activity groups and students)
		fmt.Println("Creating student enrollments...")
		enrollmentIDs, err := seedStudentEnrollments(ctx, tx, activityGroupIDs, studentIDs, rng)
		if err != nil {
			return fmt.Errorf("failed to seed student enrollments: %w", err)
		}
		fmt.Printf("Created %d student enrollments\n", len(enrollmentIDs))

		// 13. Create IoT Devices (depends on persons)
		fmt.Println("Creating IoT devices...")
		deviceIDs, err := seedIoTDevices(ctx, tx, personIDs[:5], rng) // First 5 persons register devices
		if err != nil {
			return fmt.Errorf("failed to seed IoT devices: %w", err)
		}
		fmt.Printf("Created %d IoT devices\n", len(deviceIDs))

		// 14. Create Privacy Consents (depends on students)
		fmt.Println("Creating privacy consents...")
		consentIDs, err := seedPrivacyConsents(ctx, tx, studentIDs, rng)
		if err != nil {
			return fmt.Errorf("failed to seed privacy consents: %w", err)
		}
		fmt.Printf("Created %d privacy consents\n", len(consentIDs))

		return nil
	})

	if err != nil {
		log.Fatal("Failed to seed database:", err)
	}

	fmt.Println("Database seeding completed successfully!")
}

func resetData(ctx context.Context, db *bun.DB) error {
	// Delete in reverse order of dependencies
	tables := []string{
		"users.privacy_consents",
		"iot.devices",
		"activities.student_enrollments",
		"activities.supervisors",
		"activities.schedules",
		"activities.groups",
		"activities.categories",
		"schedule.timeframes",
		"users.students",
		"education.group_teacher", // Must be deleted before teachers and groups
		"users.teachers",
		"users.staff",
		"users.persons",
		"education.groups",
		"facilities.rooms",
	}

	for _, table := range tables {
		query := fmt.Sprintf("DELETE FROM %s", table)
		if _, err := db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to delete from %s: %w", table, err)
		}
		fmt.Printf("Cleared table: %s\n", table)
	}

	return nil
}

func seedRooms(ctx context.Context, tx bun.Tx) ([]int64, error) {
	roomData := []struct {
		name     string
		building string
		floor    int
		capacity int
		category string
		color    string
	}{
		// Klassenzimmer
		{"101", "Hauptgebäude", 1, 30, "Klassenzimmer", "#4A90E2"},
		{"102", "Hauptgebäude", 1, 30, "Klassenzimmer", "#4A90E2"},
		{"103", "Hauptgebäude", 1, 25, "Klassenzimmer", "#4A90E2"},
		{"201", "Hauptgebäude", 2, 35, "Klassenzimmer", "#4A90E2"},
		{"202", "Hauptgebäude", 2, 30, "Klassenzimmer", "#4A90E2"},
		{"203", "Hauptgebäude", 2, 28, "Klassenzimmer", "#4A90E2"},
		// Naturwissenschaftliche Labore
		{"Labor 1", "Naturwissenschaftstrakt", 1, 24, "Labor", "#50E3C2"},
		{"Labor 2", "Naturwissenschaftstrakt", 1, 24, "Labor", "#50E3C2"},
		{"Chemielabor", "Naturwissenschaftstrakt", 2, 20, "Labor", "#50E3C2"},
		{"Physiklabor", "Naturwissenschaftstrakt", 2, 20, "Labor", "#50E3C2"},
		// Sportanlagen
		{"Hauptsporthalle", "Sportkomplex", 1, 100, "Sport", "#7ED321"},
		{"Kleine Sporthalle", "Sportkomplex", 1, 50, "Sport", "#7ED321"},
		// Kunsträume
		{"Kunstraum 1", "Kreativtrakt", 1, 20, "Kunst", "#F5A623"},
		{"Musikraum", "Kreativtrakt", 1, 25, "Musik", "#BD10E0"},
		// Computerräume
		{"Computerraum 1", "IT-Zentrum", 1, 30, "Computer", "#9013FE"},
		{"Computerraum 2", "IT-Zentrum", 2, 30, "Computer", "#9013FE"},
		// Bibliothek
		{"Hauptbibliothek", "Bibliotheksgebäude", 1, 80, "Bibliothek", "#B8E986"},
		{"Lernraum 1", "Bibliotheksgebäude", 2, 15, "Lernraum", "#B8E986"},
		// Sonderzweckräume
		{"Mensa", "Hauptgebäude", 0, 200, "Speiseraum", "#F8E71C"},
		{"Aula", "Hauptgebäude", 1, 300, "Versammlung", "#D0021B"},
		{"Krankenzimmer", "Hauptgebäude", 1, 10, "Medizin", "#FF6900"},
		// Büros
		{"Rektorat", "Verwaltungsgebäude", 1, 5, "Büro", "#A020F0"},
		{"Lehrerzimmer", "Verwaltungsgebäude", 1, 40, "Büro", "#A020F0"},
		{"Konferenzraum", "Verwaltungsgebäude", 2, 20, "Besprechung", "#A020F0"},
	}

	roomIDs := make([]int64, 0, len(roomData))
	for _, data := range roomData {
		var id int64
		query := `INSERT INTO facilities.rooms (name, building, floor, capacity, category, color, created_at, updated_at) 
		          VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`

		err := tx.QueryRowContext(ctx, query,
			data.name, data.building, data.floor, data.capacity,
			data.category, data.color, time.Now(), time.Now()).Scan(&id)

		if err != nil {
			return nil, fmt.Errorf("failed to create room %s: %w", data.name, err)
		}
		roomIDs = append(roomIDs, id)
	}

	return roomIDs, nil
}

func seedGroups(ctx context.Context, tx bun.Tx, roomIDs []int64) ([]int64, error) {
	// Only first 6 rooms are classrooms
	var room1, room2, room3, room4, room5, room6 *int64
	if len(roomIDs) >= 6 {
		room1 = &roomIDs[0]
		room2 = &roomIDs[1]
		room3 = &roomIDs[2]
		room4 = &roomIDs[3]
		room5 = &roomIDs[4]
		room6 = &roomIDs[5]
	}

	groupData := []struct {
		name   string
		roomID *int64
	}{
		// Schulklassen - Zuweisung zu verfügbaren Klassenräumen
		{"Klasse 1A", room1},
		{"Klasse 1B", room2},
		{"Klasse 2A", room3},
		{"Klasse 2B", room4},
		{"Klasse 3A", room5},
		{"Klasse 3B", room6},
		{"Klasse 4A", nil}, // Keine weiteren Klassenräume verfügbar
		{"Klasse 4B", nil},
		{"Klasse 5A", nil},
		{"Klasse 5B", nil},
		// Aktivitätsgruppen - keine spezifische Raumzuweisung
		{"Naturwissenschafts-AG", nil},
		{"Kunst-AG", nil},
		{"Theater-AG", nil},
		{"Mathematik-AG", nil},
		{"Schach-AG", nil},
		{"Sportmannschaft A", nil},
		{"Sportmannschaft B", nil},
		{"Schulband", nil},
		{"Debattier-AG", nil},
		{"Computer-AG", nil},
		// Betreuungsgruppen
		{"Nachmittagsbetreuung", nil},
		{"Frühbetreuung", nil},
		{"Hausaufgabenhilfe", nil},
		{"Lese-AG", nil},
		{"Umwelt-AG", nil},
	}

	groupIDs := make([]int64, 0, len(groupData))
	for _, data := range groupData {
		var id int64
		query := `INSERT INTO education.groups (name, room_id, created_at, updated_at) 
		          VALUES (?, ?, ?, ?) RETURNING id`

		err := tx.QueryRowContext(ctx, query,
			data.name, data.roomID, time.Now(), time.Now()).Scan(&id)

		if err != nil {
			return nil, fmt.Errorf("failed to create group %s: %w", data.name, err)
		}
		groupIDs = append(groupIDs, id)
	}

	return groupIDs, nil
}

func seedRFIDCards(ctx context.Context, tx bun.Tx, rng *rand.Rand) ([]string, error) {
	// Create RFID cards first
	rfidIDs := make([]string, 0, 150)
	usedTags := make(map[string]bool) // Track used tags to avoid duplicates
	
	for i := 0; i < 150; i++ {
		// Generate a unique realistic RFID tag
		var rfidID string
		for {
			// Mix of 7-byte (70%) and 4-byte (30%) UIDs for variety
			if i%10 < 7 {
				// 7-byte UID (14 hex characters)
				rfidID = generateRFIDTag(rng, 7)
			} else {
				// 4-byte UID (8 hex characters)
				rfidID = generateRFIDTag(rng, 4)
			}
			
			// Ensure uniqueness
			if !usedTags[rfidID] {
				usedTags[rfidID] = true
				break
			}
		}

		query := `INSERT INTO users.rfid_cards (id, active, created_at, updated_at) 
		          VALUES (?, ?, ?, ?) ON CONFLICT (id) DO NOTHING`

		_, err := tx.ExecContext(ctx, query,
			rfidID, true, time.Now(), time.Now())

		if err != nil {
			return nil, fmt.Errorf("failed to create RFID card %s: %w", rfidID, err)
		}
		rfidIDs = append(rfidIDs, rfidID)
	}

	return rfidIDs, nil
}

// generateRFIDTag generates a realistic RFID tag in normalized format
// For example: 
//   - 7-byte tag "04:D6:94:82:97:6A:80" becomes "04D69482976A80"
//   - 4-byte tag "04:D6:94:82" becomes "04D69482"
func generateRFIDTag(rng *rand.Rand, byteCount int) string {
	// Generate random bytes
	bytes := make([]byte, byteCount)
	for i := range bytes {
		bytes[i] = byte(rng.Intn(256))
	}
	
	// Convert to hex string (normalized: uppercase, no separators)
	// This matches what the API normalization does
	return fmt.Sprintf("%X", bytes)
}

func seedPersons(ctx context.Context, tx bun.Tx, rfidIDs []string, rng *rand.Rand) ([]int64, error) {
	// Vornamen
	firstNames := []string{
		"Emma", "Ben", "Mia", "Finn", "Hannah", "Paul", "Lina", "Felix",
		"Sophia", "Noah", "Emilia", "Leon", "Ella", "Elias", "Clara", "Anton",
		"Anna", "Julian", "Lea", "Emil", "Marie", "Luca", "Leni", "Maximilian",
		"Ida", "Jonas", "Greta", "Moritz", "Amelie", "Jakob", "Frieda", "David",
		"Mathilda", "Theo", "Luisa", "Tim", "Charlotte", "Samuel", "Mila", "Alexander",
		"Johanna", "Matteo", "Nele", "Friedrich", "Paula", "Oskar", "Alma", "Gabriel",
		"Marlene", "Carl", "Pia", "Leonard", "Juna", "Karl", "Lotte",
	}

	// Nachnamen
	lastNames := []string{
		"Müller", "Schmidt", "Schneider", "Fischer", "Weber", "Meyer", "Wagner",
		"Becker", "Schulz", "Hoffmann", "Schäfer", "Koch", "Bauer",
		"Richter", "Klein", "Wolf", "Schröder", "Neumann", "Schwarz", "Zimmermann",
		"Braun", "Krüger", "Hofmann", "Hartmann", "Lange", "Schmitt", "Werner",
		"Schmitz", "Krause", "Meier", "Lehmann", "Schmid", "Schulze", "Maier",
		"Köhler", "Herrmann", "König", "Walter", "Peters", "Lang", "Möller",
		"Weis", "Jung", "Hahn", "Schubert", "Vogel", "Friedrich", "Keller",
		"Schwarz", "Günther",
	}

	// Create 150 persons (30 staff/teachers + 120 students)
	personIDs := make([]int64, 0, 150)
	for i := 0; i < 150; i++ {
		firstName := firstNames[rng.Intn(len(firstNames))]
		lastName := lastNames[rng.Intn(len(lastNames))]

		var id int64
		query := `INSERT INTO users.persons (first_name, last_name, tag_id, created_at, updated_at) 
		          VALUES (?, ?, ?, ?, ?) RETURNING id`

		err := tx.QueryRowContext(ctx, query,
			firstName, lastName, rfidIDs[i], time.Now(), time.Now()).Scan(&id)

		if err != nil {
			return nil, fmt.Errorf("failed to create person %s %s: %w", firstName, lastName, err)
		}
		personIDs = append(personIDs, id)
	}

	return personIDs, nil
}

func seedStaff(ctx context.Context, tx bun.Tx, personIDs []int64) ([]int64, error) {
	notes := []string{
		"Erfahrenes Teammitglied",
		"Abteilungsleiter",
		"Fachkoordinator",
		"Verwaltungsunterstützung",
		"Erfahrene Lehrkraft",
		"Neue Lehrkraft - in Einarbeitung",
		"Teilzeitkraft",
		"Vollzeitkraft",
		"Unterstützungspersonal",
		"Fachspezialist",
	}

	staffIDs := make([]int64, 0, len(personIDs))
	for i, personID := range personIDs {
		var id int64
		query := `INSERT INTO users.staff (person_id, staff_notes, created_at, updated_at) 
		          VALUES (?, ?, ?, ?) RETURNING id`

		err := tx.QueryRowContext(ctx, query,
			personID, notes[i%len(notes)], time.Now(), time.Now()).Scan(&id)

		if err != nil {
			return nil, fmt.Errorf("failed to create staff for person %d: %w", personID, err)
		}
		staffIDs = append(staffIDs, id)
	}

	return staffIDs, nil
}

func seedTeachers(ctx context.Context, tx bun.Tx, staffIDs []int64) ([]int64, error) {
	teacherData := []struct {
		specialization string
		role           string
		qualifications string
	}{
		{"Mathematik", "Lehrerin für Mathematik", "Master of Education Mathematik, 10 Jahre Erfahrung"},
		{"Mathematik", "Mathematiklehrer", "Bachelor of Education Mathematik, 5 Jahre Erfahrung"},
		{"Naturwissenschaften", "Fachbereichsleiter Naturwissenschaften", "Promotion in Physik, 15 Jahre Erfahrung"},
		{"Naturwissenschaften", "Naturwissenschaftslehrerin", "Master Chemie, 7 Jahre Erfahrung"},
		{"Deutsch", "Fachbereichsleiter Deutsch", "Master Deutsche Literatur, 12 Jahre Erfahrung"},
		{"Deutsch", "Deutschlehrerin", "Bachelor Germanistik, 3 Jahre Erfahrung"},
		{"Geschichte", "Geschichtslehrer", "Master Geschichte, 8 Jahre Erfahrung"},
		{"Geografie", "Geografielehrerin", "Bachelor of Education Geografie, 6 Jahre Erfahrung"},
		{"Sport", "Sportkoordinator", "Bachelor of Education Sport, 10 Jahre Erfahrung"},
		{"Sport", "Sportlehrerin", "Sportwissenschaft, 4 Jahre Erfahrung"},
		{"Kunst", "Kunstlehrer", "Bachelor Bildende Kunst, 5 Jahre Erfahrung"},
		{"Musik", "Musiklehrerin", "Bachelor Musik, 7 Jahre Erfahrung"},
		{"Informatik", "IT-Lehrer", "Bachelor Informatik, 6 Jahre Erfahrung"},
		{"Fremdsprachen", "Spanischlehrerin", "Bachelor Spanisch, Muttersprachlerin"},
		{"Fremdsprachen", "Französischlehrer", "Master Französische Literatur, 9 Jahre Erfahrung"},
		{"Sonderpädagogik", "Sonderpädagogik-Koordinatorin", "Master Sonderpädagogik, 11 Jahre Erfahrung"},
		{"Bibliothek", "Bibliothekarin", "Master Bibliothekswissenschaft, 8 Jahre Erfahrung"},
		{"Beratung", "Schulberaterin", "Master Pädagogik, 13 Jahre Erfahrung"},
		{"Verwaltung", "Stellvertretende Schulleiterin", "Master Schulmanagement, 15 Jahre Erfahrung"},
		{"Verwaltung", "Schulleiter", "Promotion Pädagogik, 20 Jahre Erfahrung"},
	}

	teacherIDs := make([]int64, 0, len(staffIDs))
	for i, staffID := range staffIDs {
		data := teacherData[i%len(teacherData)]

		var id int64
		query := `INSERT INTO users.teachers (staff_id, specialization, role, qualifications, created_at, updated_at) 
		          VALUES (?, ?, ?, ?, ?, ?) RETURNING id`

		err := tx.QueryRowContext(ctx, query,
			staffID, data.specialization, data.role, data.qualifications,
			time.Now(), time.Now()).Scan(&id)

		if err != nil {
			return nil, fmt.Errorf("failed to create teacher for staff %d: %w", staffID, err)
		}
		teacherIDs = append(teacherIDs, id)
	}

	return teacherIDs, nil
}

func seedStudents(ctx context.Context, tx bun.Tx, personIDs []int64, classGroupIDs []int64, rng *rand.Rand) ([]int64, error) {
	// Vornamen der Erziehungsberechtigten
	guardianFirstNames := []string{
		"Andreas", "Sabine", "Michael", "Petra", "Thomas", "Andrea", "Stefan",
		"Claudia", "Frank", "Monika", "Markus", "Birgit", "Christian", "Ute",
		"Martin", "Karin", "Ralf", "Gabriele", "Jörg", "Susanne",
	}

	// Klassen
	grades := []string{"1A", "1B", "2A", "2B", "3A", "3B", "4A", "4B", "5A", "5B"}

	studentIDs := make([]int64, 0, len(personIDs))
	for i, personID := range personIDs {
		// Assign to a grade
		gradeIndex := i % len(grades)

		// Generate guardian info - get last name from person query first
		var personLastName string
		err := tx.QueryRowContext(ctx, "SELECT last_name FROM users.persons WHERE id = ?", personID).Scan(&personLastName)
		if err != nil {
			return nil, fmt.Errorf("failed to get person last name: %w", err)
		}

		guardianFirstName := guardianFirstNames[rng.Intn(len(guardianFirstNames))]
		guardianName := fmt.Sprintf("%s %s", guardianFirstName, personLastName)

		// Generate contact info
		guardianPhone := fmt.Sprintf("+49 %d %d-%d", 30+rng.Intn(900), rng.Intn(900)+100, rng.Intn(9000)+1000)

		// Normalize German characters for email addresses
		emailFirstName := normalizeForEmail(guardianFirstName)
		emailLastName := normalizeForEmail(personLastName)
		guardianEmail := fmt.Sprintf("%s.%s@gmx.de", emailFirstName, emailLastName)

		// Randomly set initial location (most are "in house")
		bus := false
		inHouse := true
		if rng.Float32() < 0.3 {
			bus = true
			inHouse = false
		}

		var id int64
		query := `INSERT INTO users.students (person_id, school_class, bus, in_house, wc, school_yard, 
		          guardian_name, guardian_contact, guardian_email, guardian_phone, group_id, created_at, updated_at) 
		          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`

		err = tx.QueryRowContext(ctx, query,
			personID, grades[gradeIndex], bus, inHouse, false, false,
			guardianName, guardianPhone, guardianEmail, guardianPhone, classGroupIDs[gradeIndex],
			time.Now(), time.Now()).Scan(&id)

		if err != nil {
			return nil, fmt.Errorf("failed to create student for person %d: %w", personID, err)
		}
		studentIDs = append(studentIDs, id)
	}

	return studentIDs, nil
}

// Helper function to normalize German characters for email addresses
func normalizeForEmail(name string) string {
	// Convert to lowercase first
	name = strings.ToLower(name)

	// Replace German umlauts and special characters with ASCII equivalents
	replacements := map[string]string{
		"ä": "ae",
		"ö": "oe",
		"ü": "ue",
		"ß": "ss",
		"é": "e",
		"è": "e",
		"ê": "e",
		"à": "a",
		"á": "a",
		"ô": "o",
		"û": "u",
		"ç": "c",
	}

	for german, ascii := range replacements {
		name = strings.ReplaceAll(name, german, ascii)
	}

	return name
}

func seedActivityCategories(ctx context.Context, tx bun.Tx) ([]int64, error) {
	categoryData := []struct {
		name        string
		description string
		color       string
	}{
		{"Sport", "Sportliche Aktivitäten für Kinder", "#7ED321"},
		{"Kunst & Basteln", "Kreative Aktivitäten und Handwerken", "#F5A623"},
		{"Musik", "Musikalische Aktivitäten und Gesang", "#BD10E0"},
		{"Spiele", "Brett-, Karten- und Gruppenspiele", "#50E3C2"},
		{"Lesen", "Leseförderung und Literatur", "#B8E986"},
		{"Hausaufgabenhilfe", "Unterstützung bei den Hausaufgaben", "#4A90E2"},
		{"Natur & Forschen", "Naturerkundung und einfache Experimente", "#7ED321"},
		{"Computer", "Grundlagen im Umgang mit dem Computer", "#9013FE"},
	}

	categoryIDs := make([]int64, 0, len(categoryData))
	for _, data := range categoryData {
		var id int64
		query := `INSERT INTO activities.categories (name, description, color, created_at, updated_at) 
		          VALUES (?, ?, ?, ?, ?) 
		          ON CONFLICT (name) DO UPDATE 
		          SET updated_at = EXCLUDED.updated_at
		          RETURNING id`

		err := tx.QueryRowContext(ctx, query,
			data.name, data.description, data.color, time.Now(), time.Now()).Scan(&id)

		if err != nil {
			return nil, fmt.Errorf("failed to create activity category %s: %w", data.name, err)
		}
		categoryIDs = append(categoryIDs, id)
	}

	return categoryIDs, nil
}

func seedScheduleTimeframes(ctx context.Context, tx bun.Tx) ([]int64, error) {
	// OGS timeframes for elementary school afternoon supervision
	today := time.Now()
	timeframeData := []struct {
		description string
		startHour   int
		startMinute int
		endHour     int
		endMinute   int
		isActive    bool
	}{
		{"Mittagessen", 12, 0, 13, 0, true},
		{"Freispiel/Ruhephase", 13, 0, 14, 0, true},
		{"1. AG-Block", 14, 0, 15, 0, true},
		{"Pause", 15, 0, 15, 15, true},
		{"2. AG-Block", 15, 15, 16, 15, true},
		{"Freispiel/Abholzeit", 16, 15, 17, 0, true},
	}

	timeframeIDs := make([]int64, 0, len(timeframeData))
	for _, data := range timeframeData {
		startTime := time.Date(today.Year(), today.Month(), today.Day(),
			data.startHour, data.startMinute, 0, 0, today.Location())
		endTime := time.Date(today.Year(), today.Month(), today.Day(),
			data.endHour, data.endMinute, 0, 0, today.Location())

		var id int64
		query := `INSERT INTO schedule.timeframes (start_time, end_time, is_active, description, created_at, updated_at) 
		          VALUES (?, ?, ?, ?, ?, ?) RETURNING id`

		err := tx.QueryRowContext(ctx, query,
			startTime, endTime, data.isActive, data.description, time.Now(), time.Now()).Scan(&id)

		if err != nil {
			return nil, fmt.Errorf("failed to create timeframe %s: %w", data.description, err)
		}
		timeframeIDs = append(timeframeIDs, id)
	}

	return timeframeIDs, nil
}

func seedActivityGroups(ctx context.Context, tx bun.Tx, categoryIDs []int64, roomIDs []int64, rng *rand.Rand) ([]int64, error) {
	activityData := []struct {
		name            string
		maxParticipants int
		categoryIndex   int  // Index into categoryIDs
		roomIndex       *int // Index into roomIDs, nil if no specific room
	}{
		// Sport (category 0)
		{"Fußball-AG", 16, 0, intPtr(10)},              // Hauptsporthalle
		{"Basketball für Anfänger", 12, 0, intPtr(11)}, // Kleine Sporthalle
		{"Tanzen", 15, 0, intPtr(11)},                  // Kleine Sporthalle

		// Kunst & Basteln (category 1)
		{"Basteln und Malen", 12, 1, intPtr(12)}, // Kunstraum 1
		{"Töpfern", 8, 1, intPtr(12)},            // Kunstraum 1

		// Musik (category 2)
		{"Kinderchor", 20, 2, intPtr(13)},              // Musikraum
		{"Rhythmus und Percussion", 10, 2, intPtr(13)}, // Musikraum

		// Spiele (category 3)
		{"Schach für Anfänger", 12, 3, nil},
		{"Brett- und Kartenspiele", 16, 3, nil},
		{"Gesellschaftsspiele", 10, 3, nil},

		// Lesen (category 4)
		{"Lese-AG", 15, 4, intPtr(16)},              // Hauptbibliothek
		{"Geschichten erfinden", 10, 4, intPtr(17)}, // Lernraum 1

		// Hausaufgabenhilfe (category 5)
		{"Hausaufgabenhilfe Klasse 1-2", 8, 5, intPtr(0)}, // Classroom 101
		{"Hausaufgabenhilfe Klasse 3-4", 8, 5, intPtr(1)}, // Classroom 102
		{"Mathematik-Hilfe", 6, 5, intPtr(2)},             // Classroom 103

		// Natur & Forschen (category 6)
		{"Naturforschergruppe", 10, 6, intPtr(6)}, // Labor 1
		{"Garten-AG", 12, 6, nil},

		// Computer (category 7)
		{"Computer-Grundlagen", 10, 7, intPtr(14)}, // Computerraum 1
		{"Erste Programmierung", 8, 7, intPtr(15)}, // Computerraum 2
	}

	activityGroupIDs := make([]int64, 0, len(activityData))
	for _, data := range activityData {
		var plannedRoomID *int64
		if data.roomIndex != nil && *data.roomIndex < len(roomIDs) {
			plannedRoomID = &roomIDs[*data.roomIndex]
		}

		var id int64
		query := `INSERT INTO activities.groups (name, max_participants, is_open, category_id, planned_room_id, created_at, updated_at) 
		          VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`

		err := tx.QueryRowContext(ctx, query,
			data.name, data.maxParticipants, true, categoryIDs[data.categoryIndex], plannedRoomID,
			time.Now(), time.Now()).Scan(&id)

		if err != nil {
			return nil, fmt.Errorf("failed to create activity group %s: %w", data.name, err)
		}
		activityGroupIDs = append(activityGroupIDs, id)
	}

	return activityGroupIDs, nil
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}

func seedActivitySchedules(ctx context.Context, tx bun.Tx, activityGroupIDs []int64, timeframeIDs []int64, rng *rand.Rand) ([]int64, error) {
	weekdays := []int{1, 2, 3, 4, 5} // Monday to Friday (ISO 8601: 1=Monday, 5=Friday)
	// Activity blocks are timeframes 2 and 4 (1. AG-Block and 2. AG-Block)
	activityTimeframes := []int64{timeframeIDs[2], timeframeIDs[4]}

	scheduleIDs := make([]int64, 0)

	// Assign each activity group to 1-3 random weekdays and time slots
	for _, groupID := range activityGroupIDs {
		// Each activity happens 1-3 times per week
		sessionsPerWeek := rng.Intn(3) + 1

		// Pick random weekdays
		shuffledWeekdays := make([]int, len(weekdays))
		copy(shuffledWeekdays, weekdays)
		for i := len(shuffledWeekdays) - 1; i > 0; i-- {
			j := rng.Intn(i + 1)
			shuffledWeekdays[i], shuffledWeekdays[j] = shuffledWeekdays[j], shuffledWeekdays[i]
		}

		for i := 0; i < sessionsPerWeek; i++ {
			weekday := shuffledWeekdays[i]
			timeframeID := activityTimeframes[rng.Intn(len(activityTimeframes))]

			var id int64
			query := `INSERT INTO activities.schedules (weekday, timeframe_id, activity_group_id, created_at, updated_at) 
			          VALUES (?, ?, ?, ?, ?) RETURNING id`

			err := tx.QueryRowContext(ctx, query,
				weekday, timeframeID, groupID, time.Now(), time.Now()).Scan(&id)

			if err != nil {
				return nil, fmt.Errorf("failed to create activity schedule for group %d: %w", groupID, err)
			}
			scheduleIDs = append(scheduleIDs, id)
		}
	}

	return scheduleIDs, nil
}

func seedActivitySupervisors(ctx context.Context, tx bun.Tx, activityGroupIDs []int64, staffIDs []int64, rng *rand.Rand) ([]int64, error) {
	supervisorIDs := make([]int64, 0)

	// Assign 1-2 supervisors per activity group
	for i, groupID := range activityGroupIDs {
		// Primary supervisor (cycle through staff)
		primaryStaffID := staffIDs[i%len(staffIDs)]

		var id int64
		query := `INSERT INTO activities.supervisors (staff_id, group_id, is_primary, created_at, updated_at) 
		          VALUES (?, ?, ?, ?, ?) RETURNING id`

		err := tx.QueryRowContext(ctx, query,
			primaryStaffID, groupID, true, time.Now(), time.Now()).Scan(&id)

		if err != nil {
			return nil, fmt.Errorf("failed to create primary supervisor for group %d: %w", groupID, err)
		}
		supervisorIDs = append(supervisorIDs, id)

		// 50% chance of additional supervisor
		if rng.Float32() < 0.5 {
			secondaryStaffID := staffIDs[(i+10)%len(staffIDs)] // Different staff member
			if secondaryStaffID != primaryStaffID {
				err := tx.QueryRowContext(ctx, query,
					secondaryStaffID, groupID, false, time.Now(), time.Now()).Scan(&id)

				if err != nil {
					return nil, fmt.Errorf("failed to create secondary supervisor for group %d: %w", groupID, err)
				}
				supervisorIDs = append(supervisorIDs, id)
			}
		}
	}

	return supervisorIDs, nil
}

func seedStudentEnrollments(ctx context.Context, tx bun.Tx, activityGroupIDs []int64, studentIDs []int64, rng *rand.Rand) ([]int64, error) {
	// Constants for enrollment generation
	const (
		minFillRate           = 0.70
		fillRateRange         = 0.15
		enrollmentInsertQuery = `INSERT INTO activities.student_enrollments (student_id, activity_group_id, enrollment_date, attendance_status, created_at, updated_at) 
		                        VALUES (?, ?, ?, ?, ?, ?) RETURNING id`
	)

	enrollmentIDs := make([]int64, 0)
	attendanceStatuses := []string{"regelmäßig", "gelegentlich", "neu angemeldet"}

	// Track enrollment counts for each activity group
	enrollmentCounts := make(map[int64]int)
	maxParticipants := make(map[int64]int)
	targetFillRates := make(map[int64]float64)

	// Get max participants for each activity group and set target fill rates
	for _, groupID := range activityGroupIDs {
		var max int
		err := tx.QueryRowContext(ctx,
			"SELECT max_participants FROM activities.groups WHERE id = ?", groupID).Scan(&max)
		if err != nil {
			return nil, fmt.Errorf("failed to get max_participants for group %d: %w", groupID, err)
		}
		maxParticipants[groupID] = max
		enrollmentCounts[groupID] = 0
		// Random fill rate between 70-85%
		targetFillRates[groupID] = minFillRate + (rng.Float64() * fillRateRange)
	}

	// First pass: Ensure every student gets at least 1 activity
	studentsWithActivities := make(map[int64]int) // Track how many activities each student has
	activityIndex := 0                            // For round-robin distribution

	fmt.Println("First pass: Ensuring every student gets at least one activity...")
	for _, studentID := range studentIDs {
		enrolled := false
		attempts := 0

		// Try to enroll student in at least one activity
		for !enrolled && attempts < len(activityGroupIDs) {
			groupID := activityGroupIDs[activityIndex%len(activityGroupIDs)]
			activityIndex++
			attempts++

			// Check if activity has reached its target fill rate
			currentFillRate := float64(enrollmentCounts[groupID]) / float64(maxParticipants[groupID])
			if currentFillRate >= targetFillRates[groupID] {
				continue
			}

			status := attendanceStatuses[rng.Intn(len(attendanceStatuses))]
			enrollmentDate := time.Now().AddDate(0, 0, -rng.Intn(30))

			var id int64
			err := tx.QueryRowContext(ctx, enrollmentInsertQuery,
				studentID, groupID, enrollmentDate, status, time.Now(), time.Now()).Scan(&id)

			if err != nil {
				return nil, fmt.Errorf("failed to create enrollment for student %d: %w", studentID, err)
			}
			enrollmentIDs = append(enrollmentIDs, id)
			enrollmentCounts[groupID]++
			studentsWithActivities[studentID] = 1
			enrolled = true
		}

		if !enrolled {
			return nil, fmt.Errorf("could not enroll student %d in any activity - check capacity", studentID)
		}
	}

	// Second pass: Add additional activities to students (1-2 more each)
	fmt.Println("Second pass: Adding additional activities to students...")
	for _, studentID := range studentIDs {
		// Each student gets 1-2 additional activities (total 2-3)
		additionalActivities := rng.Intn(2) + 1

		// Shuffle activities for random assignment
		shuffledActivities := make([]int64, len(activityGroupIDs))
		copy(shuffledActivities, activityGroupIDs)
		for i := len(shuffledActivities) - 1; i > 0; i-- {
			j := rng.Intn(i + 1)
			shuffledActivities[i], shuffledActivities[j] = shuffledActivities[j], shuffledActivities[i]
		}

		enrolledCount := 0
		for i := 0; i < len(shuffledActivities) && enrolledCount < additionalActivities; i++ {
			groupID := shuffledActivities[i]

			// Check if student is already enrolled in this activity
			var exists bool
			err := tx.QueryRowContext(ctx,
				"SELECT EXISTS(SELECT 1 FROM activities.student_enrollments WHERE student_id = ? AND activity_group_id = ?)",
				studentID, groupID).Scan(&exists)
			if err != nil {
				return nil, fmt.Errorf("failed to check existing enrollment: %w", err)
			}
			if exists {
				continue
			}

			// Check if activity has reached its target fill rate
			currentFillRate := float64(enrollmentCounts[groupID]) / float64(maxParticipants[groupID])
			if currentFillRate >= targetFillRates[groupID] {
				continue
			}

			status := attendanceStatuses[rng.Intn(len(attendanceStatuses))]
			enrollmentDate := time.Now().AddDate(0, 0, -rng.Intn(30))

			var id int64
			err = tx.QueryRowContext(ctx, enrollmentInsertQuery,
				studentID, groupID, enrollmentDate, status, time.Now(), time.Now()).Scan(&id)

			if err != nil {
				return nil, fmt.Errorf("failed to create additional enrollment for student %d: %w", studentID, err)
			}
			enrollmentIDs = append(enrollmentIDs, id)
			enrollmentCounts[groupID]++
			studentsWithActivities[studentID]++
			enrolledCount++
		}
	}

	// Print statistics
	fmt.Println("\nActivity enrollment statistics:")
	totalEnrollments := 0
	totalCapacity := 0
	for groupID, count := range enrollmentCounts {
		totalEnrollments += count
		totalCapacity += maxParticipants[groupID]
		fillRate := float64(count) / float64(maxParticipants[groupID]) * 100
		if count > 0 {
			fmt.Printf("  Activity Group %d: %d/%d enrolled (%.1f%% full, target: %.1f%%)\n",
				groupID, count, maxParticipants[groupID], fillRate, targetFillRates[groupID]*100)
		}
	}

	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Total enrollments: %d\n", totalEnrollments)
	fmt.Printf("  Total capacity: %d\n", totalCapacity)
	fmt.Printf("  Overall fill rate: %.1f%%\n", float64(totalEnrollments)/float64(totalCapacity)*100)

	// Student distribution statistics
	activityCounts := make(map[int]int) // How many students have X activities
	for _, count := range studentsWithActivities {
		activityCounts[count]++
	}
	fmt.Printf("\nStudent activity distribution:\n")
	for count := 1; count <= 3; count++ {
		if num, ok := activityCounts[count]; ok {
			fmt.Printf("  %d students have %d %s\n", num, count,
				func() string {
					if count == 1 {
						return "activity"
					} else {
						return "activities"
					}
				}())
		}
	}

	return enrollmentIDs, nil
}

func seedIoTDevices(ctx context.Context, tx bun.Tx, registrarPersonIDs []int64, rng *rand.Rand) ([]int64, error) {
	deviceData := []struct {
		deviceID   string
		deviceType string
		name       string
		status     string
	}{
		{"RFID-MAIN-001", "rfid_reader", "Haupteingang-Scanner", "active"},
		{"RFID-MENSA-001", "rfid_reader", "Mensa-Leser", "active"},
		{"RFID-OGS-001", "rfid_reader", "OGS-Bereich-Scanner", "active"},
		{"RFID-SPORT-001", "rfid_reader", "Sporthalle-Terminal", "active"},
		{"RFID-LIB-001", "rfid_reader", "Bibliothek-Terminal", "active"},
		{"TEMP-CLASS-001", "temperature_sensor", "Temperatursensor Klassenzimmer", "active"},
		{"TEMP-MENSA-001", "temperature_sensor", "Temperatursensor Mensa", "active"},
	}

	deviceIDs := make([]int64, 0, len(deviceData))
	for i, data := range deviceData {
		registrarID := registrarPersonIDs[i%len(registrarPersonIDs)]
		lastSeen := time.Now().Add(-time.Duration(rng.Intn(60)) * time.Minute) // Last seen within last hour

		var id int64
		query := `INSERT INTO iot.devices (device_id, device_type, name, status, last_seen, registered_by_id, created_at, updated_at) 
		          VALUES (?, ?, ?, ?::device_status, ?, ?, ?, ?) RETURNING id`

		err := tx.QueryRowContext(ctx, query,
			data.deviceID, data.deviceType, data.name, data.status, lastSeen, registrarID,
			time.Now(), time.Now()).Scan(&id)

		if err != nil {
			return nil, fmt.Errorf("failed to create IoT device %s: %w", data.name, err)
		}
		deviceIDs = append(deviceIDs, id)
	}

	return deviceIDs, nil
}

func seedPrivacyConsents(ctx context.Context, tx bun.Tx, studentIDs []int64, rng *rand.Rand) ([]int64, error) {
	policyVersions := []string{"DSGVO-2023-v1.0", "DSGVO-2023-v1.1", "DSGVO-2024-v1.0"}
	consentIDs := make([]int64, 0)

	for _, studentID := range studentIDs {
		// 90% of students have consents
		if rng.Float32() < 0.9 {
			policyVersion := policyVersions[rng.Intn(len(policyVersions))]
			accepted := true
			acceptedAt := time.Now().AddDate(0, 0, -rng.Intn(180))           // Accepted within last 6 months
			durationDays := 365                                              // 1 year validity
			expiresAt := acceptedAt.AddDate(1, 0, 0)                         // 1 year from acceptance
			renewalRequired := expiresAt.Before(time.Now().AddDate(0, 1, 0)) // Renewal needed if expires within a month

			var id int64
			query := `INSERT INTO users.privacy_consents (student_id, policy_version, accepted, accepted_at, expires_at, duration_days, renewal_required, created_at, updated_at) 
			          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`

			err := tx.QueryRowContext(ctx, query,
				studentID, policyVersion, accepted, acceptedAt, expiresAt, durationDays, renewalRequired,
				time.Now(), time.Now()).Scan(&id)

			if err != nil {
				return nil, fmt.Errorf("failed to create privacy consent for student %d: %w", studentID, err)
			}
			consentIDs = append(consentIDs, id)
		}
	}

	return consentIDs, nil
}

func seedGroupTeachers(ctx context.Context, tx bun.Tx, groupIDs []int64, teacherIDs []int64, rng *rand.Rand) error {
	// Assign 1-2 teachers to each education group
	for _, groupID := range groupIDs {
		// Randomly pick 1-2 teachers for this group
		numTeachers := rng.Intn(2) + 1 // 1 or 2 teachers

		// Shuffle teachers and pick the first numTeachers
		shuffledTeachers := make([]int64, len(teacherIDs))
		copy(shuffledTeachers, teacherIDs)
		for i := len(shuffledTeachers) - 1; i > 0; i-- {
			j := rng.Intn(i + 1)
			shuffledTeachers[i], shuffledTeachers[j] = shuffledTeachers[j], shuffledTeachers[i]
		}

		for i := 0; i < numTeachers && i < len(shuffledTeachers); i++ {
			teacherID := shuffledTeachers[i]

			query := `INSERT INTO education.group_teacher (group_id, teacher_id, created_at, updated_at) 
			          VALUES (?, ?, ?, ?) ON CONFLICT (group_id, teacher_id) DO NOTHING`

			_, err := tx.ExecContext(ctx, query,
				groupID, teacherID, time.Now(), time.Now())

			if err != nil {
				return fmt.Errorf("failed to assign teacher %d to group %d: %w", teacherID, groupID, err)
			}
		}
	}

	return nil
}
