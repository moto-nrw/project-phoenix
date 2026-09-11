package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/migrations"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestPresenceBackfillCommandRejectsUnsafeArgumentsBeforeOpeningDB(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{}, {"--tenant-id=-1"}, {"--tenant-id=1", "--all-tenants"},
		{"restart", "--all-tenants"}, {"--tenant-id=1", "--batch-size=0"},
		{"--tenant-id=1", "--max-batches=-1"}, {"unknown", "--tenant-id=1"},
	} {
		cmd := newPresenceBackfillCommand(migrateRoot{openDatabase: func() (*testpkg.DB, error) {
			t.Fatal("invalid input opened a database")
			return nil, nil
		}})
		cmd.SetArgs(args)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		require.Error(t, cmd.Execute())
	}
}

func TestPresenceBackfillCommandWritesDurableProgressAndStopsAtLimit(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	var calls int
	run := func(context.Context) (migrations.PresenceBackfillReport, *migrations.PresenceBackfillTelemetry, error) {
		calls++
		return migrations.PresenceBackfillReport{Phase: "attendance", BatchesCompleted: int64(calls)}, nil, nil
	}
	err := writePresenceBackfillProgress(t.Context(), &output, 2, run)
	require.ErrorContains(t, err, "batch limit reached")
	require.Equal(t, 2, calls)
	require.Equal(t, 2, bytes.Count(output.Bytes(), []byte("\n")))
	require.Contains(t, output.String(), `"complete":false`)
}

func TestPresenceBackfillCommandRunsAllTenantsAndStatusDoesNotWrite(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	a := testpkg.UniqueTestTenantID(t)
	b := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, a)
	testpkg.EnsureTestTenant(t, db, b)
	execute := func(args ...string) string {
		t.Helper()
		cmd := newPresenceBackfillCommand(migrateRoot{openDatabase: func() (*testpkg.DB, error) {
			return testpkg.SetupClosableTestDB(t), nil
		}})
		var output bytes.Buffer
		cmd.SetArgs(args)
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		require.NoError(t, cmd.Execute())
		return output.String()
	}
	before := execute("status", "--tenant-id", strconv.FormatInt(a, 10))
	require.Contains(t, before, `"phase":"not_started"`)
	output := execute("run", "--all-tenants", "--batch-size", "1")
	var previous int64
	var completed []int64
	decoder := json.NewDecoder(bytes.NewBufferString(output))
	for decoder.More() {
		var event struct {
			Checkpoint migrations.PresenceBackfillReport     `json:"checkpoint"`
			Metrics    *migrations.PresenceBackfillTelemetry `json:"metrics"`
		}
		require.NoError(t, decoder.Decode(&event))
		require.GreaterOrEqual(t, event.Checkpoint.TenantID, previous)
		previous = event.Checkpoint.TenantID
		if event.Checkpoint.Complete {
			require.NotNil(t, event.Metrics)
			if event.Checkpoint.TenantID == a || event.Checkpoint.TenantID == b {
				require.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", event.Checkpoint.Sessions.SourceChecksum, "empty tenant has the canonical SHA-256 digest")
			}
			completed = append(completed, previous)
		} else {
			require.Nil(t, event.Metrics, "copy progress must not perform full-tenant metric scans")
		}
	}
	require.Contains(t, completed, a)
	require.Contains(t, completed, b)
	var storedBefore, storedAfter string
	require.NoError(t, db.NewRaw(`SELECT state::text FROM active.presence_backfill_checkpoints WHERE tenant_id = ?`, a).Scan(t.Context(), &storedBefore))
	require.Contains(t, execute("status", "--tenant-id", strconv.FormatInt(a, 10)), `"complete":true`)
	require.NoError(t, db.NewRaw(`SELECT state::text FROM active.presence_backfill_checkpoints WHERE tenant_id = ?`, a).Scan(t.Context(), &storedAfter))
	require.Equal(t, storedBefore, storedAfter)
	require.Contains(t, execute("restart", "--tenant-id", strconv.FormatInt(a, 10)), `"complete":false`)
}
