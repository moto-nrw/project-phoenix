package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/uptrace/bun"
)

const (
	MigrateLegacyGuardiansVersion     = "1.4.2"
	MigrateLegacyGuardiansDescription = "Migrate legacy guardian data from students table to guardians table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[MigrateLegacyGuardiansVersion] = &Migration{
		Version:     MigrateLegacyGuardiansVersion,
		Description: MigrateLegacyGuardiansDescription,
		DependsOn:   []string{"1.4.1"}, // Depends on guardians table
	}

	// Migration 1.4.2: Migrate legacy guardian data
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return migrateLegacyGuardiansUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			// Cannot easily rollback data migration
			fmt.Println("Warning: Rolling back data migration is not supported")
			return nil
		},
	)
}

// LegacyStudent represents student record with legacy guardian fields
type LegacyStudent struct {
	ID              int64   `bun:"id"`
	GuardianName    string  `bun:"guardian_name"`
	GuardianContact string  `bun:"guardian_contact"`
	GuardianEmail   *string `bun:"guardian_email"`
	GuardianPhone   *string `bun:"guardian_phone"`
}

// migrateLegacyGuardiansUp migrates guardian data from students table to new guardians table
func migrateLegacyGuardiansUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.4.2: Migrating legacy guardian data...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Get all students with legacy guardian data

	var students []LegacyStudent
	err = tx.NewSelect().
		Table("users.students").
		Column("id", "guardian_name", "guardian_contact", "guardian_email", "guardian_phone").
		Where("guardian_name IS NOT NULL AND guardian_name != ''").
		Scan(ctx, &students)

	if err != nil {
		return fmt.Errorf("failed to fetch students with legacy guardian data: %w", err)
	}

	fmt.Printf("Found %d students with legacy guardian data\n", len(students))

	guardianCache := make(map[string]int64) // Map of email/phone -> guardian_id
	migratedCount := 0

	for _, student := range students {
		// Parse guardian name into first and last name
		firstName, lastName := parseGuardianName(student.GuardianName)

		// Determine primary contact (email or phone)
		email, phone := parseGuardianContact(student)

		// Create cache key
		cacheKey := email
		if cacheKey == "" {
			cacheKey = phone
		}

		var guardianID int64
		var exists bool

		// Check if we've already created this guardian
		if guardianID, exists = guardianCache[cacheKey]; !exists {
			// Check if guardian already exists in database
			var existingGuardianID int64
			err = tx.NewSelect().
				Table("users.guardians").
				Column("id").
				Where("LOWER(email) = LOWER(?) OR phone = ?", email, phone).
				Limit(1).
				Scan(ctx, &existingGuardianID)

			if err != nil && err != sql.ErrNoRows {
				log.Printf("Error checking for existing guardian for student %d: %v", student.ID, err)
				continue
			}

			if existingGuardianID > 0 {
				guardianID = existingGuardianID
			} else {
				// Create new guardian
				_, err = tx.Exec(`
					INSERT INTO users.guardians (first_name, last_name, email, phone, active, created_at, updated_at)
					VALUES (?, ?, ?, ?, true, NOW(), NOW())
					RETURNING id
				`, firstName, lastName, email, phone)

				if err != nil {
					log.Printf("Error creating guardian for student %d: %v", student.ID, err)
					continue
				}

				// Get the ID of the newly created guardian
				err = tx.NewSelect().
					Table("users.guardians").
					Column("id").
					Where("email = ? OR phone = ?", email, phone).
					Order("id DESC").
					Limit(1).
					Scan(ctx, &guardianID)

				if err != nil {
					log.Printf("Error retrieving new guardian ID for student %d: %v", student.ID, err)
					continue
				}
			}

			guardianCache[cacheKey] = guardianID
		}

		// Create student-guardian relationship
		// Note: Migration 1.3.6 creates the table with guardian_id column (not guardian_account_id)
		_, err = tx.Exec(`
			INSERT INTO users.students_guardians
			(student_id, guardian_id, relationship_type, is_primary, is_emergency_contact, can_pickup, created_at, updated_at)
			VALUES (?, ?, 'parent', true, true, true, NOW(), NOW())
			ON CONFLICT (student_id, guardian_id, relationship_type) DO NOTHING
		`, student.ID, guardianID)

		if err != nil {
			log.Printf("Error creating student-guardian relationship for student %d: %v", student.ID, err)
			continue
		}

		migratedCount++
	}

	fmt.Printf("Successfully migrated %d guardian relationships\n", migratedCount)

	// Commit the transaction
	return tx.Commit()
}

// parseGuardianName splits a full name into first and last name
func parseGuardianName(fullName string) (string, string) {
	fullName = strings.TrimSpace(fullName)
	parts := strings.Fields(fullName)

	if len(parts) == 0 {
		return "Unknown", "Guardian"
	} else if len(parts) == 1 {
		return parts[0], "Guardian"
	} else {
		// First part is first name, rest is last name
		firstName := parts[0]
		lastName := strings.Join(parts[1:], " ")
		return firstName, lastName
	}
}

// parseGuardianContact determines email and phone from legacy contact fields
func parseGuardianContact(student LegacyStudent) (email string, phone string) {
	// Use guardian_email if available
	if student.GuardianEmail != nil && *student.GuardianEmail != "" {
		email = *student.GuardianEmail
	}

	// Use guardian_phone if available
	if student.GuardianPhone != nil && *student.GuardianPhone != "" {
		phone = *student.GuardianPhone
	}

	// If we still don't have email/phone, try parsing guardian_contact
	if email == "" && phone == "" {
		contact := strings.TrimSpace(student.GuardianContact)
		if contact != "" {
			// Check if it looks like an email
			if strings.Contains(contact, "@") {
				email = contact
				phone = "000000000" // Placeholder
			} else {
				// Assume it's a phone number
				phone = contact
				email = fmt.Sprintf("guardian_%d@placeholder.local", student.ID)
			}
		}
	}

	// Ensure we have both email and phone (requirements of guardian table)
	if email == "" {
		email = fmt.Sprintf("guardian_%d@placeholder.local", student.ID)
	}
	if phone == "" {
		phone = "000000000"
	}

	return email, phone
}
