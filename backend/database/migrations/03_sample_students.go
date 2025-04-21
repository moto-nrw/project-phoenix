package migrations

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/uptrace/bun"
)

// AddSampleStudents is a standalone function to add sample data
func AddSampleStudents() {
	db, err := database.DBConn()
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer db.Close()

	fmt.Println("Adding sample students...")
	err = addSampleData(context.Background(), db)
	if err != nil {
		log.Fatalf("Error adding sample data: %v", err)
		os.Exit(1)
	}
	fmt.Println("Sample students added successfully!")
}

func addSampleData(ctx context.Context, db *bun.DB) error {
	fmt.Println("Creating and populating student tables...")

	// First, let's check if the custom_users table exists
	var tableCount int
	err := db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'custom_users'").Scan(&tableCount)
	if err != nil {
		return fmt.Errorf("error checking if custom_users table exists: %w", err)
	}

	if tableCount == 0 {
		fmt.Println("Creating custom_users table...")
		_, err = db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS custom_users (
				id SERIAL PRIMARY KEY,
				first_name TEXT NOT NULL,
				second_name TEXT NOT NULL,
				tag_id TEXT UNIQUE,
				account_id BIGINT UNIQUE,
				created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
				modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			return fmt.Errorf("error creating custom_users table: %w", err)
		}
	}

	// Check if the groups table exists
	err = db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'groups'").Scan(&tableCount)
	if err != nil {
		return fmt.Errorf("error checking if groups table exists: %w", err)
	}

	if tableCount == 0 {
		fmt.Println("Creating groups table...")
		_, err = db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS groups (
				id SERIAL PRIMARY KEY,
				name TEXT NOT NULL,
				created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
				modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			return fmt.Errorf("error creating groups table: %w", err)
		}
	}

	// Check if the students table exists
	err = db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'students'").Scan(&tableCount)
	if err != nil {
		return fmt.Errorf("error checking if students table exists: %w", err)
	}

	if tableCount == 0 {
		fmt.Println("Creating students table...")
		_, err = db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS students (
				id SERIAL PRIMARY KEY,
				school_class TEXT NOT NULL,
				bus BOOLEAN NOT NULL DEFAULT FALSE,
				name_lg TEXT NOT NULL,
				contact_lg TEXT NOT NULL,
				in_house BOOLEAN NOT NULL DEFAULT FALSE,
				wc BOOLEAN NOT NULL DEFAULT FALSE,
				school_yard BOOLEAN NOT NULL DEFAULT FALSE,
				custom_user_id BIGINT NOT NULL,
				group_id BIGINT NOT NULL,
				created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
				modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			return fmt.Errorf("error creating students table: %w", err)
		}
	}

	// Sample student data
	sampleStudents := []struct {
		FirstName   string
		LastName    string
		Grade       string
		InHouse     bool
		ParentName  string
		ParentEmail string
	}{
		{"Anna", "Müller", "1A", true, "Parent of Anna Müller", "parent_anna@example.com"},
		{"Max", "Schmidt", "1A", false, "Parent of Max Schmidt", "parent_max@example.com"},
		{"Sophie", "Weber", "2B", true, "Parent of Sophie Weber", "parent_sophie@example.com"},
		{"Lena", "Fischer", "2B", false, "Parent of Lena Fischer", "parent_lena@example.com"},
		{"Noah", "Meyer", "3C", true, "Parent of Noah Meyer", "parent_noah@example.com"},
		{"Emma", "Wagner", "3C", false, "Parent of Emma Wagner", "parent_emma@example.com"},
		{"Luis", "Becker", "4D", true, "Parent of Luis Becker", "parent_luis@example.com"},
		{"Mia", "Hoffmann", "4D", false, "Parent of Mia Hoffmann", "parent_mia@example.com"},
		{"Finn", "Schneider", "5E", true, "Parent of Finn Schneider", "parent_finn@example.com"},
		{"Lara", "Schulz", "5E", false, "Parent of Lara Schulz", "parent_lara@example.com"},
	}

	// Create sample groups if they don't exist
	for i := 1; i <= 3; i++ {
		// Check if group exists
		var groupExists int
		err := db.QueryRow("SELECT COUNT(*) FROM groups WHERE id = ?", i).Scan(&groupExists)
		if err != nil {
			return fmt.Errorf("error checking if group exists: %w", err)
		}

		if groupExists == 0 {
			_, err = db.ExecContext(ctx, `
				INSERT INTO groups (id, name, created_at, modified_at)
				VALUES (?, ?, ?, ?)
			`, i, fmt.Sprintf("Group %d", i), time.Now(), time.Now())
			if err != nil {
				return fmt.Errorf("error creating group: %w", err)
			}
		}
	}

	// Create sample students
	for i, student := range sampleStudents {
		// First check if the user already exists
		var userCount int
		err := db.QueryRow("SELECT COUNT(*) FROM custom_users WHERE first_name = ? AND second_name = ?",
			student.FirstName, student.LastName).Scan(&userCount)
		if err != nil {
			return fmt.Errorf("error checking existing user: %w", err)
		}

		// Skip if user already exists
		if userCount > 0 {
			fmt.Printf("User %s %s already exists, skipping\n", student.FirstName, student.LastName)
			continue
		}

		// Create a custom user
		var customUserID int64
		err = db.QueryRow(`
			INSERT INTO custom_users (first_name, second_name, created_at, modified_at)
			VALUES (?, ?, ?, ?)
			RETURNING id
		`, student.FirstName, student.LastName, time.Now(), time.Now()).Scan(&customUserID)
		if err != nil {
			return fmt.Errorf("error creating custom user: %w", err)
		}

		// Calculate group ID (1-3)
		groupID := (i % 3) + 1

		// Create a student with this custom user
		_, err = db.ExecContext(ctx, `
			INSERT INTO students (
				custom_user_id, school_class, bus, name_lg, contact_lg,
				in_house, wc, school_yard, group_id, created_at, modified_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			customUserID,
			student.Grade,
			i%2 == 0, // Alternate bus status
			student.ParentName,
			student.ParentEmail,
			student.InHouse,
			false, // Default WC status
			false, // Default school yard status
			groupID,
			time.Now(),
			time.Now(),
		)
		if err != nil {
			return fmt.Errorf("error creating student: %w", err)
		}
	}

	fmt.Println("Successfully added sample students")
	return nil
}

// The below code is only for the migration system, not for direct use
func init() {
	// Register the migration
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			// For the migration system, just do a no-op
			// Use the AddSampleStudents function directly to populate data
			fmt.Println("Migration 3: This migration is a no-op. Use AddSampleStudents() to add sample data.")
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 3: No changes to revert.")
			return nil
		},
	)
}
