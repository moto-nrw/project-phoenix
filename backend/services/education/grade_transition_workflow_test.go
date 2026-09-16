// The grade transition workflow (#2711) is the only coordinator of the
// school-year rollover. These tests compose the real owners the way the
// production root does and prove the ticket's acceptance list: preview and
// apply agree, every owner performs its own mutation inside one unit of
// work, a failure after any owner command rolls everything back, a stale
// preview is refused before the first mutation, authorization and tenant
// isolation hold, the blocker-defined lock order serializes concurrent
// writers, a repeated command is answered as a stale-state conflict, and
// the history survives the revert.
package education_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	gradetransitioncompose "github.com/moto-nrw/project-phoenix/workflows/gradetransition/compose"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var errInjected = errors.New("injected owner failure")

// faultInjector fails one named owner command AFTER it ran, so the rollback
// must undo that command and every earlier one.
type faultInjector struct{ step string }

func (i *faultInjector) after(step string, err error) error {
	if err != nil {
		return err
	}
	if i.step == step {
		return fmt.Errorf("%s: %w", step, errInjected)
	}
	return nil
}

type faultyStructure struct {
	gradetransition.Structure
	inj *faultInjector
}

func (s faultyStructure) AppendTransitionHistory(ctx context.Context, entries []gradetransition.TransitionHistoryEntry) error {
	return s.inj.after("history", s.Structure.AppendTransitionHistory(ctx, entries))
}

func (s faultyStructure) AppendTransitionClassTeacherLedger(ctx context.Context, entries []gradetransition.TransitionClassTeacherEntry) error {
	return s.inj.after("class_teacher_ledger", s.Structure.AppendTransitionClassTeacherLedger(ctx, entries))
}

func (s faultyStructure) AppendTransitionClassListLedger(ctx context.Context, entries []gradetransition.TransitionClassListEntry) error {
	return s.inj.after("class_list_ledger", s.Structure.AppendTransitionClassListLedger(ctx, entries))
}

func (s faultyStructure) MarkTransitionApplied(ctx context.Context, id, accountID int64, at time.Time, baseline *int64) error {
	return s.inj.after("mark_applied", s.Structure.MarkTransitionApplied(ctx, id, accountID, at, baseline))
}

type faultyDirectory struct {
	gradetransition.Directory
	inj *faultInjector
}

func (d faultyDirectory) ReleaseTags(ctx context.Context, personIDs []int64) ([]gradetransition.ReleasedTag, error) {
	released, err := d.Directory.ReleaseTags(ctx, personIDs)
	return released, d.inj.after("release_tags", err)
}

func (d faultyDirectory) GraduateStudents(ctx context.Context, ids []int64) (int64, error) {
	count, err := d.Directory.GraduateStudents(ctx, ids)
	return count, d.inj.after("graduate", err)
}

func (d faultyDirectory) PromoteStudents(ctx context.Context, ids []int64, from, to string) (int64, error) {
	count, err := d.Directory.PromoteStudents(ctx, ids, from, to)
	return count, d.inj.after("promote", err)
}

type faultyMembership struct {
	gradetransition.Membership
	inj *faultInjector
}

func (m faultyMembership) DeleteClassAssignment(ctx context.Context, id int64) error {
	return m.inj.after("class_teacher_delete", m.Membership.DeleteClassAssignment(ctx, id))
}

func (m faultyMembership) DeleteClassListEntry(ctx context.Context, id int64) error {
	return m.inj.after("class_list_delete", m.Membership.DeleteClassListEntry(ctx, id))
}

type faultyRosters struct {
	gradetransition.Rosters
	inj *faultInjector
}

func (r faultyRosters) RemoveStudentsFromFutureRosters(ctx context.Context, transitionID int64, studentIDs []int64) error {
	return r.inj.after("rosters_archive", r.Rosters.RemoveStudentsFromFutureRosters(ctx, transitionID, studentIDs))
}

