package education_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
)

// classListEntryClassesOf reads the current classes an entry name sits in,
// straight from the table, sorted.
func classListEntryClassesOf(t *testing.T, db *bun.DB, ctx context.Context, firstName, lastName string) []string {
	t.Helper()
	var classes []string
	err := db.NewSelect().
		TableExpr("users.class_list_entries").
		Column("school_class").
		Where("first_name = ? AND last_name = ?", firstName, lastName).
		OrderExpr("LOWER(BTRIM(school_class)) ASC").
		Scan(ctx, &classes)
	require.NoError(t, err)
	return classes
}

// classListAuditRow mirrors the audit trail the class-list rewrite writes.
type classListAuditRow struct {
	EntryID   int64  `bun:"entry_id"`
	Action    string `bun:"action"`
	OldValue  string `bun:"old_value"`
	NewValue  string `bun:"new_value"`
	ChangedBy int64  `bun:"changed_by"`
}

// classListAuditRows reads the tenant's class-list audit trail in append
// order, from the same table the legacy test's audit repository wrote to.
func classListAuditRows(t *testing.T, db *bun.DB, ctx context.Context) []classListAuditRow {
	t.Helper()
	rows := []classListAuditRow{}
	err := db.NewSelect().
		Model(&rows).
		ModelTableExpr(`audit.class_list_entry_changes AS "class_list_audit_row"`).
		Where("tenant_id = ?", testpkg.Tenant(t)).
		OrderExpr("id ASC").
		Scan(ctx)
	require.NoError(t, err)
	return rows
}

// auditRowByAction returns the single audit row carrying the given action.
func auditRowByAction(t *testing.T, rows []classListAuditRow, action string) classListAuditRow {
	t.Helper()
	var found []classListAuditRow
	for _, row := range rows {
		if row.Action == action {
			found = append(found, row)
		}
	}
	require.Len(t, found, 1, "exactly one %q audit row", action)
	return found[0]
}

func auditActions(rows []classListAuditRow) []string {
	actions := make([]string, 0, len(rows))
	for _, row := range rows {
		actions = append(actions, row.Action)
	}
	return actions
}

// The class-list entries must follow the school-year rollover like the
// students and the Klassenlehrer assignments (#2382): promoted classes carry
// their entries along, graduating classes lose them — and the revert replays
// the recorded ledger backwards, restoring exactly what the apply rewrote
// while leaving pre-existing and newly created entries untouched (#2399
// review blocker).
func TestGradeTransitionWorkflow_Apply_RemapsClassListEntries(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 30*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	class1 := fmt.Sprintf("1a-%s", suffix)
	class2 := fmt.Sprintf("2a-%s", suffix)
	class4 := fmt.Sprintf("4a-%s", suffix)
	classOther := fmt.Sprintf("1b-%s", suffix)

	// One child per mapped class so the transition has a locked cohort.
	testpkg.CreateTestStudent(t, db, "CleMove", "Cohort1", class1)
	testpkg.CreateTestStudent(t, db, "CleMove", "Cohort4", class4)

	promotedEntry := testpkg.CreateTestClassListEntry(t, db, "CleZoe", "Promoted", class1)
	graduatedEntry := testpkg.CreateTestClassListEntry(t, db, "CleBen", "Graduated", class4)
	testpkg.CreateTestClassListEntry(t, db, "CleUli", "Unrelated", classOther)

	transitionID := f.createDraft(t, ctx, "2026-2027", promote(class1, class2), graduate(class4))

	_, err := wf.Apply(ctx, transitionID, "")
	require.NoError(t, err)

	assert.Equal(t, []string{class2}, classListEntryClassesOf(t, db, ctx, "CleZoe", "Promoted"),
		"promoted class must carry its entry along")
	assert.Empty(t, classListEntryClassesOf(t, db, ctx, "CleBen", "Graduated"),
		"graduating class must lose its entry")
	assert.Equal(t, []string{classOther}, classListEntryClassesOf(t, db, ctx, "CleUli", "Unrelated"),
		"an entry in an unmapped class must not move")

	// Both rewrites are ledgered: the two deletes and the single re-insert.
	ledger, err := f.deps.Structure.ListTransitionClassListLedger(ctx, transitionID)
	require.NoError(t, err)
	require.Len(t, ledger, 3, "two removals plus the one promoted re-insert")

	// The rewrite leaves an audit trail attributed to the acting account: the
	// promotion is an update of the old row, the graduation a deletion.
	applyAudit := classListAuditRows(t, db, ctx)
	require.ElementsMatch(t, []string{gradetransition.ClassListAuditUpdated, gradetransition.ClassListAuditDeleted}, auditActions(applyAudit),
		"the apply records one update for the promoted entry and one deletion for the graduated one")
	for _, row := range applyAudit {
		assert.Equal(t, f.actorID, row.ChangedBy, "the audit row is attributed to the acting account")
	}
	updated := auditRowByAction(t, applyAudit, gradetransition.ClassListAuditUpdated)
	assert.Contains(t, updated.OldValue, class1)
	assert.Contains(t, updated.NewValue, class2)
	assert.NotEqual(t, promotedEntry.ID, updated.EntryID,
		"the update is recorded against the row the remap inserted")
	deleted := auditRowByAction(t, applyAudit, gradetransition.ClassListAuditDeleted)
	assert.Equal(t, graduatedEntry.ID, deleted.EntryID)
	assert.Contains(t, deleted.OldValue, class4)
	assert.Empty(t, deleted.NewValue)

	// Created BETWEEN apply and revert, in the rename-target class: the
	// mapping-derived reverse rename would drag it to class1 — the ledger
	// replay must leave it alone.
	testpkg.CreateTestClassListEntry(t, db, "CleNeu", "Dazwischen", class2)

	_, err = wf.Revert(ctx, transitionID)
	require.NoError(t, err)

	assert.Equal(t, []string{class1}, classListEntryClassesOf(t, db, ctx, "CleZoe", "Promoted"),
		"revert must carry the promoted entry back")
	assert.Equal(t, []string{class4}, classListEntryClassesOf(t, db, ctx, "CleBen", "Graduated"),
		"revert must restore the graduated entry from the ledger, like it reactivates the students")
	assert.Equal(t, []string{class2}, classListEntryClassesOf(t, db, ctx, "CleNeu", "Dazwischen"),
		"an entry created after the apply must stay untouched by the revert")
	assert.Equal(t, []string{classOther}, classListEntryClassesOf(t, db, ctx, "CleUli", "Unrelated"),
		"an entry in an unmapped class must survive the revert unchanged")

	// The revert audits what it undid: the row the apply created is deleted
	// again, the two removed rows come back as creations.
	revertAudit := classListAuditRows(t, db, ctx)[len(applyAudit):]
	assert.ElementsMatch(t, []string{
		gradetransition.ClassListAuditDeleted,
		gradetransition.ClassListAuditCreated,
		gradetransition.ClassListAuditCreated,
	}, auditActions(revertAudit))
	for _, row := range revertAudit {
		assert.Equal(t, f.actorID, row.ChangedBy)
	}
}

