package migrations

import (
	"context"
	"testing"
	"time"

	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func departureDaysColumnExists(t *testing.T, db *bun.DB) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.NewRaw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'users' AND table_name = 'students'
				AND column_name = 'departure_days'
		)
	`).Scan(context.Background(), &exists))
	return exists
}

// TestStudentsDepartureDaysMigration verifies migration 1.15.124 (#1610): the
// backfill folds the legacy bus_days/pickup_days maps into departure_days, with
// pickup winning on a contradictory day, and the column survives a Down/Up
// round-trip.
func TestStudentsDepartureDaysMigration(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	// Ensure the column exists at baseline regardless of shared DB state.
	if !departureDaysColumnExists(t, db) {
		require.NoError(t, studentsDepartureDaysUp(ctx, db))
	}

	tenantID := time.Now().UnixNano()
	testpkg.EnsureTestTenant(t, db, tenantID)
	t.Cleanup(func() {
		testpkg.CleanupTenantTestData(t, db, tenantID)
		if !departureDaysColumnExists(t, db) {
			require.NoError(t, studentsDepartureDaysUp(ctx, db))
		}
	})

	busKid := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Bus", "Kid", "3a")
	pickupKid := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Pickup", "Kid", "3a")
	contradictKid := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Both", "Kid", "3a")

	// Seed legacy maps and reset departure_days to '{}' so the backfill (which
	// only touches empty rows) re-derives them.
	setLegacy := func(id int64, bus, pickup string) {
		_, err := db.NewRaw(
			`UPDATE users.students SET bus_days = ?::jsonb, pickup_days = ?::jsonb, departure_days = '{}'::jsonb WHERE id = ?`,
			bus, pickup, id,
		).Exec(ctx)
		require.NoError(t, err)
	}
	setLegacy(busKid.ID, `{"mon":true}`, `{}`)
	setLegacy(pickupKid.ID, `{}`, `{"wed":true}`)
	setLegacy(contradictKid.ID, `{"mon":true}`, `{"mon":true}`) // both → pickup wins

	// Re-run the migration; the ADD COLUMN/constraint are idempotent and the
	// backfill folds the legacy maps into departure_days.
	require.NoError(t, studentsDepartureDaysUp(ctx, db))

	read := func(id int64) usersModel.DepartureDays {
		var row struct {
			DepartureDays usersModel.DepartureDays `bun:"departure_days"`
		}
		require.NoError(t, db.NewRaw(`SELECT departure_days FROM users.students WHERE id = ?`, id).Scan(ctx, &row))
		return row.DepartureDays.Normalize()
	}

	assert.Equal(t, usersModel.DepartureBus, read(busKid.ID).ModeFor(usersModel.PickupDayMonday))
	assert.Equal(t, usersModel.DeparturePickup, read(pickupKid.ID).ModeFor(usersModel.PickupDayWednesday))
	assert.Equal(t, usersModel.DeparturePickup, read(contradictKid.ID).ModeFor(usersModel.PickupDayMonday),
		"a day flagged as both bus and pickup must fold to pickup")

	// Down/Up round-trip: dropping and re-adding the column must not error and
	// must restore the backfilled values from the still-present legacy maps.
	require.NoError(t, studentsDepartureDaysDown(ctx, db))
	require.False(t, departureDaysColumnExists(t, db), "Down should drop departure_days")
	require.NoError(t, studentsDepartureDaysUp(ctx, db))
	require.True(t, departureDaysColumnExists(t, db), "Up should re-add departure_days")
	assert.Equal(t, usersModel.DepartureBus, read(busKid.ID).ModeFor(usersModel.PickupDayMonday))
}
