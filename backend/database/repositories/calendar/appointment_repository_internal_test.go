package calendar

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The reminder push claim is the row that decides whether a family gets a
// second copy of the same reminder. Its guards run before any SQL: a malformed
// claim must fail loudly instead of writing a row nothing can match later.
func TestClaimReminderPushRejectsIncompleteClaims(t *testing.T) {
	t.Parallel()

	repo := &AppointmentRecipientRepository{}
	ctx := testpkg.TenantContext(41)
	occurrence := testpkg.Date(2026, 4, 2)

	cases := map[string]struct {
		appointmentID     int64
		revision          int
		missingOccurrence bool
		guardianProfileID int64
	}{
		"missing appointment":       {0, 3, false, 7},
		"negative revision":         {42, -1, false, 7},
		"missing occurrence date":   {42, 3, true, 7},
		"missing guardian profile":  {42, 3, false, 0},
		"negative guardian profile": {42, 3, false, -1},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			occurrenceDate := occurrence
			if tc.missingOccurrence {
				occurrenceDate = ""
			}
			claimed, err := repo.ClaimReminderPush(ctx, tc.appointmentID, tc.revision, occurrenceDate, tc.guardianProfileID)
			require.Error(t, err)
			assert.False(t, claimed)

			assert.Error(t, repo.ReleaseReminderPush(ctx, tc.appointmentID, tc.revision, occurrenceDate, tc.guardianProfileID))
		})
	}
}

// Claims live in a tenant-scoped table. Without a tenant in context the SQL
// would run unscoped, so the repository refuses rather than reaching the DB.
func TestReminderPushClaimsRequireATenant(t *testing.T) {
	t.Parallel()

	repo := &AppointmentRecipientRepository{}
	occurrence := testpkg.Date(2026, 4, 2)

	claimed, err := repo.ClaimReminderPush(context.Background(), 42, 3, occurrence, 7)
	require.EqualError(t, err, "tenant id is required")
	assert.False(t, claimed)

	assert.EqualError(t, repo.ReleaseReminderPush(context.Background(), 42, 3, occurrence, 7),
		"tenant id is required")
}

// The reminder scan asks for the overrides of the appointments it found. An
// empty ask must not become an unbounded `IN ()` query.
func TestFindByAppointmentIDsAndStartDatesShortCircuitsEmptyInput(t *testing.T) {
	t.Parallel()

	repo := &AppointmentOccurrenceOverrideRepository{}
	ctx := testpkg.TenantContext(41)
	dates := listOf(testpkg.Date(2026, 4, 2))

	rows, err := repo.FindByAppointmentIDsAndStartDates(ctx, nil, dates)
	require.NoError(t, err)
	assert.Empty(t, rows)

	rows, err = repo.FindByAppointmentIDsAndStartDates(ctx, []int64{42}, nil)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

// listOf builds a typed slice from the helper's return type without naming it.
func listOf[T any](items ...T) []T { return items }
