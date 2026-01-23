package fixed

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/users"
)

// seedStudents creates student records for persons 31-150
func (s *Seeder) seedStudents(ctx context.Context) error {
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

			// Legacy guardian fields are deprecated - use guardian_profiles table instead
			student := &users.Student{
				PersonID:        person.ID,
				SchoolClass:     classGroup.Name,
				GuardianName:    nil, // Deprecated: Use guardian_profiles table
				GuardianContact: nil, // Deprecated: Use guardian_profiles table
				GuardianEmail:   nil, // Deprecated: Use guardian_profiles table
				GuardianPhone:   nil, // Deprecated: Use guardian_profiles table
				GroupID:         &classGroup.ID,
			}
			student.CreatedAt = time.Now()
			student.UpdatedAt = time.Now()

			_, err := s.tx.NewInsert().Model(student).
				ModelTableExpr("users.students").
				On("CONFLICT (person_id) DO UPDATE").
				Set("school_class = EXCLUDED.school_class").
				Set("guardian_name = EXCLUDED.guardian_name").
				Set("guardian_contact = EXCLUDED.guardian_contact").
				Set("guardian_email = EXCLUDED.guardian_email").
				Set("guardian_phone = EXCLUDED.guardian_phone").
				Set("group_id = EXCLUDED.group_id").
				Set(SQLExcludedUpdatedAt).
				Returning(SQLBaseColumns).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to upsert student for person %d: %w", person.ID, err)
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
				Returning(SQLBaseColumns).
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

// seedGuardianRelationships creates guardian profiles and links them to students
func (s *Seeder) seedGuardianRelationships(ctx context.Context) error {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	relationshipTypes := []string{"parent", "guardian", "relative"}
	guardianCount := 0

	// Create 1-2 guardians for each student
	for _, student := range s.result.Students {
		// Check if this student already has guardians
		existingCount, err := s.tx.NewSelect().
			Table("users.students_guardians").
			Where("student_id = ?", student.ID).
			Count(ctx)
		if err != nil {
			return fmt.Errorf("failed to check existing guardians for student %d: %w", student.ID, err)
		}

		// Skip if student already has guardians
		if existingCount > 0 {
			guardianCount += existingCount
			continue
		}

		numGuardians := 1
		if rng.Float32() < 0.3 { // 30% chance of 2 guardians
			numGuardians = 2
		}

		for i := 0; i < numGuardians; i++ {
			// Generate guardian data
			guardianFirstName := firstNames[rng.Intn(35)] // Adult names
			var guardianLastName string
			if student.GuardianName != nil && *student.GuardianName != "" {
				// Extract last name from the full guardian name
				parts := splitName(*student.GuardianName)
				if len(parts) > 1 {
					guardianLastName = parts[len(parts)-1]
				} else {
					guardianLastName = *student.GuardianName
				}
			} else {
				// Fallback to a random last name if guardian name is empty
				guardianLastName = lastNames[rng.Intn(len(lastNames))]
			}

			homePhone := fmt.Sprintf("+49 %d %d-%d",
				30+rng.Intn(900),
				rng.Intn(900)+100,
				rng.Intn(9000)+1000)
			mobilePhone := fmt.Sprintf("+49 15%d %d-%d",
				rng.Intn(10),
				rng.Intn(900)+100,
				rng.Intn(9000)+1000)
			// Make email unique by adding student ID and guardian index to prevent collisions
			email := fmt.Sprintf("%s.%s.s%d.g%d@beispiel.de",
				normalizeForEmail(guardianFirstName),
				normalizeForEmail(guardianLastName),
				student.ID,
				i)

			// Random address
			street := fmt.Sprintf("Musterstraße %d", rng.Intn(100)+1)
			city := []string{"Berlin", "Hamburg", "München", "Köln", "Frankfurt"}[rng.Intn(5)]
			postalCode := fmt.Sprintf("%d", 10000+rng.Intn(90000))

			// Create guardian profile (phone numbers are added via seedGuardianPhoneNumbers)
			guardian := &users.GuardianProfile{
				FirstName:              guardianFirstName,
				LastName:               guardianLastName,
				Email:                  &email,
				AddressStreet:          &street,
				AddressCity:            &city,
				AddressPostalCode:      &postalCode,
				PreferredContactMethod: "mobile",
				LanguagePreference:     "de",
			}
			guardian.CreatedAt = time.Now()
			guardian.UpdatedAt = time.Now()

			_, err := s.tx.NewInsert().
				Model(guardian).
				ModelTableExpr("users.guardian_profiles").
				On("CONFLICT (email) DO UPDATE").
				Set("first_name = EXCLUDED.first_name").
				Set("last_name = EXCLUDED.last_name").
				Set("address_street = EXCLUDED.address_street").
				Set("address_city = EXCLUDED.address_city").
				Set("address_postal_code = EXCLUDED.address_postal_code").
				Set(SQLExcludedUpdatedAt).
				Returning(SQLBaseColumns).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to upsert guardian profile: %w", err)
			}

			// Create phone numbers in the new flexible table
			if err := s.seedGuardianPhoneNumbers(ctx, guardian.ID, homePhone, mobilePhone, rng); err != nil {
				return fmt.Errorf("failed to seed guardian phone numbers: %w", err)
			}

			// Link guardian to student
			isPrimary := i == 0 // First guardian is primary
			relationshipType := relationshipTypes[rng.Intn(len(relationshipTypes))]
			emergencyPriority := i + 1

			relationship := &users.StudentGuardian{
				StudentID:          student.ID,
				GuardianProfileID:  guardian.ID,
				RelationshipType:   relationshipType,
				IsPrimary:          isPrimary,
				IsEmergencyContact: true,
				CanPickup:          rng.Float32() < 0.8, // 80% can pickup
				EmergencyPriority:  emergencyPriority,
			}
			relationship.CreatedAt = time.Now()
			relationship.UpdatedAt = time.Now()

			// Add pickup notes for some guardians
			if rng.Float32() < 0.2 {
				notes := "Darf Kind nur nach telefonischer Rücksprache abholen"
				relationship.PickupNotes = &notes
			}

			_, err = s.tx.NewInsert().
				Model(relationship).
				ModelTableExpr("users.students_guardians").
				On("CONFLICT (student_id, guardian_profile_id) DO UPDATE").
				Set("relationship_type = EXCLUDED.relationship_type").
				Set("is_primary = EXCLUDED.is_primary").
				Set("is_emergency_contact = EXCLUDED.is_emergency_contact").
				Set("can_pickup = EXCLUDED.can_pickup").
				Set("emergency_priority = EXCLUDED.emergency_priority").
				Set("pickup_notes = EXCLUDED.pickup_notes").
				Set(SQLExcludedUpdatedAt).
				Returning(SQLBaseColumns).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to upsert student-guardian relationship: %w", err)
			}

			guardianCount++
		}
	}

	if s.verbose {
		log.Printf("Created %d guardian profiles with relationships", guardianCount)
	}

	return nil
}

