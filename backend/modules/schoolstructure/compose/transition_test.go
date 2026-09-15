package compose

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The transition capability is the School Structure half of a grade
// transition (#2711): the draft with its mappings, the per-child history the
// apply writes, and the two ledgers the revert replays. These tests drive it
// through the composed module, the way the workflow does.

// withTenant runs fn inside a tenant transaction of the test's own tenant,
// which is what every transition command sees in production.
func withTenant(t *testing.T, db *bun.DB, fn func(ctx context.Context) error) {
	t.Helper()
	err := testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		return fn(ctx)
	})
	require.NoError(t, err)
}

// draftWith stores a draft with the given mappings and returns it.
func draftWith(t *testing.T, ctx context.Context, module *schoolstructure.Module, academicYear string, createdBy int64, mappings ...schoolstructure.TransitionMappingInput) schoolstructure.Transition {
	t.Helper()
	transition, err := module.CreateTransition(ctx, schoolstructure.TransitionDraft{
		AcademicYear: academicYear, CreatedBy: createdBy, Mappings: mappings,
	})
	require.NoError(t, err)
	return transition
}

func promoteTo(from, to string) schoolstructure.TransitionMappingInput {
	return schoolstructure.TransitionMappingInput{FromClass: from, ToClass: &to}
}

func graduates(from string) schoolstructure.TransitionMappingInput {
	return schoolstructure.TransitionMappingInput{FromClass: from}
}

func TestCreateTransitionStoresTheDraftWithItsMappings(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	account := testpkg.CreateTestAccount(t, db, "transition-create@example.com")
	notes := "Rollover notes"

	withTenant(t, db, func(ctx context.Context) error {
		created, err := module.CreateTransition(ctx, schoolstructure.TransitionDraft{
			AcademicYear: " 2025-2026 ", Notes: &notes, CreatedBy: account.ID,
			Mappings: []schoolstructure.TransitionMappingInput{promoteTo(" 1b ", " 2b "), graduates("4a"), {FromClass: "3a", ToClass: testpkg.StrPtr("  ")}},
		})
		require.NoError(t, err)
		assert.NotZero(t, created.ID)
		assert.Equal(t, "2025-2026", created.AcademicYear, "the academic year is trimmed")
		assert.Equal(t, schoolstructure.TransitionStatusDraft, created.Status)
		assert.Equal(t, testpkg.Tenant(t), created.TenantID)
		assert.Equal(t, account.ID, created.CreatedBy)
		require.NotNil(t, created.Notes)
		assert.Equal(t, notes, *created.Notes)
		assert.True(t, created.IsDraft())
		assert.True(t, created.CanApply(), "a draft with mappings can be applied")

		found, err := module.FindTransition(ctx, created.ID)
		require.NoError(t, err)
		require.Len(t, found.Mappings, 3)
		assert.Equal(t, []string{"1b", "3a", "4a"}, mappingSources(found.Mappings), "mappings come back ordered by from_class")
		assert.Equal(t, "2b", *mappingByFrom(t, found.Mappings, "1b").ToClass, "the target is trimmed")
		assert.True(t, mappingByFrom(t, found.Mappings, "4a").IsGraduating())
		assert.True(t, mappingByFrom(t, found.Mappings, "3a").IsGraduating(), "a blank target graduates the class")
		assert.Equal(t, testpkg.Tenant(t), found.Mappings[0].TenantID)
		assert.Equal(t, created.ID, found.Mappings[0].TransitionID)
		return nil
	})
}

func mappingSources(mappings []schoolstructure.TransitionMapping) []string {
	sources := make([]string, 0, len(mappings))
	for _, mapping := range mappings {
		sources = append(sources, mapping.FromClass)
	}
	return sources
}

func mappingByFrom(t *testing.T, mappings []schoolstructure.TransitionMapping, from string) schoolstructure.TransitionMapping {
	t.Helper()
	for _, mapping := range mappings {
		if mapping.FromClass == from {
			return mapping
		}
	}
	require.FailNowf(t, "mapping not found", "no mapping from %q", from)
	return schoolstructure.TransitionMapping{}
}

