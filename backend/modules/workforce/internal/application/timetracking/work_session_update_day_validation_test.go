package timetracking

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sessionDayValidationNow is the fixed clock of the #6288 (ZE-1) save checks.
var sessionDayValidationNow = calendar.NewDate(2026, 9, 9).BerlinMidnight().Add(12 * time.Hour)

func sessionDayValidationService(t *testing.T) (*workSessionService, *wsMockWorkSessionRepository, *WorkSession) {
	t.Helper()
	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	svc.nowFunc = func() time.Time { return sessionDayValidationNow }
	staffID := int64(100)
	stored := &WorkSession{
		Model: Model{ID: 7}, StaffID: staffID, Date: calendar.NewDate(2026, 9, 9),
		CheckInTime: sessionDayValidationNow.Add(-4 * time.Hour), Status: WorkSessionStatusPresent, CreatedBy: staffID,
	}
	sessionRepo.findByIDFunc = func(context.Context, any) (*WorkSession, error) {
		copied := *stored
		return &copied, nil
	}
	sessionRepo.updateFunc = func(context.Context, *WorkSession) error { return nil }
	auditRepo.createBatchFunc = func(context.Context, []*WorkSessionEdit) error { return nil }
	return svc, sessionRepo, stored
}

// TestWSUpdateSession_RejectsSessionsTheNightlyCleanupCannotClose pins the
// save half of #6288 (ZE-1): a self-edit may neither file an open session on
// a day before its check-in nor move the check-in into the future. Either one
// put a row into the database that the nightly cleanup closed before its
// check-in, which the work_sessions check constraint refuses.
func TestWSUpdateSession_RejectsSessionsTheNightlyCleanupCannotClose(t *testing.T) {
	t.Parallel()
	earlierDay := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	futureCheckIn := sessionDayValidationNow.Add(time.Hour)
	for _, tc := range []struct {
		name    string
		updates SessionUpdateRequest
		want    error
	}{
		{"date before the check-in day", SessionUpdateRequest{Date: &earlierDay}, workforce.ErrSessionCheckInAfterDate},
		{"check-in in the future", SessionUpdateRequest{CheckInTime: &futureCheckIn}, workforce.ErrSessionCheckInInFuture},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc, sessionRepo, stored := sessionDayValidationService(t)
			updated := false
			sessionRepo.updateFunc = func(context.Context, *WorkSession) error {
				updated = true
				return nil
			}
			_, err := svc.UpdateSession(tenant.WithTenantID(context.Background(), 43), stored.StaffID, stored.ID, tc.updates)
			require.ErrorIs(t, err, tc.want)
			assert.False(t, updated, "a rejected edit writes nothing")
		})
	}
}

// TestWSUpdateSession_KeepsValidAndLegacyEdits keeps the check narrow: an
// edit on the session's own day passes, and a notes edit of a row that a
// former edit already broke stays possible, so staff can still repair it.
func TestWSUpdateSession_KeepsValidAndLegacyEdits(t *testing.T) {
	t.Parallel()
	svc, sessionRepo, stored := sessionDayValidationService(t)
	earlierCheckIn := sessionDayValidationNow.Add(-5 * time.Hour)
	_, err := svc.UpdateSession(tenant.WithTenantID(context.Background(), 43), stored.StaffID, stored.ID, SessionUpdateRequest{CheckInTime: &earlierCheckIn})
	require.NoError(t, err)

	broken := *stored
	broken.Date = calendar.NewDate(2026, 9, 1)
	sessionRepo.findByIDFunc = func(context.Context, any) (*WorkSession, error) {
		copied := broken
		return &copied, nil
	}
	_, err = svc.UpdateSession(tenant.WithTenantID(context.Background(), 43), stored.StaffID, stored.ID, SessionUpdateRequest{Notes: wsStrPtr("Datum korrigieren")})
	require.NoError(t, err)
}

// TestWSAdminSessionWrites_RejectCheckInAfterTheSessionDay applies the day
// rule to the admin edit and the admin Nachtrag as well.
func TestWSAdminSessionWrites_RejectCheckInAfterTheSessionDay(t *testing.T) {
	t.Parallel()
	svc, _, stored := sessionDayValidationService(t)
	earlierDay := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	_, err := svc.UpdateSessionAsAdmin(tenant.WithTenantID(context.Background(), 43), 200, stored.StaffID, stored.ID,
		SessionUpdateRequest{Date: &earlierDay, Notes: wsStrPtr("Nachtrag")})
	require.ErrorIs(t, err, workforce.ErrSessionCheckInAfterDate)

	checkIn := calendar.NewDate(2026, 9, 8).BerlinMidnight().Add(8 * time.Hour)
	_, err = svc.CreateSessionAsAdmin(tenant.WithTenantID(context.Background(), 43), 200, stored.StaffID, AdminCreateSessionRequest{
		Date: earlierDay, CheckInTime: checkIn, CheckOutTime: checkIn.Add(4 * time.Hour),
		Status: WorkSessionStatusPresent, Notes: "Nachtrag",
	})
	require.ErrorIs(t, err, workforce.ErrSessionCheckInAfterDate)
}

// TestBalanceSessionEnd_NeverEndsBeforeACheckInAfterTheSessionDay covers the
// rows #6288 found already in the database: an open session filed on a day
// before its check-in. Its stale limit used to be the end of its date, before
// its check-in, so the balance credited negative time and the next check-in
// closed the row against chk_work_sessions_checkout_after_checkin and failed.
func TestBalanceSessionEnd_NeverEndsBeforeACheckInAfterTheSessionDay(t *testing.T) {
	t.Parallel()
	checkIn := sessionDayValidationNow.Add(-2 * maxOpenWorkSessionDuration)
	session := &WorkSession{Date: calendar.DateFromTime(checkIn).AddDays(-3), CheckInTime: checkIn}

	end := BalanceSessionEnd(session, sessionDayValidationNow)
	assert.True(t, end.After(checkIn), "a stale open block ends after its check-in")
	expired, stale := ExpireStaleOpenBlock(session, sessionDayValidationNow)
	require.True(t, stale)
	assert.True(t, expired.CheckOutTime.After(checkIn), "the check-in repair closes the row after its check-in")
}