func (r faultyRosters) CurrentRosterBaseline(ctx context.Context) (int64, error) {
	baseline, err := r.Rosters.CurrentRosterBaseline(ctx)
	return baseline, r.inj.after("roster_baseline", err)
}

// rolloverScene is one graduating and one promoted child with everything
// the apply touches: a bracelet, a Klassenlehrer, a class-list entry and a
// still-planned roster row.
type rolloverScene struct {
	graduate, promoted         *users.Student
	tag                        string
	staffID, assignmentID      int64
	classListEntryID           int64
	instanceID, rosterRowID    int64
	transitionID               int64
	promotedTeacherClassBefore string
}

func newRolloverScene(t *testing.T, f *transitionFixture, ctx context.Context) *rolloverScene {
	t.Helper()
	db := f.db
	scene := &rolloverScene{}
	scene.graduate = testpkg.CreateTestStudent(t, db, "Rollover", "Graduate", "4a")
	scene.promoted = testpkg.CreateTestStudent(t, db, "Rollover", "Promoted", "1a")
	card := testpkg.CreateTestRFIDCard(t, db, "ROLLOVER"+fmt.Sprint(scene.graduate.ID))
	scene.tag = card.ID
	_, err := db.NewRaw(`UPDATE users.persons SET tag_id = ? WHERE id = ?`, card.ID, scene.graduate.PersonID).Exec(ctx)
	require.NoError(t, err)
	staff := testpkg.CreateTestStaff(t, db, "Rollover", "Teacher")
	scene.staffID = staff.ID
	assignment := testpkg.CreateTestClassTeacher(t, db, staff.ID, "1a")
	scene.assignmentID = assignment.ID
	scene.promotedTeacherClassBefore = assignment.SchoolClass
	entry := testpkg.CreateTestClassListEntry(t, db, "Rollover", "Listed", "1a")
	scene.classListEntryID = entry.ID
	room := testpkg.CreateTestRoom(t, db, "rollover-room-"+fmt.Sprint(scene.graduate.ID))
	instance := testpkg.CreateTestActivityInstance(t, db, f.today().AddDays(1), room.ID, testpkg.ActivityInstanceOpts{IsSpontaneous: true})
	scene.instanceID = instance.ID
	row := testpkg.CreateTestInstanceStudent(t, db, instance.ID, scene.graduate.ID, "")
	scene.rosterRowID = row.ID
	scene.transitionID = f.createDraft(t, ctx, "2026-2027", promote("1a", "2a"), graduate("4a"))
	return scene
}

func studentClassAndStatus(t *testing.T, ctx context.Context, db *bun.DB, id int64) (string, string) {
	t.Helper()
	var row struct {
		SchoolClass string
		Status      string
	}
	require.NoError(t, db.NewRaw(`SELECT school_class, status FROM users.students WHERE id = ?`, id).Scan(ctx, &row))
	return row.SchoolClass, row.Status
}

func personTag(t *testing.T, ctx context.Context, db *bun.DB, personID int64) string {
	t.Helper()
	var tags []string
	require.NoError(t, db.NewSelect().TableExpr(`users.persons`).ColumnExpr(`COALESCE(tag_id, '')`).Where("id = ?", personID).Scan(ctx, &tags))
	require.Len(t, tags, 1)
	return tags[0]
}

func countRows(t *testing.T, ctx context.Context, db *bun.DB, query string, args ...any) int {
	t.Helper()
	var count int
	require.NoError(t, db.NewRaw(query, args...).Scan(ctx, &count))
	return count
}

func transitionStatus(t *testing.T, ctx context.Context, db *bun.DB, id int64) string {
	t.Helper()
	var status string
	require.NoError(t, db.NewRaw(`SELECT status FROM education.grade_transitions WHERE id = ?`, id).Scan(ctx, &status))
	return status
}