// A malformed draft is rejected before anything is written; every reason
// classifies as ErrInvalidTransition so the workflow can map it to a 400.
func TestCreateTransitionRejectsInvalidDrafts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	account := testpkg.CreateTestAccount(t, db, "transition-invalid@example.com")

	withTenant(t, db, func(ctx context.Context) error {
		cases := map[string]schoolstructure.TransitionDraft{
			"missing academic year":   {AcademicYear: "   ", CreatedBy: account.ID},
			"malformed academic year": {AcademicYear: "2025/2026", CreatedBy: account.ID},
			"missing created_by":      {AcademicYear: "2025-2026"},
			"empty from class": {AcademicYear: "2025-2026", CreatedBy: account.ID,
				Mappings: []schoolstructure.TransitionMappingInput{{FromClass: "   ", ToClass: testpkg.StrPtr("2a")}}},
			"from equals to": {AcademicYear: "2025-2026", CreatedBy: account.ID,
				Mappings: []schoolstructure.TransitionMappingInput{promoteTo("1a", "1a")}},
			"duplicate from class": {AcademicYear: "2025-2026", CreatedBy: account.ID,
				Mappings: []schoolstructure.TransitionMappingInput{promoteTo("1a", "2a"), promoteTo(" 1a ", "2b")}},
		}
		for name, draft := range cases {
			_, err := module.CreateTransition(ctx, draft)
			require.ErrorIsf(t, err, schoolstructure.ErrInvalidTransition, "%s must be rejected", name)
		}

		listed, total, err := module.ListTransitions(ctx, schoolstructure.TransitionFilter{})
		require.NoError(t, err)
		assert.Zero(t, total, "no rejected draft was written")
		assert.Empty(t, listed)
		return nil
	})
}

func TestFindTransitionReportsMissingRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	account := testpkg.CreateTestAccount(t, db, "transition-find@example.com")

	withTenant(t, db, func(ctx context.Context) error {
		existing := draftWith(t, ctx, module, "2025-2026", account.ID)

		_, err := module.FindTransition(ctx, existing.ID+1_000_000)
		require.ErrorIs(t, err, schoolstructure.ErrTransitionNotFound)

		_, err = module.FindTransition(ctx, 0)
		require.ErrorIs(t, err, schoolstructure.ErrInvalidTransition)
		return nil
	})
}

// Listing is the Übersicht's read: filters, a total independent of the page,
// newest first, and an ascending keyset window for the incremental loader.
func TestListTransitionsFiltersPaginatesAndOrders(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	account := testpkg.CreateTestAccount(t, db, "transition-list@example.com")

	withTenant(t, db, func(ctx context.Context) error {
		empty, total, err := module.ListTransitions(ctx, schoolstructure.TransitionFilter{})
		require.NoError(t, err)
		assert.Zero(t, total)
		require.NotNil(t, empty, "an empty result is an empty slice, not nil")
		assert.Empty(t, empty)

		first := draftWith(t, ctx, module, "2024-2025", account.ID, promoteTo("1a", "2a"))
		second := draftWith(t, ctx, module, "2025-2026", account.ID)
		third := draftWith(t, ctx, module, "2025-2026", account.ID)

		all, total, err := module.ListTransitions(ctx, schoolstructure.TransitionFilter{})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		// created_at DESC with an id DESC tiebreak: the three rows share the
		// statement timestamp, so the id decides.
		assert.Equal(t, []int64{third.ID, second.ID, first.ID}, transitionIDs(all), "newest first")
		require.Len(t, all[2].Mappings, 1, "each listed transition carries its mappings")

		byYear, total, err := module.ListTransitions(ctx, schoolstructure.TransitionFilter{AcademicYear: " 2025-2026 "})
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Equal(t, []int64{third.ID, second.ID}, transitionIDs(byYear))

		byStatus, total, err := module.ListTransitions(ctx, schoolstructure.TransitionFilter{Status: schoolstructure.TransitionStatusApplied})
		require.NoError(t, err)
		assert.Zero(t, total)
		assert.Empty(t, byStatus)

		page, total, err := module.ListTransitions(ctx, schoolstructure.TransitionFilter{Limit: 1, Offset: 1})
		require.NoError(t, err)
		assert.Equal(t, 3, total, "the total counts every match, not the page")
		assert.Equal(t, []int64{second.ID}, transitionIDs(page))

		window, total, err := module.ListTransitions(ctx, schoolstructure.TransitionFilter{AfterID: first.ID})
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Equal(t, []int64{second.ID, third.ID}, transitionIDs(window), "the keyset window runs ascending")

		_, _, err = module.ListTransitions(ctx, schoolstructure.TransitionFilter{Limit: -1})
		require.ErrorIs(t, err, schoolstructure.ErrInvalidTransition)
		return nil
	})
}

