package users_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func requireStudentsDepartureDaysColumn(t *testing.T, db *bun.DB) {
	t.Helper()
	var exists bool
	err := db.NewRaw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'users'
			  AND table_name = 'students'
			  AND column_name = 'departure_days'
		)
	`).Scan(testpkg.TenantContext(1), &exists)
	require.NoError(t, err)
	if !exists {
		t.Skip("users.students.departure_days column is not present in this test database")
	}
}

// TestStudentRepository_DepartureDaysRoundtrip exercises the unified
// departure_days source of truth (#1610): persisting a unified plan derives the
// legacy bus_days/pickup_days/pickup_status mirrors, hydration reads the plan
// back, and a unified replacement fully overwrites the legacy maps.
func TestStudentRepository_DepartureDaysRoundtrip(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Student
	ctx := testpkg.TenantContext(1)

	t.Run("unified plan derives legacy mirrors", func(t *testing.T) {
		requireStudentsDepartureDaysColumn(t, db)

		student := testpkg.CreateTestStudent(t, db, "Departure", "Roundtrip", "1a")
		defer cleanupStudentRecords(t, db, student.ID)

		student.DepartureDays = users.DepartureDays{
			users.PickupDayMonday:    users.DepartureBus,
			users.PickupDayWednesday: users.DeparturePickup,
		}
		require.NoError(t, repo.Update(ctx, student))

		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		assert.Equal(t, users.DepartureBus, found.DepartureDays.ModeFor(users.PickupDayMonday))
		assert.Equal(t, users.DeparturePickup, found.DepartureDays.ModeFor(users.PickupDayWednesday))
		assert.Equal(t, users.DepartureAlone, found.DepartureDays.ModeFor(users.PickupDayFriday))
		// Derived legacy mirrors.
		assert.True(t, found.BusDays[users.PickupDayMonday])
		assert.False(t, found.BusDays[users.PickupDayWednesday])
		assert.True(t, found.PickupDays[users.PickupDayWednesday])
		require.NotNil(t, found.PickupStatus)
		assert.Equal(t, users.PickupStatusPickedUp, *found.PickupStatus)
	})

	t.Run("legacy maps fold into departure_days", func(t *testing.T) {
		requireStudentsDepartureDaysColumn(t, db)

		student := testpkg.CreateTestStudent(t, db, "Departure", "Fold", "2a")
		defer cleanupStudentRecords(t, db, student.ID)

		// A contradictory legacy state (bus AND pickup on Monday) folds with
		// pickup winning, leaving no bus/pickup overlap.
		student.BusDays = users.BusDays{users.PickupDayMonday: true, users.PickupDayTuesday: true}
		student.PickupDays = users.PickupDays{users.PickupDayMonday: true}
		require.NoError(t, repo.Update(ctx, student))

		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		assert.Equal(t, users.DeparturePickup, found.DepartureDays.ModeFor(users.PickupDayMonday))
		assert.Equal(t, users.DepartureBus, found.DepartureDays.ModeFor(users.PickupDayTuesday))
		assert.False(t, found.BusDays[users.PickupDayMonday]) // pickup won
	})

	t.Run("unified replacement clears prior days", func(t *testing.T) {
		requireStudentsDepartureDaysColumn(t, db)

		student := testpkg.CreateTestStudent(t, db, "Departure", "Replace", "3a")
		defer cleanupStudentRecords(t, db, student.ID)

		student.DepartureDays = users.DepartureDays{users.PickupDayMonday: users.DepartureBus}
		require.NoError(t, repo.Update(ctx, student))

		// Replace with an empty plan via the unified field (re-fetch first so the
		// working maps are populated, mirroring how a handler operates).
		fresh, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		fresh.DepartureDays = users.DepartureDays{}
		fresh.BusDays = users.BusDays{}
		fresh.PickupDays = users.PickupDays{}
		require.NoError(t, repo.Update(ctx, fresh))

		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		assert.False(t, found.DepartureDays.HasAny())
		assert.False(t, found.BusDays.HasAny())
		require.NotNil(t, found.PickupStatus)
		assert.Equal(t, users.PickupStatusGoesAlone, *found.PickupStatus)
	})
}
