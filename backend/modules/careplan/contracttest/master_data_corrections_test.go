package contracttest_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestMasterDataReview_CompanionEventOnlyOnEffectiveChange pins WHO an approval
// wakes: student_companions_changed makes every open "läuft mit" editor in the
// school mark itself stale and refuse to save, so it may only fire when the
// approval actually reconciled a link. The request's TARGET is not that
// question — a departure approval that drops no link (because the child has
// none, or because the affected weekdays keep allowing "Anderes Kind") must stay
// silent, or routine parent requests cost unrelated staff their unsaved edits.
func TestMasterDataReview_CompanionEventOnlyOnEffectiveChange(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	bc := testpkg.NewRecordingBroadcaster()
	svc := f.decisions(t, func(o *services.MasterDataDecisionTestOptions) { o.Broadcaster = bc })

	approve := func(t *testing.T, row *userModels.StudentDataChangeRequest) {
		t.Helper()
		bc.Reset()
		_, err := f.decide(t, svc, masterdatarequests.DecideInput{RequestID: row.ID, Approve: true, ReviewedBy: f.chain.AccountID})
		require.NoError(t, err)
	}

	t.Run("a departure approval without links stays silent", func(t *testing.T) {
		row := f.insert(t, userModels.DataChangeTargetDeparture, "allowed_departure_modes", `{}`, `{"mon":["bus"]}`)
		approve(t, row)
		assert.False(t, bc.HasEventType(realtime.EventStudentCompanionsChanged),
			"a plan change that trims no link changed no Laufgemeinschaft to announce")
		assert.True(t, bc.HasEventType(realtime.EventStudentUpdated),
			"the ordinary student_updated invalidation still has to fire")
	})

	t.Run("a departure approval that drops a link announces it", func(t *testing.T) {
		f.linkCompanionOnTuesday(t)

		// The approval is refused as stale unless it names the plan it starts
		// from, so read the live one the linking left behind.
		student, err := f.repos.Student.FindByID(testpkg.TenantContext(f.chain.TenantID), f.chain.StudentID)
		require.NoError(t, err)
		current, err := json.Marshal(student.AllowedDepartureModes)
		require.NoError(t, err)
		narrowed := userModels.AllowedDepartureModes{}
		for day, allowed := range student.AllowedDepartureModes {
			narrowed[day] = allowed
		}
		narrowed[userModels.PickupDayTuesday] = []userModels.DepartureMode{userModels.DepartureBus}
		requested, err := json.Marshal(narrowed)
		require.NoError(t, err)

		row := f.insert(t, userModels.DataChangeTargetDeparture, "allowed_departure_modes", string(current), string(requested))
		approve(t, row)
		assert.True(t, bc.HasEventType(realtime.EventStudentCompanionsChanged),
			"Tuesday no longer allows 'Anderes Kind', so the link is gone from the partner's card too")
	})
}

// linkCompanionOnTuesday gives the fixture's child a Laufgemeinschaft partner on
// Tuesday. Both children keep their own free-text "mit wem" note, so dropping
// the link later does not strand the partner (a different rule, with its own
// test).
func (f masterDataFixture) linkCompanionOnTuesday(t *testing.T) {
	t.Helper()
	ctx := testpkg.TenantContext(f.chain.TenantID)
	partner := testpkg.CreateTestStudent(t, f.db, "ReviewCompanion", "Partner", "1a")
	for _, studentID := range []int64{partner.ID, f.chain.StudentID} {
		student, err := f.repos.Student.FindByID(ctx, studentID)
		require.NoError(t, err)
		modes := userModels.AllowedDepartureModes{}
		for day, allowed := range student.AllowedDepartureModes {
			modes[day] = allowed
		}
		modes[userModels.PickupDayTuesday] = []userModels.DepartureMode{userModels.DepartureAccompanied}
		student.AllowedDepartureModes = modes
		student.DepartureDays = nil
		student.BusDays = nil
		student.PickupDays = nil
		note := "Nachbarskind"
		student.DepartureCompanionNote = &note
		require.NoError(t, f.repos.Student.Update(ctx, student))
	}
	edge, err := repositories.NewStudentCompanionEdge(f.chain.StudentID, partner.ID, 2)
	require.NoError(t, err)
	require.NoError(t, repositories.ReplaceStudentCompanions(ctx, repositories.NewStudentCompanionRepository(f.repos.CarePlan()), f.chain.StudentID, []*userModels.StudentCompanion{edge}))
}

func (f masterDataFixture) personFirstName(t *testing.T) string {
	t.Helper()
	var name string
	require.NoError(t, f.db.NewSelect().TableExpr("users.persons").ColumnExpr("first_name").
		Where("id = ?", f.chain.PersonID).Scan(testpkg.Ctx(t), &name))
	return name
}

