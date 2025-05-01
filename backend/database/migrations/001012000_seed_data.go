package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"golang.org/x/crypto/bcrypt"
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
			DELETE FROM pedagogical_specialist WHERE role LIKE 'Sample%';
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
		adminPassword = "admin123" // Default password - should be changed immediately

		// Hash the password
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
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
			`{"admin"}`, string(passwordHash))

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
			// First make sure the tag ID exists in rfid_cards
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

	// 2. Create sample groups
	sampleGroups := []string{
		"Sample Group A",
		"Sample Group B",
		"Sample Combined Group",
	}

	for _, group := range sampleGroups {
		var exists bool
		err := tx.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM groups WHERE name = ?)
		`, group).Scan(&exists)

		if err != nil {
			return fmt.Errorf("error checking if sample group exists: %w", err)
		}

		if !exists {
			// Insert the group
			_, err = tx.ExecContext(ctx, `
				INSERT INTO groups (
					name, created_at, modified_at
				) VALUES (
					?, ?, ?
				)
			`, group, time.Now(), time.Now())

			if err != nil {
				return fmt.Errorf("error inserting sample group: %w", err)
			}
		}
	}

	// 3. Create sample pedagogical specialists
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
			SELECT 1 FROM pedagogical_specialist
			WHERE user_id = ?
		)
	`, sampleTeacherID).Scan(&specialistExists)

	if err != nil {
		return fmt.Errorf("error checking if specialist exists: %w", err)
	}

	if !specialistExists && sampleTeacherID > 0 {
		// Create a sample account for the specialist
		_, err := tx.ExecContext(ctx, `
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
			`{"teacher"}`, "$2a$10$RgXMYCgWUn9OJ6rqUH.PBOjFRTLvgcOJvOTQqfy3BKTjUGFBQkvX2")

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
			INSERT INTO pedagogical_specialist (
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

	// 5. Create another example student with different details
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

	return nil
}

// Note: We don't need to redefine registerMigration here as it's already defined in main.go
