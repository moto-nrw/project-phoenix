package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The excused-absence request workflow (#3093) over the real Care Plan
// tables. The calendar is pinned so no assertion depends on the wall clock;
// only reported_at instants, which the workflow compares with created_at,
// use time.Now.
var (
	excusedToday    = careplan.Date("2026-08-24")
	excusedTomorrow = careplan.Date("2026-08-25")
	excusedIn3Days  = careplan.Date("2026-08-27")
	excusedIn4Days  = careplan.Date("2026-08-28")
	excusedIn6Days  = careplan.Date("2026-08-30")
	excusedPast     = careplan.Date("2026-08-20")
)

type excusedFixture struct {
	db       *bun.DB
	repos    *repositories.Factory
	chain    testpkg.ParentChain
	requests careplan.ExcusedAbsenceRequests
}

// newExcusedFixture composes the workflow over the legacy repository graph
// with a school-wide review scope and the pinned calendar.
func newExcusedFixture(t *testing.T) *excusedFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	return &excusedFixture{db: db, repos: repos, chain: testpkg.CreateTestParentGuardianChain(t, db), requests: newExcusedRequests(t, repos, repos.CarePlan())}
}

func newExcusedRequests(t *testing.T, repos *repositories.Factory, capability careplan.Capability) careplan.ExcusedAbsenceRequests {
	t.Helper()
	requests, err := repositories.NewExcusedAbsenceRequests(repositories.ExcusedRequestWiring{
		CarePlan: capability, Students: repos.Student, Persons: repos.Person,
		Scope: repositories.SchoolWideReviewScope, Today: func() careplan.Date { return excusedToday },
	})
	require.NoError(t, err)
	return requests
}

func (f *excusedFixture) inTenant(t *testing.T, fn func(ctx context.Context) error) error {
	t.Helper()
	return testpkg.WithTenantTx(t, context.Background(), f.db, f.chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

func (f *excusedFixture) createPending(t *testing.T, dates []careplan.Date, note string) *careplan.ExcusedAbsenceRequest {
	t.Helper()
	return f.createPendingStatus(t, dates, note, careplan.StudentStatusDayExcused)
}

func (f *excusedFixture) createPendingStatus(t *testing.T, dates []careplan.Date, note, status string) *careplan.ExcusedAbsenceRequest {
	t.Helper()
	var req *careplan.ExcusedAbsenceRequest
	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		var err error
		req, err = f.requests.CreateRequestForStatus(ctx, f.chain.StudentID, f.chain.AccountID, dates, note, status)
		return err
	}))
	require.NotNil(t, req)
	return req
}

func (f *excusedFixture) decide(t *testing.T, input careplan.ExcusedRequestDecideInput) (*careplan.ExcusedRequestReviewItem, error) {
	t.Helper()
	var item *careplan.ExcusedRequestReviewItem
	err := f.inTenant(t, func(ctx context.Context) error {
		var decideErr error
		item, decideErr = f.requests.Decide(ctx, input)
		return decideErr
	})
	return item, err
}

func (f *excusedFixture) approve(t *testing.T, requestID int64) {
	t.Helper()
	_, err := f.decide(t, careplan.ExcusedRequestDecideInput{RequestID: requestID, Approve: true, ReviewedBy: f.chain.AccountID})
	require.NoError(t, err)
}

func (f *excusedFixture) activeStatusDays(t *testing.T) []careplan.StudentStatusDay {
	t.Helper()
	rows, err := f.repos.CarePlan().ListStudentStatusDays(testpkg.TenantContext(f.chain.TenantID), careplan.StudentStatusDayFilter{
		StudentIDs: []int64{f.chain.StudentID}, ActiveOnly: true,
	})
	require.NoError(t, err)
	return rows
}

func (f *excusedFixture) requestStatus(t *testing.T, requestID int64) string {
	t.Helper()
	row, err := f.repos.CarePlan().FindExcusedAbsenceRequest(testpkg.TenantContext(f.chain.TenantID), requestID, false)
	require.NoError(t, err)
	return row.Status
}

// upsertStatusDay positions an active status relative to a request's
// created_at, so the approval-conflict tests can say "newer" or "older".
func (f *excusedFixture) upsertStatusDay(t *testing.T, date careplan.Date, status string, reportedAt time.Time) {
	t.Helper()
	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		_, err := f.repos.CarePlan().UpsertStudentStatusDay(ctx, careplan.StudentStatusDay{
			StudentID: f.chain.StudentID, Date: date, Status: status, ReportedAt: reportedAt, Source: careplan.StudentStatusSourceManual,
		})
		return err
	}))
}

