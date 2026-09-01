package audit

import (
	"context"
	"testing"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type auditTestStaffShift struct {
	ID        int64            `bun:"id,pk,autoincrement"`
	TenantID  int64            `bun:"tenant_id,notnull"`
	StaffID   int64            `bun:"staff_id,notnull"`
	Date      auditModels.Date `bun:"date,notnull,type:date"`
	StartTime time.Time        `bun:"start_time,notnull"`
	EndTime   time.Time        `bun:"end_time,notnull"`
	CreatedBy int64            `bun:"created_by,notnull"`
}

func newDeviationRepository(runtime Runtime) auditModels.DeviationEventRepository {
	return NewDeviationEventRepository(runtime)
}

func createTestDeviationEvent(t *testing.T, repo auditModels.DeviationEventRepository, scope testpkg.TenantScope, activityGroupID int64, date auditModels.Date, startHHMM string, eventType string) *auditModels.DeviationEvent {
	t.Helper()

	parsed, err := time.Parse("15:04", startHHMM)
	require.NoError(t, err)
	start := time.Date(1, time.January, 1, parsed.Hour(), parsed.Minute(), 0, 0, time.UTC)

	event := &auditModels.DeviationEvent{
		ActivityGroupID: &activityGroupID,
		OccurrenceDate:  date,
		StartTime:       start,
		EventType:       eventType,
	}
	require.NoError(t, repo.Create(scope.Context(), event))
	return event
}

func TestDeviationEventRepository_ListByRangeFiltersSlotAndRange(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	scope := testpkg.NewTenantScope(t, db)
	group := testpkg.CreateTestActivityGroupForTenant(t, db, scope.TenantID, "Protokoll-Gruppe")
	otherGroup := testpkg.CreateTestActivityGroupForTenant(t, db, scope.TenantID, "Andere Gruppe")
	repo := newDeviationRepository(NewRuntime(db, func(context.Context) int64 { return scope.TenantID }))

	day := auditModels.NewDate(2026, time.September, 22)
	inRange := createTestDeviationEvent(t, repo, scope, group.ID, day, "14:00", auditModels.DeviationEventAbsence)
	createTestDeviationEvent(t, repo, scope, group.ID, day, "15:30", auditModels.DeviationEventSubstitution)
	createTestDeviationEvent(t, repo, scope, otherGroup.ID, day, "14:00", auditModels.DeviationEventCancellation)
	createTestDeviationEvent(t, repo, scope, group.ID, day.AddDays(10), "14:00", auditModels.DeviationEventAbsence)

	all, err := repo.ListByRange(scope.Context(), auditModels.Date(day), auditModels.Date(day.AddDays(4)), nil, nil)
	require.NoError(t, err)
	require.Len(t, all, 3)

	startTime := "14:00"
	slot, err := repo.ListByRange(scope.Context(), auditModels.Date(day), auditModels.Date(day), &group.ID, &startTime)
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

	day := auditModels.NewDate(2026, time.September, 22)
	repoA := newDeviationRepository(NewRuntime(db, func(context.Context) int64 { return scopeA.TenantID }))
	createTestDeviationEvent(t, repoA, scopeA, groupA.ID, day, "14:00", auditModels.DeviationEventAbsence)

	repo := newDeviationRepository(NewRuntime(db, func(context.Context) int64 { return scopeB.TenantID }))
	rows, err := repo.ListByRange(scopeB.Context(), auditModels.Date(day), auditModels.Date(day), nil, nil)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestDeviationEventRepository_DeleteOlderThan(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	scope := testpkg.NewTenantScope(t, db)
	group := testpkg.CreateTestActivityGroupForTenant(t, db, scope.TenantID, "Retention-Gruppe")
	repo := newDeviationRepository(NewRuntime(db, func(context.Context) int64 { return scope.TenantID }))

	cutoff := auditModels.NewDate(2026, time.September, 22)
	createTestDeviationEvent(t, repo, scope, group.ID, cutoff.AddDays(-30), "14:00", auditModels.DeviationEventAbsence)
	recent := createTestDeviationEvent(t, repo, scope, group.ID, cutoff.AddDays(5), "14:00", auditModels.DeviationEventAbsence)

	deleted, err := repo.DeleteOlderThan(scope.Context(), auditModels.Date(cutoff))
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)

	rows, err := repo.ListByRange(scope.Context(), auditModels.Date(cutoff.AddDays(-60)), auditModels.Date(cutoff.AddDays(60)), nil, nil)
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
	repo := newDeviationRepository(NewRuntime(db, func(context.Context) int64 { return scope.TenantID }))

	day := auditModels.NewDate(2026, time.September, 22)
	slotEvent := createTestDeviationEvent(t, repo, scope, group.ID, day, "14:00", auditModels.DeviationEventAbsence)

	shift := &auditTestStaffShift{
		TenantID:  scope.TenantID,
		StaffID:   staff.ID,
		Date:      day,
		StartTime: time.Date(1, 1, 1, 8, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC),
		CreatedBy: staff.ID,
	}
	_, err := db.NewInsert().Model(shift).ModelTableExpr(`schedule.staff_shifts`).Exec(scope.Context())
	require.NoError(t, err)

	shiftEvent := &auditModels.DeviationEvent{
		OccurrenceDate: auditModels.Date(day),
		StartTime:      shift.StartTime,
		StaffShiftID:   &shift.ID,
		EventType:      auditModels.DeviationEventShiftMoved,
	}
	require.NoError(t, repo.Create(scope.Context(), shiftEvent))

	rows, err := repo.ListByRange(scope.Context(), auditModels.Date(day), auditModels.Date(day), nil, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1, "only the slot-anchored event surfaces")
	require.Equal(t, slotEvent.ID, rows[0].ID)

	// Deleting the shift clears the FK anchor (ON DELETE SET NULL); the
	// event-type exclusion must still keep the row out of slot history.
	_, err = db.NewDelete().
		Model((*auditTestStaffShift)(nil)).
		ModelTableExpr(`schedule.staff_shifts AS "staff_shift"`).
		Where(`"staff_shift".id = ?`, shift.ID).
		Exec(scope.Context())
	require.NoError(t, err)

	rows, err = repo.ListByRange(scope.Context(), auditModels.Date(day), auditModels.Date(day), nil, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1, "a deleted shift must not leak its event into slot history")
	require.Equal(t, slotEvent.ID, rows[0].ID)
}
