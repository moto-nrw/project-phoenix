package timetracking

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
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

// TestCheckInWithinSessionDay_IsExactAtTheDayBoundary pins the boundary of the
// #6288 day rule: a check-in belongs to its day until the next Berlin midnight,
// including the last fraction of a second and on both 2026 clock changes.
func TestCheckInWithinSessionDay_IsExactAtTheDayBoundary(t *testing.T) {
	t.Parallel()
	for _, day := range []calendar.Date{
		calendar.NewDate(2026, 9, 9),
		calendar.NewDate(2026, 3, 29),  // 23 hours
		calendar.NewDate(2026, 10, 25), // 25 hours
	} {
		nextMidnight := day.AddDays(1).BerlinMidnight()
		assert.NoError(t, checkInWithinSessionDay(day, nextMidnight.Add(-500*time.Millisecond)),
			"%s: 23:59:59.5 still belongs to the day", day)
		assert.NoError(t, checkInWithinSessionDay(day, day.BerlinMidnight()), "%s: midnight starts the day", day)
		assert.ErrorIs(t, checkInWithinSessionDay(day, nextMidnight), workforce.ErrSessionCheckInAfterDate,
			"%s: the next midnight belongs to the next day", day)
	}
}

// Both entry points must validate the entire proposed state before writing any
// pause, session, cache or audit row, including the second item of a batch.
func TestSessionCorrections_RejectInvalidCombinedState(t *testing.T) {
	t.Parallel()
	for _, admin := range []bool{false, true} {
		for _, scenario := range []string{"active_pause", "pause_after_checkout", "overlap", "checkin_after_pause", "checkout_before_pause_end", "scalar_mismatch", "second_edit_invalid", "negative_legacy_scalar", "implicit_reason_required", "duplicate_edit", "duration_overflow"} {
			t.Run(fmt.Sprintf("admin=%t/%s", admin, scenario), func(t *testing.T) {
				t.Parallel()
				svc, sessions, pauses, audit, _ := wsCreateTestService()
				start := calendar.NewDate(2026, 9, 9).BerlinMidnight().Add(8 * time.Hour)
				end := start.Add(8 * time.Hour)
				stored := &WorkSession{Model: Model{ID: 7}, StaffID: 100, CreatedBy: 100, Date: calendar.DateFromTime(start), Status: WorkSessionStatusPresent, CheckInTime: start, CheckOutTime: &end, BreakMinutes: 30}
				pauseEnd := start.Add(4*time.Hour + 15*time.Minute)
				secondEnd := start.Add(5*time.Hour + 15*time.Minute)
				rows := []*WorkSessionBreak{
					{Model: Model{ID: 8}, SessionID: stored.ID, StartedAt: start.Add(4 * time.Hour), EndedAt: &pauseEnd, DurationMinutes: 15},
					{Model: Model{ID: 9}, SessionID: stored.ID, StartedAt: start.Add(5 * time.Hour), EndedAt: &secondEnd, DurationMinutes: 15},
				}
				updates := SessionUpdateRequest{Notes: wsStrPtr("Korrektur")}
				switch scenario {
				case "active_pause":
					stored.CheckOutTime = nil
					rows[1].EndedAt = nil
					updates.CheckOutTime = &end
				case "pause_after_checkout":
					updates.Breaks = []BreakDurationUpdate{{ID: rows[1].ID, DurationMinutes: 240}}
				case "overlap":
					updates.Breaks = []BreakDurationUpdate{{ID: rows[0].ID, DurationMinutes: 90}}
				case "checkin_after_pause":
					later := start.Add(4*time.Hour + time.Minute)
					updates.CheckInTime = &later
				case "checkout_before_pause_end":
					earlier := secondEnd.Add(-time.Minute)
					updates.CheckOutTime = &earlier
				case "scalar_mismatch":
					minutes := 60
					updates.BreakMinutes = &minutes
				case "second_edit_invalid":
					updates.Breaks = []BreakDurationUpdate{{ID: rows[0].ID, DurationMinutes: 20}, {ID: rows[1].ID, DurationMinutes: -1}}
				case "implicit_reason_required":
					updates = SessionUpdateRequest{}
					stored.BreakMinutes = 99
					svc.settings = &wsMockSettingsResolver{resolveBoolFunc: func(context.Context, string) (bool, error) { return true, nil }}
				case "duplicate_edit":
					updates.Breaks = []BreakDurationUpdate{{ID: rows[0].ID, DurationMinutes: 20}, {ID: rows[0].ID, DurationMinutes: 25}}
				case "duration_overflow":
					updates.Breaks = []BreakDurationUpdate{{ID: rows[0].ID, DurationMinutes: 1 << 62}}
				case "negative_legacy_scalar":
					rows = nil
					minutes := -1
					updates.BreakMinutes = &minutes
				}
				sessions.findByIDFunc = func(context.Context, any) (*WorkSession, error) { copied := *stored; return &copied, nil }
				pauses.getBySessionIDFunc = func(context.Context, int64) ([]*WorkSessionBreak, error) { return rows, nil }
				writes := 0
				sessions.updateFunc = func(context.Context, *WorkSession) error { writes++; return nil }
				sessions.updateBreakMinutesFunc = func(context.Context, int64, int) error { writes++; return nil }
				pauses.updateDurationFunc = func(context.Context, int64, int, time.Time) error { writes++; return nil }
				audit.createBatchFunc = func(context.Context, []*WorkSessionEdit) error { writes++; return nil }
				var err error
				if admin {
					_, err = svc.UpdateSessionAsAdmin(context.Background(), stored.CreatedBy, stored.StaffID, stored.ID, updates)
				} else {
					_, err = svc.UpdateSession(context.Background(), stored.StaffID, stored.ID, updates)
				}
				assert.Error(t, err)
				assert.Zero(t, writes, "rejection must leave session, pauses, cache and audit unchanged")
			})
		}
	}
}