// assertUntouched proves the scene is exactly as it was before the apply.
func assertUntouched(t *testing.T, ctx context.Context, db *bun.DB, scene *rolloverScene) {
	t.Helper()
	assert.Equal(t, "draft", transitionStatus(t, ctx, db, scene.transitionID))
	class, status := studentClassAndStatus(t, ctx, db, scene.graduate.ID)
	assert.Equal(t, "4a", class)
	assert.Equal(t, string(users.StudentStatusActive), status)
	class, _ = studentClassAndStatus(t, ctx, db, scene.promoted.ID)
	assert.Equal(t, "1a", class)
	assert.Equal(t, scene.tag, personTag(t, ctx, db, scene.graduate.PersonID), "the bracelet is still held")
	assert.Zero(t, countRows(t, ctx, db, `SELECT COUNT(*) FROM education.grade_transition_history WHERE transition_id = ?`, scene.transitionID))
	assert.Zero(t, countRows(t, ctx, db, `SELECT COUNT(*) FROM education.grade_transition_class_teachers WHERE transition_id = ?`, scene.transitionID))
	assert.Zero(t, countRows(t, ctx, db, `SELECT COUNT(*) FROM education.grade_transition_class_list_entries WHERE transition_id = ?`, scene.transitionID))
	assert.Equal(t, 1, countRows(t, ctx, db, `SELECT COUNT(*) FROM education.class_teachers WHERE id = ? AND school_class = '1a'`, scene.assignmentID))
	assert.Equal(t, 1, countRows(t, ctx, db, `SELECT COUNT(*) FROM users.class_list_entries WHERE id = ? AND school_class = '1a'`, scene.classListEntryID))
	assert.Equal(t, 1, countRows(t, ctx, db, `SELECT COUNT(*) FROM schedule.instance_students WHERE id = ?`, scene.rosterRowID))
	assert.Zero(t, countRows(t, ctx, db, `SELECT COUNT(*) FROM schedule.grade_transition_roster_removals WHERE transition_id = ?`, scene.transitionID))
	assert.Zero(t, countRows(t, ctx, db, `SELECT COUNT(*) FROM audit.class_list_entry_changes WHERE entry_id = ?`, scene.classListEntryID))
}

// assertApplied proves every owner performed its mutation.
func assertApplied(t *testing.T, ctx context.Context, db *bun.DB, scene *rolloverScene) {
	t.Helper()
	assert.Equal(t, "applied", transitionStatus(t, ctx, db, scene.transitionID))
	class, status := studentClassAndStatus(t, ctx, db, scene.graduate.ID)
	assert.Equal(t, "4a", class)
	assert.Equal(t, string(users.StudentStatusAlumnus), status)
	class, _ = studentClassAndStatus(t, ctx, db, scene.promoted.ID)
	assert.Equal(t, "2a", class)
	assert.Empty(t, personTag(t, ctx, db, scene.graduate.PersonID), "graduation released the bracelet")
	assert.Equal(t, 2, countRows(t, ctx, db, `SELECT COUNT(*) FROM education.grade_transition_history WHERE transition_id = ?`, scene.transitionID))
	assert.Equal(t, 1, countRows(t, ctx, db, `SELECT COUNT(*) FROM education.grade_transition_history WHERE transition_id = ? AND student_id = ? AND rfid_tag = ?`, scene.transitionID, scene.graduate.ID, scene.tag))
	assert.Equal(t, 2, countRows(t, ctx, db, `SELECT COUNT(*) FROM education.grade_transition_class_teachers WHERE transition_id = ?`, scene.transitionID))
	assert.Equal(t, 1, countRows(t, ctx, db, `SELECT COUNT(*) FROM education.class_teachers WHERE staff_id = ? AND school_class = '2a'`, scene.staffID))
	assert.Equal(t, 2, countRows(t, ctx, db, `SELECT COUNT(*) FROM education.grade_transition_class_list_entries WHERE transition_id = ?`, scene.transitionID))
	assert.Equal(t, 1, countRows(t, ctx, db, `SELECT COUNT(*) FROM users.class_list_entries WHERE tenant_id = ? AND first_name = 'Rollover' AND last_name = 'Listed' AND school_class = '2a'`, testpkg.Tenant(t)))
	assert.Zero(t, countRows(t, ctx, db, `SELECT COUNT(*) FROM schedule.instance_students WHERE id = ?`, scene.rosterRowID), "the still-planned roster row is archived")
	assert.Equal(t, 1, countRows(t, ctx, db, `SELECT COUNT(*) FROM schedule.grade_transition_roster_removals WHERE transition_id = ? AND student_id = ?`, scene.transitionID, scene.graduate.ID))
}

