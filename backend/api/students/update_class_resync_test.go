package students_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// recordingOfferingResyncer captures ResyncOfferingSourcedTemplates calls. The
// resync's roster effects are covered by the enrollment-side service tests;
// here only the handler contract matters: WHEN the hook fires.
type recordingOfferingResyncer struct {
	calls []timezone.Date
}

func (r *recordingOfferingResyncer) ResyncOfferingSourcedTemplates(_ context.Context, effectiveFrom timezone.Date) error {
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
	tc := setupTestContext(t)
	rec := &recordingOfferingResyncer{}
	tc.resource.OfferingSourceResyncer = rec
	tc.resource.LockTemplateRecurrence = func(ctx context.Context) error {
		return scheduleSvc.LockTenantRecurrenceWrites(ctx, tc.db)
	}

	student := testpkg.CreateTestStudent(t, tc.db, "KlassenResync", "Kind", "2a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	putStudent(t, tc, student.ID, map[string]any{"school_class": "3a"})
	require.Len(t, rec.calls, 1,
		"a class change must resync the offering-sourced templates in the same request")
	assert.Equal(t, timezone.TodayDate(), rec.calls[0],
		"history stays untouched: the resync boundary is today, like a grade transition")

	putStudent(t, tc, student.ID, map[string]any{"school_class": "3a"})
	assert.Len(t, rec.calls, 1,
		"resubmitting the unchanged class must not resync")

	putStudent(t, tc, student.ID, map[string]any{"notes": "kein Klassenwechsel"})
	assert.Len(t, rec.calls, 1,
		"an update without school_class must not resync")
}