// An entry the admin deliberately removed between apply and revert stays
// removed: the ledger replay must not resurrect the child under the
// pre-transition class name.
func TestGradeTransitionWorkflow_Revert_KeepsAdminDeletedEntriesDeleted(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 30*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	class1 := fmt.Sprintf("1c-%s", suffix)
	class2 := fmt.Sprintf("2c-%s", suffix)

	testpkg.CreateTestStudent(t, db, "CleDel", "Cohort", class1)

	testpkg.CreateTestClassListEntry(t, db, "CleMia", "Entfernt", class1)

	transitionID := f.createDraft(t, ctx, "2026-2027", promote(class1, class2))

	_, err := wf.Apply(ctx, transitionID, "")
	require.NoError(t, err)
	require.Equal(t, []string{class2}, classListEntryClassesOf(t, db, ctx, "CleMia", "Entfernt"))

	// The admin takes the child off the list after the apply.
	_, err = db.NewDelete().TableExpr("users.class_list_entries").
		Where("first_name = ? AND last_name = ?", "CleMia", "Entfernt").
		Exec(ctx)
	require.NoError(t, err)

	_, err = wf.Revert(ctx, transitionID)
	require.NoError(t, err)

	assert.Empty(t, classListEntryClassesOf(t, db, ctx, "CleMia", "Entfernt"),
		"a deliberately deleted entry must not be resurrected by the revert")
}

// Restores that would duplicate a child are skipped: an identical entry the
// admin re-created under the old class, or a student who got a real record
// under that name and class in the meantime — either way the child already
// has exactly one row on the class list.
func TestGradeTransitionWorkflow_Revert_SkipsCollidingRestores(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 30*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	class1 := fmt.Sprintf("1d-%s", suffix)
	class2 := fmt.Sprintf("2d-%s", suffix)

	testpkg.CreateTestStudent(t, db, "CleColl", "Cohort", class1)

	testpkg.CreateTestClassListEntry(t, db, "CleAnna", "Kollision", class1)
	testpkg.CreateTestClassListEntry(t, db, "CleBen2", "Kollision", class1)

	transitionID := f.createDraft(t, ctx, "2026-2027", promote(class1, class2))

	_, err := wf.Apply(ctx, transitionID, "")
	require.NoError(t, err)

	// Between apply and revert: Anna gets re-created as an entry under the
	// OLD class name, Ben gets a real student record under the old class.
	testpkg.CreateTestClassListEntry(t, db, "CleAnna", "Kollision", class1)
	testpkg.CreateTestStudent(t, db, "CleBen2", "Kollision", class1)

	_, err = wf.Revert(ctx, transitionID)
	require.NoError(t, err)

	assert.Equal(t, []string{class1}, classListEntryClassesOf(t, db, ctx, "CleAnna", "Kollision"),
		"the re-created entry survives; the ledger restore must not duplicate it")
	assert.Empty(t, classListEntryClassesOf(t, db, ctx, "CleBen2", "Kollision"),
		"a child with a real student record must not get a second class-list row")
}