func transitionIDs(transitions []schoolstructure.Transition) []int64 {
	ids := make([]int64, 0, len(transitions))
	for _, transition := range transitions {
		ids = append(ids, transition.ID)
	}
	return ids
}

func TestUpdateTransitionPatchesFieldsAndReplacesMappings(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	account := testpkg.CreateTestAccount(t, db, "transition-update@example.com")

	withTenant(t, db, func(ctx context.Context) error {
		transition := draftWith(t, ctx, module, "2025-2026", account.ID, promoteTo("1a", "2a"))
		notes := "Nach Rücksprache"

		kept, err := module.UpdateTransition(ctx, schoolstructure.TransitionUpdate{
			ID: transition.ID, AcademicYear: testpkg.StrPtr("2026-2027"), Notes: &notes,
		})
		require.NoError(t, err)
		assert.Equal(t, "2026-2027", kept.AcademicYear)
		require.NotNil(t, kept.Notes)
		assert.Equal(t, notes, *kept.Notes)

		reread, err := module.FindTransition(ctx, transition.ID)
		require.NoError(t, err)
		assert.Equal(t, "2026-2027", reread.AcademicYear)
		require.Len(t, reread.Mappings, 1, "a nil mapping slice keeps the stored mappings")
		assert.Equal(t, "1a", reread.Mappings[0].FromClass)

		replaced, err := module.UpdateTransition(ctx, schoolstructure.TransitionUpdate{
			ID: transition.ID, Mappings: []schoolstructure.TransitionMappingInput{promoteTo("2a", "3a"), graduates("4a")},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"2a", "4a"}, mappingSources(replaced.Mappings))
		assert.Equal(t, "2026-2027", replaced.AcademicYear, "a patch without an academic year keeps the stored one")

		cleared, err := module.UpdateTransition(ctx, schoolstructure.TransitionUpdate{
			ID: transition.ID, Mappings: []schoolstructure.TransitionMappingInput{},
		})
		require.NoError(t, err)
		assert.Empty(t, cleared.Mappings, "an empty mapping slice clears the mappings")
		assert.False(t, cleared.CanApply(), "a draft without mappings cannot be applied")

		afterClear, err := module.FindTransition(ctx, transition.ID)
		require.NoError(t, err)
		assert.Empty(t, afterClear.Mappings)

		_, err = module.UpdateTransition(ctx, schoolstructure.TransitionUpdate{
			ID: transition.ID, AcademicYear: testpkg.StrPtr("2026/2027"),
		})
		require.ErrorIs(t, err, schoolstructure.ErrInvalidTransition)

		_, err = module.UpdateTransition(ctx, schoolstructure.TransitionUpdate{ID: transition.ID + 1_000_000})
		require.ErrorIs(t, err, schoolstructure.ErrTransitionNotFound)
		return nil
	})
}

func TestDeleteTransitionCascadesItsMappings(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	account := testpkg.CreateTestAccount(t, db, "transition-delete@example.com")

	withTenant(t, db, func(ctx context.Context) error {
		transition := draftWith(t, ctx, module, "2025-2026", account.ID, promoteTo("1a", "2a"), graduates("4a"))

		require.NoError(t, module.DeleteTransition(ctx, transition.ID))

		_, err := module.FindTransition(ctx, transition.ID)
		require.ErrorIs(t, err, schoolstructure.ErrTransitionNotFound)

		count, err := db.NewSelect().
			TableExpr("education.grade_transition_mappings").
			Where("transition_id = ?", transition.ID).
			Count(ctx)
		require.NoError(t, err)
		assert.Zero(t, count, "the mappings are deleted with their transition")

		require.ErrorIs(t, module.DeleteTransition(ctx, transition.ID), schoolstructure.ErrTransitionNotFound)
		require.ErrorIs(t, module.DeleteTransition(ctx, 0), schoolstructure.ErrInvalidTransition)
		return nil
	})
}

