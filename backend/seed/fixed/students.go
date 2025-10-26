package fixed

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// seedStudents creates student records for persons 31-150
func (s *Seeder) seedStudents(ctx context.Context) error {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Students are persons 31-150 (120 students total)
	studentPersons := s.result.Persons[30:]

	// Distribute students evenly across class groups
	studentsPerClass := len(studentPersons) / len(s.result.ClassGroups)
	extraStudents := len(studentPersons) % len(s.result.ClassGroups)

	// Track guardians to avoid duplicates (by email)
	guardianCache := make(map[string]*users.Guardian)

	studentIndex := 0
	for i, classGroup := range s.result.ClassGroups {
		// Calculate how many students for this class
		numStudents := studentsPerClass
		if i < extraStudents {
			numStudents++
		}

		// Assign students to this class
		for j := 0; j < numStudents && studentIndex < len(studentPersons); j++ {
			person := studentPersons[studentIndex]
			studentIndex++

			// Generate guardian information based on student's last name
			guardianFirstName := firstNames[rng.Intn(35)] // Adult names
			guardianPhone := fmt.Sprintf("+49 %d %d-%d",
				30+rng.Intn(900),
				rng.Intn(900)+100,
				rng.Intn(9000)+1000)
			guardianEmail := fmt.Sprintf("%s.%s@gmx.de",
				normalizeForEmail(guardianFirstName),
				normalizeForEmail(person.LastName))

			// Check if guardian already exists in cache
			var guardian *users.Guardian
			var exists bool
			if guardian, exists = guardianCache[guardianEmail]; !exists {
				// Create new guardian
				guardian = &users.Guardian{
					FirstName: guardianFirstName,
					LastName:  person.LastName,
					Email:     &guardianEmail, // Pointer to string for optional field
					Phone:     &guardianPhone, // Pointer to string for optional field
					Active:    true,
				}
				guardian.CreatedAt = time.Now()
				guardian.UpdatedAt = time.Now()

				// Insert guardian (no conflict handling - cache prevents duplicates)
				// Note: Can't use ON CONFLICT with partial unique index on email
				_, err := s.tx.NewInsert().Model(guardian).
					ModelTableExpr("users.guardians").
					Returning("id, created_at, updated_at").
					Exec(ctx)
				if err != nil {
					return fmt.Errorf("failed to create guardian for student %d: %w", person.ID, err)
				}

				guardianCache[guardianEmail] = guardian
			}

			// Set bus permission (30% of students)
			bus := rng.Float32() < 0.3

			// Create student
			student := &users.Student{
				PersonID:    person.ID,
				SchoolClass: classGroup.Name,
				Bus:         bus,
				GroupID:     &classGroup.ID,
			}
			student.CreatedAt = time.Now()
			student.UpdatedAt = time.Now()

			_, err := s.tx.NewInsert().Model(student).
				ModelTableExpr("users.students").
				On("CONFLICT (person_id) DO UPDATE").
				Set("school_class = EXCLUDED.school_class").
				Set("bus = EXCLUDED.bus").
				Set("group_id = EXCLUDED.group_id").
				Set("updated_at = EXCLUDED.updated_at").
				Returning("id, created_at, updated_at").
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to upsert student for person %d: %w", person.ID, err)
			}

			// Create student-guardian relationship
			relationship := &users.StudentGuardian{
				StudentID:          student.ID,
				GuardianID:         guardian.ID,
				RelationshipType:   "parent",
				IsPrimary:          true,
				IsEmergencyContact: true,
				CanPickup:          true,
			}
			relationship.CreatedAt = time.Now()
			relationship.UpdatedAt = time.Now()

			_, err = s.tx.NewInsert().Model(relationship).
				ModelTableExpr("users.students_guardians").
				On("CONFLICT (student_id, guardian_id, relationship_type) DO NOTHING").
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to create student-guardian relationship for student %d: %w", student.ID, err)
			}

			s.result.Students = append(s.result.Students, student)
			s.result.StudentByPersonID[person.ID] = student
		}
	}

	if s.verbose {
		log.Printf("Created %d students with guardians distributed across %d classes",
			len(s.result.Students), len(s.result.ClassGroups))
	}

	return nil
}