// createManualPartialAbsence records a staff-entered partial-day excusal,
// the kind that conflicts with a full-day request.
func (f *excusedFixture) createManualPartialAbsence(t *testing.T, date careplan.Date) {
	t.Helper()
	staff := testpkg.CreateTestStaff(t, f.db, "Partial", "Author")
	from := time.Date(2000, time.January, 1, 13, 30, 0, 0, time.UTC)
	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		_, err := f.repos.CarePlan().CreatePickupException(ctx, careplan.PickupException{
			StudentID: f.chain.StudentID, ExceptionDate: date, PickupTime: &from, ExcusedFrom: &from,
			ExcusedCreatedBy: &staff.ID, ExcusedOwnsPickupTime: true, Source: "staff", CreatedBy: staff.ID,
		})
		return err
	}))
}

func TestExcusedRequestCreateDistinguishesSickAndExcusedRetries(t *testing.T) {
	t.Parallel()
	f := newExcusedFixture(t)

	sick := f.createPendingStatus(t, []careplan.Date{excusedTomorrow}, "Fieber", careplan.StudentStatusDaySick)
	assert.Equal(t, careplan.StudentStatusDaySick, sick.AbsenceStatus)
	retried := f.createPendingStatus(t, []careplan.Date{excusedTomorrow}, "Fieber", careplan.StudentStatusDaySick)
	assert.Equal(t, sick.ID, retried.ID, "an identical sick request must be idempotent")

	err := f.inTenant(t, func(ctx context.Context) error {
		_, err := f.requests.CreateRequestForStatus(ctx, f.chain.StudentID, f.chain.AccountID, []careplan.Date{excusedTomorrow}, "Arzttermin", careplan.StudentStatusDayExcused)
		return err
	})
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestOverlap, "the same day cannot have pending sick and excused requests")
}

func TestExcusedRequestCreateIdempotentOverlapDisjoint(t *testing.T) {
	t.Parallel()
	f := newExcusedFixture(t)
	first := f.createPending(t, []careplan.Date{excusedTomorrow, excusedIn3Days}, "Arzttermin")
	second := f.createPending(t, []careplan.Date{excusedIn3Days, excusedTomorrow}, "nochmal")
	assert.Equal(t, first.ID, second.ID, "an identical resubmit in another order must be idempotent")

	err := f.inTenant(t, func(ctx context.Context) error {
		_, err := f.requests.CreateRequest(ctx, f.chain.StudentID, f.chain.AccountID, []careplan.Date{excusedIn3Days, excusedIn6Days}, "ueberschneidung")
		return err
	})
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestOverlap)

	third := f.createPending(t, []careplan.Date{excusedIn6Days}, "anderer Tag")
	assert.NotEqual(t, first.ID, third.ID)
	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		items, _, err := f.requests.ListPending(ctx, careplan.RequestQueueFilter{})
		require.NoError(t, err)
		assert.Len(t, items, 2, "only the deduplicated and the disjoint request remain pending")
		return nil
	}))
}

func TestExcusedRequestQueueBadgeAndParentList(t *testing.T) {
	t.Parallel()
	f := newExcusedFixture(t)
	pending := f.createPending(t, []careplan.Date{excusedTomorrow}, "bleibt sichtbar")
	rejected := f.createPending(t, []careplan.Date{excusedIn3Days}, "wird abgelehnt")
	withdrawn := f.createPending(t, []careplan.Date{excusedIn6Days}, "wird zurueckgezogen")

	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		items, _, err := f.requests.ListPending(ctx, careplan.RequestQueueFilter{})
		require.NoError(t, err)
		require.Len(t, items, 3, "every pending request surfaces in the staff queue")
		for _, item := range items {
			assert.Equal(t, f.chain.StudentID, item.Request.StudentID)
			assert.Equal(t, "Felix", item.FirstName, "child name must be enriched")
			assert.Equal(t, "Schneider", item.LastName)
			assert.True(t, item.BulkEligible)
		}
		badges, err := f.requests.PendingByStudentForDate(ctx, excusedIn3Days)
		require.NoError(t, err)
		require.Contains(t, badges, f.chain.StudentID)
		assert.Equal(t, rejected.ID, badges[f.chain.StudentID].ID)
		miss, err := f.requests.PendingByStudentForDate(ctx, excusedPast)
		require.NoError(t, err)
		assert.Empty(t, miss, "a day no request covers yields no badge")
		return nil
	}))

	// Reject one through the workflow; historic withdrawn rows are written
	// straight through the owner, guardians edit instead of withdrawing now.
	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		if _, err := f.requests.Decide(ctx, careplan.ExcusedRequestDecideInput{RequestID: rejected.ID, Approve: false, Reason: "telefonisch klaeren", ReviewedBy: f.chain.AccountID}); err != nil {
			return err
		}
		return f.repos.CarePlan().DecideExcusedAbsenceRequest(ctx, careplan.ExcusedAbsenceDecision{ID: withdrawn.ID, Status: careplan.ExcusedRequestStatusWithdrawn})
	}))

	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		rows, err := f.requests.ListForStudent(ctx, f.chain.StudentID, time.Now().Add(-24*time.Hour))
		require.NoError(t, err)
		statuses := map[int64]string{}
		for _, row := range rows {
			statuses[row.ID] = row.Status
		}
		assert.Equal(t, careplan.ExcusedRequestStatusPending, statuses[pending.ID], "pending stays visible")
		assert.Equal(t, careplan.ExcusedRequestStatusRejected, statuses[rejected.ID], "rejected stays visible so the parent learns the outcome")
		assert.NotContains(t, statuses, withdrawn.ID, "a withdrawn request must not appear in the parent list")
		items, _, err := f.requests.ListPending(ctx, careplan.RequestQueueFilter{})
		require.NoError(t, err)
		assert.Len(t, items, 1)
		return nil
	}))
}

