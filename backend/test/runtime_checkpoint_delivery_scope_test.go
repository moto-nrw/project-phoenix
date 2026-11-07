//go:build testhooks

package test_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Keep Delivery's three real checkpoint modes, including actual render and
// unavailable-transport failures. Only their accounting is under test; no
// latency, retry duration or sampled-lock statistic is asserted.
func TestDeliveryCheckpointReportsOnlyItsWorkerQueries(t *testing.T) {
	t.Parallel()
	fixtureDB := testpkg.SetupIsolatedTestDB(t)
	// Setup initializes the package's APP_ENV. Retain the checkpoint's
	// process-safety guards without mutating process environment in parallel.
	require.Equal(t, "test", os.Getenv("APP_ENV"))
	require.Empty(t, os.Getenv("EMAIL_SMTP_HOST"))
	runtimeDB := testpkg.SetupClosableTestDB(t)
	t.Cleanup(func() { require.NoError(t, runtimeDB.Close()) })
	database := testpkg.NewDatabase(t, runtimeDB)
	unit := database.UnitOfWork()
	module, err := api.NewDeliveryTestModule(database, unit)
	require.NoError(t, err)
	settings, err := owners.NewSettingsTestModule(database, unit)
	require.NoError(t, err)
	schools, err := owners.NewOrganizationTenancy(database)
	require.NoError(t, err)
	identity, err := api.NewIdentityAccessForTests(database)
	require.NoError(t, err)
	env := owners.RootEnvironment{Logger: slog.Default(), Config: owners.ServiceConfig{AppEnv: "test"}}
	mailer, _, from, vapid, err := owners.BuildMailTransport(env)
	require.NoError(t, err)
	mailIdentity := owners.TenantMailIdentity(schools, settings.Settings, env)
	// This is the real root's MailPushProvider/registry/module gate over the
	// real worker store. The explicit test transport cannot dial SMTP. Only
	// guardian_invitation intents are generated; appointment/reminder/document
	// collaborators are unused in these three modes, not simulated providers.
	worker, err := owners.BuildDeliveryWorker(database, env, mailer, from, vapid, mailIdentity,
		owners.DeliveryRuntime{Runtime: module.Runtime, Outbox: module.EmailOutbox}, nil,
		owners.IdentityDeliveryRenderers(identity, from, mailIdentity, module.Runtime.EmailStatus, env),
		nil, settings.Settings, nil)
	require.NoError(t, err)

	counter := testpkg.CaptureQueriesForContext(t, runtimeDB)
	counter.Stop()
	reference := testpkg.CaptureQueries(t, runtimeDB)
	reference.Stop()
	observed := &checkpointDeliveryScopeWorker{worker: worker, db: runtimeDB, reference: reference}
	metrics := func() string {
		var metric int
		require.NoError(t, runtimeDB.NewRaw("SELECT 42 /* checkpoint-delivery-metrics */").Scan(context.Background(), &metric))
		return fmt.Sprintf("delivery_scope_metric %d\n", metric)
	}

	results := testpkg.MeasureDeliveryCheckpoint(t, runtimeDB, fixtureDB, observed, unit, counter, metrics)
	counter.Stop()
	reference.Stop()
	require.Len(t, results, 3)
	require.Len(t, observed.operations, 105)
	for mode, result := range results {
		require.Len(t, result.Samples, 30, result.Name)
		require.Equal(t, "delivery_scope_metric 42\n", result.MetricsBefore, result.Name)
		require.Equal(t, "delivery_scope_metric 42\n", result.MetricsAfter, result.Name)
		for index, sample := range result.Samples {
			actual := observed.operations[mode*35+index+5]
			require.Positive(t, actual.queries, "the real worker must execute SQL even when its outboxes are idle")
			require.Positive(t, actual.rowsAffected, "the reference must observe real driver rows")
			require.Positive(t, actual.statementsWithRows, "the reference must observe row-bearing statements")
			if index == 0 {
				t.Logf("%s: same-call reference queries=%d rows=%d row_statements=%d; published queries=%d rows=%d row_statements=%d",
					result.Name, actual.queries, actual.rowsAffected, actual.statementsWithRows,
					sample.Queries, sample.RowsAffected, sample.StatementsWithRows)
			}
			require.Equal(t, actual.queries, sample.Queries, "%s sample %d must report the worker's real SQL", result.Name, index)
			require.Equal(t, actual.rowsAffected, sample.RowsAffected, "%s sample %d must report only worker driver rows", result.Name, index)
			require.Equal(t, actual.statementsWithRows, sample.StatementsWithRows, "%s sample %d must exclude Backlog and observers", result.Name, index)
		}
	}
	// The last operation is idle. Subsequent real Backlog/metrics calls must
	// not replace any of its SQL, even if aggregate counts happened to agree.
	require.Equal(t, observed.operations[len(observed.operations)-1].sql, counter.Queries())
	for _, query := range counter.Queries() {
		upper := strings.ToUpper(query)
		require.NotContains(t, upper, "CHECKPOINT-DELIVERY-OBSERVER")
		require.NotContains(t, upper, "CHECKPOINT-DELIVERY-METRICS")
		require.NotContains(t, upper, "CHECKPOINT-DELIVERY-BACKLOG")
		require.NotContains(t, upper, "PG_STAT_ACTIVITY")
		require.NotContains(t, upper, "PG_STAT_DATABASE")
		require.NotContains(t, upper, "SELECT STATUS, ATTEMPTS, LAST_ERROR FROM PLATFORM.EMAIL_OUTBOX WHERE ID")
		require.NotContains(t, upper, "DELETE FROM PLATFORM.EMAIL_OUTBOX WHERE ID =")
	}
}

type checkpointDeliveryScopeEvidence struct {
	queries            int
	rowsAffected       int64
	statementsWithRows int
	sql                []string
}

// This decorator forwards every worker argument, result and error unchanged.
// Its reference capture observes the real production RunOnce; it implements
// neither outbox logic, rendering, provider outcomes nor fabricated counters.
type checkpointDeliveryScopeWorker struct {
	worker     testpkg.CheckpointDeliveryWorker
	db         *bun.DB
	reference  *testpkg.QueryCounter
	operations []checkpointDeliveryScopeEvidence
}

func (w *checkpointDeliveryScopeWorker) RunOnce(ctx context.Context, batchSize, maxAttempts int) (int, error) {
	var observer int
	if err := w.db.NewRaw("SELECT 41 /* checkpoint-delivery-observer */").Scan(context.Background(), &observer); err != nil {
		return 0, err
	}
	w.reference.Reset()
	w.reference.Start()
	claimed, err := w.worker.RunOnce(ctx, batchSize, maxAttempts)
	w.reference.Stop()
	rows, statements := w.reference.Rows()
	w.operations = append(w.operations, checkpointDeliveryScopeEvidence{
		queries: w.reference.Total(), rowsAffected: rows,
		statementsWithRows: statements, sql: w.reference.Queries(),
	})
	return claimed, err
}

func (w *checkpointDeliveryScopeWorker) Backlog(ctx context.Context) (int, error) {
	backlog, err := w.worker.Backlog(ctx)
	if err != nil {
		return 0, err
	}
	// This marked real query is deliberately OUTSIDE RunOnce. Its driver
	// row would leak into the sample if the helper kept counting Backlog.
	var observer int
	if err := w.db.NewRaw("SELECT 43 /* checkpoint-delivery-backlog */").Scan(ctx, &observer); err != nil {
		return 0, err
	}
	return backlog, nil
}