func TestGradeTransitionWorkflow_RollsBackAfterEachOwnerCommand(t *testing.T) {
	t.Parallel()
	steps := []string{
		"release_tags", "history", "graduate", "promote", "class_teacher_delete", "class_teacher_ledger",
		"class_list_delete", "class_list_ledger", "rosters_archive", "offering_resync", "roster_baseline", "mark_applied",
	}
	for _, step := range steps {
		t.Run(step, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			ctx := testpkg.Ctx(t)
			f := newTransitionFixture(t, db)
			inj := &faultInjector{step: step}
			f.deps.Structure = faultyStructure{Structure: f.deps.Structure, inj: inj}
			f.deps.Directory = faultyDirectory{Directory: f.deps.Directory, inj: inj}
			f.deps.Membership = faultyMembership{Membership: f.deps.Membership, inj: inj}
			f.deps.Rosters = faultyRosters{Rosters: f.deps.Rosters, inj: inj}
			resync := f.deps.ResyncOfferingRosters
			f.deps.ResyncOfferingRosters = func(ctx context.Context, effectiveFrom string) error {
				return inj.after("offering_resync", resync(ctx, effectiveFrom))
			}
			scene := newRolloverScene(t, f, ctx)
			workflow := f.workflow(t)

			preview, err := workflow.Preview(ctx, scene.transitionID)
			require.NoError(t, err)
			_, err = workflow.Apply(ctx, scene.transitionID, preview.Fingerprint)
			require.ErrorIs(t, err, errInjected, "the injected failure surfaces")
			assertUntouched(t, ctx, db, scene)

			// Once the owner recovers, the same confirmed preview applies in full.
			inj.step = ""
			result, err := workflow.Apply(ctx, scene.transitionID, preview.Fingerprint)
			require.NoError(t, err)
			assert.Equal(t, 1, result.StudentsGraduated)
			assert.Equal(t, 1, result.StudentsPromoted)
			assertApplied(t, ctx, db, scene)
		})
	}
}

func TestGradeTransitionWorkflow_RejectsStalePreviewBeforeAnyMutation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newTransitionFixture(t, db)
	scene := newRolloverScene(t, f, ctx)
	workflow := f.workflow(t)
	preview, err := workflow.Preview(ctx, scene.transitionID)
	require.NoError(t, err)
	require.Equal(t, 2, preview.TotalStudents)

	t.Run("a child moved into a mapped class", func(t *testing.T) {
		newcomer := testpkg.CreateTestStudent(t, db, "Rollover", "Newcomer", "1a")
		_, err := workflow.Apply(ctx, scene.transitionID, preview.Fingerprint)
		require.ErrorIs(t, err, gradetransition.ErrPreviewStale)
		assertUntouched(t, ctx, db, scene)
		_, err = db.NewRaw(`DELETE FROM users.students WHERE id = ?`, newcomer.ID).Exec(ctx)
		require.NoError(t, err)
	})

	t.Run("a mapping edited after the preview", func(t *testing.T) {
		_, err := workflow.UpdateDraft(ctx, scene.transitionID, gradetransition.DraftPatch{Mappings: []gradetransition.Mapping{promote("1a", "2b"), graduate("4a")}})
		require.NoError(t, err)
		_, err = workflow.Apply(ctx, scene.transitionID, preview.Fingerprint)
		require.ErrorIs(t, err, gradetransition.ErrPreviewStale)
		_, err = workflow.UpdateDraft(ctx, scene.transitionID, gradetransition.DraftPatch{Mappings: []gradetransition.Mapping{promote("1a", "2a"), graduate("4a")}})
		require.NoError(t, err)
		assertUntouched(t, ctx, db, scene)
	})

	t.Run("a garbled fingerprint is refused, not ignored", func(t *testing.T) {
		_, err := workflow.Apply(ctx, scene.transitionID, "not-a-digest")
		require.ErrorIs(t, err, gradetransition.ErrPreviewStale)
		assertUntouched(t, ctx, db, scene)
	})

	t.Run("the unchanged preview applies", func(t *testing.T) {
		fresh, err := workflow.Preview(ctx, scene.transitionID)
		require.NoError(t, err)
		assert.Equal(t, preview.Fingerprint, fresh.Fingerprint, "the restored mappings and cohort yield the original digest")
		_, err = workflow.Apply(ctx, scene.transitionID, fresh.Fingerprint)
		require.NoError(t, err)
		assertApplied(t, ctx, db, scene)
	})
}

