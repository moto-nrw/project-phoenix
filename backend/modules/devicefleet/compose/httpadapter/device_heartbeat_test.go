package httpadapter_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func heartbeatState(t *testing.T, ctx context.Context, db bun.IDB, id int64) (json.RawMessage, *time.Time) {
	t.Helper()
	var row struct {
		State    json.RawMessage `bun:"state"`
		LastSeen *time.Time      `bun:"last_seen"`
	}
	require.NoError(t, db.NewRaw(`SELECT to_jsonb(d) - 'last_seen' AS state, last_seen FROM iot.devices d WHERE id = ?`, id).Scan(ctx, &row))
	return row.State, row.LastSeen
}

func TestRecordDeviceHeartbeatMatchesExistingPingAndJoinsCaller(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	legacy := setupIoTService(t, db)
	fleet := legacy.Fleet()
	device := testpkg.CreateTestDevice(t, db, "heartbeat-owned-unit")
	var state json.RawMessage
	var seen *time.Time
	counter := testpkg.CaptureQueriesForContext(t, db)
	require.NoError(t, testpkg.WithTenantTx(t, counter.Context(testpkg.Ctx(t)), db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
		// Both writes share PostgreSQL's transaction timestamp, including the
		// existing update_modified_column trigger. Compare every stored column
		// except last_seen against the existing PingDevice, not a trigger-free row.
		require.NoError(t, legacy.PingDevice(ctx, device.DeviceID))
		state, seen = heartbeatState(t, ctx, tx, device.ID)
		counter.Reset()
		require.NoError(t, fleet.RecordDeviceHeartbeat(ctx, device.DeviceID))
		assert.Equal(t, 1, counter.Total(), "one guarded write, with no lookup or transaction frame")
		return nil
	}))
	updated, lastSeen := heartbeatState(t, testpkg.Ctx(t), db, device.ID)
	assert.JSONEq(t, string(state), string(updated), "new heartbeat must preserve the existing ping, including updated_at")
	require.NotNil(t, lastSeen)
	assert.NotEqual(t, seen, lastSeen)

	stop := errors.New("roll back caller")
	require.ErrorIs(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		require.NoError(t, fleet.RecordDeviceHeartbeat(ctx, device.DeviceID))
		return stop
	}), stop)
	rolledBack, rolledBackSeen := heartbeatState(t, testpkg.Ctx(t), db, device.ID)
	assert.JSONEq(t, string(updated), string(rolledBack))
	assert.Equal(t, lastSeen, rolledBackSeen, "the heartbeat may not commit independently")
}

func TestRecordDeviceHeartbeatKeepsLiveDeviceAndTenantGuards(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	fleet := setupIoTService(t, db).Fleet()
	archived := testpkg.CreateTestDevice(t, db, "heartbeat-archived")
	_, err := db.NewRaw("UPDATE iot.devices SET archived_at = ? WHERE id = ?", time.Now(), archived.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	foreignTenant, _ := testpkg.CreateTestTenant(t, db)
	foreign := testpkg.CreateTestDeviceForTenant(t, db, foreignTenant, "heartbeat-foreign")
	for _, device := range []*testpkg.Device{archived, foreign} {
		before, seen := heartbeatState(t, testpkg.Ctx(t), db, device.ID)
		require.ErrorIs(t, fleet.RecordDeviceHeartbeat(testpkg.Ctx(t), device.DeviceID), devicefleet.ErrDeviceNotFound)
		after, lastSeen := heartbeatState(t, testpkg.Ctx(t), db, device.ID)
		assert.JSONEq(t, string(before), string(after))
		assert.Equal(t, seen, lastSeen)
	}
	require.ErrorIs(t, fleet.RecordDeviceHeartbeat(testpkg.Ctx(t), ""), devicefleet.ErrInvalidDevice)
	require.ErrorIs(t, fleet.RecordDeviceHeartbeat(testpkg.Ctx(t), "missing-heartbeat-device"), devicefleet.ErrDeviceNotFound)
}
