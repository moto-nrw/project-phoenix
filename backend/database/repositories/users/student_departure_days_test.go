package users_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	repousers "github.com/moto-nrw/project-phoenix/database/repositories/users"
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

func requireStudentsAllowedDepartureModesColumn(t *testing.T, db *bun.DB) {
	t.Helper()
	var exists bool
	err := db.NewRaw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'users'
			  AND table_name = 'students'
			  AND column_name = 'allowed_departure_modes'
		)
	`).Scan(testpkg.TenantContext(1), &exists)
	require.NoError(t, err)
	if !exists {
		t.Skip("users.students.allowed_departure_modes column is not present in this test database")
	}
}

// TestStudentRepository_DepartureDaysRoundtrip exercises the unified
// departure_days source of truth (#1610): persisting a unified plan derives the
// legacy bus_days/pickup_days/pickup_status mirrors, hydration reads the plan
// back, and a unified replacement fully overwrites the legacy maps.
func TestStudentRepository_DepartureDaysRoundtrip(t *testing.T) {
	db := testpkg.SetupTestDB(t)

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
		requireStudentsAllowedDepartureModesColumn(t, db)

		student := testpkg.CreateTestStudent(t, db, "Departure", "Fold", "2a")
		defer cleanupStudentRecords(t, db, student.ID)

		// A legacy state can still say bus AND pickup on Monday. The exclusive
		// departure_days projection uses pickup, but allowed_departure_modes
		// must preserve both permissions.
		student.BusDays = users.BusDays{users.PickupDayMonday: true, users.PickupDayTuesday: true}
		student.PickupDays = users.PickupDays{users.PickupDayMonday: true}
		require.NoError(t, repo.Update(ctx, student))

		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		assert.Equal(t, users.DeparturePickup, found.DepartureDays.ModeFor(users.PickupDayMonday))
		assert.Equal(t, users.DepartureBus, found.DepartureDays.ModeFor(users.PickupDayTuesday))
		assert.Equal(t, users.AllowedDepartureModes{
			users.PickupDayMonday: []users.DepartureMode{
				users.DepartureBus,
				users.DeparturePickup,
			},
			users.PickupDayTuesday: []users.DepartureMode{
				users.DepartureBus,
			},
		}, found.AllowedDepartureModes)
		assert.True(t, found.BusDays[users.PickupDayMonday])
	})

	t.Run("legacy maps merge into existing allowed modes", func(t *testing.T) {
		requireStudentsDepartureDaysColumn(t, db)
		requireStudentsAllowedDepartureModesColumn(t, db)

		student := testpkg.CreateTestStudent(t, db, "Departure", "Merge", "2b")
		defer cleanupStudentRecords(t, db, student.ID)

		student.AllowedDepartureModes = users.AllowedDepartureModes{
			users.PickupDayMonday: []users.DepartureMode{
				users.DepartureAlone,
				users.DepartureBus,
			},
			users.PickupDayTuesday: []users.DepartureMode{
				users.DeparturePickup,
			},
		}
		require.NoError(t, repo.Update(ctx, student))

		fresh, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		fresh.PickupDays = users.PickupDays{
			users.PickupDayMonday:  true,
			users.PickupDayTuesday: true,
		}
		require.NoError(t, repo.Update(ctx, fresh))

		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		assert.Equal(t, users.AllowedDepartureModes{
			users.PickupDayMonday: []users.DepartureMode{
				users.DepartureAlone,
				users.DepartureBus,
				users.DeparturePickup,
			},
			users.PickupDayTuesday: []users.DepartureMode{
				users.DeparturePickup,
			},
		}, found.AllowedDepartureModes)

		found.BusDays = users.BusDays{}
		require.NoError(t, repo.Update(ctx, found))

		afterRemoval, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		assert.Equal(t, users.AllowedDepartureModes{
			users.PickupDayMonday: []users.DepartureMode{
				users.DepartureAlone,
				users.DeparturePickup,
			},
			users.PickupDayTuesday: []users.DepartureMode{
				users.DeparturePickup,
			},
		}, afterRemoval.AllowedDepartureModes)
	})

	t.Run("legacy bus change preserves existing accompanied day", func(t *testing.T) {
		requireStudentsDepartureDaysColumn(t, db)
		requireStudentsAllowedDepartureModesColumn(t, db)

		student := testpkg.CreateTestStudent(t, db, "Departure", "Accompanied", "2c")
		defer cleanupStudentRecords(t, db, student.ID)

		student.AllowedDepartureModes = users.AllowedDepartureModes{
			users.PickupDayMonday:  []users.DepartureMode{users.DepartureAccompanied},
			users.PickupDayTuesday: []users.DepartureMode{users.DepartureAlone},
		}
		// An accompanied day requires the coupled "mit wem" note (#1694); without
		// it Validate() now rejects the write. The note is incidental to this test
		// (it asserts the accompanied day survives a later legacy bus change).
		companion := "Geschwisterkind"
		student.DepartureCompanionNote = &companion
		require.NoError(t, repo.Update(ctx, student))

		fresh, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		// A legacy bus_days change must NOT drop the unrelated accompanied day
		// (the legacy merge used to rebuild only alone/bus/pickup).
		fresh.BusDays = users.BusDays{users.PickupDayTuesday: true}
		require.NoError(t, repo.Update(ctx, fresh))

		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		assert.Equal(t,
			[]users.DepartureMode{users.DepartureAccompanied},
			found.AllowedDepartureModes[users.PickupDayMonday],
			"accompanied on Monday survives an unrelated bus_days change",
		)
		assert.Contains(t, found.AllowedDepartureModes[users.PickupDayTuesday], users.DepartureBus)
	})

	t.Run("companion note is cleared when modes drop accompanied", func(t *testing.T) {
		requireStudentsDepartureDaysColumn(t, db)
		requireStudentsAllowedDepartureModesColumn(t, db)

		student := testpkg.CreateTestStudent(t, db, "Departure", "NoteClear", "2d")
		defer cleanupStudentRecords(t, db, student.ID)

		note := "Geschwisterkind Mia"
		student.AllowedDepartureModes = users.AllowedDepartureModes{
			users.PickupDayMonday: []users.DepartureMode{users.DepartureAccompanied},
		}
		student.DepartureCompanionNote = &note
		require.NoError(t, repo.Update(ctx, student))

		seeded, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		require.NotNil(t, seeded.DepartureCompanionNote, "note persists while accompanied is allowed")
		assert.Equal(t, note, *seeded.DepartureCompanionNote)

		// Switch Monday to bus via the unified field: no accompanied day remains,
		// so the "mit wem" note must not survive (#1694).
		seeded.AllowedDepartureModes = nil
		seeded.DepartureDays = users.DepartureDays{users.PickupDayMonday: users.DepartureBus}
		require.NoError(t, repo.Update(ctx, seeded))

		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		assert.Nil(t, found.DepartureCompanionNote,
			"the companion note must be cleared once no day allows accompanied")
	})

	t.Run("note-only update does not persist an orphan note without an accompanied day", func(t *testing.T) {
		requireStudentsAllowedDepartureModesColumn(t, db)

		// CreateTestStudent leaves the departure plan empty (no accompanied day),
		// so every plan field scans back nil. A stale or direct client that PUTs
		// only the "mit wem" note (omitting allowed_departure_modes/departure_days)
		// must not be able to persist it: persistDepartureDays used to short-circuit
		// on the all-nil plan and leave the orphan note in place (#1694). Loading
		// fresh mirrors the updateStudent handler, which patches a locked row.
		student := testpkg.CreateTestStudent(t, db, "Departure", "OrphanNote", "2e")
		defer cleanupStudentRecords(t, db, student.ID)

		fresh, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		note := "Geschwisterkind Mia"
		fresh.DepartureCompanionNote = &note
		require.NoError(t, repo.Update(ctx, fresh))

		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		assert.Nil(t, found.DepartureCompanionNote,
			"a companion note must not survive on a student with no accompanied day")
	})

	t.Run("accompanied-only child persists a dedicated non-self pickup_status", func(t *testing.T) {
		requireStudentsDepartureDaysColumn(t, db)
		requireStudentsAllowedDepartureModesColumn(t, db)

		// An accompanied-only child must never persist the self-goer pickup_status:
		// legacy search/list/admin-filter consumers read pickup_status and would
		// otherwise bucket a child who leaves with a companion as going home alone,
		// which is safety-sensitive state (#1694).
		student := testpkg.CreateTestStudent(t, db, "Departure", "AccompaniedStatus", "5a")
		defer cleanupStudentRecords(t, db, student.ID)

		note := "Geschwisterkind"
		student.AllowedDepartureModes = users.AllowedDepartureModes{
			users.PickupDayMonday: []users.DepartureMode{users.DepartureAccompanied},
		}
		student.DepartureCompanionNote = &note
		require.NoError(t, repo.Update(ctx, student))

		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		require.NotNil(t, found.PickupStatus)
		assert.Equal(t, users.PickupStatusAccompanied, *found.PickupStatus,
			"accompanied-only child must not be stored as a self-goer")
		assert.False(t, found.PickupDays.HasAny(), "accompanied is not a pickup day")
		assert.False(t, found.BusDays.HasAny(), "accompanied is not a bus day")
	})

	t.Run("bus and accompanied on the same weekday keeps the accompanied pickup_status", func(t *testing.T) {
		requireStudentsDepartureDaysColumn(t, db)
		requireStudentsAllowedDepartureModesColumn(t, db)

		// A day allowing BOTH bus and accompanied must still store the accompanied
		// pickup_status. The exclusive departure_days projection ranks bus above
		// accompanied, so deriving pickup_status from it would drop the accompanied
		// signal and bucket the child as a self-goer (#1694). The companion note must
		// also survive because an accompanied day remains in the plan.
		student := testpkg.CreateTestStudent(t, db, "Departure", "BusAndAccompanied", "5c")
		defer cleanupStudentRecords(t, db, student.ID)

		note := "Geschwisterkind Mia"
		student.AllowedDepartureModes = users.AllowedDepartureModes{
			users.PickupDayMonday: []users.DepartureMode{users.DepartureBus, users.DepartureAccompanied},
		}
		student.DepartureCompanionNote = &note
		require.NoError(t, repo.Update(ctx, student))

		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		require.NotNil(t, found.PickupStatus)
		assert.Equal(t, users.PickupStatusAccompanied, *found.PickupStatus,
			"bus+accompanied day must not collapse pickup_status to self-goer")
		require.NotNil(t, found.DepartureCompanionNote, "companion note survives while accompanied is allowed")
		assert.Equal(t, note, *found.DepartureCompanionNote)
		// The exclusive departure_days mirror still shows bus (operationally the staff
		// must see the bus instruction); only the coarse pickup_status flag carries the
		// accompanied signal here.
		assert.True(t, found.BusDays[users.BusDayMonday], "bus day is still recorded")
		assert.Equal(t, users.DepartureBus, found.DepartureDays.ModeFor(users.PickupDayMonday),
			"exclusive departure_days keeps the bus instruction visible to staff")
	})

	t.Run("legacy departure_days update can drop accompanied and clear its note in one write", func(t *testing.T) {
		requireStudentsDepartureDaysColumn(t, db)
		requireStudentsAllowedDepartureModesColumn(t, db)

		// Seed an accompanied child with a coupled note.
		student := testpkg.CreateTestStudent(t, db, "Departure", "LegacyDropAccompanied", "5b")
		defer cleanupStudentRecords(t, db, student.ID)

		note := "Geschwisterkind"
		student.AllowedDepartureModes = users.AllowedDepartureModes{
			users.PickupDayMonday: []users.DepartureMode{users.DepartureAccompanied},
		}
		student.DepartureCompanionNote = &note
		require.NoError(t, repo.Update(ctx, student))

		// A legacy client (one that never sends allowed_departure_modes) switches
		// Monday to a non-accompanied mode via departure_days AND clears the note in
		// the same request. The hydrated allowed_departure_modes still carries the
		// stale accompanied mode; validating against it used to reject the blank
		// note before the repository could resolve precedence (#1694). Loading fresh
		// and leaving allowed_departure_modes untouched mirrors that handler path.
		fresh, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		require.True(t, fresh.AllowedDepartureModes.HasMode(users.DepartureAccompanied),
			"precondition: hydrated plan still carries the accompanied mode")
		fresh.DepartureDays = users.DepartureDays{users.PickupDayMonday: users.DepartureBus}
		blank := ""
		fresh.DepartureCompanionNote = &blank
		require.NoError(t, repo.Update(ctx, fresh),
			"removing accompanied via departure_days while clearing the note must be accepted")

		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		assert.Nil(t, found.DepartureCompanionNote, "note is cleared with the accompanied mode")
		assert.Equal(t, users.DepartureBus, found.DepartureDays.ModeFor(users.PickupDayMonday))
		assert.False(t, found.AllowedDepartureModes.HasMode(users.DepartureAccompanied),
			"accompanied must not survive the legacy departure_days replacement")
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

	t.Run("hydrated unified update replaces stale legacy mirrors", func(t *testing.T) {
		requireStudentsDepartureDaysColumn(t, db)

		student := testpkg.CreateTestStudent(t, db, "Departure", "HydratedReplace", "4a")
		defer cleanupStudentRecords(t, db, student.ID)

		student.DepartureDays = users.DepartureDays{users.PickupDayMonday: users.DepartureBus}
		require.NoError(t, repo.Update(ctx, student))

		fresh, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		require.True(t, fresh.BusDays[users.PickupDayMonday])

		fresh.DepartureDays = users.DepartureDays{users.PickupDayThursday: users.DeparturePickup}
		require.NoError(t, repo.Update(ctx, fresh))

		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		assert.Equal(t, users.DepartureAlone, found.DepartureDays.ModeFor(users.PickupDayMonday))
		assert.Equal(t, users.DeparturePickup, found.DepartureDays.ModeFor(users.PickupDayThursday))
		assert.False(t, found.BusDays.HasAny())
		assert.True(t, found.PickupDays[users.PickupDayThursday])
	})
}

func companionNoteColumnExists(t *testing.T, db *bun.DB) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.NewRaw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'users'
			  AND table_name = 'students'
			  AND column_name = 'departure_companion_note'
		)
	`).Scan(testpkg.TenantContext(1), &exists))
	return exists
}

