package contracttest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	enrollmentTest "github.com/moto-ogs/project-phoenix/modules/enrollment/enrollmenttest"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// A phase edit that shortens the end names the children already in care
// whose care now runs past it, with both days, and changes none of them
// (#6598, Option B): the school ends each care on purpose through the
// care-end process. The env's phase runs from 2026-09-01 to 2027-07-31; the
// edits run on 2026-08-24.

// activate makes the approved child active, as the activation tick does on
// the first day of care; its window stays the one the approval copied.
func activate(t *testing.T, env *decisionTestEnv, c *sourcedChild) {
	t.Helper()
	testpkg.SetStudentLifecycle(t, env.db, c.studentID, peopledirectory.StudentStatusActive, &c.start, &c.end)
}

func shortenPhase(t *testing.T, env *decisionTestEnv, endDays int) enrollmentTest.PhaseUpdateReport {
	t.Helper()
	report, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), env.db,
		boundsPhases(t, env, recurrenceGate(t, env)).UpdatePhase, movedSourcePhase(env, 0, endDays))
	require.NoError(t, err)
	return report
}

// careState reads the child's status and care end.
func careState(t *testing.T, env *decisionTestEnv, studentID int64) (string, string) {
	t.Helper()
	student := directoryStudent(t, env, studentID)
	return student.Status, student.EnrolledUntil
}

func directoryStudent(t *testing.T, env *decisionTestEnv, studentID int64) peopledirectory.Student {
	t.Helper()
	students, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), env.db,
		owners.MustNewPeopleDirectory(testpkg.NewDatabase(t, env.db)).ListStudentsByID, []int64{studentID})
	require.NoError(t, err)
	require.Len(t, students, 1)
	return students[0]
}

func reportedChildren(report enrollmentTest.PhaseUpdateReport) []int64 {
	ids := []int64{}
	for _, child := range report.CareBeyondEnd {
		ids = append(ids, child.StudentID)
	}
	return ids
}

// The issue's example: a child confirmed for a phase ending 31.07. and
// already in care, the phase shortened to 30.06. The edit names the child
// with its care end 31.07. and the new phase end, and leaves its status and
// care end alone. Saving again without a new cut names nobody, so repeated
// saves create no second task; extending the end names nobody either.
func TestPhaseShortening_NamesActiveChildAndLeavesItsCare(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	c := approveSourcedChild(t, env, "Gekuerzt")
	activate(t, env, c)
	// A child whose care ended on the old phase end is history.
	ended := approveSourcedChild(t, env, "Beendet")
	testpkg.SetStudentLifecycle(t, env.db, ended.studentID, peopledirectory.StudentStatusInactive, &ended.start, &ended.end)
	newEnd := c.end.AddDays(-31)

	report := shortenPhase(t, env, -31)
	require.Len(t, report.CareBeyondEnd, 1, "only the child in care is named")
	assert.Equal(t, enrollmentTest.CareBeyondPhaseEnd{
		StudentID: c.studentID, FirstName: "Gekuerzt", LastName: "Quelle", SchoolClass: directoryStudent(t, env, c.studentID).SchoolClass,
		CareEnd: c.end, PhaseEnd: newEnd,
	}, report.CareBeyondEnd[0])
	status, careEnd := careState(t, env, c.studentID)
	assert.Equal(t, peopledirectory.StudentStatusActive, status, "no silent status change")
	assert.Equal(t, c.end.String(), careEnd, "the care end stays until the school ends the care")
	assert.Equal(t, []bookingWindow{{c.start, newEnd.AddDays(1)}}, childBookings(t, env, c.childID), "the booking follows the phase (#6597)")

	assert.Empty(t, shortenPhase(t, env, -31).CareBeyondEnd, "saving the same end again names nobody")
	assert.Empty(t, shortenPhase(t, env, 0).CareBeyondEnd, "a longer end names nobody")
}

// A pending child follows the phase (#6505): its care end moves with the
// edit, so nothing runs past the new end.
func TestPhaseShortening_PendingChildFollowsAndIsNotNamed(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	c := approveSourcedChild(t, env, "Wartend")

	report := shortenPhase(t, env, -31)
	assert.NotContains(t, reportedChildren(report), c.studentID)
	status, careEnd := careState(t, env, c.studentID)
	assert.Equal(t, peopledirectory.StudentStatusPending, status)
	assert.Equal(t, c.end.AddDays(-31).String(), careEnd, "the pending child's window followed the phase")
}

// A care end confirmed while the edit waits at the recurrence gate is the
// state the edit reads: a child whose care now ends before the new phase
// end is not named.
func TestPhaseShortening_WaitsForAnOpenCareEndAndReadsIt(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 30*time.Second)
	defer cancel()
	f := newRestoreFlow(t, env, "Beendet")
	activate(t, env, f.sourcedChild)
	careEnd := f.start.AddDays(30)
	preview, err := testpkg.TenantQuery1(t, ctx, env.db, f.lifecycle.Preview, f.exitInput(careEnd))
	require.NoError(t, err)

	report := editWhileHolding(t, env, ctx, -31, func(holdCtx context.Context) error {
		_, err := f.workflow.Confirm(holdCtx, preview.Token, f.exitInput(careEnd), f.actorID)
		return err
	})
	assert.NotContains(t, reportedChildren(report), f.studentID, "the care already ends before the new phase end")
	_, stored := careState(t, env, f.studentID)
	assert.Equal(t, careEnd.String(), stored)
}

