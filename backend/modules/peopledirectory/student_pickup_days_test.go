package peopledirectory_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func requireStudentsPickupDaysColumn(t *testing.T, db *bun.DB) {
	t.Helper()
	var exists bool
	err := db.NewRaw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'users'
			  AND table_name = 'student_care_profiles'
			  AND column_name = 'pickup_days'
		)
	`).Scan(testpkg.Ctx(t), &exists)
	require.NoError(t, err)
	require.True(t, exists, "users.student_care_profiles.pickup_days column is not present in this test database")
}

// TestStudentRepository_PickupDaysRoundtrip exercises persistPickupDays (on both
// Create and Update) and the hydration path that reads the per-weekday map back,
// plus the legacy pickup_status → pickup_days seeding that keeps old callers and
// the importer working. Mirrors the bus_days roundtrip coverage.
func TestStudentRepository_PickupDaysRoundtrip(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewPeopleRepositorySuiteFactory(db).Student
	ctx := testpkg.Ctx(t)

	t.Run("persists and hydrates an explicit weekday map", func(t *testing.T) {
		requireStudentsPickupDaysColumn(t, db)

		person := testpkg.CreateTestPerson(t, db, "Pickup", "Map")

		student := &testpkg.Student{
			PersonID:    person.ID,
			SchoolClass: "1a",
			PickupDays: departure.PickupDays{
				departure.PickupDayMonday:    true,
				departure.PickupDayWednesday: true,
			},
		}

		require.NoError(t, repo.Create(ctx, student))

		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		assert.True(t, found.PickupDays[departure.PickupDayMonday])
		assert.True(t, found.PickupDays[departure.PickupDayWednesday])
		assert.False(t, found.PickupDays[departure.PickupDayTuesday])
		assert.True(t, found.PickupDays.HasAny())

	})

	t.Run("seeds all weekdays from legacy pickup_status picked up", func(t *testing.T) {
		requireStudentsPickupDaysColumn(t, db)

		person := testpkg.CreateTestPerson(t, db, "Pickup", "LegacyPickedUp")

		status := departure.PickupStatusPickedUp
		student := &testpkg.Student{
			PersonID:     person.ID,
			SchoolClass:  "2a",
			PickupStatus: &status,
		}

		require.NoError(t, repo.Create(ctx, student))

		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		for _, day := range departure.PickupDayOrder {
			assert.True(t, found.PickupDays[day], "legacy picked-up should enable %s", day)
		}

	})

	t.Run("seeds empty map from legacy goes-alone status", func(t *testing.T) {
		requireStudentsPickupDaysColumn(t, db)

		person := testpkg.CreateTestPerson(t, db, "Pickup", "LegacyAlone")

		status := departure.PickupStatusGoesAlone
		student := &testpkg.Student{
			PersonID:     person.ID,
			SchoolClass:  "2b",
			PickupStatus: &status,
		}

		require.NoError(t, repo.Create(ctx, student))

		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		assert.False(t, found.PickupDays.HasAny())
		assert.Empty(t, found.PickupDays.Normalize())

	})

	t.Run("updates the weekday map on an existing student", func(t *testing.T) {
		requireStudentsPickupDaysColumn(t, db)

		student := testpkg.CreateTestStudent(t, db, "Pickup", "Update", "3a")

		student.PickupDays = departure.PickupDays{departure.PickupDayFriday: true}
		require.NoError(t, repo.Update(ctx, student))

		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		assert.True(t, found.PickupDays[departure.PickupDayFriday])
		assert.False(t, found.PickupDays[departure.PickupDayMonday])
		assert.Equal(t, 1, len(found.PickupDays.Normalize()))

	})

	// A direct PickupDays write must keep the legacy pickup_status column in
	// sync even when the caller never sets PickupStatus — the repository is the
	// single source of truth, so filters/exports reading the legacy string can
	// never disagree with the weekday map. Covers the three distinct states.
	t.Run("syncs legacy pickup_status from a direct weekday write", func(t *testing.T) {
		requireStudentsPickupDaysColumn(t, db)

		t.Run("selected days derive picked-up", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Pickup", "SyncPicked", "4a")

			// Set only the map, leave PickupStatus nil (the stale-write case).
			student.PickupStatus = nil
			student.PickupDays = departure.PickupDays{departure.PickupDayFriday: true}
			require.NoError(t, repo.Update(ctx, student))

			found, err := repo.FindByID(ctx, student.ID)
			require.NoError(t, err)
			require.NotNil(t, found.PickupStatus)
			assert.Equal(t, departure.PickupStatusPickedUp, *found.PickupStatus)
		})

		t.Run("explicit empty map derives goes-alone", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Pickup", "SyncAlone", "4b")

			// Seed a picked-up student, then clear the map to the empty answer.
			picked := departure.PickupStatusPickedUp
			student.PickupStatus = &picked
			student.PickupDays = departure.PickupDays{departure.PickupDayMonday: true}
			require.NoError(t, repo.Update(ctx, student))

			student.PickupStatus = nil
			student.PickupDays = departure.PickupDays{}
			require.NoError(t, repo.Update(ctx, student))

			found, err := repo.FindByID(ctx, student.ID)
			require.NoError(t, err)
			require.NotNil(t, found.PickupStatus)
			assert.Equal(t, departure.PickupStatusGoesAlone, *found.PickupStatus)
			assert.False(t, found.PickupDays.HasAny())
		})
	})

	t.Run("rejects an invalid weekday before persistence", func(t *testing.T) {
		person := testpkg.CreateTestPerson(t, db, "Pickup", "Invalid")

		student := &testpkg.Student{
			PersonID:    person.ID,
			SchoolClass: "1a",
			PickupDays:  departure.PickupDays{"sat": true},
		}

		err := repo.Create(ctx, student)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `weekday "sat"`)
		assert.Zero(t, student.ID)
	})
}
