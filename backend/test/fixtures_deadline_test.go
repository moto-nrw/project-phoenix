package test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestComposingFixtureDeadlineStartsAfterNestedFixtures pins the budget rule
// behind #5191: the deadline of a composing fixture's own statements is
// created only after its nested fixtures have returned. Each nested fixture
// brings its own 5s budget; a deadline started before them shrank by their
// run time, and under a loaded test database the chain's guardian profile or
// link ran out of time although no single step was slow.
//
// Every fixture deadline is time.Now() plus the same 5s, so deadlines order
// exactly like their creation. The test compares the recorded statement
// deadlines instead of measuring or provoking any delay: the first own
// statement must carry a later deadline than every nested statement before
// it. A deadline taken before the nested fixtures is earlier than theirs.
func TestComposingFixtureDeadlineStartsAfterNestedFixtures(t *testing.T) {
	t.Parallel()

	// The counter sees every statement on its DB, so the test owns its clone.
	db := SetupIsolatedTestDB(t)
	Tenant(t)
	student := CreateTestStudent(t, db, "Mira", "Vogel", "2b")
	counter := CaptureQueries(t, db)

	fixtures := []struct {
		name   string
		create func()
	}{
		{"CreateTestParentGuardianChain", func() { CreateTestParentGuardianChain(t, db) }},
		{"CreateTestCoGuardianForStudent", func() { CreateTestCoGuardianForStudent(t, db, student.ID, "Jonas", "Vogel") }},
	}
	for _, fixture := range fixtures {
		counter.Reset()
		fixture.create()
		assertOwnDeadlineAfterNested(t, fixture.name, counter.Deadlines(), "users.guardian_profiles")
	}
}

// assertOwnDeadlineAfterNested finds the composing fixture's first own
// statement by the table it writes and checks that its deadline is later than
// the deadline of every statement the nested fixtures issued before it.
func assertOwnDeadlineAfterNested(t *testing.T, fixture string, statements []StatementDeadline, ownTable string) {
	t.Helper()
	own := -1
	for i, statement := range statements {
		if strings.Contains(strings.ToLower(statement.SQL), ownTable) {
			own = i
			break
		}
	}
	require.Positive(t, own, "%s: nested fixtures must run before the first own statement", fixture)
	ownDeadline := statements[own].Deadline
	require.False(t, ownDeadline.IsZero(), "%s: the own statement runs under a deadline", fixture)
	for _, nested := range statements[:own] {
		require.False(t, nested.Deadline.IsZero(), "%s: nested statement without deadline: %s", fixture, nested.SQL)
		require.True(t, ownDeadline.After(nested.Deadline),
			"%s: own deadline %v starts before nested statement deadline %v: %s",
			fixture, ownDeadline, nested.Deadline, nested.SQL)
	}
}
