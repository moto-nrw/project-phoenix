package contracttest_test

import (
	"testing"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/api"

	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
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

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Student
	ctx := testpkg.Ctx(t)

	t.Run("persists and hydrates an explicit weekday map", func(t *testing.T) {
		requireStudentsPickupDaysColumn(t, db)

		person := testpkg.CreateTestPerson(t, db, "Pickup", "Map")

		student := &peopleValues.StudentState{
			PersonID:    person.ID,
			SchoolClass: "1a",
			PickupDays: peopleValues.PickupDays{
				peopleValues.PickupDayMonday:    true,
				peopleValues.PickupDayWednesday: true,
			},
		}

		require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Create, student))

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(student.ID))
		require.NoError(t, err)
		assert.True(t, found.PickupDays[peopleValues.PickupDayMonday])
		assert.True(t, found.PickupDays[peopleValues.PickupDayWednesday])
		assert.False(t, found.PickupDays[peopleValues.PickupDayTuesday])
		assert.True(t, found.PickupDays.HasAny())

	})

	t.Run("seeds all weekdays from legacy pickup_status picked up", func(t *testing.T) {
		requireStudentsPickupDaysColumn(t, db)

		person := testpkg.CreateTestPerson(t, db, "Pickup", "LegacyPickedUp")

		status := userModels.PickupStatusPickedUp
		student := &peopleValues.StudentState{
			PersonID:     person.ID,
			SchoolClass:  "2a",
			PickupStatus: &status,
		}

		require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Create, student))

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(student.ID))
		require.NoError(t, err)
		for _, day := range peopleValues.PickupDayOrder {
			assert.True(t, found.PickupDays[day], "legacy picked-up should enable %s", day)
		}

	})

	t.Run("seeds empty map from legacy goes-alone status", func(t *testing.T) {
		requireStudentsPickupDaysColumn(t, db)

		person := testpkg.CreateTestPerson(t, db, "Pickup", "LegacyAlone")

		status := userModels.PickupStatusGoesAlone
		student := &peopleValues.StudentState{
			PersonID:     person.ID,
			SchoolClass:  "2b",
			PickupStatus: &status,
		}

		require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Create, student))

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(student.ID))
		require.NoError(t, err)
		assert.False(t, found.PickupDays.HasAny())
		assert.Empty(t, found.PickupDays.Normalize())

	})

	t.Run("updates the weekday map on an existing student", func(t *testing.T) {
		requireStudentsPickupDaysColumn(t, db)

		studentFixture := testpkg.CreateTestStudent(t, db, "Pickup", "Update", "3a")
		student, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(studentFixture.ID))
		require.NoError(t, err)

		student.PickupDays = peopleValues.PickupDays{peopleValues.PickupDayFriday: true}
		require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Update, student))

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(student.ID))
		require.NoError(t, err)
		assert.True(t, found.PickupDays[peopleValues.PickupDayFriday])
		assert.False(t, found.PickupDays[peopleValues.PickupDayMonday])
		assert.Equal(t, 1, len(found.PickupDays.Normalize()))

	})

	// A direct PickupDays write must keep the legacy pickup_status column in
	// sync even when the caller never sets PickupStatus — the repository is the
	// single source of truth, so filters/exports reading the legacy string can
	// never disagree with the weekday map. Covers the three distinct states.
	t.Run("syncs legacy pickup_status from a direct weekday write", func(t *testing.T) {
		requireStudentsPickupDaysColumn(t, db)

		t.Run("selected days derive picked-up", func(t *testing.T) {
			studentFixture := testpkg.CreateTestStudent(t, db, "Pickup", "SyncPicked", "4a")
			student, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(studentFixture.ID))
			require.NoError(t, err)

			// Set only the map, leave PickupStatus nil (the stale-write case).
			student.PickupStatus = nil
			student.PickupDays = peopleValues.PickupDays{peopleValues.PickupDayFriday: true}
			require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Update, student))

			found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(student.ID))
			require.NoError(t, err)
			require.NotNil(t, found.PickupStatus)
			assert.Equal(t, userModels.PickupStatusPickedUp, *found.PickupStatus)
		})

		t.Run("explicit empty map derives goes-alone", func(t *testing.T) {
			studentFixture := testpkg.CreateTestStudent(t, db, "Pickup", "SyncAlone", "4b")
			student, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(studentFixture.ID))
			require.NoError(t, err)

			// Seed a picked-up student, then clear the map to the empty answer.
			picked := userModels.PickupStatusPickedUp
			student.PickupStatus = &picked
			student.PickupDays = peopleValues.PickupDays{peopleValues.PickupDayMonday: true}
			require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Update, student))

			student.PickupStatus = nil
			student.PickupDays = peopleValues.PickupDays{}
			require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Update, student))

			found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(student.ID))
			require.NoError(t, err)
			require.NotNil(t, found.PickupStatus)
			assert.Equal(t, userModels.PickupStatusGoesAlone, *found.PickupStatus)
			assert.False(t, found.PickupDays.HasAny())
		})
	})

	t.Run("rejects an invalid weekday before persistence", func(t *testing.T) {
		person := testpkg.CreateTestPerson(t, db, "Pickup", "Invalid")

		student := &peopleValues.StudentState{
			PersonID:    person.ID,
			SchoolClass: "1a",
			PickupDays:  peopleValues.PickupDays{"sat": true},
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, student)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `weekday "sat"`)
		assert.Zero(t, student.ID)
	})
}
