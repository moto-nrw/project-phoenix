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
			guardianName := fmt.Sprintf("%s %s", guardianFirstName, person.LastName)
			guardianPhone := fmt.Sprintf("+49 %d %d-%d", 
				30+rng.Intn(900), 
				rng.Intn(900)+100, 
				rng.Intn(9000)+1000)
			guardianEmail := fmt.Sprintf("%s.%s@gmx.de",
				normalizeForEmail(guardianFirstName),
				normalizeForEmail(person.LastName))

			// Set bus permission (30% of students)
			bus := rng.Float32() < 0.3

			student := &users.Student{
				PersonID:          person.ID,
				SchoolClass:       classGroup.Name,
				Bus:               bus,
				GuardianName:      guardianName,
				GuardianContact:   guardianPhone,
				GuardianEmail:     &guardianEmail,
				GuardianPhone:     &guardianPhone,
				GroupID:           &classGroup.ID,
			}
			student.CreatedAt = time.Now()
			student.UpdatedAt = time.Now()

			_, err := s.tx.NewInsert().Model(student).ModelTableExpr("users.students").Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to create student for person %d: %w", person.ID, err)
			}

			s.result.Students = append(s.result.Students, student)
			s.result.StudentByPersonID[person.ID] = student
		}
	}

	if s.verbose {
		log.Printf("Created %d students distributed across %d classes", 
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
			policyVersion := policyVersions[rng.Intn(len(policyVersions))]
			acceptedAt := time.Now().AddDate(0, 0, -rng.Intn(180)) // Within last 6 months
			durationDays := 365                                     // 1 year validity
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

			_, err := s.tx.NewInsert().Model(consent).ModelTableExpr("users.privacy_consents").Exec(ctx)
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

// seedGuardianRelationships creates some guardian relationships
func (s *Seeder) seedGuardianRelationships(ctx context.Context) error {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create guardian accounts for 20% of students
	guardianCount := 0
	for _, student := range s.result.Students {
		if rng.Float32() < 0.2 {
			// Find guardian role
			var guardianRoleID int64
			for _, role := range s.result.Roles {
				if role.Name == "guardian" {
					guardianRoleID = role.ID
					break
				}
			}

			// Create guardian account
			if student.GuardianEmail == nil {
				continue
			}
			passwordHash, err := userpass.HashPassword("Test1234%", nil)
			if err != nil {
				return fmt.Errorf("failed to hash password: %w", err)
			}
			account := &auth.Account{
				Email:        *student.GuardianEmail,
				PasswordHash: &passwordHash,
				Active:       true,
			}
			account.CreatedAt = time.Now()
			account.UpdatedAt = time.Now()

			// Use raw SQL to avoid BUN adding aliases on INSERT
			var id int64
			err = s.tx.QueryRowContext(ctx, `
				INSERT INTO auth.accounts (created_at, updated_at, email, password_hash, active)
				VALUES (?, ?, ?, ?, ?)
				ON CONFLICT (email) DO NOTHING
				RETURNING id`,
				account.CreatedAt, account.UpdatedAt, account.Email, 
				account.PasswordHash, account.Active).Scan(&id)
			if err != nil {
				// Skip if email already exists
				continue
			}
			account.ID = id

			// Assign guardian role
			accountRole := &auth.AccountRole{
				AccountID: account.ID,
				RoleID:    guardianRoleID,
			}
			accountRole.CreatedAt = time.Now()
			accountRole.UpdatedAt = time.Now()

			_, err = s.tx.NewInsert().Model(accountRole).ModelTableExpr("auth.account_roles").Exec(ctx)
			if err != nil {
				continue
			}

			// Create student-guardian relationship
			guardianRel := &users.StudentGuardian{
				StudentID:         student.ID,
				GuardianAccountID: account.ID,
				RelationshipType:  "parent",
				IsPrimary:         true, // All guardians created are primary for simplicity
				IsEmergencyContact: true,
				CanPickup:         true,
			}
			guardianRel.CreatedAt = time.Now()
			guardianRel.UpdatedAt = time.Now()

			_, err = s.tx.NewInsert().
				Model(&guardianRel).
				ModelTableExpr("users.students_guardians").
				Exec(ctx)
			if err != nil {
				continue
			}

			guardianCount++
		}
	}

	if s.verbose {
		log.Printf("Created %d guardian accounts with relationships", guardianCount)
	}

	return nil
}