// countingStructure records whether any owner read happened.
type countingStructure struct {
	gradetransition.Structure
	reads *atomic.Int32
}

func (s countingStructure) FindTransition(ctx context.Context, id int64) (gradetransition.Transition, error) {
	s.reads.Add(1)
	return s.Structure.FindTransition(ctx, id)
}

func TestGradeTransitionWorkflow_AuthorizationAndTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newTransitionFixture(t, db)
	transitionID := f.createDraft(t, ctx, "2026-2027", graduate("4a"))

	t.Run("an unauthorized principal reaches no owner read", func(t *testing.T) {
		reads := &atomic.Int32{}
		f.deps.Structure = countingStructure{Structure: f.deps.Structure, reads: reads}
		authorize := f.deps.Authorize
		f.deps.Authorize = func(context.Context, string) (gradetransition.Actor, error) {
			return gradetransition.Actor{}, gradetransition.ErrUnauthorized
		}
		t.Cleanup(func() { f.deps.Authorize = authorize })
		workflow := f.workflow(t)
		_, err := workflow.Preview(ctx, transitionID)
		require.ErrorIs(t, err, gradetransition.ErrUnauthorized)
		_, err = workflow.Apply(ctx, transitionID, "")
		require.ErrorIs(t, err, gradetransition.ErrUnauthorized)
		_, err = workflow.Revert(ctx, transitionID)
		require.ErrorIs(t, err, gradetransition.ErrUnauthorized)
		assert.Zero(t, reads.Load())
		assert.Equal(t, "draft", transitionStatus(t, ctx, db, transitionID))
	})

	t.Run("the composed gate refuses a request without a tenant principal", func(t *testing.T) {
		for _, operation := range []string{gradetransition.OperationRead, gradetransition.OperationApply, "unknown"} {
			_, err := gradetransitioncompose.Authorize(ctx, operation)
			require.ErrorIs(t, err, gradetransition.ErrUnauthorized, operation)
		}
	})

	t.Run("a transition of another tenant is invisible", func(t *testing.T) {
		foreignTenant := testpkg.UniqueTestTenantID(t)
		testpkg.EnsureTestTenant(t, db, foreignTenant)
		var foreignID int64
		require.NoError(t, db.NewRaw(`INSERT INTO education.grade_transitions (tenant_id, academic_year, status, created_by)
			VALUES (?, '2026-2027', 'draft', ?) RETURNING id`, foreignTenant, f.actorID).Scan(ctx, &foreignID))
		workflow := f.workflow(t)
		_, err := workflow.FindTransition(ctx, foreignID)
		require.ErrorIs(t, err, gradetransition.ErrTransitionNotFound)
		_, err = workflow.Preview(ctx, foreignID)
		require.ErrorIs(t, err, gradetransition.ErrTransitionNotFound)
		_, err = workflow.Apply(ctx, foreignID, "")
		require.ErrorIs(t, err, gradetransition.ErrTransitionNotFound)
		_, err = workflow.History(ctx, foreignID)
		require.ErrorIs(t, err, gradetransition.ErrTransitionNotFound)
		require.ErrorIs(t, workflow.DeleteDraft(ctx, foreignID), gradetransition.ErrTransitionNotFound)
		transitions, total, err := workflow.ListTransitions(ctx, gradetransition.ListFilter{})
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		require.Len(t, transitions, 1)
		assert.Equal(t, transitionID, transitions[0].ID)
	})
}