func TestSessionCorrections_ConsistentTotals(t *testing.T) {
	t.Parallel()
	for _, day := range []calendar.Date{calendar.NewDate(2026, 9, 9), calendar.NewDate(2026, 3, 29), calendar.NewDate(2026, 10, 25)} {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/legacy=%t", day, legacy), func(t *testing.T) {
				t.Parallel()
				svc, sessions, pauses, audit, _ := wsCreateTestService()
				start := day.BerlinMidnight().Add(-time.Hour)
				end := day.BerlinMidnight().Add(4 * time.Hour)
				stored := &WorkSession{Model: Model{ID: 7}, StaffID: 100, CreatedBy: 100, Date: calendar.DateFromTime(start), Status: WorkSessionStatusPresent, CheckInTime: start, CheckOutTime: &end, BreakMinutes: 99}
				breakEnd := start.Add(75*time.Minute + 30*time.Second)
				rows := []*WorkSessionBreak{{Model: Model{ID: 8}, SessionID: stored.ID, StartedAt: start.Add(45*time.Minute + 30*time.Second), EndedAt: &breakEnd, DurationMinutes: 30}}
				zeroEnd := start.Add(time.Hour)
				rows = append(rows, &WorkSessionBreak{Model: Model{ID: 9}, SessionID: stored.ID, StartedAt: start.Add(time.Hour), EndedAt: &zeroEnd})
				updates := SessionUpdateRequest{Notes: wsStrPtr("Korrektur")}
				want := 30
				if legacy {
					rows = nil
					want = 45
					updates.BreakMinutes = &want
				}
				sessions.findByIDFunc = func(context.Context, any) (*WorkSession, error) { copy := *stored; return &copy, nil }
				pauses.getBySessionIDFunc = func(context.Context, int64) ([]*WorkSessionBreak, error) { return rows, nil }
				var saved *WorkSession
				sessions.updateFunc = func(_ context.Context, value *WorkSession) error { saved = value; return nil }
				var edits []*WorkSessionEdit
				audit.createBatchFunc = func(_ context.Context, value []*WorkSessionEdit) error { edits = value; return nil }
				result, err := svc.UpdateSession(context.Background(), stored.StaffID, stored.ID, updates)
				require.NoError(t, err)
				require.NotNil(t, saved)
				assert.Equal(t, want, result.BreakMinutes)
				assert.Equal(t, want, saved.BreakMinutes)
				net := netMinutesWithBreaks(result, rows, end)
				assert.Equal(t, 300-want, net)
				split := netMinutesByDate(result, rows, end, calendar.DateFromTime(start), day)
				sum := 0
				for _, minutes := range split {
					sum += minutes
				}
				assert.Equal(t, net, sum, "history, month and export must agree with the Berlin day split")
				found := false
				for _, edit := range edits {
					if edit.FieldName == "break_minutes" {
						found = true
						assert.Equal(t, "99", *edit.OldValue)
						assert.Equal(t, strconv.Itoa(want), *edit.NewValue)
					}
				}
				assert.True(t, found, "cache reconciliation is audited")
			})
		}
	}
}

func TestSessionCorrections_FractionalDayAllocation(t *testing.T) {
	t.Parallel()
	day := calendar.NewDate(2026, 10, 25)
	midnight := day.BerlinMidnight()
	for _, mode := range []string{"none", "ended", "running", "break_starts_with_session"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			start := midnight.Add(-2*time.Hour + 30*time.Second)
			end := midnight.Add(4*time.Hour + 30*time.Second)
			breakStart := midnight.Add(-15*time.Minute + 30*time.Second)
			breakEnd := midnight.Add(15*time.Minute + 10*time.Second)
			brk := &WorkSessionBreak{StartedAt: breakStart, EndedAt: &breakEnd, DurationMinutes: 30}
			block := &WorkSession{CheckInTime: start, CheckOutTime: &end, BreakMinutes: 30}
			if mode == "running" {
				block.CheckOutTime = nil
				block.BreakMinutes = 0
				brk.EndedAt = nil
			}
			if mode == "break_starts_with_session" {
				block.CheckInTime = breakStart
				end = midnight.Add(16*time.Minute + 10*time.Second)
			}
			breaks := []*WorkSessionBreak{brk}
			if mode == "none" {
				breaks = nil
				block.BreakMinutes = 0
			}
			split := netMinutesByDate(block, breaks, end, day.AddDays(-1), day)
			sum := 0
			for _, minutes := range split {
				sum += minutes
			}
			assert.Equal(t, netMinutesWithBreaks(block, breaks, end), sum)
		})
	}
}