// An entry whose stored class differs from the mapping's display form only in
// case belongs to the same cohort — the entry's class identity is the
// normalized form everywhere else (unique index, filters, exports). The remap
// must carry it along via the normalized fallback instead of leaving it
// attached to the old class name, and the revert must restore its original
// display form (#2399 review round 9).
func TestGradeTransitionWorkflow_Apply_RemapsCaseDivergentEntryClass(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 30*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	class1 := fmt.Sprintf("1f-%s", suffix)
	class1Upper := strings.ToUpper(class1)
	class2 := fmt.Sprintf("2f-%s", suffix)

	testpkg.CreateTestStudent(t, db, "CleCase", "Cohort", class1)

	testpkg.CreateTestClassListEntry(t, db, "CleIda", "Grossklein", class1Upper)

	transitionID := f.createDraft(t, ctx, "2026-2027", promote(class1, class2))

	_, err := wf.Apply(ctx, transitionID, "")
	require.NoError(t, err)

	assert.Equal(t, []string{class2}, classListEntryClassesOf(t, db, ctx, "CleIda", "Grossklein"),
		"a case-divergent entry class must follow its cohort's rename")

	_, err = wf.Revert(ctx, transitionID)
	require.NoError(t, err)

	assert.Equal(t, []string{class1Upper}, classListEntryClassesOf(t, db, ctx, "CleIda", "Grossklein"),
		"the revert must restore the entry with its original display form")
}

// An entry the admin edited between apply and revert keeps the edit, even when
// the edit leaves the normalized identity untouched ("2g-xy" → "2G-XY"). The
// revert resolves the row it created by the recorded ID, so a display-form
// change is not mistaken for the untouched transition row and replayed over
// (#2399 review round 11).
func TestGradeTransitionWorkflow_Revert_KeepsAdminEditedCreatedEntry(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 30*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	class1 := fmt.Sprintf("1g-%s", suffix)
	class2 := fmt.Sprintf("2g-%s", suffix)
	class2Upper := strings.ToUpper(class2)

	testpkg.CreateTestStudent(t, db, "CleEdit", "Cohort", class1)

	testpkg.CreateTestClassListEntry(t, db, "CleNina", "Bearbeitet", class1)

	transitionID := f.createDraft(t, ctx, "2026-2027", promote(class1, class2))

	_, err := wf.Apply(ctx, transitionID, "")
	require.NoError(t, err)
	require.Equal(t, []string{class2}, classListEntryClassesOf(t, db, ctx, "CleNina", "Bearbeitet"))

	// The admin corrects the class spelling after the apply — same child, same
	// normalized class, different display form.
	_, err = db.NewUpdate().TableExpr("users.class_list_entries").
		Set("school_class = ?", class2Upper).
		Where("first_name = ? AND last_name = ?", "CleNina", "Bearbeitet").
		Exec(ctx)
	require.NoError(t, err)

	_, err = wf.Revert(ctx, transitionID)
	require.NoError(t, err)

	assert.Equal(t, []string{class2Upper}, classListEntryClassesOf(t, db, ctx, "CleNina", "Bearbeitet"),
		"an entry edited after the apply must survive the revert unchanged, in exactly one row")
}

// A transition over classes without any entries writes no ledger — apply and
// revert both no-op cleanly, and nothing lands in the audit trail.
func TestGradeTransitionWorkflow_ClassListEntries_NoEntriesNoLedger(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 30*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	class1 := fmt.Sprintf("1e-%s", suffix)
	class2 := fmt.Sprintf("2e-%s", suffix)

	testpkg.CreateTestStudent(t, db, "CleEmpty", "Cohort", class1)

	transitionID := f.createDraft(t, ctx, "2026-2027", promote(class1, class2))

	_, err := wf.Apply(ctx, transitionID, "")
	require.NoError(t, err)

	ledger, err := f.deps.Structure.ListTransitionClassListLedger(ctx, transitionID)
	require.NoError(t, err)
	assert.Empty(t, ledger, "no entries, no ledger rows")
	assert.Empty(t, classListAuditRows(t, db, ctx), "nothing was rewritten, so nothing is audited")

	_, err = wf.Revert(ctx, transitionID)
	require.NoError(t, err)
}
