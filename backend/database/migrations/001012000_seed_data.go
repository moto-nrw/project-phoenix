package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/uptrace/bun"
)

const (
	SeedDataVersion     = "1.12.0"
	SeedDataDescription = "Seed essential data and sample records"
)

func init() {
	// Migration 12: Seed essential data and sample records
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return seedDataUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return seedDataDown(ctx, db)
		},
	)
}

// seedDataUp populates the database with essential and sample data
func seedDataUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Seeding essential and sample data...")

	// Determine environment
	appEnv := strings.ToLower(os.Getenv("APP_ENV"))
	isDev := appEnv == "development" || appEnv == "dev" || appEnv == ""

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Seed essential system settings (needed for all environments)
	if err := seedEssentialSettings(ctx, tx); err != nil {
		return fmt.Errorf("error seeding essential settings: %w", err)
	}

	// 2. Seed default admin account (needed for all environments)
	if err := seedDefaultAdmin(ctx, tx); err != nil {
		return fmt.Errorf("error seeding default admin account: %w", err)
	}

	// 3. Seed development/testing data (only in development mode)
	if isDev {
		fmt.Println("Development environment detected - seeding sample data...")
		if err := seedSampleData(ctx, tx); err != nil {
			return fmt.Errorf("error seeding sample data: %w", err)
		}
	}

	// Commit the transaction
	return tx.Commit()
}

// seedDataDown removes seeded data (mainly used for testing/development)
func seedDataDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Removing seeded data...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Allow overriding to remove all data even in production
	forceCleanup := strings.ToLower(os.Getenv("FORCE_SEED_CLEANUP")) == "true"

	// In production, don't remove essential data unless explicitly requested
	appEnv := strings.ToLower(os.Getenv("APP_ENV"))
	isProduction := appEnv == "production" || appEnv == "prod"

	if !isProduction || forceCleanup {
		// Remove sample data and essential data
		_, err = tx.ExecContext(ctx, `
			-- Remove sample data
			DELETE FROM students WHERE school_class LIKE 'Sample%';
			DELETE FROM groups WHERE name LIKE 'Sample%';
			DELETE FROM rfid_cards WHERE id NOT IN (SELECT tag_id FROM custom_users WHERE tag_id IS NOT NULL);
			DELETE FROM pedagogical_specialists WHERE role LIKE 'Sample%';
			DELETE FROM custom_users WHERE first_name LIKE 'Sample%';
			
			-- Remove admin accounts except the default one if in production
			DELETE FROM accounts WHERE email != 'admin@example.com' 
				OR (email = 'admin@example.com' AND ? = true);
			
			-- Only remove all settings if cleanup is forced
			DELETE FROM settings WHERE ? = true;
		`, forceCleanup, forceCleanup)

		if err != nil {
			return fmt.Errorf("error removing seeded data: %w", err)
		}
	} else {
		fmt.Println("Production environment detected - essential data not removed")
		fmt.Println("Use FORCE_SEED_CLEANUP=true to override this behavior")
	}

	// Commit the transaction
	return tx.Commit()
}

// seedEssentialSettings adds core system settings needed for application function
func seedEssentialSettings(ctx context.Context, tx bun.Tx) error {
	// Define essential settings
	settings := []map[string]interface{}{
		{
			"key":              "system.name",
			"value":            "Project Phoenix",
			"category":         "system",
			"description":      "The name of the application instance",
			"requires_restart": false,
		},
		{
			"key":              "system.maintenance_mode",
			"value":            "false",
			"category":         "system",
			"description":      "When enabled, only admins can access the system",
			"requires_restart": false,
		},
		{
			"key":              "security.session_timeout_minutes",
			"value":            "60",
			"category":         "security",
			"description":      "User session timeout in minutes",
			"requires_restart": false,
		},
		{
			"key":              "security.max_login_attempts",
			"value":            "5",
			"category":         "security",
			"description":      "Maximum number of failed login attempts before temporary lockout",
			"requires_restart": false,
		},
		{
			"key":              "security.lockout_duration_minutes",
			"value":            "15",
			"category":         "security",
			"description":      "Account lockout duration in minutes after too many failed login attempts",
			"requires_restart": false,
		},
		{
			"key":              "rfid.check_interval_seconds",
			"value":            "5",
			"category":         "rfid",
			"description":      "Interval between RFID presence checks in seconds",
			"requires_restart": true,
		},
		{
			"key":              "rfid.timeout_seconds",
			"value":            "60",
			"category":         "rfid",
			"description":      "Time in seconds before an RFID card is considered inactive",
			"requires_restart": false,
		},
	}

	// Insert settings if they don't exist yet
	for _, setting := range settings {
		var exists bool
		err := tx.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM settings WHERE key = ?)
		`, setting["key"]).Scan(&exists)

		if err != nil {
			return fmt.Errorf("error checking if setting '%s' exists: %w", setting["key"], err)
		}

		if !exists {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO settings (
					key, value, category, description, requires_restart
				) VALUES (?, ?, ?, ?, ?)
			`,
				setting["key"],
				setting["value"],
				setting["category"],
				setting["description"],
				setting["requires_restart"])

			if err != nil {
				return fmt.Errorf("error inserting setting '%s': %w", setting["key"], err)
			}
		}
	}

	return nil
}