// The lifecycle writes are status-guarded: only a draft can be applied, only
// an applied transition can be reverted, and the latest-applied lock reads
// the newest applied row.
func TestTransitionLifecycleWritesAreStatusGuarded(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	account := testpkg.CreateTestAccount(t, db, "transition-lifecycle@example.com")
	appliedAt := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	revertedAt := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)

	withTenant(t, db, func(ctx context.Context) error {
		require.NoError(t, module.LockTransitions(ctx), "the tenant gate is re-entrant within one transaction")
		require.NoError(t, module.LockTransitions(ctx))

		_, found, err := module.LockLatestAppliedTransition(ctx)
		require.NoError(t, err)
		assert.False(t, found, "nothing is applied yet")

		first := draftWith(t, ctx, module, "2025-2026", account.ID, promoteTo("1a", "2a"))
		second := draftWith(t, ctx, module, "2026-2027", account.ID, promoteTo("2a", "3a"))

		locked, err := module.LockTransitionForMutation(ctx, first.ID)
		require.NoError(t, err)
		assert.Equal(t, first.ID, locked.ID)
		require.Len(t, locked.Mappings, 1, "the locked row carries its mappings")

		baseline := int64(4242)
		require.NoError(t, module.MarkTransitionApplied(ctx, first.ID, account.ID, appliedAt, &baseline))

		applied, err := module.FindTransition(ctx, first.ID)
		require.NoError(t, err)
		assert.Equal(t, schoolstructure.TransitionStatusApplied, applied.Status)
		assert.True(t, applied.IsApplied())
		require.NotNil(t, applied.AppliedBy)
		assert.Equal(t, account.ID, *applied.AppliedBy)
		require.NotNil(t, applied.AppliedAt)
		assert.WithinDuration(t, appliedAt, *applied.AppliedAt, time.Second)
		require.NotNil(t, applied.RosterBaselineInstanceID)
		assert.Equal(t, baseline, *applied.RosterBaselineInstanceID)

		require.ErrorIs(t, module.MarkTransitionApplied(ctx, first.ID, account.ID, appliedAt, nil),
			schoolstructure.ErrTransitionStateConflict, "an applied transition cannot be applied again")
		require.ErrorIs(t, module.MarkTransitionReverted(ctx, second.ID, account.ID, revertedAt),
			schoolstructure.ErrTransitionStateConflict, "a draft cannot be reverted")
		require.ErrorIs(t, module.MarkTransitionApplied(ctx, first.ID, 0, appliedAt, nil), schoolstructure.ErrInvalidTransition)
		require.ErrorIs(t, module.MarkTransitionReverted(ctx, first.ID, account.ID, time.Time{}), schoolstructure.ErrInvalidTransition)

		latest, found, err := module.LockLatestAppliedTransition(ctx)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, first.ID, latest.ID)

		require.NoError(t, module.MarkTransitionApplied(ctx, second.ID, account.ID, appliedAt.Add(time.Hour), nil))
		latest, found, err = module.LockLatestAppliedTransition(ctx)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, second.ID, latest.ID, "the most recently applied transition wins")

		require.NoError(t, module.MarkTransitionReverted(ctx, second.ID, account.ID, revertedAt))
		reverted, err := module.FindTransition(ctx, second.ID)
		require.NoError(t, err)
		assert.Equal(t, schoolstructure.TransitionStatusReverted, reverted.Status)
		assert.True(t, reverted.IsReverted())
		require.NotNil(t, reverted.RevertedBy)
		assert.Equal(t, account.ID, *reverted.RevertedBy)
		require.NotNil(t, reverted.RevertedAt)
		assert.WithinDuration(t, revertedAt, *reverted.RevertedAt, time.Second)

		require.ErrorIs(t, module.MarkTransitionReverted(ctx, second.ID, account.ID, revertedAt),
			schoolstructure.ErrTransitionStateConflict, "a reverted transition cannot be reverted again")

		latest, found, err = module.LockLatestAppliedTransition(ctx)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, first.ID, latest.ID, "the reverted row drops out of the latest-applied lock")
		return nil
	})
}

