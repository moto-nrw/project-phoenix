package postgres

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestMain(m *testing.M) { testpkg.PerTestTenants(); testpkg.Run(m) }

func TestListPickupNotesIncludesRecurringNotesWithinDateRange(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	store := New(func(context.Context) (bun.IDB, int64, error) {
		return db, testpkg.Tenant(t), nil
	})
	student := testpkg.CreateTestStudent(t, db, "Wiederkehrend", "Notiz", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Notiz", "Erstellt")
	ctx := testpkg.Ctx(t)

	_, _, err := store.CreatePickupNote(ctx, careplan.PickupNote{
		StudentID: student.ID,
		Weekday:   1,
		Content:   "Montags nicht erwartet",
		CreatedBy: staff.ID,
	})
	require.NoError(t, err)
	_, _, err = store.CreatePickupNote(ctx, careplan.PickupNote{
		StudentID: student.ID,
		Weekday:   3,
		Content:   "Mittwochs nicht erwartet",
		CreatedBy: staff.ID,
	})
	require.NoError(t, err)
	_, _, err = store.CreatePickupNote(ctx, careplan.PickupNote{
		StudentID: student.ID,
		NoteDate:  careplan.Date("2026-01-31"), // Saturday
		Content:   "Nur am Samstag",
		CreatedBy: staff.ID,
	})
	require.NoError(t, err)

	notes, _, err := store.ListPickupNotes(ctx, careplan.StudentScheduleFilter{
		StudentIDs: []int64{student.ID},
		From:       careplan.Date("2026-01-26"), // Monday
		To:         careplan.Date("2026-01-27"), // Tuesday
	})
	require.NoError(t, err)
	require.Len(t, notes, 1)
	assert.Equal(t, "Montags nicht erwartet", notes[0].Content)

	weekendNotes, _, err := store.ListPickupNotes(ctx, careplan.StudentScheduleFilter{
		StudentIDs: []int64{student.ID},
		From:       careplan.Date("2026-01-31"), // Saturday
		To:         careplan.Date("2026-02-01"), // Sunday
	})
	require.NoError(t, err)
	require.Len(t, weekendNotes, 1)
	assert.Equal(t, "Nur am Samstag", weekendNotes[0].Content)
}
