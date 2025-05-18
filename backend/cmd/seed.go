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
	Short: "Seed the database with dummy data",
	Long: `Seed the database with dummy data for testing purposes.
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
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
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
	defer db.Close()

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
		// Initialize random seed
		rand.Seed(time.Now().UnixNano())

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
		personIDs, err := seedPersons(ctx, tx, rfidIDs)
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
		studentPersonIDs := personIDs[30:]                                        // Remaining persons are students
		studentIDs, err := seedStudents(ctx, tx, studentPersonIDs, groupIDs[:10]) // First 10 groups are grade classes
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
		// Classrooms
		{"101", "Main Building", 1, 30, "Classroom", "#4A90E2"},
		{"102", "Main Building", 1, 30, "Classroom", "#4A90E2"},
		{"103", "Main Building", 1, 25, "Classroom", "#4A90E2"},
		{"201", "Main Building", 2, 35, "Classroom", "#4A90E2"},
		{"202", "Main Building", 2, 30, "Classroom", "#4A90E2"},
		{"203", "Main Building", 2, 28, "Classroom", "#4A90E2"},
		// Science Labs
		{"Lab 1", "Science Building", 1, 24, "Laboratory", "#50E3C2"},
		{"Lab 2", "Science Building", 1, 24, "Laboratory", "#50E3C2"},
		{"Chemistry Lab", "Science Building", 2, 20, "Laboratory", "#50E3C2"},
		{"Physics Lab", "Science Building", 2, 20, "Laboratory", "#50E3C2"},
		// Sports Facilities
		{"Main Gym", "Sports Complex", 1, 100, "Sports", "#7ED321"},
		{"Small Gym", "Sports Complex", 1, 50, "Sports", "#7ED321"},
		// Art Rooms
		{"Art Studio 1", "Creative Wing", 1, 20, "Art", "#F5A623"},
		{"Music Room", "Creative Wing", 1, 25, "Music", "#BD10E0"},
		// Computer Labs
		{"Computer Lab 1", "Tech Center", 1, 30, "Computer", "#9013FE"},
		{"Computer Lab 2", "Tech Center", 2, 30, "Computer", "#9013FE"},
		// Library
		{"Main Library", "Library Building", 1, 80, "Library", "#B8E986"},
		{"Study Room 1", "Library Building", 2, 15, "Study", "#B8E986"},
		// Special Purpose
		{"Cafeteria", "Main Building", 0, 200, "Dining", "#F8E71C"},
		{"Auditorium", "Main Building", 1, 300, "Assembly", "#D0021B"},
		{"Nurse's Office", "Main Building", 1, 10, "Medical", "#FF6900"},
		// Offices
		{"Principal's Office", "Admin Building", 1, 5, "Office", "#A020F0"},
		{"Teachers' Lounge", "Admin Building", 1, 40, "Office", "#A020F0"},
		{"Conference Room", "Admin Building", 2, 20, "Meeting", "#A020F0"},
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
		// Grade classes - assign to available classrooms
		{"Class 1A", room1},
		{"Class 1B", room2},
		{"Class 2A", room3},
		{"Class 2B", room4},
		{"Class 3A", room5},
		{"Class 3B", room6},
		{"Class 4A", nil}, // No more classrooms available
		{"Class 4B", nil},
		{"Class 5A", nil},
		{"Class 5B", nil},
		// Activity groups - no specific room assignment
		{"Science Club", nil},
		{"Art Club", nil},
		{"Drama Club", nil},
		{"Math Club", nil},
		{"Chess Club", nil},
		{"Sports Team A", nil},
		{"Sports Team B", nil},
		{"Music Band", nil},
		{"Debate Club", nil},
		{"Computer Club", nil},
		// Special groups
		{"After School Care", nil},
		{"Morning Care", nil},
		{"Homework Help", nil},
		{"Reading Club", nil},
		{"Environmental Club", nil},
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

func seedPersons(ctx context.Context, tx bun.Tx, rfidIDs []string) ([]int64, error) {
	// First names
	firstNames := []string{
		"Emma", "Liam", "Olivia", "Noah", "Ava", "Ethan", "Sophia", "Mason",
		"Isabella", "William", "Mia", "James", "Charlotte", "Benjamin", "Amelia",
		"Lucas", "Harper", "Henry", "Evelyn", "Alexander", "Abigail", "Michael",
		"Emily", "Elijah", "Elizabeth", "Daniel", "Mila", "Aiden", "Ella",
		"Matthew", "Avery", "Joseph", "Sofia", "Samuel", "Camila", "David",
		"Aria", "Carter", "Scarlett", "Jackson", "Victoria", "Sebastian", "Madison",
		"Jack", "Luna", "Owen", "Grace", "Luke", "Chloe", "Gabriel",
	}

	// Last names
	lastNames := []string{
		"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller",
		"Davis", "Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez",
		"Wilson", "Anderson", "Thomas", "Taylor", "Moore", "Jackson", "Martin",
		"Lee", "Perez", "Thompson", "White", "Harris", "Sanchez", "Clark",
		"Ramirez", "Lewis", "Robinson", "Walker", "Young", "Allen", "King",
		"Wright", "Scott", "Torres", "Nguyen", "Hill", "Flores", "Green",
		"Adams", "Nelson", "Baker", "Hall", "Rivera", "Campbell", "Mitchell",
		"Carter", "Roberts",
	}

	// Create 150 persons (30 staff/teachers + 120 students)
	personIDs := make([]int64, 0, 150)
	for i := 0; i < 150; i++ {
		firstName := firstNames[rand.Intn(len(firstNames))]
		lastName := lastNames[rand.Intn(len(lastNames))]

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
		"Experienced team member",
		"Department head",
		"Subject coordinator",
		"Administrative support",
		"Senior staff member",
		"New hire - training in progress",
		"Part-time staff",
		"Full-time staff",
		"Support staff",
		"Technical specialist",
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
		{"Mathematics", "Senior Math Teacher", "M.Ed. Mathematics, 10 years experience"},
		{"Mathematics", "Math Teacher", "B.Ed. Mathematics, 5 years experience"},
		{"Science", "Head of Science", "Ph.D. Physics, 15 years experience"},
		{"Science", "Science Teacher", "M.Sc. Chemistry, 7 years experience"},
		{"English", "English Department Head", "M.A. English Literature, 12 years experience"},
		{"English", "English Teacher", "B.A. English, 3 years experience"},
		{"History", "History Teacher", "M.A. History, 8 years experience"},
		{"Geography", "Geography Teacher", "B.Ed. Geography, 6 years experience"},
		{"Physical Education", "PE Coordinator", "B.Ed. Physical Education, 10 years experience"},
		{"Physical Education", "PE Teacher", "Sports Science Degree, 4 years experience"},
		{"Art", "Art Teacher", "B.F.A., 5 years experience"},
		{"Music", "Music Teacher", "B.Mus., 7 years experience"},
		{"Computer Science", "IT Teacher", "B.Sc. Computer Science, 6 years experience"},
		{"Languages", "Spanish Teacher", "B.A. Spanish, Native Speaker"},
		{"Languages", "French Teacher", "M.A. French Literature, 9 years experience"},
		{"Special Education", "Special Ed Coordinator", "M.Ed. Special Education, 11 years experience"},
		{"Library", "Librarian", "M.L.S., 8 years experience"},
		{"Counseling", "School Counselor", "M.Ed. Counseling, 13 years experience"},
		{"Administration", "Vice Principal", "M.Ed. Administration, 15 years experience"},
		{"Administration", "Principal", "Ph.D. Education, 20 years experience"},
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

func seedStudents(ctx context.Context, tx bun.Tx, personIDs []int64, classGroupIDs []int64) ([]int64, error) {
	// Guardian first names
	guardianFirstNames := []string{
		"John", "Mary", "Robert", "Patricia", "James", "Jennifer", "Michael",
		"Linda", "William", "Elizabeth", "David", "Barbara", "Richard", "Susan",
		"Joseph", "Jessica", "Thomas", "Sarah", "Charles", "Karen",
	}

	// Grades for school classes
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

		guardianFirstName := guardianFirstNames[rand.Intn(len(guardianFirstNames))]
		guardianName := fmt.Sprintf("%s %s", guardianFirstName, personLastName)

		// Generate contact info
		guardianPhone := fmt.Sprintf("+1 555-%03d-%04d", rand.Intn(1000), rand.Intn(10000))
		guardianEmail := fmt.Sprintf("%s.%s@email.com",
			strings.ToLower(guardianFirstName),
			strings.ToLower(personLastName))

		// Randomly set initial location (most are "in house")
		bus := false
		inHouse := true
		if rand.Float32() < 0.3 {
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