func TestExcusedRequestGuardianEdit(t *testing.T) {
	t.Parallel()
	f := newExcusedFixture(t)
	pending := f.createPending(t, []careplan.Date{excusedTomorrow}, "Arzttermin")

	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		_, err := f.requests.EditRequest(ctx, careplan.ExcusedRequestEditInput{
			RequestID: pending.ID, StudentID: f.chain.StudentID, GuardianAccountID: f.chain.AccountID + 1,
			Dates: []careplan.Date{excusedIn3Days}, Note: "fremd", NoteRequired: true,
		})
		assert.ErrorIs(t, err, careplan.ErrExcusedRequestNotFound, "a stranger must not learn that the id exists")
		_, err = f.requests.EditRequest(ctx, careplan.ExcusedRequestEditInput{
			RequestID: pending.ID, StudentID: f.chain.StudentID, GuardianAccountID: f.chain.AccountID,
			ExpectedVersion: "stale", Dates: []careplan.Date{excusedIn3Days}, Note: "neu", NoteRequired: true,
		})
		assert.ErrorIs(t, err, careplan.ErrParentRequestStale)
		edited, err := f.requests.EditRequest(ctx, careplan.ExcusedRequestEditInput{
			RequestID: pending.ID, StudentID: f.chain.StudentID, GuardianAccountID: f.chain.AccountID,
			ExpectedVersion: careplan.ParentRequestVersion(pending.UpdatedAt), Dates: []careplan.Date{excusedIn3Days}, Note: "neuer Termin", NoteRequired: true,
		})
		require.NoError(t, err)
		assert.Equal(t, pending.ID, edited.ID, "the request keeps its id")
		assert.Equal(t, []careplan.Date{excusedIn3Days}, edited.Dates)
		assert.Equal(t, "neuer Termin", edited.Note)
		return nil
	}))
}

func TestExcusedRequestDecideGuards(t *testing.T) {
	t.Parallel()
	f := newExcusedFixture(t)
	pending := f.createPending(t, []careplan.Date{excusedTomorrow}, "Privater Termin")

	_, err := f.decide(t, careplan.ExcusedRequestDecideInput{RequestID: pending.ID, Approve: true, ExpectedVersion: "stale"})
	assert.ErrorIs(t, err, careplan.ErrParentRequestStale)
	assert.Empty(t, f.activeStatusDays(t), "a refused decision writes nothing")

	_, err = f.decide(t, careplan.ExcusedRequestDecideInput{RequestID: pending.ID + 999999, Approve: true})
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestNotFound)

	f.approve(t, pending.ID)
	_, err = f.decide(t, careplan.ExcusedRequestDecideInput{RequestID: pending.ID, Approve: true})
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestNotPending, "a second decision finds the row decided")
}

func TestExcusedRequestApproveRejectsDatesAfterPlannedCareEnd(t *testing.T) {
	t.Parallel()
	f := newExcusedFixture(t)
	pending := f.createPending(t, []careplan.Date{excusedIn4Days}, "Termin nach Betreuungsende")
	_, err := f.db.NewUpdate().TableExpr("users.students").Set("enrolled_until = ?", excusedIn3Days.String()).
		Where("id = ?", f.chain.StudentID).Exec(context.Background())
	require.NoError(t, err)

	_, err = f.decide(t, careplan.ExcusedRequestDecideInput{RequestID: pending.ID, Approve: true, ReviewedBy: f.chain.AccountID})
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestNotFound)
	assert.Empty(t, f.activeStatusDays(t), "approval must not create an absence after care ends")
}