// The per-child history carries the snapshot the revert needs: the lifecycle
// status to restore and the bracelet graduation released.
func TestTransitionHistoryRoundTripsAndRejectsInvalidRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	account := testpkg.CreateTestAccount(t, db, "transition-history@example.com")
	promotedChild := testpkg.CreateTestStudent(t, db, "Auf", "Steiger", "1a")
	graduateChild := testpkg.CreateTestStudent(t, db, "Ab", "Gang", "4a")

	withTenant(t, db, func(ctx context.Context) error {
		transition := draftWith(t, ctx, module, "2025-2026", account.ID, promoteTo("1a", "2a"), graduates("4a"))

		empty, err := module.ListTransitionHistory(ctx, transition.ID)
		require.NoError(t, err)
		assert.Empty(t, empty)

		require.NoError(t, module.AppendTransitionHistory(ctx, nil), "an empty batch is a no-op")

		fromStatus, tag := "active", "AABBCC"
		require.NoError(t, module.AppendTransitionHistory(ctx, []schoolstructure.TransitionHistoryEntry{
			{TransitionID: transition.ID, StudentID: promotedChild.ID, PersonName: "Auf Steiger", FromClass: "1a",
				ToClass: testpkg.StrPtr("2a"), Action: schoolstructure.TransitionActionPromoted},
			{TransitionID: transition.ID, StudentID: graduateChild.ID, PersonName: "Ab Gang", FromClass: "4a",
				Action: schoolstructure.TransitionActionGraduated, FromStatus: &fromStatus, RFIDTag: &tag},
		}))

		entries, err := module.ListTransitionHistory(ctx, transition.ID)
		require.NoError(t, err)
		require.Len(t, entries, 2)
		assert.Equal(t, testpkg.Tenant(t), entries[0].TenantID)
		assert.True(t, entries[0].WasPromoted())
		require.NotNil(t, entries[0].ToClass)
		assert.Equal(t, "2a", *entries[0].ToClass)
		assert.Nil(t, entries[0].FromStatus)
		assert.Nil(t, entries[0].RFIDTag)
		assert.True(t, entries[1].WasGraduated())
		assert.Nil(t, entries[1].ToClass)
		require.NotNil(t, entries[1].FromStatus)
		assert.Equal(t, fromStatus, *entries[1].FromStatus, "the revert restores the exact lifecycle status")
		require.NotNil(t, entries[1].RFIDTag)
		assert.Equal(t, tag, *entries[1].RFIDTag, "the released bracelet is recorded so the revert can hand it back")

		invalid := map[string]schoolstructure.TransitionHistoryEntry{
			"no transition": {StudentID: promotedChild.ID, PersonName: "Auf Steiger", FromClass: "1a", Action: schoolstructure.TransitionActionPromoted},
			"no student":    {TransitionID: transition.ID, PersonName: "Auf Steiger", FromClass: "1a", Action: schoolstructure.TransitionActionPromoted},
			"no name":       {TransitionID: transition.ID, StudentID: promotedChild.ID, FromClass: "1a", Action: schoolstructure.TransitionActionPromoted},
			"no from class": {TransitionID: transition.ID, StudentID: promotedChild.ID, PersonName: "Auf Steiger", Action: schoolstructure.TransitionActionPromoted},
			"unknown action": {TransitionID: transition.ID, StudentID: promotedChild.ID, PersonName: "Auf Steiger",
				FromClass: "1a", Action: "verschoben"},
		}
		for name, entry := range invalid {
			require.ErrorIsf(t, module.AppendTransitionHistory(ctx, []schoolstructure.TransitionHistoryEntry{entry}),
				schoolstructure.ErrInvalidTransition, "%s must be rejected", name)
		}

		unchanged, err := module.ListTransitionHistory(ctx, transition.ID)
		require.NoError(t, err)
		assert.Len(t, unchanged, 2, "no rejected row landed in the ledger")

		_, err = module.ListTransitionHistory(ctx, 0)
		require.ErrorIs(t, err, schoolstructure.ErrInvalidTransition)
		return nil
	})
}