// seedDefaultAdmin adds a default admin account
func seedDefaultAdmin(ctx context.Context, tx bun.Tx) error {
	// Check if admin exists
	var adminExists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM accounts WHERE email = 'admin@example.com')
	`).Scan(&adminExists)

	if err != nil {
		return fmt.Errorf("error checking if admin exists: %w", err)
	}

	if !adminExists {
		// Get default admin password from environment or use default
		adminPassword := os.Getenv("DEFAULT_ADMIN_PASSWORD")
		if adminPassword == "" {
			adminPassword = "Test1234%" // Default password - should be changed immediately
		}

		// Hash the password using Argon2id (consistent with authentication)
		passwordHash, err := userpass.HashPassword(adminPassword, userpass.DefaultParams())
		if err != nil {
			return fmt.Errorf("error hashing admin password: %w", err)
		}

		// Insert admin account
		_, err = tx.ExecContext(ctx, `
			INSERT INTO accounts (
				created_at, updated_at, email, username, name, active, 
				roles, password_hash
			) VALUES (
				?, ?, ?, ?, ?, ?,
				?, ?
			)
		`,
			time.Now(), time.Now(), "admin@example.com", "admin",
			"System Administrator", true,
			`{"admin"}`, passwordHash)

		if err != nil {
			return fmt.Errorf("error inserting admin account: %w", err)
		}

		fmt.Println("Created default admin account (admin@example.com)")
		fmt.Println("With password:", adminPassword)
		fmt.Println("IMPORTANT: Please change the default admin password immediately!")
	}

	return nil
}

// seedSampleData adds sample data for development and testing
func seedSampleData(ctx context.Context, tx bun.Tx) error {
	// 1. Create sample users
	sampleUsers := []struct {
		FirstName  string
		SecondName string
		TagID      string
	}{
		{"Sample", "Teacher", "SAMPLE001"},
		{"Sample", "Student", "SAMPLE002"},
		{"Sample", "Admin", "SAMPLE003"},
	}

	for _, user := range sampleUsers {
		var exists bool
		err := tx.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM custom_users 
				WHERE first_name = ? AND second_name = ?
			)
		`, user.FirstName, user.SecondName).Scan(&exists)

		if err != nil {
			return fmt.Errorf("error checking if sample user exists: %w", err)
		}

		if !exists {
			// First make sure the tags ID exists in rfid_cards
			var cardExists bool
			err = tx.QueryRowContext(ctx, `
				SELECT EXISTS(SELECT 1 FROM rfid_cards WHERE id = ?)
			`, user.TagID).Scan(&cardExists)

			if err != nil {
				return fmt.Errorf("error checking if RFID card exists: %w", err)
			}

			if !cardExists {
				_, err = tx.ExecContext(ctx, `
					INSERT INTO rfid_cards (id, active) VALUES (?, true)
				`, user.TagID)

				if err != nil {
					return fmt.Errorf("error inserting RFID card: %w", err)
				}
			}

			// Then create the user
			_, err = tx.ExecContext(ctx, `
				INSERT INTO custom_users (
					first_name, second_name, tag_id, created_at, modified_at
				) VALUES (
					?, ?, ?, ?, ?
				)
			`, user.FirstName, user.SecondName, user.TagID, time.Now(), time.Now())

			if err != nil {
				return fmt.Errorf("error inserting sample user: %w", err)
			}
		}
	}

	// 2. Create sample rooms
	sampleRooms := []struct {
		RoomName string
		Building string
		Floor    int
		Capacity int
		Category string
		Color    string
	}{
		{"Room 101", "Main Building", 1, 25, "Classroom", "#4287f5"}, // Blue
		{"Room 102", "Main Building", 1, 30, "Classroom", "#42f560"}, // Green
		{"Library", "Main Building", 2, 50, "Study", "#f54242"},      // Red
		{"Computer Lab", "Science Wing", 1, 20, "Lab", "#f5a442"},    // Orange
		{"Cafeteria", "Main Building", 0, 100, "Other", "#b042f5"},   // Purple
		{"Gymnasium", "Sports Hall", 0, 150, "Activity", "#f542e3"},  // Pink
	}

	roomIds := make(map[string]int64)

	for _, room := range sampleRooms {
		var exists bool
		err := tx.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM rooms WHERE room_name = ?)
		`, room.RoomName).Scan(&exists)

		if err != nil {
			return fmt.Errorf("error checking if room exists: %w", err)
		}

		if !exists {
			// Insert the room
			_, err = tx.ExecContext(ctx, `
				INSERT INTO rooms (
					room_name, building, floor, capacity, category, color, created_at, modified_at
				) VALUES (
					?, ?, ?, ?, ?, ?, ?, ?
				)
				RETURNING id
			`, room.RoomName, room.Building, room.Floor, room.Capacity, room.Category, room.Color, time.Now(), time.Now())

			if err != nil {
				return fmt.Errorf("error inserting room: %w", err)
			}

			// Get last inserted ID
			var roomId int64
			err = tx.QueryRowContext(ctx, `SELECT lastval()`).Scan(&roomId)
			if err != nil {
				return fmt.Errorf("error getting room id: %w", err)
			}

			roomIds[room.RoomName] = roomId
			fmt.Printf("Created room %s with ID %d\n", room.RoomName, roomId)
		} else {
			// Get existing room ID
			var roomId int64
			err = tx.QueryRowContext(ctx, `
				SELECT id FROM rooms WHERE room_name = ?
			`, room.RoomName).Scan(&roomId)

			if err != nil {
				return fmt.Errorf("error getting existing room id: %w", err)
			}

			roomIds[room.RoomName] = roomId
		}
	}

	// 3. Create sample groups
	sampleGroups := []struct {
		Name     string
		RoomName string // Will be mapped to room_id
	}{
		{"Sample Group A", "Room 101"},
		{"Sample Group B", "Room 102"},
		{"Sample Combined Group", "Library"},
		{"Computer Club", "Computer Lab"},
		{"Sports Team", "Gymnasium"},
		{"Lunch Group", "Cafeteria"},
	}

	for _, group := range sampleGroups {
		var exists bool
		err := tx.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM groups WHERE name = ?)
		`, group.Name).Scan(&exists)

		if err != nil {
			return fmt.Errorf("error checking if sample group exists: %w", err)
		}

		if !exists {
			roomId, ok := roomIds[group.RoomName]
			if !ok {
				fmt.Printf("Warning: Room %s not found for group %s\n", group.RoomName, group.Name)
				roomId = 0 // NULL value for room_id
			}

			// Insert the group with room_id reference
			_, err = tx.ExecContext(ctx, `
				INSERT INTO groups (
					name, room_id, created_at, modified_at
				) VALUES (
					?, ?, ?, ?
				)
			`, group.Name, roomId, time.Now(), time.Now())

			if err != nil {
				return fmt.Errorf("error inserting sample group: %w", err)
			}

			fmt.Printf("Created group %s with room_id %d\n", group.Name, roomId)
		} else {
			// Check if we need to update the room_id for existing groups
			roomId, ok := roomIds[group.RoomName]
			if ok {
				_, err = tx.ExecContext(ctx, `
					UPDATE groups SET room_id = ? WHERE name = ?
				`, roomId, group.Name)

				if err != nil {
					return fmt.Errorf("error updating group with room_id: %w", err)
				}

				fmt.Printf("Updated existing group %s with room_id %d\n", group.Name, roomId)
			}
		}
	}

	// 4. Create sample pedagogical specialists
	// First get the user ID for the sample teacher
	var sampleTeacherID int64
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM custom_users 
		WHERE first_name = 'Sample' AND second_name = 'Teacher'
	`).Scan(&sampleTeacherID)

	if err != nil {
		return fmt.Errorf("error getting sample teacher ID: %w", err)
	}

	var specialistExists bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM pedagogical_specialists
			WHERE user_id = ?
		)
	`, sampleTeacherID).Scan(&specialistExists)

	if err != nil {
		return fmt.Errorf("error checking if specialist exists: %w", err)
	}

	if !specialistExists && sampleTeacherID > 0 {
		// Create a sample account for the specialist
		teacherPassword, err := userpass.HashPassword("Teacher1234%", userpass.DefaultParams())
		if err != nil {
			return fmt.Errorf("error hashing teacher password: %w", err)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO accounts (
				created_at, updated_at, email, username, name, active, 
				roles, password_hash
			) VALUES (
				?, ?, ?, ?, ?, ?,
				?, ?
			)
		`,
			time.Now(), time.Now(),
			"sample.teacher@example.com", "sample.teacher",
			"Sample Teacher", true,
			`{"teacher"}`, teacherPassword)

		if err != nil {
			return fmt.Errorf("error inserting sample specialist account: %w", err)
		}

		// Get the account ID
		var accountID int64
		err = tx.QueryRowContext(ctx, `
			SELECT id FROM accounts WHERE email = 'sample.teacher@example.com'
		`).Scan(&accountID)

		if err != nil {
			return fmt.Errorf("error getting sample account ID: %w", err)
		}

		// Update the custom user with the account ID
		_, err = tx.ExecContext(ctx, `
			UPDATE custom_users SET account_id = ? WHERE id = ?
		`, accountID, sampleTeacherID)

		if err != nil {
			return fmt.Errorf("error updating sample user with account ID: %w", err)
		}

		// Create the specialist record
		_, err = tx.ExecContext(ctx, `
			INSERT INTO pedagogical_specialists (
				specialization, user_id, created_at, modified_at
			) VALUES (
				'Teacher', ?, ?, ?
			)
		`, accountID, time.Now(), time.Now())

		if err != nil {
			return fmt.Errorf("error inserting pedagogical specialist: %w", err)
		}
	}

	// 4. Create student records linked to sample users
	// Get the student user ID
	var sampleStudentID int64
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM custom_users 
		WHERE first_name = 'Sample' AND second_name = 'Student'
	`).Scan(&sampleStudentID)

	if err != nil {
		return fmt.Errorf("error getting sample student ID: %w", err)
	}

	// Check if a student record already exists for this user
	var studentExists bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM students 
			WHERE custom_users_id = ?
		)
	`, sampleStudentID).Scan(&studentExists)

	if err != nil {
		return fmt.Errorf("error checking if student record exists: %w", err)
	}

	// If the student doesn't exist and we have a valid user ID, create the student record
	if !studentExists && sampleStudentID > 0 {
		// Get the first group ID to use
		var groupID int64
		err = tx.QueryRowContext(ctx, `
			SELECT id FROM groups WHERE name = 'Sample Group A'
		`).Scan(&groupID)

		if err != nil {
			return fmt.Errorf("error getting sample group ID: %w", err)
		}

		if groupID > 0 {
			// Create the student record
			_, err = tx.ExecContext(ctx, `
				INSERT INTO students (
					school_class, bus, name_lg, contact_lg, in_house, wc, school_yard, 
					custom_users_id, group_id, created_at, modified_at
				) VALUES (
					?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
				)
			`,
				"Sample Class 5A",    // School class
				false,                // Bus
				"Parent Name",        // Legal guardian name
				"parent@example.com", // Legal guardian contact
				false,                // In house
				false,                // WC
				false,                // School yard
				sampleStudentID,      // Custom user ID
				groupID,              // Group ID
				time.Now(),           // Created at
				time.Now(),           // Modified at
			)

			if err != nil {
				return fmt.Errorf("error creating student record: %w", err)
			}

			fmt.Println("Created sample student record")
		}
	}

	// 6. Create another example student with different details
	// Check if we've already created the second sample student
	var secondStudentExists bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM students 
			WHERE school_class = 'Sample Class 6B'
		)
	`).Scan(&secondStudentExists)

	if err != nil {
		return fmt.Errorf("error checking if second student record exists: %w", err)
	}

	if !secondStudentExists {
		// Get the second group ID to use
		var groupID int64
		err = tx.QueryRowContext(ctx, `
			SELECT id FROM groups WHERE name = 'Sample Group B'
		`).Scan(&groupID)

		if err != nil {
			return fmt.Errorf("error getting second sample group ID: %w", err)
		}

		// Get the admin user ID for our example
		var adminUserID int64
		err = tx.QueryRowContext(ctx, `
			SELECT id FROM custom_users 
			WHERE first_name = 'Sample' AND second_name = 'Admin'
		`).Scan(&adminUserID)

		if err != nil {
			return fmt.Errorf("error getting sample admin ID: %w", err)
		}

		if groupID > 0 && adminUserID > 0 {
			// Create the student record
			_, err = tx.ExecContext(ctx, `
				INSERT INTO students (
					school_class, bus, name_lg, contact_lg, in_house, wc, school_yard, 
					custom_users_id, group_id, created_at, modified_at
				) VALUES (
					?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
				)
			`,
				"Sample Class 6B",      // School class
				true,                   // Bus (this one takes the bus)
				"Guardian Example",     // Legal guardian name
				"guardian@example.com", // Legal guardian contact
				true,                   // In house (this one is in the house)
				false,                  // WC
				false,                  // School yard
				adminUserID,            // Custom user ID
				groupID,                // Group ID
				time.Now(),             // Created at
				time.Now(),             // Modified at
			)

			if err != nil {
				return fmt.Errorf("error creating second student record: %w", err)
			}

			fmt.Println("Created second sample student record")
		}
	}

	// 7. Create activity group categories
	sampleAGCategories := []string{
		"Sport",
		"Music",
		"Art",
		"Science",
		"Languages",
	}

	agCategoryIds := make(map[string]int64)

	for _, category := range sampleAGCategories {
		var exists bool
		err := tx.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM ag_categories WHERE name = ?)
		`, category).Scan(&exists)

		if err != nil {
			return fmt.Errorf("error checking if AG category exists: %w", err)
		}

		if !exists {
			// Insert the category
			_, err = tx.ExecContext(ctx, `
				INSERT INTO ag_categories (
					name, created_at
				) VALUES (
					?, ?
				)
				RETURNING id
			`, category, time.Now())

			if err != nil {
				return fmt.Errorf("error inserting AG category: %w", err)
			}

			// Get last inserted ID
			var categoryId int64
			err = tx.QueryRowContext(ctx, `SELECT lastval()`).Scan(&categoryId)
			if err != nil {
				return fmt.Errorf("error getting AG category id: %w", err)
			}

			agCategoryIds[category] = categoryId
			fmt.Printf("Created AG category %s with ID %d\n", category, categoryId)
		} else {
			// Get existing category ID
			var categoryId int64
			err = tx.QueryRowContext(ctx, `
				SELECT id FROM ag_categories WHERE name = ?
			`, category).Scan(&categoryId)

			if err != nil {
				return fmt.Errorf("error getting existing AG category id: %w", err)
			}

			agCategoryIds[category] = categoryId
		}
	}

	// 8. Create timespans for activity groups
	sampleTimespans := []struct {
		StartTime string
		EndTime   string
	}{
		{"08:00:00", "09:30:00"}, // Morning session
		{"10:00:00", "11:30:00"}, // Mid-morning session
		{"13:00:00", "14:30:00"}, // Afternoon session
		{"15:00:00", "16:30:00"}, // Late afternoon session
	}

	timespanIds := make(map[string]int64)

	for _, timespan := range sampleTimespans {
		key := timespan.StartTime + "-" + timespan.EndTime
		var exists bool
		err := tx.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM timespans 
				WHERE start_time::time::text LIKE ? AND end_time::time::text LIKE ?
			)
		`, timespan.StartTime+"%", timespan.EndTime+"%").Scan(&exists)

		if err != nil {
			return fmt.Errorf("error checking if timespan exists: %w", err)
		}

		if !exists {
			// Create a timespan that's valid for the current day
			now := time.Now()
			startDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

			// Parse the time values
			startTime, _ := time.Parse("15:04:05", timespan.StartTime)
			endTime, _ := time.Parse("15:04:05", timespan.EndTime)

			// Combine date and time
			start := startDate.Add(time.Duration(startTime.Hour())*time.Hour +
				time.Duration(startTime.Minute())*time.Minute +
				time.Duration(startTime.Second())*time.Second)

			end := startDate.Add(time.Duration(endTime.Hour())*time.Hour +
				time.Duration(endTime.Minute())*time.Minute +
				time.Duration(endTime.Second())*time.Second)

			// Insert the timespan
			_, err = tx.ExecContext(ctx, `
				INSERT INTO timespans (
					start_time, end_time, created_at
				) VALUES (
					?, ?, ?
				)
				RETURNING id
			`, start, end, time.Now())

			if err != nil {
				return fmt.Errorf("error inserting timespan: %w", err)
			}

			// Get last inserted ID
			var timespanId int64
			err = tx.QueryRowContext(ctx, `SELECT lastval()`).Scan(&timespanId)
			if err != nil {
				return fmt.Errorf("error getting timespan id: %w", err)
			}

			timespanIds[key] = timespanId
			fmt.Printf("Created timespan %s with ID %d\n", key, timespanId)
		} else {
			// Get existing timespan ID
			var timespanId int64
			err = tx.QueryRowContext(ctx, `
				SELECT id FROM timespans 
				WHERE start_time::time::text LIKE ? AND end_time::time::text LIKE ? 
				LIMIT 1
			`, timespan.StartTime+"%", timespan.EndTime+"%").Scan(&timespanId)

			if err != nil {
				return fmt.Errorf("error getting existing timespan id: %w", err)
			}

			timespanIds[key] = timespanId
		}
	}

	// 9. Create sample activity groups
	// First make sure we have a pedagogical specialist (supervisor) id
	var supervisorID int64
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM pedagogical_specialists LIMIT 1
	`).Scan(&supervisorID)

	if err != nil {
		fmt.Println("Warning: No supervisor found for AGs, skipping AG creation")
		return nil
	}

	sampleAGs := []struct {
		Name           string
		MaxParticipant int
		IsOpenAG       bool
		Category       string
	}{
		{"Football Club", 20, true, "Sport"},
		{"Chess Club", 15, true, "Sport"},
		{"Piano Lessons", 10, false, "Music"},
		{"Painting Workshop", 15, true, "Art"},
		{"Robotics Lab", 12, true, "Science"},
		{"Spanish Language", 15, false, "Languages"},
	}

	for _, ag := range sampleAGs {
		var exists bool
		err := tx.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM ags WHERE name = ?)
		`, ag.Name).Scan(&exists)

		if err != nil {
			return fmt.Errorf("error checking if AG exists: %w", err)
		}

		if !exists {
			categoryID, ok := agCategoryIds[ag.Category]
			if !ok {
				fmt.Printf("Warning: Category %s not found for AG %s\n", ag.Category, ag.Name)
				continue
			}

			// Insert the activity group
			_, err = tx.ExecContext(ctx, `
				INSERT INTO ags (
					name, max_participant, is_open_ags, supervisor_id, ag_categories_id, 
					created_at, modified_at
				) VALUES (
					?, ?, ?, ?, ?, ?, ?
				)
				RETURNING id
			`, ag.Name, ag.MaxParticipant, ag.IsOpenAG, supervisorID, categoryID, time.Now(), time.Now())

			if err != nil {
				return fmt.Errorf("error inserting AG: %w", err)
			}

			// Get last inserted ID
			var agId int64
			err = tx.QueryRowContext(ctx, `SELECT lastval()`).Scan(&agId)
			if err != nil {
				return fmt.Errorf("error getting AG id: %w", err)
			}

			fmt.Printf("Created AG %s with ID %d\n", ag.Name, agId)

			// 10. Create activity group times (weekdays and timeslots)
			// Assign different weekdays and time slots to each activity
			weekdays := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday"}
			timeSlotKeys := make([]string, 0)
			for k := range timespanIds {
				timeSlotKeys = append(timeSlotKeys, k)
			}

			// Use the AG's position in the list to determine weekday and timeslot
			// This ensures each AG gets a different schedule
			weekdayIndex := int(agId) % len(weekdays)
			timeSlotIndex := (int(agId) / len(weekdays)) % len(timeSlotKeys)

			weekday := weekdays[weekdayIndex]
			timeSlotKey := timeSlotKeys[timeSlotIndex]
			timespanId := timespanIds[timeSlotKey]

			// Insert the AG time
			_, err = tx.ExecContext(ctx, `
				INSERT INTO ag_times (
					weekday, timespans_id, ag_id, created_at
				) VALUES (
					?, ?, ?, ?
				)
			`, weekday, timespanId, agId, time.Now())

			if err != nil {
				return fmt.Errorf("error inserting AG time: %w", err)
			}

			fmt.Printf("Created AG time for %s on %s with timespan ID %d\n", ag.Name, weekday, timespanId)

			// For some AGs, add a second day
			if agId%2 == 0 { // Every other AG gets a second day
				secondWeekdayIndex := (weekdayIndex + 2) % len(weekdays) // Skip a day
				secondWeekday := weekdays[secondWeekdayIndex]

				// Insert the second AG time
				_, err = tx.ExecContext(ctx, `
					INSERT INTO ag_times (
						weekday, timespans_id, ag_id, created_at
					) VALUES (
						?, ?, ?, ?
					)
				`, secondWeekday, timespanId, agId, time.Now())

				if err != nil {
					return fmt.Errorf("error inserting second AG time: %w", err)
				}

				fmt.Printf("Created second AG time for %s on %s with timespan ID %d\n", ag.Name, secondWeekday, timespanId)
			}
		}
	}

	// 11. Enroll some sample students in activities
	// First check if we have sample students
	var studentIDs []int64
	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM students WHERE school_class LIKE 'Sample%' LIMIT 5
	`)
	if err != nil {
		return fmt.Errorf("error querying sample students: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("error scanning student ID: %w", err)
		}
		studentIDs = append(studentIDs, id)
	}

	if len(studentIDs) > 0 {
		// Get all activity group IDs
		var agIDs []int64
		agRows, err := tx.QueryContext(ctx, `
			SELECT id FROM ags WHERE name IN (
				'Football Club', 'Chess Club', 'Piano Lessons', 
				'Painting Workshop', 'Robotics Lab', 'Spanish Language'
			)
		`)
		if err != nil {
			return fmt.Errorf("error querying AGs: %w", err)
		}
		defer agRows.Close()

		for agRows.Next() {
			var id int64
			if err := agRows.Scan(&id); err != nil {
				return fmt.Errorf("error scanning AG ID: %w", err)
			}
			agIDs = append(agIDs, id)
		}

		// Enroll each student in 1-3 activities
		for _, studentID := range studentIDs {
			// Determine how many activities (1-3)
			numActivities := (int(studentID) % 3) + 1
			for i := 0; i < numActivities && i < len(agIDs); i++ {
				agID := agIDs[i]

				// Check if enrollment already exists
				var exists bool
				err := tx.QueryRowContext(ctx, `
					SELECT EXISTS(
						SELECT 1 FROM student_ags 
						WHERE student_id = ? AND ag_id = ?
					)
				`, studentID, agID).Scan(&exists)

				if err != nil {
					return fmt.Errorf("error checking if student enrollment exists: %w", err)
				}

				if !exists {
					// Enroll the student
					_, err = tx.ExecContext(ctx, `
						INSERT INTO student_ags (
							student_id, ag_id, created_at
						) VALUES (
							?, ?, ?
						)
					`, studentID, agID, time.Now())

					if err != nil {
						return fmt.Errorf("error enrolling student in AG: %w", err)
					}

					fmt.Printf("Enrolled student ID %d in activity ID %d\n", studentID, agID)
				}
			}
		}
	}

	return nil
}

// Note: We don't need to redefine registerMigration here as it's already defined in main.go
