package studentdeletion

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateConfirmationRejectsEveryMissingPart(t *testing.T) {
	t.Parallel()
	require.ErrorIs(t, validateConfirmation(Confirmation{}), ErrPreviewChanged)
	require.ErrorIs(t, validateConfirmation(Confirmation{ExpectedFingerprint: " "}), ErrPreviewChanged)
	require.ErrorIs(t, validateConfirmation(Confirmation{ExpectedFingerprint: "aa"}), ErrNotAcknowledged)
	require.ErrorIs(t, validateConfirmation(Confirmation{ExpectedFingerprint: "aa", Acknowledged: true}), ErrInvalidReason)
	require.ErrorIs(t, validateConfirmation(Confirmation{ExpectedFingerprint: "aa", Acknowledged: true, Reason: ReasonGraduatePurge}), ErrInvalidReason,
		"the purge reason belongs to the graduate route, never to a typed confirmation")
	for _, reason := range []string{ReasonTestData, ReasonIncorrectEntry, ReasonDuplicate, ReasonPrivacyRequest, ReasonRetentionExpired} {
		require.NoError(t, validateConfirmation(Confirmation{ExpectedFingerprint: "aa", Acknowledged: true, Reason: reason}))
	}
}

func TestFingerprintBindsStudentPersonAndCounts(t *testing.T) {
	t.Parallel()
	student := time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)
	person := student.Add(time.Minute)
	base, err := fingerprintOf(student, person, Counts{TimetableAssignments: 1})
	require.NoError(t, err)
	same, err := fingerprintOf(student, person, Counts{TimetableAssignments: 1})
	require.NoError(t, err)
	assert.Equal(t, base, same)
	assert.True(t, equalFingerprint(base, base))
	assert.True(t, equalFingerprint(base, " "+base+" "), "the confirmation may carry whitespace around the token")

	changedCounts, err := fingerprintOf(student, person, Counts{TimetableAssignments: 2})
	require.NoError(t, err)
	assert.False(t, equalFingerprint(base, changedCounts))
	changedPerson, err := fingerprintOf(student, person.Add(time.Nanosecond), Counts{TimetableAssignments: 1})
	require.NoError(t, err)
	assert.False(t, equalFingerprint(base, changedPerson))
	assert.False(t, equalFingerprint(base, "not-hex"))
	assert.False(t, equalFingerprint(base, ""))
}

func TestCountsTotalExcludesThePrimaryRows(t *testing.T) {
	t.Parallel()
	counts := Counts{
		TimetableAssignments: 1, ActivityEnrollments: 2, AttendanceRecords: 3, CareSchedules: 4, GuardianLinks: 5,
		CompanionLinks: 6, Communications: 7, Consents: 8, EnrollmentReferences: 9, OtherRecords: 10,
	}
	assert.Equal(t, 55, counts.Total())
}

func TestAccompaniedWeekdaysReadsBothPlanShapes(t *testing.T) {
	t.Parallel()
	accompanied := accompaniedWeekdays(
		map[string][]string{"mon": {"alone", "accompanied"}, "tue": {"alone"}},
		map[string]string{"wed": "accompanied", "thu": "pickup"},
	)
	assert.Equal(t, map[string]bool{"mon": true, "wed": true}, accompanied)
}

func TestAscendingDeduplicatesAndDropsInvalidIDs(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []int64{1, 2, 9}, ascending([]int64{9, 2, 0, 2, -3, 1}))
	assert.Empty(t, ascending(nil))
}

func TestNewRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	_, err := New(Dependencies{})
	require.Error(t, err)
}