// An edit that is open when the school confirms a care end names the
// child. The confirmation waits for the edit and then refuses, because the
// edit moved the child's bookings since the preview; the school checks the
// new preview and ends the care on the new phase end.
func TestPhaseShortening_OpenEditNamesTheChildAndTheCareEndFollows(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 30*time.Second)
	defer cancel()
	f := newRestoreFlow(t, env, "Offen")
	activate(t, env, f.sourcedChild)
	newEnd := f.end.AddDays(-31)
	preview, err := testpkg.TenantQuery1(t, ctx, env.db, f.lifecycle.Preview, f.exitInput(newEnd))
	require.NoError(t, err)

	var report enrollmentTest.PhaseUpdateReport
	phases := boundsPhases(t, env, recurrenceGate(t, env))
	done := make(chan error, 1)
	holdErr := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(holdCtx context.Context, tx bun.Tx) error {
		if report, err = phases.UpdatePhase(holdCtx, movedSourcePhase(env, 0, -31)); err != nil {
			return err
		}
		pid := backendPID(t, holdCtx, tx)
		go func() {
			done <- testpkg.WithinTenantContext(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context) error {
				_, err := f.workflow.Confirm(txCtx, preview.Token, f.exitInput(newEnd), f.actorID)
				return err
			})
		}()
		if waiting, err := waitUntilBlockedBy(t, holdCtx, env.db, pid, done); !waiting {
			return errors.Join(errors.New("the care end did not wait for the open phase edit"), err)
		}
		return nil
	})
	require.NoError(t, holdErr)
	select {
	case err := <-done:
		require.ErrorIs(t, err, careplan.ErrCareExitPreviewChanged, "the preview from before the edit is stale")
	case <-ctx.Done():
		t.Fatal("the care end did not finish")
	}
	assert.Equal(t, []int64{f.studentID}, reportedChildren(report))
	_, stored := careState(t, env, f.studentID)
	assert.Equal(t, f.end.String(), stored, "nothing ended the care behind the school's back")

	f.endCareOn(t, newEnd)
	_, stored = careState(t, env, f.studentID)
	assert.Equal(t, newEnd.String(), stored, "the school ended the care on the new phase end")
	assert.Empty(t, shortenPhase(t, env, -31).CareBeyondEnd, "an ended care is not named again")
}

// editWhileHolding runs write in a transaction, keeps it open until the
// phase edit, started then, waits for it, and returns the edit's report.
func editWhileHolding(t *testing.T, env *decisionTestEnv, ctx context.Context, endDays int, write func(context.Context) error) enrollmentTest.PhaseUpdateReport {
	t.Helper()
	phases := boundsPhases(t, env, recurrenceGate(t, env))
	type result struct {
		report enrollmentTest.PhaseUpdateReport
		err    error
	}
	edit := make(chan result, 1)
	editDone := make(chan error, 1)
	holdErr := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(holdCtx context.Context, tx bun.Tx) error {
		if err := write(holdCtx); err != nil {
			return err
		}
		pid := backendPID(t, holdCtx, tx)
		go func() {
			var r result
			r.err = testpkg.WithinTenantContext(t, ctx, env.db, testpkg.Tenant(t), func(editCtx context.Context) error {
				var err error
				r.report, err = phases.UpdatePhase(editCtx, movedSourcePhase(env, 0, endDays))
				return err
			})
			edit <- r
			editDone <- r.err
		}()
		if waiting, err := waitUntilBlockedBy(t, holdCtx, env.db, pid, editDone); !waiting {
			return errors.Join(errors.New("the phase edit did not wait for the open write"), err)
		}
		return nil
	})
	require.NoError(t, holdErr)
	select {
	case r := <-edit:
		require.NoError(t, r.err, "the edit works on the committed write")
		return r.report
	case <-ctx.Done():
		t.Fatal("the phase edit did not finish")
		return enrollmentTest.PhaseUpdateReport{}
	}
}

// A Träger rollout that shortens the school's copy of a phase counts the
// children in care past the new end, as a school's own edit names them, and
// changes none of them (#6813, ADR 0211): the Träger office learns it in the
// preview and the result, the school ends each care on purpose.
func TestTraegerCopyShortening_CountsTheChildrenInCare(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	c := approveSourcedChild(t, env, "Traeger")
	activate(t, env, c)
	const sourceID = 9_000_001
	_, err := env.db.NewRaw(`UPDATE enrollment.phases SET traeger_source_id = ?, traeger_source_version = 1,
		traeger_values = jsonb_build_object('service_end_date', service_end_date::text) WHERE id = ?`,
		sourceID, env.sourcePhase.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	shortened := movedSourcePhase(env, 0, -31)
	active := true
	applied, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), env.db,
		boundsPhases(t, env, recurrenceGate(t, env)).TraegerCopies().ApplyTraegerPhase, enrollmentTest.TraegerPhaseCopy{
			SourceID: sourceID, Version: 2, Fields: enrollmentTest.TraegerPhaseFields{
				IsActive: &active, Name: shortened.Name, Kind: shortened.Kind,
				ServiceStartDate: shortened.ServiceStartDate, ServiceEndDate: shortened.ServiceEndDate,
				EnrollmentOpenAt: shortened.EnrollmentOpenAt, EnrollmentCloseAt: shortened.EnrollmentCloseAt,
				Audience: shortened.Audience, CareOverflowMode: shortened.CareOverflowMode,
				CareOfferingSelectionMode: shortened.CareOfferingSelectionMode,
			},
		})
	require.NoError(t, err)
	assert.Equal(t, env.sourcePhase.ID, applied.CopyID, "the rollout updates the existing copy")
	assert.Equal(t, 1, applied.CareBeyondEnd, "the child in care past the new end is counted")
	status, careEnd := careState(t, env, c.studentID)
	assert.Equal(t, peopledirectory.StudentStatusActive, status, "no silent status change")
	assert.Equal(t, c.end.String(), careEnd, "the care end stays until the school ends the care")
}