// TestStudentRepository_CompanionNoteSchemaCompatibility verifies the scanonly
// departure_companion_note column (1.15.138) round-trips through the explicit
// hydration/persist paths, and that a schema MISSING the column is rejected
// loudly instead of tolerated: since #2059 the repository no longer probes
// optional columns per request — the server verifies the fully migrated schema
// once at startup (VerifyStudentSchema) and every read/write path assumes it.
// A missing mandatory column must surface as an error, never as a silent
// fallback.
func TestStudentRepository_CompanionNoteSchemaCompatibility(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Student
	ctx := testpkg.TenantContext(1)

	if !companionNoteColumnExists(t, db) {
		t.Skip("users.students.departure_companion_note column is not present in this test database")
	}

	t.Run("note round-trips through the explicit guarded write and hydration", func(t *testing.T) {
		requireStudentsAllowedDepartureModesColumn(t, db)

		student := testpkg.CreateTestStudent(t, db, "Companion", "Present", "1a")
		defer cleanupStudentRecords(t, db, student.ID)

		note := "Geschwisterkind Mia"
		student.AllowedDepartureModes = users.AllowedDepartureModes{
			users.PickupDayMonday: []users.DepartureMode{users.DepartureAccompanied},
		}
		student.DepartureCompanionNote = &note
		require.NoError(t, repo.Update(ctx, student))

		// FindByID hydrates the scanonly note (it is no longer in the generic
		// model SELECT), and the bulk FindByIDs hydration path must surface it too.
		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		require.NotNil(t, found.DepartureCompanionNote)
		assert.Equal(t, note, *found.DepartureCompanionNote)

		byIDs, err := repo.FindByIDs(ctx, []int64{student.ID})
		require.NoError(t, err)
		require.Contains(t, byIDs, student.ID)
		require.NotNil(t, byIDs[student.ID].DepartureCompanionNote)
		assert.Equal(t, note, *byIDs[student.ID].DepartureCompanionNote)
	})

	t.Run("missing column fails loudly and is rejected by the startup guard", func(t *testing.T) {
		// Create the fixture BEFORE dropping the column: the insert path itself
		// never references the scanonly note column.
		student := testpkg.CreateTestStudent(t, db, "Companion", "Absent", "1a")
		defer cleanupStudentRecords(t, db, student.ID)

		// Simulate the pre-1.15.138 / post-rollback schema by dropping just the
		// column. Restore it immediately via Cleanup so the shared DB is left
		// intact even on failure.
		_, err := db.NewRaw(`ALTER TABLE users.students DROP COLUMN IF EXISTS departure_companion_note`).Exec(ctx)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, restoreErr := db.NewRaw(`ALTER TABLE users.students ADD COLUMN IF NOT EXISTS departure_companion_note TEXT`).Exec(context.Background())
			require.NoError(t, restoreErr)
		})

		// The startup guard is what keeps a server from ever serving requests
		// against this schema — it must name the missing column.
		err = repousers.VerifyStudentSchema(context.Background(), db)
		require.Error(t, err, "VerifyStudentSchema must reject a schema without departure_companion_note")
		assert.Contains(t, err.Error(), "departure_companion_note")

		// The request paths assume the verified schema: hydration and the
		// departure-plan write must fail loudly, never silently drop data.
		_, err = repo.FindByID(ctx, student.ID)
		require.Error(t, err, "hydration must not silently skip a missing mandatory column")

		note := "Geschwisterkind"
		student.AllowedDepartureModes = users.AllowedDepartureModes{
			users.PickupDayMonday: []users.DepartureMode{users.DepartureAccompanied},
		}
		student.DepartureCompanionNote = &note
		require.Error(t, repo.Update(ctx, student),
			"the departure-plan write must not silently drop the companion note")
	})
}

