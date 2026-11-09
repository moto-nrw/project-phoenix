package devices

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A school's device edit reads the row without a lock. It must write only the
// fields the edit owns: an operator key rotation and a status change that
// commit between that read and the write stay in force, so a rotated-away
// (possibly leaked) key does not work again.
func TestUpdateDevice_KeepsConcurrentKeyRotationAndStatus(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)
	device := testpkg.CreateTestDevice(t, ctx.db, fmt.Sprintf("race-device-%d", time.Now().UnixNano()))
	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())
	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/devices/%d", device.ID), map[string]any{
		"device_id":   device.DeviceID,
		"device_type": device.DeviceType,
		"name":        "Renamed during rotation",
	},
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:update"),
	)

	callCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	holder, err := ctx.db.BeginTx(callCtx, nil)
	require.NoError(t, err)
	committed := false
	defer func() {
		if !committed {
			assert.NoError(t, holder.Rollback())
		}
	}()
	var holderPID int
	require.NoError(t, holder.NewRaw(`SELECT pg_backend_pid()`).Scan(callCtx, &holderPID))
	rotatedHash := fmt.Sprintf("%064x", time.Now().UnixNano())
	_, err = holder.NewRaw(`UPDATE iot.devices SET api_key_hash = ?, masked_api_key = 'rotated', status = 'inactive' WHERE id = ?`,
		rotatedHash, device.ID).Exec(callCtx)
	require.NoError(t, err)

	result := make(chan *httptest.ResponseRecorder, 1)
	go func() { result <- testutil.ExecuteRequest(router, req) }()
	awaitBlockedBy(t, ctx.db, holderPID, result)
	require.NoError(t, holder.Commit())
	committed = true
	var rr *httptest.ResponseRecorder
	select {
	case rr = <-result:
	case <-callCtx.Done():
		t.Fatal("the device edit did not finish after the rotation committed")
	}

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	var stored struct {
		Name         *string `bun:"name"`
		APIKeyHash   *string `bun:"api_key_hash"`
		MaskedAPIKey string  `bun:"masked_api_key"`
		Status       string  `bun:"status"`
	}
	require.NoError(t, ctx.db.NewRaw(`SELECT name, api_key_hash, masked_api_key, status FROM iot.devices WHERE id = ?`, device.ID).
		Scan(context.Background(), &stored))
	require.NotNil(t, stored.APIKeyHash)
	assert.Equal(t, rotatedHash, *stored.APIKeyHash, "the rotated key stays the only valid key")
	assert.Equal(t, "rotated", stored.MaskedAPIKey)
	assert.Equal(t, "inactive", stored.Status, "an edit without a status keeps the committed status")
	require.NotNil(t, stored.Name)
	assert.Equal(t, "Renamed during rotation", *stored.Name, "the edit itself still lands")
}

// awaitBlockedBy waits until another backend waits on a lock holderPID holds.
// A request that finishes first never touched the held row.
func awaitBlockedBy[T any](t *testing.T, db *bun.DB, holderPID int, finished <-chan T) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-finished:
			t.Fatal("the write finished without waiting on the held row lock")
		default:
		}
		var waiting bool
		require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE ?::int = ANY (pg_blocking_pids(pid)))`, holderPID).
			Scan(context.Background(), &waiting))
		if waiting {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no backend waited on the held row lock")
}

// Device pings are debounced and written after the fact, so an older
// observation can reach the store after a newer one. last_seen only moves
// forward; a device without one takes the first observation.
func TestUpdateDeviceLastSeen_NeverMovesBackwards(t *testing.T) {
	t.Parallel()
	db, fleet := ComposeDeviceFleet(t)
	device := testpkg.CreateTestDevice(t, db, fmt.Sprintf("last-seen-%d", time.Now().UnixNano()))
	ctx := testpkg.Ctx(t)
	_, err := db.NewRaw(`UPDATE iot.devices SET last_seen = NULL WHERE id = ?`, device.ID).Exec(ctx)
	require.NoError(t, err)
	newer := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	older := newer.Add(-time.Minute)

	require.NoError(t, fleet.UpdateDeviceLastSeen(ctx, device.ID, newer))
	require.NoError(t, fleet.UpdateDeviceLastSeen(ctx, device.ID, older))

	var lastSeen *time.Time
	require.NoError(t, db.NewRaw(`SELECT last_seen FROM iot.devices WHERE id = ?`, device.ID).Scan(ctx, &lastSeen))
	require.NotNil(t, lastSeen, "a device without last_seen takes the first observation")
	assert.True(t, newer.Equal(*lastSeen), "the late, older observation must not move last_seen back (got %s)", lastSeen)
}

type fixedHeartbeat time.Time

func (h fixedHeartbeat) CheckDeviceHeartbeat(context.Context) (time.Time, bool, error) {
	return time.Time(h), true, nil
}

// The authenticated heartbeat and the kiosk ping keep the same forward-only
// rule as UpdateDeviceLastSeen.
func TestDeviceHeartbeats_NeverMoveLastSeenBackwards(t *testing.T) {
	t.Parallel()
	db, fleet := ComposeDeviceFleet(t)
	checked, ok := fleet.(devicefleet.CheckedDeviceHeartbeat)
	require.True(t, ok, "device auth uses the checked heartbeat when the fleet offers it")
	ctx := testpkg.Ctx(t)
	newer := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)
	for name, write := range map[string]func(*testpkg.Device) error{
		"checked": func(device *testpkg.Device) error {
			return checked.UpdateDeviceLastSeenChecked(ctx, device.ID, fixedHeartbeat(newer.Add(-time.Minute)))
		},
		"heartbeat": func(device *testpkg.Device) error {
			return testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(ctx context.Context) error {
				return fleet.RecordDeviceHeartbeat(ctx, device.DeviceID)
			})
		},
	} {
		device := testpkg.CreateTestDevice(t, db, fmt.Sprintf("forward-%s-%d", name, time.Now().UnixNano()))
		_, err := db.NewRaw(`UPDATE iot.devices SET last_seen = ? WHERE id = ?`, newer, device.ID).Exec(ctx)
		require.NoError(t, err)
		require.NoError(t, write(device), name)
		var lastSeen *time.Time
		require.NoError(t, db.NewRaw(`SELECT last_seen FROM iot.devices WHERE id = ?`, device.ID).Scan(ctx, &lastSeen))
		require.NotNil(t, lastSeen, name)
		assert.True(t, newer.Equal(*lastSeen), "%s moved last_seen back to %s", name, lastSeen)
	}
}