// seedPrivacyConsents creates privacy consent records for students
func (s *Seeder) seedPrivacyConsents(ctx context.Context) error {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	policyVersions := []string{"DSGVO-2023-v1.0", "DSGVO-2023-v1.1", "DSGVO-2024-v1.0"}
	consentCount := 0

	for _, student := range s.result.Students {
		// 90% of students have privacy consents
		if rng.Float32() < 0.9 {
			exists, err := s.tx.NewSelect().
				Table("users.privacy_consents").
				Where("student_id = ?", student.ID).
				Exists(ctx)
			if err != nil {
				return fmt.Errorf("failed to check privacy consent for student %d: %w", student.ID, err)
			}
			if exists {
				consentCount++
				continue
			}
			policyVersion := policyVersions[rng.Intn(len(policyVersions))]
			acceptedAt := time.Now().AddDate(0, 0, -rng.Intn(180)) // Within last 6 months
			durationDays := 365                                    // 1 year validity
			expiresAt := acceptedAt.AddDate(1, 0, 0)
			renewalRequired := expiresAt.Before(time.Now().AddDate(0, 1, 0))

			consent := &users.PrivacyConsent{
				StudentID:         student.ID,
				PolicyVersion:     policyVersion,
				Accepted:          true,
				AcceptedAt:        &acceptedAt,
				ExpiresAt:         &expiresAt,
				DurationDays:      &durationDays,
				RenewalRequired:   renewalRequired,
				DataRetentionDays: 30, // Default 30 days retention
			}
			consent.CreatedAt = time.Now()
			consent.UpdatedAt = time.Now()

			_, err = s.tx.NewInsert().Model(consent).
				ModelTableExpr("users.privacy_consents").
				Returning("id, created_at, updated_at").
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to create privacy consent for student %d: %w",
					student.ID, err)
			}
			consentCount++
		}
	}

	if s.verbose {
		log.Printf("Created %d privacy consents (%.0f%% coverage)",
			consentCount, float64(consentCount)/float64(len(s.result.Students))*100)
	}

	return nil
}

// seedGuardianRelationships creates auth accounts for some guardians so they can log in
func (s *Seeder) seedGuardianRelationships(ctx context.Context) error {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Get all unique guardians from the guardians table
	var guardians []*users.Guardian
	err := s.tx.NewSelect().
		Model(&guardians).
		ModelTableExpr(`users.guardians AS "guardian"`).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch guardians: %w", err)
	}

	// Create auth accounts for 20% of guardians so they can log in
	guardianCount := 0
	for _, guardian := range guardians {
		if rng.Float32() < 0.2 {
			passwordHash, err := userpass.HashPassword("Test1234%", nil)
			if err != nil {
				return fmt.Errorf("failed to hash password: %w", err)
			}
			// Guardian email is required for account creation (cached by email)
			account := &auth.AccountParent{
				Email:        *guardian.Email, // Dereference pointer (guaranteed non-nil in cache)
				PasswordHash: &passwordHash,
				Active:       true,
			}
			account.CreatedAt = time.Now()
			account.UpdatedAt = time.Now()

			var id int64
			var createdAt time.Time
			var updatedAt time.Time
			err = s.tx.QueryRowContext(ctx, `
				INSERT INTO auth.accounts_parents (created_at, updated_at, email, password_hash, active)
				VALUES (?, ?, ?, ?, ?)
				ON CONFLICT (email) DO UPDATE SET
					password_hash = EXCLUDED.password_hash,
					active = EXCLUDED.active,
					updated_at = EXCLUDED.updated_at
				RETURNING id, created_at, updated_at`,
				account.CreatedAt, account.UpdatedAt, account.Email,
				account.PasswordHash, account.Active).Scan(&id, &createdAt, &updatedAt)
			if err != nil {
				return fmt.Errorf("failed to upsert guardian account %s: %w", account.Email, err)
			}
			account.ID = id
			account.CreatedAt = createdAt
			account.UpdatedAt = updatedAt

			// Link the auth account to the guardian profile
			_, err = s.tx.NewUpdate().
				Model(guardian).
				ModelTableExpr("users.guardians").
				Set("account_id = ?", id).
				Where("id = ?", guardian.ID).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to link guardian %d to account %d: %w", guardian.ID, id, err)
			}

			guardianCount++
		}
	}

	if s.verbose {
		log.Printf("Created %d guardian auth accounts (%.0f%% of %d guardians)",
			guardianCount, float64(guardianCount)/float64(len(guardians))*100, len(guardians))
	}

	return nil
}