// holdLock keeps a tenant transaction open on its own connection until
// release is closed, so a workflow command on another connection must wait
// for it. The gate is taken with the same advisory keys the owners use.
func holdLock(t *testing.T, ctx context.Context, db *bun.DB, take func(context.Context, bun.Tx) error) (held <-chan struct{}, release func()) {
	t.Helper()
	heldCh := make(chan struct{})
	releaseCh := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
			if err := take(txCtx, tx); err != nil {
				return err
			}
			close(heldCh)
			<-releaseCh
			return nil
		})
	}()
	release = func() {
		close(releaseCh)
		require.NoError(t, <-done)
	}
	return heldCh, release
}

func awaitBlockedThenDone(t *testing.T, result <-chan error, release func()) {
	t.Helper()
	select {
	case err := <-result:
		t.Fatalf("the command did not wait for the held gate: %v", err)
	case <-time.After(400 * time.Millisecond):
	}
	release()
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-time.After(20 * time.Second):
		t.Fatal("the command did not complete after the gate was released")
	}
}

func TestGradeTransitionWorkflow_LockOrderSerializesConcurrentWriters(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newTransitionFixture(t, db)
	scene := newRolloverScene(t, f, ctx)
	workflow := f.workflow(t)

	t.Run("an apply waits behind a shared class-writes holder", func(t *testing.T) {
		// A create-student request holds the shared gate; the apply takes it
		// exclusively FIRST, so no child can be created into a mapped class
		// behind the cohort it is about to lock.
		held, release := holdLock(t, ctx, db, func(txCtx context.Context, tx bun.Tx) error {
			_, err := tx.NewRaw(`SELECT pg_advisory_xact_lock_shared(?, ?)`, int32(0x636c6173), int32(testpkg.Tenant(t))).Exec(txCtx)
			return err
		})
		<-held
		result := make(chan error, 1)
		go func() {
			_, err := workflow.Apply(ctx, scene.transitionID, "")
			result <- err
		}()
		awaitBlockedThenDone(t, result, release)
		assertApplied(t, ctx, db, scene)
	})

	t.Run("a draft edit waits behind the transition gate an apply or revert holds", func(t *testing.T) {
		draftID := f.createDraft(t, ctx, "2027-2028", promote("2a", "3a"))
		held, release := holdLock(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
			return f.deps.Structure.LockTransitions(txCtx)
		})
		<-held
		result := make(chan error, 1)
		go func() {
			_, err := workflow.UpdateDraft(ctx, draftID, gradetransition.DraftPatch{Mappings: []gradetransition.Mapping{promote("2a", "3b")}})
			result <- err
		}()
		awaitBlockedThenDone(t, result, release)
		updated, err := workflow.FindTransition(ctx, draftID)
		require.NoError(t, err)
		require.Len(t, updated.Mappings, 1)
		assert.Equal(t, "3b", *updated.Mappings[0].ToClass)
	})

	t.Run("a revert waits behind the recurrence gate a re-plan holds", func(t *testing.T) {
		held, release := holdLock(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
			return f.deps.LockRecurrenceWrites(txCtx)
		})
		<-held
		result := make(chan error, 1)
		go func() {
			_, err := workflow.Revert(ctx, scene.transitionID)
			result <- err
		}()
		awaitBlockedThenDone(t, result, release)
		assert.Equal(t, "reverted", transitionStatus(t, ctx, db, scene.transitionID))
	})
}

