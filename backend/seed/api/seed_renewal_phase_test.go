package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// legacyDemoStudentBirthYear is the birth year per group as the fixed seeder
// set it before birthdays followed the seed day (#3922). Only day and month
// moved; the year still fits the child's class.
func legacyDemoStudentBirthYear(groupKey string) int {
	switch groupKey {
	case "bärengruppe", "sonnengruppe":
		return 2018
	case "mondgruppe", "regenbogengruppe", "meeresgruppe":
		return 2017
	case "blumengruppe", "schmetterlingsgruppe", "wiesengruppe":
		return 2016
	}
	return 2019
}

func TestDemoStudentBirthdayKeepsTheYearOfTheGroup(t *testing.T) {
	t.Parallel()

	seedDay := fixedSeedDate(t, "2026-10-10")
	require.NotEmpty(t, DemoStudents)
	for i, student := range DemoStudents {
		birthday, err := time.Parse(seedDateLayout, demoStudentBirthday(i, student, seedDay))
		require.NoError(t, err)
		assert.Equal(t, legacyDemoStudentBirthYear(student.GroupKey), birthday.Year(), "child %d (%s)", i, student.GroupKey)
	}
	assert.Equal(t, 2019, mustParseYear(t, demoStudentBirthday(0, DemoStudent{GroupKey: "unbekannt"}, seedDay)),
		"an unknown group keeps the first-grade default")
}

// The birthday card shows a Monday-to-Sunday week and pages four weeks back
// and forth (#3777). In each of them at least two children have a birthday,
// whatever weekday the school was seeded on, and one of them on the seed day.
func TestDemoStudentBirthdayFillsEveryWeekAroundTheSeedDay(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"2026-10-05", "2026-10-09", "2026-10-10", "2026-10-11", "2027-02-24"} {
		seedDay := fixedSeedDate(t, value)
		monday := seedDay.AddDays(-((int(seedDay.Weekday()) + 6) % 7))
		perWeek := map[int]int{}
		today := 0
		for i, student := range DemoStudents {
			birthday, err := time.Parse(seedDateLayout, demoStudentBirthday(i, student, seedDay))
			require.NoError(t, err)
			for week := -demoBirthdayWeeksBefore; week <= 8; week++ {
				for day := 0; day < 7; day++ {
					date := monday.AddDays(7*week + day)
					if date.Month() == birthday.Month() && date.Day() == birthday.Day() {
						perWeek[week]++
					}
				}
			}
			if seedDay.Month() == birthday.Month() && seedDay.Day() == birthday.Day() {
				today++
			}
		}
		for week := -demoBirthdayWeeksBefore; week <= 8; week++ {
			assert.GreaterOrEqual(t, perWeek[week], 2, "seed day %s, week %+d", value, week)
		}
		assert.GreaterOrEqual(t, today, 1, "seed day %s: a child has a birthday today", value)
	}
}

func TestDemoStudentBirthdayNeverInventsFebruary29(t *testing.T) {
	t.Parallel()

	// 2028-02-29 is the seed day; 2019 has no 29 February.
	assert.Equal(t, "2019-02-28", demoStudentBirthday(2*demoBirthdayWeeksBefore, DemoStudent{GroupKey: "sternengruppe"}, fixedSeedDate(t, "2028-02-29")))
}

func fixedSeedDate(t *testing.T, value string) seedDate {
	t.Helper()
	day, err := parseSeedDate(value)
	require.NoError(t, err)
	return day
}

func mustParseYear(t *testing.T, value string) int {
	t.Helper()
	day, err := time.Parse(seedDateLayout, value)
	require.NoError(t, err)
	return day.Year()
}

func TestRenewalChildForResolvesTheChildBehindAParent(t *testing.T) {
	t.Parallel()

	rt := &Runtime{FixedSeeder: &FixedSeeder{studentIDByIndex: map[int]int64{0: 900, 1: 901}, seedDay: fixedSeedDate(t, "2026-10-10")}}

	child, err := renewalChildFor(rt, ParentCredentials{Email: "p@example.test", StudentIDs: []int64{901}})
	require.NoError(t, err)
	assert.Equal(t, DemoStudents[1].FirstName, child.student.FirstName)
	assert.Equal(t, demoStudentBirthday(1, DemoStudents[1], rt.FixedSeeder.seedDay), child.birthday)
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
		if answer.status == "" {
			continue
		}
		assert.False(t, answered[answer.parent], "parent %d answers twice", answer.parent)
		answered[answer.parent] = true
	}
	assert.NotEmpty(t, answered)
	assert.Less(t, len(answered), 6)
	assert.False(t, answered[4], "the silent family must not receive a submitted renewal")
}

func TestSubmitRenewalSkipsAParentWithoutAnswer(t *testing.T) {
	t.Parallel()

	err := (parentEnrollmentSeedStep{}).submitRenewal(nil, AuthRef{}, 0, ParentCredentials{}, nil, "")
	require.NoError(t, err)
}

// The demo visitor signs in as the first parent. An approved renewal is next
// school year's enrollment and would be that child's only care period, so the
// parents portal showed next year as current care (#3894).
func TestRenewalPhaseAnswersLeaveTheVisitorParentUndecided(t *testing.T) {
	t.Parallel()

	approved := false
	for _, answer := range renewalPhaseAnswers {
		if answer.parent == 0 {
			assert.NotEqual(t, "approved", answer.status, "the visitor's child must keep its current care period")
		}
		approved = approved || answer.status == "approved"
	}
	assert.True(t, approved, "the overview still needs one confirmed answer")
}