// TestStudentRepository_StaleDeparturePlanIsRebased pins the concurrency rule
// behind Update's row lock: a caller that loaded the child before someone else
// changed the departure plan carries a hydrated copy of the OLD plan in all
// four fields, which is indistinguishable from an intentional plan write unless
// the hydrated baseline says otherwise. Such a caller must not revert the
// committed change; a caller that genuinely edited the plan must still win.
func TestStudentRepository_StaleDeparturePlanIsRebased(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Student
	ctx := testpkg.TenantContext(1)

	t.Run("untouched stale plan does not revert a committed change", func(t *testing.T) {
		requireStudentsDepartureDaysColumn(t, db)
		requireStudentsAllowedDepartureModesColumn(t, db)

		student := testpkg.CreateTestStudent(t, db, "Departure", "Stale", "3a")
		defer cleanupStudentRecords(t, db, student.ID)

		student.DepartureDays = users.DepartureDays{users.PickupDayMonday: users.DepartureBus}
		require.NoError(t, repo.Update(ctx, student))

		// The unrelated caller (sickness auto-clear, status days, import) loads
		// the child, hydrating the plan it will never touch.
		stale, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)

		// Someone else changes the plan and commits.
		fresh, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		fresh.DepartureDays = users.DepartureDays{users.PickupDayTuesday: users.DeparturePickup}
		require.NoError(t, repo.Update(ctx, fresh))

		// The stale caller now writes its unrelated field.
		sick := true
		stale.Sick = &sick
		require.NoError(t, repo.Update(ctx, stale))

		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		assert.Equal(t, users.DepartureAlone, found.DepartureDays.ModeFor(users.PickupDayMonday),
			"the stale hydrated plan must not be re-persisted")
		assert.Equal(t, users.DeparturePickup, found.DepartureDays.ModeFor(users.PickupDayTuesday),
			"the committed change must survive")
		require.NotNil(t, found.Sick)
		assert.True(t, *found.Sick, "the unrelated field must still be written")
	})

	t.Run("intentional plan change still wins", func(t *testing.T) {
		requireStudentsDepartureDaysColumn(t, db)
		requireStudentsAllowedDepartureModesColumn(t, db)

		student := testpkg.CreateTestStudent(t, db, "Departure", "Intentional", "3b")
		defer cleanupStudentRecords(t, db, student.ID)

		student.DepartureDays = users.DepartureDays{users.PickupDayMonday: users.DepartureBus}
		require.NoError(t, repo.Update(ctx, student))

		loaded, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		loaded.DepartureDays = users.DepartureDays{users.PickupDayFriday: users.DeparturePickup}
		loaded.AllowedDepartureModes = nil
		loaded.BusDays = nil
		loaded.PickupDays = nil
		require.NoError(t, repo.Update(ctx, loaded))

		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		assert.Equal(t, users.DeparturePickup, found.DepartureDays.ModeFor(users.PickupDayFriday))
		assert.Equal(t, users.DepartureAlone, found.DepartureDays.ModeFor(users.PickupDayMonday))
	})
}
