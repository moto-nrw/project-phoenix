package test

import (
	"context"
	"math"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type RuntimeCheckpointSample struct {
	RowsAffected       int64   `json:"rows_affected"`
	StatementsWithRows int     `json:"statements_with_rows"`
	DurationMS         float64 `json:"duration_ms"`
	Queries            int     `json:"queries"`
	Status             int     `json:"status"`
	PoolWaitCount      int64   `json:"pool_wait_count"`
	PoolWaitMS         float64 `json:"pool_wait_ms"`
	ErrorBody          string  `json:"error_body,omitempty"`
}

type RuntimeCheckpointWorkerResult struct {
	RowsAffected  []int                        `json:"rows_affected,omitempty"`
	RowsSkipped   []int                        `json:"rows_skipped,omitempty"`
	NotApplicable string                       `json:"not_applicable,omitempty"`
	LockSamples   RuntimeCheckpointLockSamples `json:"lock_samples"`
	Deadlocks     int64                        `json:"deadlocks"`
	Name          string                       `json:"name"`
	Samples       []RuntimeCheckpointSample    `json:"samples"`
	Claimed       []int                        `json:"claimed"`
	BacklogBefore []int                        `json:"backlog_before"`
	BacklogAfter  []int                        `json:"backlog_after"`
	States        []string                     `json:"states"`
	Attempts      []int                        `json:"attempts"`
	Errors        []string                     `json:"errors"`
	MetricsBefore string                       `json:"metrics_before"`
	MetricsAfter  string                       `json:"metrics_after"`
}

// CheckpointDeliveryWorker is the public worker seam used by the checkpoint.
type CheckpointDeliveryWorker interface {
	RunOnce(context.Context, int, int) (int, error)
	Backlog(context.Context) (int, error)
}

// MeasureDeliveryCheckpoint drives the production worker over owned fixture
// intents. Fixture insertion, inspection and reset are outside timed regions.
func MeasureDeliveryCheckpoint(t *testing.T, db, fixtureDB *bun.DB, worker CheckpointDeliveryWorker, uow tenant.UnitOfWork, counter *QueryCounter, metrics func() string) []RuntimeCheckpointWorkerResult {
	t.Helper()
	require.Equal(t, "test", os.Getenv("APP_ENV"))
	require.Empty(t, os.Getenv("EMAIL_SMTP_HOST"))
	ctx := tenant.WithUnitOfWork(context.Background(), uow)
	var results []RuntimeCheckpointWorkerResult
	for _, name := range []string{"delivery.provider-unavailable", "delivery.render-failure", "delivery.idle"} {
		result := RuntimeCheckpointWorkerResult{Name: name}
		var stopSampling func() RuntimeCheckpointLockSamples
		deadlocks := func() int64 {
			var count int64
			require.NoError(t, fixtureDB.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(ctx, &count))
			return count
		}
		var deadlocksBefore int64
		for iteration := range 35 {
			payload := `{"recipient_email":"checkpoint@example.invalid","invitation_url":"http://localhost/invite/checkpoint","expiry_hours":48}`
			if name == "delivery.render-failure" {
				payload = `{"recipient_email":"checkpoint@example.invalid"}`
			}
			var intentID int64
			if name != "delivery.idle" {
				require.NoError(t, fixtureDB.NewRaw(`INSERT INTO platform.email_outbox (tenant_id, kind, recipient, payload, status, next_retry_at) VALUES (?, 'guardian_invitation', '{"address":"checkpoint@example.invalid"}'::jsonb, ?::jsonb, 'pending', NOW()) RETURNING id`, Tenant(t), payload).Scan(ctx, &intentID))
			}
			backlogBefore, err := worker.Backlog(ctx)
			require.NoError(t, err)
			if iteration == 5 {
				result.MetricsBefore = metrics()
				deadlocksBefore = deadlocks()
				stopSampling = SampleCheckpointLocks(func(sampleCtx context.Context) (int, error) {
					var waiting int
					err := fixtureDB.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND usename = 'phoenix_auth' AND wait_event_type = 'Lock'").Scan(sampleCtx, &waiting)
					return waiting, err
				})
				t.Cleanup(func() { _ = stopSampling() })
			}
			counter.Reset()
			before := db.Stats()
			counter.Start()
			started := time.Now()
			claimed, err := worker.RunOnce(ctx, 2, 3)
			elapsed := time.Since(started)
			counter.Stop()
			after := db.Stats()
			require.NoError(t, err)
			queryCount := counter.Total()
			backlogAfter, err := worker.Backlog(ctx)
			require.NoError(t, err)
			var status struct {
				Status    string
				Attempts  int
				LastError string
			}
			if name == "delivery.idle" {
				require.Zero(t, claimed)
			} else {
				require.Equal(t, 1, claimed)
				require.NoError(t, fixtureDB.NewRaw("SELECT status, attempts, last_error FROM platform.email_outbox WHERE id = ? AND tenant_id = ?", intentID, Tenant(t)).Scan(ctx, &status))
				require.Equal(t, "pending", status.Status)
				require.Equal(t, 1, status.Attempts)
				if name == "delivery.render-failure" {
					require.Contains(t, status.LastError, "missing invitation_url")
				} else {
					require.Contains(t, status.LastError, "email delivery is unavailable")
				}
			}
			if iteration >= 5 {
				result.Samples = append(result.Samples, RuntimeCheckpointSample{DurationMS: float64(elapsed) / float64(time.Millisecond), Queries: queryCount, PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond)})
				result.Samples[len(result.Samples)-1].RowsAffected, result.Samples[len(result.Samples)-1].StatementsWithRows = counter.Rows()
				result.Claimed = append(result.Claimed, claimed)
				result.BacklogBefore = append(result.BacklogBefore, backlogBefore)
				result.BacklogAfter = append(result.BacklogAfter, backlogAfter)
				result.States = append(result.States, status.Status)
				result.Attempts = append(result.Attempts, status.Attempts)
				result.Errors = append(result.Errors, status.LastError)
			}
			if name != "delivery.idle" {
				// Remove only the owned fixture after observing its scheduled retry, so
				// every sample starts with the same data volume and backlog.
				_, err = fixtureDB.NewRaw("DELETE FROM platform.email_outbox WHERE id = ? AND tenant_id = ?", intentID, Tenant(t)).Exec(ctx)
				require.NoError(t, err)
			}
		}
		result.MetricsAfter = metrics()
		result.LockSamples = stopSampling()
		require.Empty(t, result.LockSamples.Error)
		result.Deadlocks = deadlocks() - deadlocksBefore
		results = append(results, result)
	}
	return results
}

