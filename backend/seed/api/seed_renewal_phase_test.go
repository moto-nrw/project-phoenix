package api

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// legacyDemoStudentBirthday is the formula exactly as it stood inline in the
// fixed seeder before it became demoStudentBirthday. A re-enrollment finds the
// child by name AND birthday, so the extraction must not move a single date.
func legacyDemoStudentBirthday(i int, groupKey string) string {
	baseYear := 2019
	switch groupKey {
	case "sternengruppe":
		baseYear = 2019
	case "bärengruppe":
		baseYear = 2018
	case "sonnengruppe":
		baseYear = 2018
	case "mondgruppe":
		baseYear = 2017
	case "regenbogengruppe":
		baseYear = 2017
	case "blumengruppe":
		baseYear = 2016
	case "schmetterlingsgruppe":
		baseYear = 2016
	case "waldgruppe":
		baseYear = 2019
	case "meeresgruppe":
		baseYear = 2017
	case "wiesengruppe":
		baseYear = 2016
	}
	return fmt.Sprintf("%d-%02d-%02d", baseYear, (i%12)+1, (i%28)+1)
}

func TestDemoStudentBirthdayMatchesTheSeededChildren(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, DemoStudents)
	for i, student := range DemoStudents {
		assert.Equal(t, legacyDemoStudentBirthday(i, student.GroupKey), demoStudentBirthday(i, student),
			"child %d (%s)", i, student.GroupKey)
	}
	assert.Equal(t, "2019-01-01", demoStudentBirthday(0, DemoStudent{GroupKey: "unbekannt"}),
		"an unknown group keeps the first-grade default")
}

func TestRenewalChildForResolvesTheChildBehindAParent(t *testing.T) {
	t.Parallel()

	rt := &Runtime{FixedSeeder: &FixedSeeder{studentIDByIndex: map[int]int64{0: 900, 1: 901}}}

	child, err := renewalChildFor(rt, ParentCredentials{Email: "p@example.test", StudentIDs: []int64{901}})
	require.NoError(t, err)
	assert.Equal(t, DemoStudents[1].FirstName, child.student.FirstName)
	assert.Equal(t, demoStudentBirthday(1, DemoStudents[1]), child.birthday)
	// "Klasse 1a" enters grade 2 next year.
	assert.Equal(t, int16(2), child.nextGrade)

	_, err = renewalChildFor(rt, ParentCredentials{Email: "p@example.test"})
	require.Error(t, err, "a parent without a child cannot re-enroll")

	_, err = renewalChildFor(rt, ParentCredentials{Email: "p@example.test", StudentIDs: []int64{12345}})
	require.Error(t, err, "an unknown child is reported, not skipped")
}

func TestDemoClassGradeReadsEveryDemoClass(t *testing.T) {
	t.Parallel()

	// The parser is deliberately narrow; this pins that it is wide enough for
	// every class the demo data actually uses.
	for i, student := range DemoStudents {
		grade, err := demoClassGrade(student.Class)
		require.NoError(t, err, "child %d class %q", i, student.Class)
		assert.GreaterOrEqual(t, grade, 1, "child %d class %q", i, student.Class)
		assert.LessOrEqual(t, grade, 4, "child %d class %q", i, student.Class)
	}

	grade, err := demoClassGrade(" Klasse 3b ")
	require.NoError(t, err)
	assert.Equal(t, 3, grade)

	_, err = demoClassGrade("Bienen")
	require.Error(t, err, "a class without a grade is reported, not read as 0")
}

func TestSplitSeedName(t *testing.T) {
	t.Parallel()

	first, last := splitSeedName("Anna Maria Schneider")
	assert.Equal(t, "Anna", first)
	assert.Equal(t, "Maria Schneider", last)

	first, last = splitSeedName("Cher")
	assert.Equal(t, "Cher", first)
	assert.Empty(t, last)
}

func TestRenewalPhaseAnswersLeaveParentsWithoutAnAnswer(t *testing.T) {
	t.Parallel()

	// The overview needs both: families that answered and families that have
	// the app but did not. Six parents are seeded; at least one must stay out.
	answered := map[int]bool{}
	for _, answer := range renewalPhaseAnswers {
		assert.False(t, answered[answer.parent], "parent %d answers twice", answer.parent)
		answered[answer.parent] = true
	}
	assert.NotEmpty(t, answered)
	assert.Less(t, len(answered), 6)
}