func (f masterDataFixture) correct(t *testing.T, svc masterdatarequests.Decisions, requestID int64, approve bool, reason string) error {
	t.Helper()
	return f.inTx(t, func(ctx context.Context) error {
		return svc.Correct(ctx, requestID, approve, "", reason, f.chain.AccountID)
	})
}

// #2267 A11: an approval turned into a rejection has to put the child's name
// back, otherwise the correction only changes the paperwork.
func TestMasterDataCorrect_ApprovedToRejectedRestoresTheOldValue(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	svc := f.decisions(t)
	row := f.insert(t, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)

	_, err := f.decide(t, svc, masterdatarequests.DecideInput{RequestID: row.ID, Approve: true, ReviewedBy: f.chain.AccountID})
	require.NoError(t, err)
	require.Equal(t, "Maximilian", f.personFirstName(t))

	require.NoError(t, f.correct(t, svc, row.ID, false, "Falsch entschieden"))
	assert.Equal(t, "Felix", f.personFirstName(t))
	assert.Equal(t, userModels.DataChangeStatusRejected, f.status(t, row.ID))
}

// If the office changed the field after the approval, the value on record is
// newer than this request. Overwriting it would discard an edit nobody can
// recover, so the correction refuses and names what is there now.
func TestMasterDataCorrect_RefusesWhenTheValueMovedOnAfterTheDecision(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	svc := f.decisions(t)
	row := f.insert(t, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)

	_, err := f.decide(t, svc, masterdatarequests.DecideInput{RequestID: row.ID, Approve: true, ReviewedBy: f.chain.AccountID})
	require.NoError(t, err)
	_, err = f.db.NewUpdate().TableExpr("users.persons").Set("first_name = ?", "Moritz").
		Where("id = ?", f.chain.PersonID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	err = f.correct(t, svc, row.ID, false, "Korrektur")
	require.ErrorIs(t, err, parentrequests.ErrCorrectionUnsupported)
	assert.Contains(t, err.Error(), "Moritz", "the message must name the value that is there now")
	assert.Equal(t, "parent requests: decision cannot be corrected: der Wert wurde nach der Entscheidung auf Moritz geändert", err.Error(),
		"the students route renders this text verbatim")
	assert.Equal(t, "Moritz", f.personFirstName(t), "the newer value must survive")
}

// A request nobody decided yet has nothing to correct.
func TestMasterDataCorrect_RefusesAnUndecidedRequest(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	row := f.insert(t, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)

	err := f.correct(t, f.decisions(t), row.ID, false, "Korrektur")
	require.ErrorIs(t, err, parentrequests.ErrNotDecided)
}

// Graduated children drop out of the master-data review surface (#405 review).
//
// Graduation is a soft delete: the guardian link and the pending request
// survive it. A departed child's request must not stay reviewable — approving
// it would rewrite an alumnus' name, departure modes or companion links.
func TestMasterDataReview_GraduatedChildLeavesQueueAndRefusesDecisions(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	svc := f.decisions(t)
	queue := f.queue(t, repositories.SchoolWideReviewScope)
	row := f.insert(t, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)

	require.NoError(t, f.inTx(t, func(ctx context.Context) error {
		items, _, err := queue.ListPending(ctx, careplan.RequestQueueFilter{})
		require.NoError(t, err)
		require.Len(t, items, 1, "the pending request must start out visible")
		return nil
	}))

	// The child graduates (grade-transition apply).
	_, err := f.db.NewUpdate().
		TableExpr("users.student_school_memberships").
		Set("status = ?", string(userModels.StudentStatusAlumnus)).
		Where("student_profile_id = ? AND deleted_at IS NULL", f.chain.StudentID).
		Exec(context.Background())
	require.NoError(t, err)

	require.NoError(t, f.inTx(t, func(ctx context.Context) error {
		items, _, listErr := queue.ListPending(ctx, careplan.RequestQueueFilter{})
		require.NoError(t, listErr)
		assert.Empty(t, items, "a graduated child's request must leave the queue and the badge")

		// Neither direction may still act on the alumnus.
		_, decideErr := svc.Decide(ctx, masterdatarequests.DecideInput{RequestID: row.ID, Approve: true, ReviewedBy: f.chain.AccountID})
		assert.ErrorIs(t, decideErr, masterdatarequests.ErrReviewNotFound)
		_, decideErr = svc.Decide(ctx, masterdatarequests.DecideInput{RequestID: row.ID, Approve: false, Reason: "child has left", ReviewedBy: f.chain.AccountID})
		assert.ErrorIs(t, decideErr, masterdatarequests.ErrReviewNotFound)
		return nil
	}))

	// The request is untouched, so a transition revert restores a coherent state.
	assert.Equal(t, userModels.DataChangeStatusPending, f.status(t, row.ID))
}

// TestMasterDataReview_ListHistory proves the decided-request history:
// decided rows (incl. auto_applied) come back newest-decision-first with the
// child's and the reviewer's names, pending rows stay out, and the keyset
// cursor pages without overlap.
func TestMasterDataReview_ListHistory(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	queue := f.queue(t, repositories.SchoolWideReviewScope)
	_, reviewerAccount := testpkg.CreateTestStaffWithAccount(t, f.db, "Rieke", "Reviewer")

	// One row per terminal state, plus a pending row that must never surface.
	rejected := f.insert(t, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)
	autoApplied := f.insert(t, userModels.DataChangeTargetPerson, "last_name", `"Schneider"`, `"Schmidt"`)
	pending := f.insert(t, userModels.DataChangeTargetPerson, "language_preference", `"de"`, `"en"`)

	decided, err := f.decide(t, f.decisions(t), masterdatarequests.DecideInput{
		RequestID: rejected.ID, Approve: false, Reason: "unpassend", ReviewedBy: reviewerAccount.ID,
	})
	require.NoError(t, err)
	autoAppliedAt := decided.Request.UpdatedAt.Add(time.Second)
	// The auto-applied row never went through a decision — flip it directly,
	// like the auto-apply path does (no reviewer).
	_, err = f.db.NewUpdate().TableExpr("users.student_data_change_requests").
		Set("status = ?", userModels.DataChangeStatusAutoApplied).
		Set("applied_at = ?", autoAppliedAt).
		Set("updated_at = ?", autoAppliedAt).
		Where("id = ?", autoApplied.ID).
		Exec(context.Background())
	require.NoError(t, err)

	require.NoError(t, f.inTx(t, func(ctx context.Context) error {
		items, next, listErr := queue.ListHistory(ctx, careplan.RequestQueueFilter{Limit: 25})
		require.NoError(t, listErr)
		assert.Nil(t, next, "one page must not report more")

		byID := map[int64]*careplan.MasterDataHistoryItem{}
		for _, item := range items {
			byID[item.Request.ID] = item
			assert.NotEqual(t, pending.ID, item.Request.ID, "pending rows must never appear in the history")
		}
		require.Contains(t, byID, rejected.ID)
		require.Contains(t, byID, autoApplied.ID)

		rej := byID[rejected.ID]
		assert.Equal(t, "Felix", rej.FirstName)
		assert.Equal(t, "Schneider", rej.LastName)
		assert.Equal(t, "Rieke Reviewer", rej.ReviewerName)
		require.NotNil(t, rej.Request.ReviewReason)
		assert.Equal(t, "unpassend", *rej.Request.ReviewReason)

		auto := byID[autoApplied.ID]
		assert.Equal(t, userModels.DataChangeStatusAutoApplied, auto.Request.Status)
		assert.Empty(t, auto.ReviewerName, "auto-applied rows carry no reviewer")
		// The auto-applied row was stamped one second later, so it must lead.
		assert.Equal(t, autoApplied.ID, items[0].Request.ID, "newest decision first")

		// Keyset pagination: page size 1 → two pages, disjoint, then done.
		page1, next1, listErr := queue.ListHistory(ctx, careplan.RequestQueueFilter{Limit: 1})
		require.NoError(t, listErr)
		require.Len(t, page1, 1)
		require.NotNil(t, next1)
		page2, next2, listErr := queue.ListHistory(ctx, careplan.RequestQueueFilter{BeforeInstant: next1.UpdatedAt, BeforeID: next1.ID, Limit: 1})
		require.NoError(t, listErr)
		require.Len(t, page2, 1)
		assert.NotEqual(t, page1[0].Request.ID, page2[0].Request.ID, "pages must not overlap")
		if next2 != nil {
			page3, next3, listErr := queue.ListHistory(ctx, careplan.RequestQueueFilter{BeforeInstant: next2.UpdatedAt, BeforeID: next2.ID, Limit: 1})
			require.NoError(t, listErr)
			assert.Empty(t, page3)
			assert.Nil(t, next3)
		}
		return nil
	}))
}

// TestMasterDataReview_ListHistoryScopedToWritableChildren proves the history
// applies the same per-child review scope as the open queue.
func TestMasterDataReview_ListHistoryScopedToWritableChildren(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	row := f.insert(t, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)
	_, err := f.decide(t, f.decisions(t), masterdatarequests.DecideInput{RequestID: row.ID, Approve: false, Reason: "nein"})
	require.NoError(t, err)

	require.NoError(t, f.inTx(t, func(ctx context.Context) error {
		items, _, listErr := f.queue(t, noReviewScope).ListHistory(ctx, careplan.RequestQueueFilter{Limit: 25})
		require.NoError(t, listErr)
		for _, item := range items {
			assert.NotEqual(t, row.ID, item.Request.ID, "a caller who cannot review the child must not see its decided request")
		}
		return nil
	}))
}
