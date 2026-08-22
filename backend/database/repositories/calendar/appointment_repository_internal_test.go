package calendar

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The reminder push claim is the row that decides whether a family gets a
// second copy of the same reminder. Its guards run before any SQL: a malformed
// claim must fail loudly instead of writing a row nothing can match later.
func TestClaimReminderPushRejectsIncompleteClaims(t *testing.T) {
	t.Parallel()

	repo := &AppointmentRecipientRepository{}
	ctx := tenant.WithTenantID(context.Background(), 41)
	occurrence := timezone.NewDate(2026, 4, 2)

	cases := map[string]struct {
		appointmentID     int64
		revision          int
		occurrence        timezone.Date
		guardianProfileID int64
	}{
		"missing appointment":       {0, 3, occurrence, 7},
		"negative revision":         {42, -1, occurrence, 7},
		"missing occurrence date":   {42, 3, timezone.Date{}, 7},
		"missing guardian profile":  {42, 3, occurrence, 0},
		"negative guardian profile": {42, 3, occurrence, -1},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			claimed, err := repo.ClaimReminderPush(ctx, tc.appointmentID, tc.revision, tc.occurrence, tc.guardianProfileID)
			require.Error(t, err)
			assert.False(t, claimed)

			assert.Error(t, repo.ReleaseReminderPush(ctx, tc.appointmentID, tc.revision, tc.occurrence, tc.guardianProfileID))
		})
	}
}

// Claims live in a tenant-scoped table. Without a tenant in context the SQL
// would run unscoped, so the repository refuses rather than reaching the DB.
func TestReminderPushClaimsRequireATenant(t *testing.T) {
	t.Parallel()

	repo := &AppointmentRecipientRepository{}
	occurrence := timezone.NewDate(2026, 4, 2)

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
	ctx := tenant.WithTenantID(context.Background(), 41)
	dates := []timezone.Date{timezone.NewDate(2026, 4, 2)}

	rows, err := repo.FindByAppointmentIDsAndStartDates(ctx, nil, dates)
	require.NoError(t, err)
	assert.Empty(t, rows)

	rows, err = repo.FindByAppointmentIDsAndStartDates(ctx, []int64{42}, nil)
	require.NoError(t, err)
	assert.Empty(t, rows)
}
