package test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposingFixtureDeadlineStartsAfterNestedFixtures pins the budget rule
// behind #5191: the deadline of a composing fixture's own statements is
// created only after its nested fixtures have returned. Each nested fixture
// brings its own budget; a deadline started before them shrank by their run
// time, and under a loaded test database the fixture's own statements ran out
// of time although no single step was slow.
//
// The test needs no delay and no timing assumption. A deadline is
// time.Now().Add(budget), so Deadline minus the fixture's budget is exactly
// the instant the deadline was created (the monotonic reading survives Add).
// That instant must lie after every statement the nested fixtures issued
// before the first own statement. A deadline taken before them lies before.
func TestComposingFixtureDeadlineStartsAfterNestedFixtures(t *testing.T) {
	t.Parallel()

	// The counter sees every statement on its DB, so the test owns its clone.
	db := SetupIsolatedTestDB(t)
	Tenant(t)
	student := CreateTestStudent(t, db, "Mira", "Vogel", "2b")
	counter := CaptureQueries(t, db)

	fixtures := []struct {
		name     string
		create   func()
		ownTable string
		budget   time.Duration
	}{
		{"CreateTestParentGuardianChain", func() { CreateTestParentGuardianChain(t, db) }, "users.guardian_profiles", 5 * time.Second},
		{"CreateTestCoGuardianForStudent", func() { CreateTestCoGuardianForStudent(t, db, student.ID, "Jonas", "Vogel") }, "users.guardian_profiles", 5 * time.Second},
		{"FillWideStudentFields", func() { FillWideStudentFields(t, db, student.ID, student.PersonID) }, "update users.student_profiles", 10 * time.Second},
	}
	for _, fixture := range fixtures {
		counter.Reset()
		fixture.create()
		assertOwnDeadlineAfterNested(t, fixture.name, counter.Deadlines(), fixture.ownTable, fixture.budget)
	}
}

// TestLeafFixtureDeadlineStartsAfterTenantCreation covers the nested fixture
// every leaf fixture has: the first fixture of a test resolves the test's
// tenant and creates it through EnsureTestTenant, under that helper's own
// deadline. The leaf's deadline must start after it, like a composing one.
func TestLeafFixtureDeadlineStartsAfterTenantCreation(t *testing.T) {
	t.Parallel()

	// A fresh clone and no Tenant call yet: CreateTestPerson creates the tenant.
	db := SetupIsolatedTestDB(t)
	counter := CaptureQueries(t, db)

	CreateTestPerson(t, db, "Lea", "Sommer")

	statements := counter.Deadlines()
	require.NotEmpty(t, counter.Matching(func(sql string) bool { return strings.Contains(sql, "platform.schools") }),
		"the person fixture must have created the test tenant")
	assertOwnDeadlineAfterNested(t, "CreateTestPerson", statements, "users.persons", 5*time.Second)
}

// assertOwnDeadlineAfterNested finds the fixture's first own statement by the
// table it writes and checks that its deadline, created budget before it
// expires, was created after every statement issued before it.
func assertOwnDeadlineAfterNested(t *testing.T, fixture string, statements []StatementDeadline, ownTable string, budget time.Duration) {
	t.Helper()
	own := -1
	for i, statement := range statements {
		if strings.Contains(strings.ToLower(statement.SQL), ownTable) {
			own = i
			break
		}
	}
	require.Positive(t, own, "%s: nested fixtures must run before the first own statement", fixture)
	require.False(t, statements[own].Deadline.IsZero(), "%s: the own statement runs under a deadline", fixture)
	require.LessOrEqual(t, statements[own].Deadline.Sub(statements[own].IssuedAt), budget,
		"%s: the own deadline is longer than the budget this test assumes", fixture)
	created := statements[own].Deadline.Add(-budget)
	for _, nested := range statements[:own] {
		require.True(t, created.After(nested.IssuedAt),
			"%s: own deadline created at %v, before nested statement issued at %v: %s",
			fixture, created, nested.IssuedAt, nested.SQL)
	}
}
