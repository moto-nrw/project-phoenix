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
This command creates:
- 24 rooms (classrooms, labs, gym, etc.)
- 25 groups (grade classes and activity groups)
- 150 persons (30 staff/teachers, 120 students)
- All necessary relationships between entities

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
		rfidIDs, err := seedRFIDCards(ctx, tx)
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
		"users.students",
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

func seedRFIDCards(ctx context.Context, tx bun.Tx) ([]string, error) {
	// Create RFID cards first
	rfidIDs := make([]string, 0, 150)
	for i := 0; i < 150; i++ {
		rfidID := fmt.Sprintf("RFID-%06d", i+1000)

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
		guardianEmail := fmt.Sprintf("%s.%s@gmx.de",
			strings.ToLower(guardianFirstName),
			strings.ToLower(personLastName))

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