// splitName splits a full name into parts by spaces
func splitName(fullName string) []string {
	return strings.Fields(fullName)
}

// seedGuardianPhoneNumbers creates phone number records for a guardian
func (s *Seeder) seedGuardianPhoneNumbers(ctx context.Context, guardianID int64, homePhone, mobilePhone string, rng *rand.Rand) error {
	now := time.Now()

	// Delete existing phone numbers for this guardian (for idempotent seeding)
	_, err := s.tx.NewDelete().
		Table("users.guardian_phone_numbers").
		Where("guardian_profile_id = ?", guardianID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete existing phone numbers: %w", err)
	}

	// Mobile phone is primary
	mobilePhoneRecord := &users.GuardianPhoneNumber{
		GuardianProfileID: guardianID,
		PhoneNumber:       mobilePhone,
		PhoneType:         users.PhoneTypeMobile,
		IsPrimary:         true,
		Priority:          1,
	}
	mobilePhoneRecord.CreatedAt = now
	mobilePhoneRecord.UpdatedAt = now

	_, err = s.tx.NewInsert().
		Model(mobilePhoneRecord).
		ModelTableExpr("users.guardian_phone_numbers").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to insert mobile phone: %w", err)
	}

	// Home phone is secondary
	homePhoneRecord := &users.GuardianPhoneNumber{
		GuardianProfileID: guardianID,
		PhoneNumber:       homePhone,
		PhoneType:         users.PhoneTypeHome,
		IsPrimary:         false,
		Priority:          2,
	}
	homePhoneRecord.CreatedAt = now
	homePhoneRecord.UpdatedAt = now

	_, err = s.tx.NewInsert().
		Model(homePhoneRecord).
		ModelTableExpr("users.guardian_phone_numbers").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to insert home phone: %w", err)
	}

	// 20% chance to also have a work phone (Dienstlich)
	if rng.Float32() < 0.2 {
		workPhone := fmt.Sprintf("+49 %d %d-%d",
			30+rng.Intn(900),
			rng.Intn(900)+100,
			rng.Intn(9000)+1000)

		workLabel := "Dienstlich"
		workPhoneRecord := &users.GuardianPhoneNumber{
			GuardianProfileID: guardianID,
			PhoneNumber:       workPhone,
			PhoneType:         users.PhoneTypeWork,
			Label:             &workLabel,
			IsPrimary:         false,
			Priority:          3,
		}
		workPhoneRecord.CreatedAt = now
		workPhoneRecord.UpdatedAt = now

		_, err = s.tx.NewInsert().
			Model(workPhoneRecord).
			ModelTableExpr("users.guardian_phone_numbers").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to insert work phone: %w", err)
		}
	}

	return nil
}
