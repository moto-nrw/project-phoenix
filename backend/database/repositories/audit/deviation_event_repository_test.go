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
	start := timezone.WallClock(parsed)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	scope := testpkg.NewTenantScope(t, db)
	group := testpkg.CreateTestActivityGroupForTenant(t, db, scope.TenantID, "Protokoll-Gruppe")
	otherGroup := testpkg.CreateTestActivityGroupForTenant(t, db, scope.TenantID, "Andere Gruppe")
	defer testpkg.CleanupActivityFixtures(t, db, group.ID, otherGroup.ID)

	day := timezone.NewDate(2026, time.September, 22)
	inRange := createTestDeviationEvent(t, db, scope, group.ID, day, "14:00", auditModels.DeviationEventAbsence)
	sameDayOtherSlot := createTestDeviationEvent(t, db, scope, group.ID, day, "15:30", auditModels.DeviationEventSubstitution)
	otherGroupEvent := createTestDeviationEvent(t, db, scope, otherGroup.ID, day, "14:00", auditModels.DeviationEventCancellation)
	outOfRange := createTestDeviationEvent(t, db, scope, group.ID, day.AddDays(10), "14:00", auditModels.DeviationEventAbsence)

	repo := repositories.NewFactory(db).DeviationEvent

	all, err := repo.ListByRange(scope.Context(), day, day.AddDays(4), nil, nil)
	require.NoError(t, err)
	require.Len(t, all, 3)

	startTime := "14:00"
	slot, err := repo.ListByRange(scope.Context(), day, day, &group.ID, &startTime)
	require.NoError(t, err)
	require.Len(t, slot, 1)
	require.Equal(t, inRange.ID, slot[0].ID)

	_ = sameDayOtherSlot
	_ = otherGroupEvent
	_ = outOfRange
}

func TestDeviationEventRepository_TenantIsolation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	scopeA := testpkg.NewTenantScope(t, db)
	scopeB := testpkg.NewTenantScope(t, db)
	groupA := testpkg.CreateTestActivityGroupForTenant(t, db, scopeA.TenantID, "Tenant-A-Gruppe")
	defer testpkg.CleanupActivityFixtures(t, db, groupA.ID)

	day := timezone.NewDate(2026, time.September, 22)
	createTestDeviationEvent(t, db, scopeA, groupA.ID, day, "14:00", auditModels.DeviationEventAbsence)

	repo := repositories.NewFactory(db).DeviationEvent
	rows, err := repo.ListByRange(scopeB.Context(), day, day, nil, nil)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestDeviationEventRepository_DeleteOlderThan(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	scope := testpkg.NewTenantScope(t, db)
	group := testpkg.CreateTestActivityGroupForTenant(t, db, scope.TenantID, "Retention-Gruppe")
	defer testpkg.CleanupActivityFixtures(t, db, group.ID)

	cutoff := timezone.NewDate(2026, time.March, 1)
	old := createTestDeviationEvent(t, db, scope, group.ID, cutoff.AddDays(-30), "14:00", auditModels.DeviationEventAbsence)
	recent := createTestDeviationEvent(t, db, scope, group.ID, cutoff.AddDays(5), "14:00", auditModels.DeviationEventAbsence)

	repo := repositories.NewFactory(db).DeviationEvent
	deleted, err := repo.DeleteOlderThan(scope.Context(), cutoff)
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)

	rows, err := repo.ListByRange(scope.Context(), cutoff.AddDays(-60), cutoff.AddDays(60), nil, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, recent.ID, rows[0].ID)

	_ = old
}

// TestDeviationEventRepository_ListByRangeExcludesShiftAnchoredRows: #1884
// Dienstplan shift-move events must not leak into the Betreuungsplan history.
func TestDeviationEventRepository_ListByRangeExcludesShiftAnchoredRows(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	scope := testpkg.NewTenantScope(t, db)
	group := testpkg.CreateTestActivityGroupForTenant(t, db, scope.TenantID, "Protokoll-Gruppe-Shift")
	defer testpkg.CleanupActivityFixtures(t, db, group.ID)
	staff := testpkg.CreateTestStaffForTenant(t, db, scope.TenantID, "Schicht", "Bewegt")
	defer testpkg.CleanupActivityFixturesForTenant(t, db, scope.TenantID, staff.ID)

	day := timezone.NewDate(2026, time.September, 22)
	slotEvent := createTestDeviationEvent(t, db, scope, group.ID, day, "14:00", auditModels.DeviationEventAbsence)

	shift := &scheduleModels.StaffShift{
		StaffID:   staff.ID,
		Date:      day,
		StartTime: timezone.WallClock(time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC)),
		EndTime:   timezone.WallClock(time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC)),
		CreatedBy: staff.ID,
	}
	shift.SetTenantID(scope.TenantID)
	_, err := db.NewInsert().Model(shift).ModelTableExpr(`schedule.staff_shifts`).Exec(scope.Context())
	require.NoError(t, err)
	defer testpkg.CleanupTableRecords(t, db, "schedule.staff_shifts", shift.ID)

	repo := repositories.NewFactory(db).DeviationEvent
	shiftEvent := &auditModels.DeviationEvent{
		OccurrenceDate: day,
		StartTime:      shift.StartTime,
		StaffShiftID:   &shift.ID,
		EventType:      auditModels.DeviationEventShiftMoved,
	}
	require.NoError(t, repo.Create(scope.Context(), shiftEvent))
	defer testpkg.CleanupTableRecords(t, db, "audit.deviation_events", shiftEvent.ID)

	rows, err := repo.ListByRange(scope.Context(), day, day, nil, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1, "only the slot-anchored event surfaces")
	require.Equal(t, slotEvent.ID, rows[0].ID)
}