func TestExcusedRequestApproveWritesDaysAndClearsLiveSickToday(t *testing.T) {
	t.Parallel()
	f := newExcusedFixture(t)
	_, err := f.db.NewUpdate().Table("users.students").Set("sick = ?", true).Set("sick_since = ?", time.Now()).
		Where("id = ?", f.chain.StudentID).Exec(context.Background())
	require.NoError(t, err)
	pending := f.createPending(t, []careplan.Date{excusedToday, excusedTomorrow}, "krank gemeldet, jetzt entschuldigt")

	item, err := f.decide(t, careplan.ExcusedRequestDecideInput{RequestID: pending.ID, Approve: true, Reason: "Mit der Leitung abgesprochen", ReviewedBy: f.chain.AccountID})
	require.NoError(t, err)
	assert.Equal(t, careplan.ExcusedRequestStatusApproved, item.Request.Status)
	require.NotNil(t, item.Request.DecisionReason, "an approval reason survives into the history")
	assert.Equal(t, "Mit der Leitung abgesprochen", *item.Request.DecisionReason)
	assert.Equal(t, "Felix", item.FirstName)

	days := f.activeStatusDays(t)
	require.Len(t, days, 2, "one parent-sourced excused day per requested date")
	for _, day := range days {
		assert.Equal(t, careplan.StudentStatusDayExcused, day.Status)
		assert.Equal(t, careplan.StudentStatusSourceParent, day.Source)
		require.NotNil(t, day.GuardianAccountID)
		assert.Equal(t, f.chain.AccountID, *day.GuardianAccountID)
	}
	var sick *bool
	require.NoError(t, f.db.NewSelect().Table("users.students").Column("sick").Where("id = ?", f.chain.StudentID).Scan(context.Background(), &sick))
	require.NotNil(t, sick)
	assert.False(t, *sick, "approving an excused request that includes today clears the live sick flag")
}

func TestExcusedRequestApproveRefusedWhenNewerStatusExists(t *testing.T) {
	t.Parallel()
	f := newExcusedFixture(t)
	pending := f.createPending(t, []careplan.Date{excusedIn3Days}, "Arzttermin")
	f.upsertStatusDay(t, excusedIn3Days, careplan.StudentStatusDaySick, time.Now().Add(time.Hour))

	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		items, _, err := f.requests.ListPending(ctx, careplan.RequestQueueFilter{})
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.False(t, items[0].BulkEligible)
		assert.Equal(t, careplan.BulkIneligibleStale, items[0].BulkIneligibleReason)
		assert.Equal(t, careplan.StudentStatusDaySick, items[0].CurrentStatusByDate[excusedIn3Days.String()])
		return nil
	}))
	_, err := f.decide(t, careplan.ExcusedRequestDecideInput{RequestID: pending.ID, Approve: true, ReviewedBy: f.chain.AccountID})
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestStatusConflict)
	assert.Equal(t, careplan.ExcusedRequestStatusPending, f.requestStatus(t, pending.ID))

	item, err := f.decide(t, careplan.ExcusedRequestDecideInput{RequestID: pending.ID, Approve: false, Reason: "bitte klaeren", ReviewedBy: f.chain.AccountID})
	require.NoError(t, err, "rejecting the stale request still works")
	assert.Equal(t, careplan.ExcusedRequestStatusRejected, item.Request.Status)
}

func TestExcusedRequestApproveOverwritesOlderStatus(t *testing.T) {
	t.Parallel()
	f := newExcusedFixture(t)
	f.upsertStatusDay(t, excusedIn3Days, careplan.StudentStatusDaySick, time.Now().Add(-24*time.Hour))
	pending := f.createPending(t, []careplan.Date{excusedIn3Days}, "Arzttermin")
	f.approve(t, pending.ID)
	days := f.activeStatusDays(t)
	require.Len(t, days, 1, "the older sick day is cleared, the excused day stands")
	assert.Equal(t, careplan.StudentStatusDayExcused, days[0].Status)
}

func TestExcusedRequestPartialAbsenceConflicts(t *testing.T) {
	t.Parallel()
	f := newExcusedFixture(t)
	pending := f.createPending(t, []careplan.Date{excusedIn3Days}, "Arzttermin")
	f.createManualPartialAbsence(t, excusedIn3Days)
	f.createManualPartialAbsence(t, excusedIn4Days)

	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		items, _, err := f.requests.ListPending(ctx, careplan.RequestQueueFilter{})
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, careplan.BulkIneligibleConflict, items[0].BulkIneligibleReason)
		return nil
	}))
	_, err := f.decide(t, careplan.ExcusedRequestDecideInput{RequestID: pending.ID, Approve: true, ReviewedBy: f.chain.AccountID})
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestStatusConflict)
	assert.Empty(t, f.activeStatusDays(t))

	err = f.inTenant(t, func(ctx context.Context) error {
		_, err := f.requests.CreateRequest(ctx, f.chain.StudentID, f.chain.AccountID, []careplan.Date{excusedIn4Days}, "Arzttermin")
		return err
	})
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestStatusConflict, "a partial absence blocks the creation too")
}