func TestGradeTransitionWorkflow_IdempotentRetryAndHistoryRetention(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newTransitionFixture(t, db)
	scene := newRolloverScene(t, f, ctx)
	workflow := f.workflow(t)

	preview, err := workflow.Preview(ctx, scene.transitionID)
	require.NoError(t, err)
	_, err = workflow.Apply(ctx, scene.transitionID, preview.Fingerprint)
	require.NoError(t, err)
	assertApplied(t, ctx, db, scene)

	// A repeated apply of the committed confirmation is a stale-state
	// conflict and changes nothing.
	_, err = workflow.Apply(ctx, scene.transitionID, preview.Fingerprint)
	require.ErrorIs(t, err, gradetransition.ErrTransitionNotDraft)
	assert.ErrorContains(t, err, "transition has already been applied")
	assertApplied(t, ctx, db, scene)

	// A draft edit or deletion of the applied transition is refused the same way.
	_, err = workflow.UpdateDraft(ctx, scene.transitionID, gradetransition.DraftPatch{Notes: testpkg.StrPtr("late edit")})
	require.ErrorIs(t, err, gradetransition.ErrTransitionNotDraft)
	require.ErrorIs(t, workflow.DeleteDraft(ctx, scene.transitionID), gradetransition.ErrTransitionNotDraft)

	history, err := workflow.History(ctx, scene.transitionID)
	require.NoError(t, err)
	require.Len(t, history, 2)
	states := map[int64]string{}
	for _, entry := range history {
		states[entry.StudentID] = entry.StudentState
	}
	assert.Equal(t, gradetransition.GraduateStateAlumnus, states[scene.graduate.ID])
	assert.Equal(t, gradetransition.GraduateStateRestored, states[scene.promoted.ID], "a promoted child is not an alumnus")

	result, err := workflow.Revert(ctx, scene.transitionID)
	require.NoError(t, err)
	assert.Equal(t, gradetransition.StatusReverted, result.Status)
	assert.Equal(t, 1, result.StudentsGraduated)
	assert.Equal(t, 1, result.StudentsPromoted)
	class, status := studentClassAndStatus(t, ctx, db, scene.graduate.ID)
	assert.Equal(t, "4a", class)
	assert.Equal(t, string(users.StudentStatusActive), status)
	class, _ = studentClassAndStatus(t, ctx, db, scene.promoted.ID)
	assert.Equal(t, "1a", class)
	assert.Equal(t, scene.tag, personTag(t, ctx, db, scene.graduate.PersonID), "the bracelet is handed back")
	assert.Equal(t, 1, countRows(t, ctx, db, `SELECT COUNT(*) FROM education.class_teachers WHERE staff_id = ? AND school_class = '1a'`, scene.staffID))
	assert.Equal(t, 1, countRows(t, ctx, db, `SELECT COUNT(*) FROM users.class_list_entries WHERE tenant_id = ? AND first_name = 'Rollover' AND last_name = 'Listed' AND school_class = '1a'`, testpkg.Tenant(t)))
	assert.Equal(t, 1, countRows(t, ctx, db, `SELECT COUNT(*) FROM schedule.instance_students WHERE instance_id = ? AND student_id = ?`, scene.instanceID, scene.graduate.ID), "the archived roster row is replayed")

	// History and both ledgers are retained after the revert; the
	// transition remembers who reverted it.
	assert.Equal(t, 2, countRows(t, ctx, db, `SELECT COUNT(*) FROM education.grade_transition_history WHERE transition_id = ?`, scene.transitionID))
	assert.Equal(t, 2, countRows(t, ctx, db, `SELECT COUNT(*) FROM education.grade_transition_class_teachers WHERE transition_id = ?`, scene.transitionID))
	assert.Equal(t, 2, countRows(t, ctx, db, `SELECT COUNT(*) FROM education.grade_transition_class_list_entries WHERE transition_id = ?`, scene.transitionID))
	reverted, err := workflow.FindTransition(ctx, scene.transitionID)
	require.NoError(t, err)
	require.NotNil(t, reverted.RevertedBy)
	assert.Equal(t, f.actorID, *reverted.RevertedBy)
	require.NotNil(t, reverted.AppliedBy)
	assert.Equal(t, f.actorID, *reverted.AppliedBy)

	// A repeated revert, or an apply of the reverted transition, is refused.
	_, err = workflow.Revert(ctx, scene.transitionID)
	require.ErrorIs(t, err, gradetransition.ErrTransitionNotApplied)
	assert.ErrorContains(t, err, "transition has already been reverted")
	_, err = workflow.Apply(ctx, scene.transitionID, "")
	require.ErrorIs(t, err, gradetransition.ErrTransitionNotDraft)
	assert.ErrorContains(t, err, "transition has been reverted")
	history, err = workflow.History(ctx, scene.transitionID)
	require.NoError(t, err)
	assert.Len(t, history, 2)
	for _, entry := range history {
		assert.Equal(t, gradetransition.GraduateStateRestored, entry.StudentState)
	}
}