type RuntimeCheckpointLockSamples struct {
	Samples               int     `json:"samples"`
	WaitingBackendSamples int     `json:"waiting_backend_samples"`
	MaxWaitingBackends    int     `json:"max_waiting_backends"`
	MaxSampleGapMS        float64 `json:"max_sample_gap_ms"`
	Error                 string  `json:"error,omitempty"`
}

// Sample actual PostgreSQL Lock wait events, not acquisition-query duration.
// Counts are sampled observations, not exact durations or proof of zero waits.
func SampleCheckpointLocks(sample func(context.Context) (int, error)) func() RuntimeCheckpointLockSamples {
	stop := make(chan struct{})
	done := make(chan RuntimeCheckpointLockSamples, 1)
	go func() {
		result := RuntimeCheckpointLockSamples{}
		ticker := time.NewTicker(2 * time.Millisecond)
		defer ticker.Stop()
		last := time.Now()
		for {
			select {
			case <-stop:
				done <- result
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				waiting, err := sample(ctx)
				cancel()
				if err != nil {
					result.Error = err.Error()
					done <- result
					return
				}
				now := time.Now()
				result.MaxSampleGapMS = math.Max(result.MaxSampleGapMS, float64(now.Sub(last))/float64(time.Millisecond))
				last = now
				result.Samples++
				result.WaitingBackendSamples += waiting
				result.MaxWaitingBackends = max(result.MaxWaitingBackends, waiting)
			}
		}
	}()
	var once sync.Once
	var result RuntimeCheckpointLockSamples
	return func() RuntimeCheckpointLockSamples {
		once.Do(func() { close(stop); result = <-done })
		return result
	}
}