func TestExcusedRequestPastRequestsMarkDoneInsteadOfApprove(t *testing.T) {
	t.Parallel()
	f := newExcusedFixture(t)
	past := f.createPending(t, []careplan.Date{excusedPast}, "Rückwirkend")
	future := f.createPending(t, []careplan.Date{excusedTomorrow}, "Kommende Woche")

	_, err := f.decide(t, careplan.ExcusedRequestDecideInput{RequestID: past.ID, Approve: true, ReviewedBy: f.chain.AccountID})
	assert.ErrorIs(t, err, careplan.ErrParentRequestPast)
	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		assert.ErrorIs(t, f.requests.MarkDone(ctx, future.ID, "", "", f.chain.AccountID), careplan.ErrParentRequestNotPast)
		assert.ErrorIs(t, f.requests.MarkDone(ctx, past.ID, "stale", "", f.chain.AccountID), careplan.ErrParentRequestStale)
		return f.requests.MarkDone(ctx, past.ID, "", "Tage sind vorbei", f.chain.AccountID)
	}))
	assert.Equal(t, careplan.ExcusedRequestStatusDone, f.requestStatus(t, past.ID))
	assert.Empty(t, f.activeStatusDays(t), "a done request never writes a status day")
}

func TestExcusedRequestCorrections(t *testing.T) {
	t.Parallel()
	f := newExcusedFixture(t)
	undecided := f.createPending(t, []careplan.Date{excusedIn6Days}, "Offen")
	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		assert.ErrorIs(t, f.requests.Correct(ctx, undecided.ID, false, "", "Korrektur", f.chain.AccountID), careplan.ErrParentRequestNotDecided)
		return nil
	}))

	approved := f.createPending(t, []careplan.Date{excusedIn3Days}, "Familienfeier")
	f.approve(t, approved.ID)
	require.Len(t, f.activeStatusDays(t), 1)
	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		return f.requests.Correct(ctx, approved.ID, false, "", "Doch nicht genehmigt", f.chain.AccountID)
	}))
	assert.Empty(t, f.activeStatusDays(t), "turning an approval into a rejection undoes the days it wrote")
	assert.Equal(t, careplan.ExcusedRequestStatusRejected, f.requestStatus(t, approved.ID))

	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		return f.requests.Correct(ctx, approved.ID, true, "", "Doch genehmigt", f.chain.AccountID)
	}))
	assert.Len(t, f.activeStatusDays(t), 1, "the other direction re-runs the approve path")

	// Staff re-entered the day themselves after the approval: the newer entry
	// belongs to them and must survive a correction.
	_, err := f.db.NewUpdate().TableExpr("active.student_status_days").
		Set("source = ?", careplan.StudentStatusSourceManual).Set("reported_at = now() + interval '1 hour'").
		Where("student_id = ?", f.chain.StudentID).Where("cleared_at IS NULL").Exec(context.Background())
	require.NoError(t, err)
	err = f.inTenant(t, func(ctx context.Context) error {
		return f.requests.Correct(ctx, approved.ID, false, "", "Korrektur", f.chain.AccountID)
	})
	assert.ErrorIs(t, err, careplan.ErrParentRequestCorrectionUnsupported)
	assert.Len(t, f.activeStatusDays(t), 1, "the newer entry must survive")
}