func TestTransitionClassTeacherLedgerRoundTripsAndRejectsInvalidRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	account := testpkg.CreateTestAccount(t, db, "transition-ct-ledger@example.com")
	staff := testpkg.CreateTestStaff(t, db, "Klassen", "Lehrkraft")

	withTenant(t, db, func(ctx context.Context) error {
		transition := draftWith(t, ctx, module, "2025-2026", account.ID, promoteTo("1a", "2a"))

		require.NoError(t, module.AppendTransitionClassTeacherLedger(ctx, nil), "an empty batch is a no-op")
		require.NoError(t, module.AppendTransitionClassTeacherLedger(ctx, []schoolstructure.TransitionClassTeacherEntry{
			{TransitionID: transition.ID, StaffID: staff.ID, SchoolClass: "1a", Action: schoolstructure.LedgerActionRemoved},
			{TransitionID: transition.ID, StaffID: staff.ID, SchoolClass: "2a", Action: schoolstructure.LedgerActionCreated},
		}))

		ledger, err := module.ListTransitionClassTeacherLedger(ctx, transition.ID)
		require.NoError(t, err)
		require.Len(t, ledger, 2)
		assert.Equal(t, transition.ID, ledger[0].TransitionID)
		assert.Equal(t, staff.ID, ledger[0].StaffID)
		assert.Equal(t, "1a", ledger[0].SchoolClass, "the original display form is preserved for the restore")
		assert.Equal(t, schoolstructure.LedgerActionRemoved, ledger[0].Action)
		assert.Equal(t, schoolstructure.LedgerActionCreated, ledger[1].Action)

		invalid := map[string]schoolstructure.TransitionClassTeacherEntry{
			"no transition": {StaffID: staff.ID, SchoolClass: "1a", Action: schoolstructure.LedgerActionRemoved},
			"no staff":      {TransitionID: transition.ID, SchoolClass: "1a", Action: schoolstructure.LedgerActionRemoved},
			"no class":      {TransitionID: transition.ID, StaffID: staff.ID, SchoolClass: "  ", Action: schoolstructure.LedgerActionRemoved},
			"bad action":    {TransitionID: transition.ID, StaffID: staff.ID, SchoolClass: "1a", Action: "moved"},
		}
		for name, entry := range invalid {
			require.ErrorIsf(t, module.AppendTransitionClassTeacherLedger(ctx, []schoolstructure.TransitionClassTeacherEntry{entry}),
				schoolstructure.ErrInvalidTransition, "%s must be rejected", name)
		}

		unchanged, err := module.ListTransitionClassTeacherLedger(ctx, transition.ID)
		require.NoError(t, err)
		assert.Len(t, unchanged, 2)

		_, err = module.ListTransitionClassTeacherLedger(ctx, 0)
		require.ErrorIs(t, err, schoolstructure.ErrInvalidTransition)
		return nil
	})
}

