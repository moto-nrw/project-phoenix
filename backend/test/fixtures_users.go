package test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Users Domain Fixtures
// ============================================================================

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

	// Store person reference for convenience
	staff.Person = person

	return staff
}

// CreateTestStaffForPerson creates a staff record for an existing person
// Use this when you need to control the person record separately
func CreateTestStaffForPerson(tb testing.TB, db *bun.DB, personID int64) *users.Staff {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	staff := &users.Staff{
		PersonID: personID,
	}

	err := db.NewInsert().
		Model(staff).
		ModelTableExpr(`users.staff`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test staff for person")

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