func TestExcusedRequestHistoryListsDecidedRowsWithNamesAndPages(t *testing.T) {
	t.Parallel()
	f := newExcusedFixture(t)
	_, staffAccount := testpkg.CreateTestStaffWithAccount(t, f.db, "Erika", "Entscheider")
	rejected := f.createPending(t, []careplan.Date{excusedIn3Days}, "Zahnarzt")
	withdrawn := f.createPending(t, []careplan.Date{excusedIn4Days}, "Familienfeier")
	pending := f.createPending(t, []careplan.Date{excusedIn6Days}, "Ausflug")

	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		if _, err := f.requests.Decide(ctx, careplan.ExcusedRequestDecideInput{RequestID: rejected.ID, Approve: false, Reason: "bitte anrufen", ReviewedBy: staffAccount.ID}); err != nil {
			return err
		}
		return f.repos.CarePlan().DecideExcusedAbsenceRequest(ctx, careplan.ExcusedAbsenceDecision{ID: withdrawn.ID, Status: careplan.ExcusedRequestStatusWithdrawn})
	}))

	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		items, next, err := f.requests.ListHistory(ctx, careplan.RequestQueueFilter{Limit: 25})
		require.NoError(t, err)
		assert.Nil(t, next)
		byID := map[int64]*careplan.ExcusedRequestHistoryItem{}
		for _, item := range items {
			byID[item.Request.ID] = item
			assert.NotEqual(t, pending.ID, item.Request.ID, "pending rows never appear in the history")
		}
		require.Contains(t, byID, rejected.ID)
		require.Contains(t, byID, withdrawn.ID)
		assert.Equal(t, "Felix", byID[rejected.ID].FirstName)
		assert.Equal(t, "Erika Entscheider", byID[rejected.ID].ReviewerName)
		require.NotNil(t, byID[rejected.ID].Request.DecisionReason)
		assert.Equal(t, "bitte anrufen", *byID[rejected.ID].Request.DecisionReason)
		assert.Empty(t, byID[withdrawn.ID].ReviewerName, "withdrawals carry no reviewer")

		page1, next1, err := f.requests.ListHistory(ctx, careplan.RequestQueueFilter{Limit: 1})
		require.NoError(t, err)
		require.Len(t, page1, 1)
		require.NotNil(t, next1)
		page2, next2, err := f.requests.ListHistory(ctx, careplan.RequestQueueFilter{BeforeInstant: next1.UpdatedAt, BeforeID: next1.ID, Limit: 1})
		require.NoError(t, err)
		require.Len(t, page2, 1)
		assert.NotEqual(t, page1[0].Request.ID, page2[0].Request.ID, "pages must not overlap")
		assert.Nil(t, next2)
		return nil
	}))
}

func TestExcusedRequestGraduatedChildLeavesQueueAndRefusesDecisions(t *testing.T) {
	t.Parallel()
	f := newExcusedFixture(t)
	req := f.createPending(t, []careplan.Date{excusedIn3Days}, "Arzttermin")
	_, err := f.db.NewUpdate().TableExpr("users.students").Set("status = ?", string(usersModels.StudentStatusAlumnus)).
		Where("id = ?", f.chain.StudentID).Exec(context.Background())
	require.NoError(t, err)

	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		items, _, err := f.requests.ListPending(ctx, careplan.RequestQueueFilter{})
		require.NoError(t, err)
		assert.Empty(t, items, "a graduated child's request must leave the queue")
		badges, err := f.requests.PendingByStudentForDate(ctx, excusedIn3Days)
		require.NoError(t, err)
		assert.NotContains(t, badges, f.chain.StudentID)
		_, err = f.requests.Decide(ctx, careplan.ExcusedRequestDecideInput{RequestID: req.ID, Approve: true, ReviewedBy: f.chain.AccountID})
		assert.ErrorIs(t, err, careplan.ErrExcusedRequestNotFound)
		_, err = f.requests.Decide(ctx, careplan.ExcusedRequestDecideInput{RequestID: req.ID, Approve: false, Reason: "child has left", ReviewedBy: f.chain.AccountID})
		assert.ErrorIs(t, err, careplan.ErrExcusedRequestNotFound)
		return nil
	}))
	assert.Equal(t, careplan.ExcusedRequestStatusPending, f.requestStatus(t, req.ID), "the request is untouched so a transition revert restores a coherent state")
}

func TestExcusedRequestTwoTenantIsolation(t *testing.T) {
	t.Parallel()
	f := newExcusedFixture(t)
	first := f.createPending(t, []careplan.Date{excusedIn3Days}, "Tenant A")

	secondTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, f.db, secondTenantID)
	secondStudent := testpkg.CreateTestStudentForTenant(t, f.db, secondTenantID, "Isolated", "Care B", "1b")
	secondAccount := testpkg.CreateTestAccount(t, f.db, "parent")
	var second *careplan.ExcusedAbsenceRequest
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), f.db, secondTenantID, func(ctx context.Context, _ bun.Tx) error {
		var err error
		second, err = f.requests.CreateRequest(ctx, secondStudent.ID, secondAccount.ID, []careplan.Date{excusedIn3Days}, "Tenant B")
		return err
	}))

	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		assert.Equal(t, f.chain.TenantID, tenant.FromContext(ctx))
		items, _, err := f.requests.ListPending(ctx, careplan.RequestQueueFilter{})
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, first.ID, items[0].Request.ID)
		_, err = f.requests.Decide(ctx, careplan.ExcusedRequestDecideInput{RequestID: second.ID, Approve: true, ReviewedBy: f.chain.AccountID})
		assert.ErrorIs(t, err, careplan.ErrExcusedRequestNotFound, "another tenant's request is invisible")
		_, err = f.requests.Decide(ctx, careplan.ExcusedRequestDecideInput{RequestID: first.ID, Approve: true, ReviewedBy: f.chain.AccountID})
		return err
	}))
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), f.db, secondTenantID, func(ctx context.Context, _ bun.Tx) error {
		items, _, err := f.requests.ListPending(ctx, careplan.RequestQueueFilter{})
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, second.ID, items[0].Request.ID, "tenant B still sees only its own pending request")
		days, err := f.repos.CarePlan().ListStudentStatusDays(ctx, careplan.StudentStatusDayFilter{ActiveOnly: true})
		require.NoError(t, err)
		assert.Empty(t, days, "tenant A's approval wrote no status day into tenant B")
		return nil
	}))
	assert.Len(t, f.activeStatusDays(t), 1)
}

