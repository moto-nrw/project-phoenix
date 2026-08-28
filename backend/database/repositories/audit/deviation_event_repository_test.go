package audit_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func createTestDeviationEvent(t *testing.T, db *bun.DB, scope testpkg.TenantScope, activityGroupID int64, date timezone.Date, startHHMM string, eventType string) *auditModels.DeviationEvent {
	t.Helper()

	parsed, err := time.Parse("15:04", startHHMM)
	require.NoError(t, err)
	start := timezone.NormalizeWallClock(parsed)

	event := &auditModels.DeviationEvent{
		ActivityGroupID: &activityGroupID,
		OccurrenceDate:  date,
		StartTime:       start,
		EventType:       eventType,
	}
	repo := repositories.NewFactory(db).DeviationEvent
	require.NoError(t, repo.Create(scope.Context(), event))
	t.Cleanup(func() {
		_, _ = db.NewDelete().
			Model((*auditModels.DeviationEvent)(nil)).
			ModelTableExpr(`audit.deviation_events AS "deviation_event"`).
			Where(`"deviation_event".id = ?`, event.ID).
			Exec(scope.Context())
	})
	return event
}

func TestDeviationEventRepository_ListByRangeFiltersSlotAndRange(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	scope := testpkg.NewTenantScope(t, db)
	group := testpkg.CreateTestActivityGroupForTenant(t, db, scope.TenantID, "Protokoll-Gruppe")
	otherGroup := testpkg.CreateTestActivityGroupForTenant(t, db, scope.TenantID, "Andere Gruppe")

	day := timezone.NewDate(2026, time.September, 22)
	inRange := createTestDeviationEvent(t, db, scope, group.ID, day, "14:00", auditModels.DeviationEventAbsence)
	createTestDeviationEvent(t, db, scope, group.ID, day, "15:30", auditModels.DeviationEventSubstitution)
	createTestDeviationEvent(t, db, scope, otherGroup.ID, day, "14:00", auditModels.DeviationEventCancellation)
	createTestDeviationEvent(t, db, scope, group.ID, day.AddDays(10), "14:00", auditModels.DeviationEventAbsence)

	repo := repositories.NewFactory(db).DeviationEvent

	all, err := repo.ListByRange(scope.Context(), day, day.AddDays(4), nil, nil)
	require.NoError(t, err)
	require.Len(t, all, 3)

	startTime := "14:00"
	slot, err := repo.ListByRange(scope.Context(), day, day, &group.ID, &startTime)
	require.NoError(t, err)
	require.Len(t, slot, 1)
	require.Equal(t, inRange.ID, slot[0].ID)

}

func TestDeviationEventRepository_TenantIsolation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	scopeA := testpkg.NewTenantScope(t, db)
	scopeB := testpkg.NewTenantScope(t, db)
	groupA := testpkg.CreateTestActivityGroupForTenant(t, db, scopeA.TenantID, "Tenant-A-Gruppe")

	day := timezone.NewDate(2026, time.September, 22)
	createTestDeviationEvent(t, db, scopeA, groupA.ID, day, "14:00", auditModels.DeviationEventAbsence)

	repo := repositories.NewFactory(db).DeviationEvent
	rows, err := repo.ListByRange(scopeB.Context(), day, day, nil, nil)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestDeviationEventRepository_DeleteOlderThan(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	scope := testpkg.NewTenantScope(t, db)
	group := testpkg.CreateTestActivityGroupForTenant(t, db, scope.TenantID, "Retention-Gruppe")

	cutoff := timezone.NewDate(2026, time.March, 1)
	createTestDeviationEvent(t, db, scope, group.ID, cutoff.AddDays(-30), "14:00", auditModels.DeviationEventAbsence)
	recent := createTestDeviationEvent(t, db, scope, group.ID, cutoff.AddDays(5), "14:00", auditModels.DeviationEventAbsence)

	repo := repositories.NewFactory(db).DeviationEvent
	deleted, err := repo.DeleteOlderThan(scope.Context(), cutoff)
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)

	rows, err := repo.ListByRange(scope.Context(), cutoff.AddDays(-60), cutoff.AddDays(60), nil, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, recent.ID, rows[0].ID)

}

// TestDeviationEventRepository_ListByRangeExcludesShiftAnchoredRows: #1884
// Dienstplan shift-move events must not leak into the Betreuungsplan history.
func TestDeviationEventRepository_ListByRangeExcludesShiftAnchoredRows(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	scope := testpkg.NewTenantScope(t, db)
	group := testpkg.CreateTestActivityGroupForTenant(t, db, scope.TenantID, "Protokoll-Gruppe-Shift")
	staff := testpkg.CreateTestStaffForTenant(t, db, scope.TenantID, "Schicht", "Bewegt")

	day := timezone.NewDate(2026, time.September, 22)
	slotEvent := createTestDeviationEvent(t, db, scope, group.ID, day, "14:00", auditModels.DeviationEventAbsence)

	shift := &scheduleModels.StaffShift{
		StaffID:   staff.ID,
		Date:      day,
		StartTime: timezone.NormalizeWallClock(time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC)),
		EndTime:   timezone.NormalizeWallClock(time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC)),
		CreatedBy: staff.ID,
	}
	shift.SetTenantID(scope.TenantID)
	_, err := db.NewInsert().Model(shift).ModelTableExpr(`schedule.staff_shifts`).Exec(scope.Context())
	require.NoError(t, err)

	repo := repositories.NewFactory(db).DeviationEvent
	shiftEvent := &auditModels.DeviationEvent{
		OccurrenceDate: day,
		StartTime:      shift.StartTime,
		StaffShiftID:   &shift.ID,
		EventType:      auditModels.DeviationEventShiftMoved,
	}
	require.NoError(t, repo.Create(scope.Context(), shiftEvent))

	rows, err := repo.ListByRange(scope.Context(), day, day, nil, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1, "only the slot-anchored event surfaces")
	require.Equal(t, slotEvent.ID, rows[0].ID)

	// Deleting the shift clears the FK anchor (ON DELETE SET NULL); the
	// event-type exclusion must still keep the row out of slot history.
	_, err = db.NewDelete().
		Model((*scheduleModels.StaffShift)(nil)).
		ModelTableExpr(`schedule.staff_shifts AS "staff_shift"`).
		Where(`"staff_shift".id = ?`, shift.ID).
		Exec(scope.Context())
	require.NoError(t, err)

	rows, err = repo.ListByRange(scope.Context(), day, day, nil, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1, "a deleted shift must not leak its event into slot history")
	require.Equal(t, slotEvent.ID, rows[0].ID)
}
