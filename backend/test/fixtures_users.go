package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// CreateTestPerson creates a real person in the database (required for staff creation)
func CreateTestPerson(tb testing.TB, db *bun.DB, firstName, lastName string) *users.Person {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	person := &users.Person{
		FirstName: firstName,
		LastName:  lastName,
	}

	err := db.NewInsert().
		Model(person).
		ModelTableExpr(`users.persons`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test person")

	return person
}

// CreateTestStaff creates a real staff member in the database
// This requires a person, so it creates one automatically
func CreateTestStaff(tb testing.TB, db *bun.DB, firstName, lastName string) *users.Staff {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create person first
	person := CreateTestPerson(tb, db, firstName, lastName)

	// Create staff record
	staff := &users.Staff{
		PersonID: person.ID,
	}

	err := db.NewInsert().
		Model(staff).
		ModelTableExpr(`users.staff`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test staff")

	return staff
}

// CreateTestStudent creates a real student in the database
// This requires a person, so it creates one automatically
func CreateTestStudent(tb testing.TB, db *bun.DB, firstName, lastName, schoolClass string) *users.Student {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create person first (Student has FK to Person)
	person := CreateTestPerson(tb, db, firstName, lastName)

	// Create student record
	student := &users.Student{
		PersonID:    person.ID,
		SchoolClass: schoolClass,
	}

	err := db.NewInsert().
		Model(student).
		ModelTableExpr(`users.students`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test student")

	return student
}

// CreateTestTeacher creates a real teacher in the database.
// Teachers require a Staff record, which requires a Person record.
// Returns the teacher with Staff reference populated for cleanup.
func CreateTestTeacher(tb testing.TB, db *bun.DB, firstName, lastName string) *users.Teacher {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create staff first (which creates person)
	staff := CreateTestStaff(tb, db, firstName, lastName)

	teacher := &users.Teacher{
		StaffID: staff.ID,
	}

	err := db.NewInsert().
		Model(teacher).
		ModelTableExpr(`users.teachers`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test teacher")

	// Store staff reference for cleanup
	teacher.Staff = staff

	return teacher
}

// CreateTestRFIDCard creates an RFID card in the database.
// The ID is uppercase alphanumeric only (no hyphens) to match normalization in PersonRepository.
func CreateTestRFIDCard(tb testing.TB, db *bun.DB, tagID string) *users.RFIDCard {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make tag ID unique - use only alphanumeric chars (no hyphens) to match normalization
	uniqueTagID := fmt.Sprintf("%s%d", tagID, time.Now().UnixNano())

	card := &users.RFIDCard{
		Active: true,
	}
	card.ID = uniqueTagID

	err := db.NewInsert().
		Model(card).
		ModelTableExpr(`users.rfid_cards`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test RFID card")

	return card
}

// CreateTestGuardianProfile creates a guardian profile in the database.
func CreateTestGuardianProfile(tb testing.TB, db *bun.DB, email string) *users.GuardianProfile {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make email unique
	uniqueEmail := fmt.Sprintf(testEmailFormat, email, time.Now().UnixNano())

	profile := &users.GuardianProfile{
		FirstName:              "Guardian",
		LastName:               "Test",
		Email:                  &uniqueEmail,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}

	err := db.NewInsert().
		Model(profile).
		ModelTableExpr(`users.guardian_profiles`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test guardian profile")

	return profile
}

// CreateTestGuest creates a guest instructor in the database.
// This requires a Staff record, which is created automatically.
func CreateTestGuest(tb testing.TB, db *bun.DB, expertise string) *users.Guest {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create staff first (which creates person)
	staff := CreateTestStaff(tb, db, "Guest", "Instructor")

	guest := &users.Guest{
		StaffID:           staff.ID,
		ActivityExpertise: expertise,
		Organization:      "Test Organization",
	}

	err := db.NewInsert().
		Model(guest).
		ModelTableExpr(`users.guests`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test guest")

	// Store staff reference for cleanup
	guest.Staff = staff

	return guest
}

// CreateTestProfile creates a user profile in the database.
// This requires an Account, which is created automatically.
func CreateTestProfile(tb testing.TB, db *bun.DB, prefix string) *users.Profile {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create account first
	account := CreateTestAccount(tb, db, prefix)

	profile := &users.Profile{
		AccountID: account.ID,
		Avatar:    "https://example.com/avatar.png",
		Bio:       "Test bio for " + prefix,
		Settings:  `{"theme": "dark"}`,
	}

	err := db.NewInsert().
		Model(profile).
		ModelTableExpr(`users.profiles`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test profile")

	// Store account reference for convenience
	profile.Account = account

	return profile
}

// CreateTestPrivacyConsent creates a privacy consent record in the database.
// This requires a Student, which is created automatically.
func CreateTestPrivacyConsent(tb testing.TB, db *bun.DB, prefix string) *users.PrivacyConsent {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create student first
	student := CreateTestStudent(tb, db, "Consent", prefix, "1a")

	now := time.Now()
	expiresAt := now.AddDate(1, 0, 0) // 1 year from now
	durationDays := 365

	consent := &users.PrivacyConsent{
		StudentID:         student.ID,
		PolicyVersion:     "v1.0",
		Accepted:          true,
		AcceptedAt:        &now,
		ExpiresAt:         &expiresAt,
		DurationDays:      &durationDays,
		RenewalRequired:   false,
		DataRetentionDays: 30,
	}

	err := db.NewInsert().
		Model(consent).
		ModelTableExpr(`users.privacy_consents`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test privacy consent")

	// Store student reference for cleanup
	consent.Student = student

	return consent
}

// CreateTestPersonGuardian creates a person-guardian relationship in the database.
// The guardianAccountID should be a parent account ID (from CreateTestParentAccount).
func CreateTestPersonGuardian(tb testing.TB, db *bun.DB, personID, guardianAccountID int64, relType string) *users.PersonGuardian {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pg := &users.PersonGuardian{
		PersonID:          personID,
		GuardianAccountID: guardianAccountID,
		RelationshipType:  users.RelationshipType(relType),
		IsPrimary:         true,
		Permissions:       "{}", // Valid empty JSON object
	}

	err := db.NewInsert().
		Model(pg).
		ModelTableExpr(`users.persons_guardians`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test person guardian relationship")

	return pg
}