// failingCarePlan injects one failure right after a chosen authoritative
// write so a test can prove the whole operation rolls back and a retry
// succeeds.
type failingCarePlan struct {
	careplan.Capability
	failAfterDecide   bool
	failAfterRedecide bool
	failAfterUpsert   bool
	failAfterClear    bool
	failAfterCreate   bool
	failAfterUpdate   bool
	upserts           int
}

var errInjected = errors.New("injected failure")

func (f *failingCarePlan) after(err error, fail bool) error {
	if err != nil {
		return err
	}
	if fail {
		return errInjected
	}
	return nil
}

func (f *failingCarePlan) DecideExcusedAbsenceRequest(ctx context.Context, value careplan.ExcusedAbsenceDecision) error {
	return f.after(f.Capability.DecideExcusedAbsenceRequest(ctx, value), f.failAfterDecide)
}

func (f *failingCarePlan) RedecideExcusedAbsenceRequest(ctx context.Context, value careplan.ExcusedAbsenceDecision) error {
	return f.after(f.Capability.RedecideExcusedAbsenceRequest(ctx, value), f.failAfterRedecide)
}

func (f *failingCarePlan) CreateExcusedAbsenceRequest(ctx context.Context, value careplan.ExcusedAbsenceRequest) (careplan.ExcusedAbsenceRequest, error) {
	row, err := f.Capability.CreateExcusedAbsenceRequest(ctx, value)
	return row, f.after(err, f.failAfterCreate)
}

func (f *failingCarePlan) UpdatePendingExcusedAbsenceRequest(ctx context.Context, id int64, dates []careplan.Date, note, status string) error {
	return f.after(f.Capability.UpdatePendingExcusedAbsenceRequest(ctx, id, dates, note, status), f.failAfterUpdate)
}

func (f *failingCarePlan) ClearStudentStatusDays(ctx context.Context, studentID int64, status string, dates []careplan.Date, at time.Time, source string) error {
	return f.after(f.Capability.ClearStudentStatusDays(ctx, studentID, status, dates, at, source), f.failAfterClear)
}

func (f *failingCarePlan) UpsertStudentStatusDay(ctx context.Context, value careplan.StudentStatusDay) (careplan.StudentStatusDay, error) {
	row, err := f.Capability.UpsertStudentStatusDay(ctx, value)
	if err != nil {
		return row, err
	}
	f.upserts++
	return row, f.after(nil, f.failAfterUpsert && f.upserts == 1)
}

func TestExcusedRequestCreateAndEditRollBackAndRetry(t *testing.T) {
	t.Parallel()
	f := newExcusedFixture(t)

	failing := newExcusedRequests(t, f.repos, &failingCarePlan{Capability: f.repos.CarePlan(), failAfterCreate: true})
	err := f.inTenant(t, func(ctx context.Context) error {
		_, err := failing.CreateRequest(ctx, f.chain.StudentID, f.chain.AccountID, []careplan.Date{excusedTomorrow}, "Arzttermin")
		return err
	})
	require.ErrorIs(t, err, errInjected)
	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		rows, err := f.requests.ListForStudent(ctx, f.chain.StudentID, time.Now().Add(-time.Hour))
		require.NoError(t, err)
		assert.Empty(t, rows, "a failed submission leaves no request row behind")
		return nil
	}))
	created := f.createPending(t, []careplan.Date{excusedTomorrow}, "Arzttermin")
	assert.Equal(t, created.ID, f.createPending(t, []careplan.Date{excusedTomorrow}, "Arzttermin").ID, "the retry is idempotent")

	failing = newExcusedRequests(t, f.repos, &failingCarePlan{Capability: f.repos.CarePlan(), failAfterUpdate: true})
	edit := careplan.ExcusedRequestEditInput{
		RequestID: created.ID, StudentID: f.chain.StudentID, GuardianAccountID: f.chain.AccountID,
		Dates: []careplan.Date{excusedIn3Days}, Note: "verschoben", NoteRequired: true,
	}
	err = f.inTenant(t, func(ctx context.Context) error {
		_, err := failing.EditRequest(ctx, edit)
		return err
	})
	require.ErrorIs(t, err, errInjected)
	row, err := f.repos.CarePlan().FindExcusedAbsenceRequest(testpkg.TenantContext(f.chain.TenantID), created.ID, false)
	require.NoError(t, err)
	assert.Equal(t, []careplan.Date{excusedTomorrow}, row.Dates, "a failed edit keeps the old dates")
	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		edited, err := f.requests.EditRequest(ctx, edit)
		if err == nil {
			assert.Equal(t, []careplan.Date{excusedIn3Days}, edited.Dates)
		}
		return err
	}))
}