// CreateRuntimeCheckpointTimetable creates one complete Monday template for a
// fixed calendar window. All IDs come from this test's owned fixtures.
func CreateRuntimeCheckpointTimetable(t *testing.T, db *bun.DB, roomID int64) int64 {
	t.Helper()
	period := CreateTestCalendarPeriod(t, db, "Checkpoint September", timezone.Date("2026-09-01"), timezone.Date("2026-09-30"))
	SetCalendarPeriodActive(t, db, period, true)
	group := CreateTestActivityGroup(t, db, "Checkpoint Recurring Activity")
	frame := CreateTestTimeframeForTenant(t, db, Tenant(t), "Checkpoint Afternoon")
	_, err := db.NewRaw("UPDATE schedule.timeframes SET start_time = '08:00', end_time = '16:00' WHERE id = ? AND tenant_id = ?", frame.ID, Tenant(t)).Exec(Ctx(t))
	require.NoError(t, err)
	_, err = db.NewRaw("UPDATE activities.groups SET is_template = TRUE, planned_room_id = ?, calendar_period_id = ? WHERE id = ? AND tenant_id = ?", roomID, period.ID, group.ID, Tenant(t)).Exec(Ctx(t))
	require.NoError(t, err)
	_, err = db.NewRaw("INSERT INTO activities.schedules (tenant_id, weekday, timeframe_id, activity_group_id, week_pattern) VALUES (?, 1, ?, ?, 0)", Tenant(t), frame.ID, group.ID).Exec(Ctx(t))
	require.NoError(t, err)
	return group.ID
}

// MeasureTimetableCheckpoint invokes the same public materialization operation
// as the scheduler. Fixture resets are outside the timed production operation.
func MeasureTimetableCheckpoint(t *testing.T, db, fixtureDB *bun.DB, uow tenant.UnitOfWork, counter *QueryCounter, templateID int64, run func(context.Context) (int, int, error), metrics func() string) []RuntimeCheckpointWorkerResult {
	t.Helper()
	ctx := tenant.WithUnitOfWork(Ctx(t), uow)
	reset := func() {
		_, err := fixtureDB.NewRaw("DELETE FROM schedule.activity_instances WHERE tenant_id = ? AND activity_group_id = ?", Tenant(t), templateID).Exec(ctx)
		require.NoError(t, err)
	}
	var results []RuntimeCheckpointWorkerResult
	for _, name := range []string{"timetable.materialize-create", "timetable.materialize-existing"} {
		reset()
		result := RuntimeCheckpointWorkerResult{Name: name, NotApplicable: "Backlog/attempt counters: synchronous materialization has no persistent queue. Transaction retries are separate counters."}
		var stopSampling func() RuntimeCheckpointLockSamples
		deadlocks := func() int64 {
			var count int64
			require.NoError(t, fixtureDB.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(ctx, &count))
			return count
		}
		var deadlocksBefore int64
		for iteration := range 35 {
			if name == "timetable.materialize-create" {
				reset()
			}
			if iteration == 5 {
				result.MetricsBefore = metrics()
				deadlocksBefore = deadlocks()
				stopSampling = SampleCheckpointLocks(func(sampleCtx context.Context) (int, error) {
					var waiting int
					err := fixtureDB.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND usename = 'phoenix_auth' AND wait_event_type = 'Lock'").Scan(sampleCtx, &waiting)
					return waiting, err
				})
				t.Cleanup(func() { _ = stopSampling() })
			}
			counter.Reset()
			before := db.Stats()
			counter.Start()
			started := time.Now()
			created, skipped, err := run(ctx)
			elapsed := time.Since(started)
			counter.Stop()
			after := db.Stats()
			require.NoError(t, err)
			if name == "timetable.materialize-create" || iteration == 0 {
				require.Equal(t, 1, created)
				require.Zero(t, skipped)
			} else {
				require.Zero(t, created)
				require.Equal(t, 1, skipped)
			}
			if iteration >= 5 {
				result.Samples = append(result.Samples, RuntimeCheckpointSample{DurationMS: float64(elapsed) / float64(time.Millisecond), Queries: counter.Total(), PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond)})
				result.Samples[len(result.Samples)-1].RowsAffected, result.Samples[len(result.Samples)-1].StatementsWithRows = counter.Rows()
				result.RowsAffected = append(result.RowsAffected, created)
				result.RowsSkipped = append(result.RowsSkipped, skipped)
			}
		}
		result.MetricsAfter = metrics()
		result.LockSamples = stopSampling()
		require.Empty(t, result.LockSamples.Error)
		result.Deadlocks = deadlocks() - deadlocksBefore
		reset()
		results = append(results, result)
	}
	return results
}
