package careplanhttp_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingOfferingResyncer captures ResyncOfferingSourcedTemplates calls. The
// resync's roster effects are covered by the enrollment-side service tests;
// here only the handler contract matters: WHEN the hook fires.
type recordingOfferingResyncer struct {
	calls []calendar.Date
}

func (r *recordingOfferingResyncer) ResyncOfferingSourcedTemplates(_ context.Context, effectiveFrom calendar.Date) error {
	r.calls = append(r.calls, effectiveFrom)
	return nil
}

// TestUpdateStudent_ClassChangeResyncsOfferingSourcedTemplates pins the #2147
// round-10 finding: a direct school_class edit via PUT /students/{id} must
// re-reconcile the Jahrgang-filtered offering-sourced Regeltermine inside the
// same transaction — exactly like a grade transition — instead of leaving
// their rosters stale until an unrelated template save. The real recurrence
// lock runs too, so the ordering contract (gate before row locks, inside the
// tenant transaction) is exercised, not just stubbed.
func TestUpdateStudent_ClassChangeResyncsOfferingSourcedTemplates(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t, fixedCalendarClock)
	rec := &recordingOfferingResyncer{}
	tc.resource.OfferingSourceResyncer = rec
	recurrenceLock, err := api.NewTimetableHTTPTestRecurrenceLock(testpkg.NewDatabase(t, tc.db))
	require.NoError(t, err)
	tc.resource.LockTemplateRecurrence = recurrenceLock.LockRecurrenceWrites

	student := testpkg.CreateTestStudent(t, tc.db, "KlassenResync", "Kind", "2a")

	putStudent(t, tc, student.ID, map[string]any{"school_class": "3a"})
	require.Len(t, rec.calls, 1,
		"a class change must resync the offering-sourced templates in the same request")
	assert.Equal(t, calendar.NewDate(2026, 8, 24), rec.calls[0],
		"history stays untouched: the resync boundary is today, like a grade transition")

	putStudent(t, tc, student.ID, map[string]any{"school_class": "3a"})
	assert.Len(t, rec.calls, 1,
		"resubmitting the unchanged class must not resync")

	putStudent(t, tc, student.ID, map[string]any{"notes": "kein Klassenwechsel"})
	assert.Len(t, rec.calls, 1,
		"an update without school_class must not resync")
}