func TestExcusedRequestMarkDoneAndCorrectRollBackAndRetry(t *testing.T) {
	t.Parallel()
	f := newExcusedFixture(t)
	past := f.createPending(t, []careplan.Date{excusedPast}, "Längst vorbei")
	failing := newExcusedRequests(t, f.repos, &failingCarePlan{Capability: f.repos.CarePlan(), failAfterDecide: true})
	err := f.inTenant(t, func(ctx context.Context) error {
		return failing.MarkDone(ctx, past.ID, "", "Tage sind vorbei", f.chain.AccountID)
	})
	require.ErrorIs(t, err, errInjected)
	assert.Equal(t, careplan.ExcusedRequestStatusPending, f.requestStatus(t, past.ID), "a failed mark-done leaves the request pending")
	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		return f.requests.MarkDone(ctx, past.ID, "", "Tage sind vorbei", f.chain.AccountID)
	}))
	assert.Equal(t, careplan.ExcusedRequestStatusDone, f.requestStatus(t, past.ID))

	approved := f.createPending(t, []careplan.Date{excusedIn3Days}, "Familienfeier")
	f.approve(t, approved.ID)
	for name, plan := range map[string]*failingCarePlan{
		"after clearing the days": {Capability: f.repos.CarePlan(), failAfterClear: true},
		"after the redecided row": {Capability: f.repos.CarePlan(), failAfterRedecide: true},
	} {
		failing := newExcusedRequests(t, f.repos, plan)
		err := f.inTenant(t, func(ctx context.Context) error {
			return failing.Correct(ctx, approved.ID, false, "", "Doch nicht", f.chain.AccountID)
		})
		require.ErrorIs(t, err, errInjected, name)
		assert.Len(t, f.activeStatusDays(t), 1, "%s: the approved day survives the rollback", name)
		assert.Equal(t, careplan.ExcusedRequestStatusApproved, f.requestStatus(t, approved.ID), "%s: the decision stands", name)
	}
	require.NoError(t, f.inTenant(t, func(ctx context.Context) error {
		return f.requests.Correct(ctx, approved.ID, false, "", "Doch nicht", f.chain.AccountID)
	}))
	assert.Empty(t, f.activeStatusDays(t), "the retry applies the complete correction")
	assert.Equal(t, careplan.ExcusedRequestStatusRejected, f.requestStatus(t, approved.ID))
}

func TestExcusedRequestDecisionRollsBackAfterEachWriteAndRetries(t *testing.T) {
	t.Parallel()
	f := newExcusedFixture(t)
	pending := f.createPending(t, []careplan.Date{excusedIn3Days, excusedIn4Days}, "Familienfeier")

	for name, plan := range map[string]*failingCarePlan{
		"after the first status day": {Capability: f.repos.CarePlan(), failAfterUpsert: true},
		"after the request row":      {Capability: f.repos.CarePlan(), failAfterDecide: true},
	} {
		failing := newExcusedRequests(t, f.repos, plan)
		err := f.inTenant(t, func(ctx context.Context) error {
			_, err := failing.Decide(ctx, careplan.ExcusedRequestDecideInput{RequestID: pending.ID, Approve: true, ReviewedBy: f.chain.AccountID})
			return err
		})
		require.ErrorIs(t, err, errInjected, name)
		assert.Empty(t, f.activeStatusDays(t), "%s: every status day rolls back", name)
		assert.Equal(t, careplan.ExcusedRequestStatusPending, f.requestStatus(t, pending.ID), "%s: the request stays pending", name)
	}

	f.approve(t, pending.ID)
	assert.Len(t, f.activeStatusDays(t), 2, "the retry applies the complete decision")
	assert.Equal(t, careplan.ExcusedRequestStatusApproved, f.requestStatus(t, pending.ID))
	_, err := f.decide(t, careplan.ExcusedRequestDecideInput{RequestID: pending.ID, Approve: true, ReviewedBy: f.chain.AccountID})
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestNotPending, "a second retry finds the work already done")
}
