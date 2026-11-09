package application

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// statusFlagLedger is the cleanup-run ledger the Worker keeps across
// restarts: the last success of each job for the one school of these tests.
type statusFlagLedger struct {
	mu   sync.Mutex
	last map[string]time.Time
}

func (l *statusFlagLedger) CleanupRun(_ context.Context, jobID string) (time.Time, int, time.Time, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.last[jobID], 0, time.Time{}, nil
}

func (l *statusFlagLedger) RecordCleanupFailed(context.Context, string, int, time.Time) error {
	return nil
}

func (l *statusFlagLedger) RecordCleanupSucceeded(_ context.Context, jobID string, at time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.last == nil {
		l.last = map[string]time.Time{}
	}
	if at.After(l.last[jobID]) {
		l.last[jobID] = at
	}
	return nil
}

// restartedScheduler is a fresh Worker process at now over the ledger the
// previous process left, with both flags on clear mode manual.
func restartedScheduler(ledger *statusFlagLedger, archives *statusFlagArchives, now time.Time) *Scheduler {
	return unitScheduler(&unitSchedulerDependencies{
		studentStatusDayRepo: archives, cleanupRuns: ledger, now: func() time.Time { return now }, logger: slog.Default(),
		settings: &fakeStatusFlagSettings{overrides: map[string]string{
			"operations.sick_clear_mode":    "manual",
			"operations.excused_clear_mode": "manual",
		}},
	})
}

func carriedDates(calls []statusFlagArchive, flagColumn string) []calendar.Date {
	var dates []calendar.Date
	for _, call := range calls {
		if call.carry && call.flagColumn == flagColumn {
			dates = append(dates, call.date)
		}
	}
	return dates
}

// TestStatusFlagClearCarriesEveryMissedDayAfterAnOutage — with manual the
// flag stands for every day until staff lift it. The Worker last succeeded
// on the evening before a daylight saving change and comes back days later
// in a new process: the run carries the flag into each missed Berlin day,
// in order, then records its success so the next poll adds nothing (#6291).
func TestStatusFlagClearCarriesEveryMissedDayAfterAnOutage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		last time.Time
		now  time.Time
		want []calendar.Date
	}{
		{
			name: "fall back",
			last: berlinTime(2026, time.October, 24, 18, 0),
			now:  berlinTime(2026, time.October, 27, 7, 0),
			want: []calendar.Date{calendar.NewDate(2026, time.October, 25), calendar.NewDate(2026, time.October, 26), calendar.NewDate(2026, time.October, 27)},
		},
		{
			name: "spring forward",
			last: berlinTime(2026, time.March, 27, 18, 0),
			now:  berlinTime(2026, time.March, 30, 9, 0),
			want: []calendar.Date{calendar.NewDate(2026, time.March, 28), calendar.NewDate(2026, time.March, 29), calendar.NewDate(2026, time.March, 30)},
		},
		{
			name: "outage from 9 to 12 November",
			last: berlinTime(2026, time.November, 9, 0, 0),
			now:  berlinTime(2026, time.November, 12, 0, 0),
			want: []calendar.Date{calendar.NewDate(2026, time.November, 10), calendar.NewDate(2026, time.November, 11), calendar.NewDate(2026, time.November, 12)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ledger := &statusFlagLedger{last: map[string]time.Time{statusFlagJobID: tc.last}}
			archives := &statusFlagArchives{}
			s := restartedScheduler(ledger, archives, tc.now)
			task := &ScheduledTask{Name: "status-flag-clear"}

			s.checkAndRunStatusFlagClear(context.Background(), task)

			calls := archives.take()
			assert.Equal(t, tc.want, carriedDates(calls, "sick"))
			assert.Equal(t, tc.want, carriedDates(calls, "excused"))
			for _, call := range calls {
				assert.True(t, call.inTransaction, "every day is carried in the school's transaction")
			}
			last, _, _, err := ledger.CleanupRun(context.Background(), statusFlagJobID)
			require.NoError(t, err)
			assert.True(t, tc.now.Equal(last), "the success is recorded in the ledger")

			restarted := restartedScheduler(ledger, archives, tc.now.Add(time.Minute))
			restarted.checkAndRunStatusFlagClear(context.Background(), task)
			assert.Empty(t, archives.take(), "the next process finds the boundary done")
		})
	}
}

// TestStatusFlagCarryDays pins the covered days: today alone without a
// recorded success, nothing older than the day after the last success, and
// at most the latest statusFlagCatchUpDays days with the rest counted as
// skipped.
func TestStatusFlagCarryDays(t *testing.T) {
	t.Parallel()
	now := berlinTime(2026, time.November, 10, 0, 0)
	today := calendar.NewDate(2026, time.November, 10)

	days, skipped := statusFlagCarryDays(now, time.Time{}, false)
	assert.Equal(t, []calendar.Date{today}, days)
	assert.Zero(t, skipped)

	days, skipped = statusFlagCarryDays(now.Add(18*time.Hour), now, true)
	assert.Equal(t, []calendar.Date{today}, days, "the evening run of the same day")
	assert.Zero(t, skipped)

	days, skipped = statusFlagCarryDays(now, berlinTime(2026, time.September, 1, 18, 0), true)
	require.Len(t, days, statusFlagCatchUpDays)
	assert.Equal(t, today.AddDays(-(statusFlagCatchUpDays - 1)), days[0])
	assert.Equal(t, today, days[len(days)-1])
	assert.Equal(t, calendar.NewDate(2026, time.September, 2).DaysUntil(today)+1-statusFlagCatchUpDays, skipped)
}