// Local cutover evidence for #2711, not a production observation or a new
// checkpoint: preview, apply and revert latency, statement counts,
// unit-of-work outcomes and the deadlock counter over the real composition.
func TestGradeTransitionRuntimeEvidence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := newTransitionFixture(t, db)
	workflow := f.workflow(t)
	counter := testpkg.CaptureQueriesForContext(t, db)
	testpkg.AttachLockWaitEvidence(db)
	ctx, events := testpkg.CaptureUnitOfWorkEvidence(counter.Context(testpkg.Ctx(t)))
	var version string
	require.NoError(t, db.NewRaw("SHOW server_version").Scan(ctx, &version))
	deadlocks := func() int64 {
		var count int64
		require.NoError(t, db.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(ctx, &count))
		return count
	}
	beforeDeadlocks := deadlocks()
	samples := map[string][]testpkg.RuntimeCheckpointSample{}
	measure := func(operation string, iteration int, fn func() error) {
		counter.Reset()
		before := db.Stats()
		started := time.Now()
		err := fn()
		elapsed := time.Since(started)
		after := db.Stats()
		require.NoError(t, err)
		if iteration < 5 {
			return
		}
		writes := counter.WriteRows()
		rows, statements := counter.Rows()
		samples[operation] = append(samples[operation], testpkg.RuntimeCheckpointSample{
			DurationMS: float64(elapsed) / float64(time.Millisecond), Queries: counter.Total(),
			WriteRowsAffected: &writes, RowsAffected: rows, StatementsWithRows: statements,
			PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond),
		})
	}
	room := testpkg.CreateTestRoom(t, db, "Rollover runtime")
	for iteration := range 35 {
		year := fmt.Sprintf("%d-%d", 2100+iteration, 2101+iteration)
		fromClass := fmt.Sprintf("r%d", iteration)
		leaving := fmt.Sprintf("l%d", iteration)
		promoted := testpkg.CreateTestStudent(t, db, "Runtime", "Promoted", fromClass)
		graduated := testpkg.CreateTestStudent(t, db, "Runtime", "Graduate", leaving)
		instance := testpkg.CreateTestActivityInstance(t, db, f.today().AddDays(1), room.ID, testpkg.ActivityInstanceOpts{IsSpontaneous: true})
		testpkg.CreateTestInstanceStudent(t, db, instance.ID, graduated.ID, "")
		_ = promoted
		transitionID := f.createDraft(t, ctx, year, promote(fromClass, fromClass+"n"), graduate(leaving))
		var preview gradetransition.Preview
		measure("preview", iteration, func() error { var err error; preview, err = workflow.Preview(ctx, transitionID); return err })
		measure("apply", iteration, func() error { _, err := workflow.Apply(ctx, transitionID, preview.Fingerprint); return err })
		measure("revert", iteration, func() error { _, err := workflow.Revert(ctx, transitionID); return err })
	}
	raw, err := json.Marshal(map[string]any{"postgres": version, "warmup": 5, "samples_per_operation": 30, "concurrency": 1,
		"samples": samples, "unit_of_work_events_including_warmup": events(), "deadlocks": deadlocks() - beforeDeadlocks})
	require.NoError(t, err)
	t.Logf("grade-transition-runtime %s", raw)
}

// The fixture clock is a Berlin instant; keep the helper in use so the file
// documents which day the roster rows are planned for.
var _ = timezone.Berlin
