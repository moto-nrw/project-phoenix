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
	skippedCount := 0
	errorCount := 0

	for i, student := range students {
		// Progress logging every 10 students
		if (i+1)%10 == 0 || i == 0 {
			fmt.Printf("Processing student %d/%d (%.1f%% complete)...\n", i+1, len(students), float64(i+1)/float64(len(students))*100)
		}

		// Parse guardian name into first and last name
		firstName, lastName := parseGuardianName(student.GuardianName)

		// Log simplified names that might lose information
		if len(strings.Fields(student.GuardianName)) > 2 {
			log.Printf("Student %d: Simplified guardian name '%s' to '%s %s'", student.ID, student.GuardianName, firstName, lastName)
		}

		// Determine primary contact (email or phone)
		email, phone := parseGuardianContact(student)

		// Log placeholder data usage
		if strings.Contains(email, "noemail") || strings.HasPrefix(phone, "000") {
			log.Printf("Student %d: Using placeholder data (email: %s, phone: %s)", student.ID, email, phone)
		}

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
				log.Printf("ERROR: Student %d - Failed to check for existing guardian: %v", student.ID, err)
				errorCount++
				continue
			}

			if existingGuardianID > 0 {
				guardianID = existingGuardianID
				log.Printf("Student %d: Reusing existing guardian ID %d", student.ID, guardianID)
			} else {
				// Create new guardian and get ID via QueryRow + Scan (required for RETURNING)
				err = tx.QueryRowContext(ctx, `
					INSERT INTO users.guardians (first_name, last_name, email, phone, active, created_at, updated_at)
					VALUES (?, ?, ?, ?, true, NOW(), NOW())
					RETURNING id
				`, firstName, lastName, email, phone).Scan(&guardianID)

				if err != nil {
					log.Printf("ERROR: Student %d - Failed to create guardian (%s %s, email: %s, phone: %s): %v",
						student.ID, firstName, lastName, email, phone, err)
					errorCount++
					continue
				}
				log.Printf("Student %d: Created new guardian ID %d (%s %s)", student.ID, guardianID, firstName, lastName)
			}

			guardianCache[cacheKey] = guardianID
		} else {
			log.Printf("Student %d: Using cached guardian ID %d", student.ID, guardianID)
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
			log.Printf("ERROR: Student %d - Failed to create student-guardian relationship with guardian ID %d: %v",
				student.ID, guardianID, err)
			errorCount++
			continue
		}

		migratedCount++
	}

	// Final statistics
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Migration 1.4.2 - Guardian Data Migration Complete")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Total students with legacy data:  %d\n", len(students))
	fmt.Printf("Successfully migrated:             %d (%.1f%%)\n", migratedCount, float64(migratedCount)/float64(len(students))*100)
	fmt.Printf("Skipped (no data):                 %d\n", skippedCount)
	fmt.Printf("Errors:                            %d\n", errorCount)
	fmt.Printf("Unique guardians created/reused:   %d\n", len(guardianCache))
	fmt.Println(strings.Repeat("=", 60))

	if errorCount > 0 {
		log.Printf("WARNING: %d errors occurred during migration. Check logs for details.", errorCount)
	}

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
				phone = fmt.Sprintf("000%07d", student.ID) // Unique placeholder per student
			} else {
				// Assume it's a phone number
				phone = contact
				email = fmt.Sprintf("noemail+guardian_%d@example.invalid", student.ID)
			}
		}
	}

	// Ensure we have both email and phone (requirements of guardian table)
	if email == "" {
		email = fmt.Sprintf("noemail+guardian_%d@example.invalid", student.ID)
	}
	if phone == "" {
		phone = fmt.Sprintf("000%07d", student.ID) // Unique placeholder per student (format: 0000000001, 0000000042, etc.)
	}

	return email, phone
}