func TestTransitionClassListLedgerRoundTripsAndRejectsInvalidRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	account := testpkg.CreateTestAccount(t, db, "transition-cl-ledger@example.com")
	entry := testpkg.CreateTestClassListEntry(t, db, "Liste", "Kind", "1a")

	withTenant(t, db, func(ctx context.Context) error {
		transition := draftWith(t, ctx, module, "2025-2026", account.ID, promoteTo("1a", "2a"))

		require.NoError(t, module.AppendTransitionClassListLedger(ctx, nil), "an empty batch is a no-op")
		require.NoError(t, module.AppendTransitionClassListLedger(ctx, []schoolstructure.TransitionClassListEntry{
			{TransitionID: transition.ID, FirstName: "Liste", LastName: "Kind", SchoolClass: "1a", Action: schoolstructure.LedgerActionRemoved},
			{TransitionID: transition.ID, EntryID: &entry.ID, FirstName: "Liste", LastName: "Kind", SchoolClass: "2a",
				Action: schoolstructure.LedgerActionCreated},
		}))

		ledger, err := module.ListTransitionClassListLedger(ctx, transition.ID)
		require.NoError(t, err)
		require.Len(t, ledger, 2)
		assert.Nil(t, ledger[0].EntryID, "a removal records no row id")
		assert.Equal(t, "1a", ledger[0].SchoolClass)
		require.NotNil(t, ledger[1].EntryID, "the created row's id lets the revert resolve an edited row")
		assert.Equal(t, entry.ID, *ledger[1].EntryID)
		assert.Equal(t, schoolstructure.LedgerActionCreated, ledger[1].Action)

		invalid := map[string]schoolstructure.TransitionClassListEntry{
			"no transition": {FirstName: "Liste", LastName: "Kind", SchoolClass: "1a", Action: schoolstructure.LedgerActionRemoved},
			"no first name": {TransitionID: transition.ID, LastName: "Kind", SchoolClass: "1a", Action: schoolstructure.LedgerActionRemoved},
			"no last name":  {TransitionID: transition.ID, FirstName: "Liste", SchoolClass: "1a", Action: schoolstructure.LedgerActionRemoved},
			"no class":      {TransitionID: transition.ID, FirstName: "Liste", LastName: "Kind", SchoolClass: " ", Action: schoolstructure.LedgerActionRemoved},
			"bad action":    {TransitionID: transition.ID, FirstName: "Liste", LastName: "Kind", SchoolClass: "1a", Action: "renamed"},
		}
		for name, row := range invalid {
			require.ErrorIsf(t, module.AppendTransitionClassListLedger(ctx, []schoolstructure.TransitionClassListEntry{row}),
				schoolstructure.ErrInvalidTransition, "%s must be rejected", name)
		}

		unchanged, err := module.ListTransitionClassListLedger(ctx, transition.ID)
		require.NoError(t, err)
		assert.Len(t, unchanged, 2)

		_, err = module.ListTransitionClassListLedger(ctx, 0)
		require.ErrorIs(t, err, schoolstructure.ErrInvalidTransition)
		return nil
	})
}

// Rows of another school stay invisible: find, list and the ledgers are all
// tenant-scoped, so one school's rollover can never read another's.
func TestTransitionsOfOtherTenantsStayInvisible(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	account := testpkg.CreateTestAccount(t, db, "transition-isolation@example.com")
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)

	var foreign schoolstructure.Transition
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, otherTenant, func(ctx context.Context, _ bun.Tx) error {
		created, err := module.CreateTransition(ctx, schoolstructure.TransitionDraft{
			AcademicYear: "2025-2026", CreatedBy: account.ID,
			Mappings: []schoolstructure.TransitionMappingInput{promoteTo("1a", "2a")},
		})
		if err != nil {
			return err
		}
		foreign = created
		return module.AppendTransitionHistory(ctx, []schoolstructure.TransitionHistoryEntry{
			{TransitionID: created.ID, StudentID: 4711, PersonName: "Fremdes Kind", FromClass: "1a",
				ToClass: testpkg.StrPtr("2a"), Action: schoolstructure.TransitionActionPromoted},
		})
	}))
	require.NotZero(t, foreign.ID)
	assert.Equal(t, otherTenant, foreign.TenantID)

	withTenant(t, db, func(ctx context.Context) error {
		own := draftWith(t, ctx, module, "2025-2026", account.ID, promoteTo("1a", "2a"))

		_, err := module.FindTransition(ctx, foreign.ID)
		require.ErrorIs(t, err, schoolstructure.ErrTransitionNotFound, "the other school's draft is not readable")

		listed, total, err := module.ListTransitions(ctx, schoolstructure.TransitionFilter{})
		require.NoError(t, err)
		assert.Equal(t, 1, total, "the count is tenant-scoped too")
		assert.Equal(t, []int64{own.ID}, transitionIDs(listed))

		history, err := module.ListTransitionHistory(ctx, foreign.ID)
		require.NoError(t, err)
		assert.Empty(t, history, "the other school's history is invisible")
		return nil
	})
}
