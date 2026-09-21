package contracttest_test

// #3135: a one-day pickup change must be traceable from the staff message
// thread. The pills name the requested day and time (never the event time),
// carry the same facts as a structured payload, and every pill's reference
// opens exactly its own request through GetForReview, decided or not.

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/services"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	requestreviewcompose "github.com/moto-nrw/project-phoenix/modules/requestreview/compose"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingParentEvents captures every pill the service emits. Outside a
// transaction RegisterAfterCommit runs immediately, so the pills are visible
// right after the call.
type recordingParentEvents struct {
	mu        sync.Mutex
	emitted   []parentmessaging.ChildEvent
	guardians []parentmessaging.PortalGuardian
}

func (r *recordingParentEvents) EmitChildEvent(_, _, _ int64, ev parentmessaging.ChildEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.emitted = append(r.emitted, ev)
}
func (r *recordingParentEvents) WakeChildGuardians(_, _ int64) {}
func (r *recordingParentEvents) PortalGuardians(context.Context, int64) ([]parentmessaging.PortalGuardian, error) {
	return r.guardians, nil
}
func (r *recordingParentEvents) InTransaction(context.Context) bool                    { return false }
func (r *recordingParentEvents) MessagingEnabledForTenant(context.Context, int64) bool { return true }

func (r *recordingParentEvents) last(t *testing.T) parentmessaging.ChildEvent {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	require.NotEmpty(t, r.emitted, "expected a pill to be emitted")
	return r.emitted[len(r.emitted)-1]
}

func newPickupPillFixture(t *testing.T) (*careFixture, *recordingParentEvents) {
	t.Helper()
	events := &recordingParentEvents{}
	f := newCareFixtureWithEmitter(t, parentmessaging.NewEmitter(events),
		services.WithCareRequestToday(func() timezone.Date { return timezone.NewDate(2026, 9, 9) }))
	// The approve path re-checks that the submitting guardian still has the
	// child; the fixture's chain is that guardian.
	events.guardians = []parentmessaging.PortalGuardian{{AccountID: f.chain.AccountID}}
	return f, events
}

// seedWeekdayPickup gives the child a weekly pickup time on the requested
// day's weekday, so the request records a previous_pickup_time.
func seedWeekdayPickup(t *testing.T, f *careFixture, ctx context.Context, date timezone.Date, hour, minute int) {
	t.Helper()
	require.NoError(t, f.sf.PickupSchedule.UpsertStudentPickupSchedule(ctx, &careplan.PickupSchedule{
		StudentID:  f.chain.StudentID,
		Weekday:    int(date.Weekday()),
		PickupTime: timezone.NormalizeWallClock(time.Date(1, 1, 1, hour, minute, 0, 0, time.UTC)),
		CreatedBy:  f.staffID,
	}))
	t.Cleanup(func() {
		require.NoError(t, f.sf.PickupSchedule.DeleteStudentPickupSchedule(ctx, f.chain.StudentID))
	})
}

func TestPickupChangePills_NameRequestedDayAndTime(t *testing.T) {
	t.Parallel()

	f, events := newPickupPillFixture(t)
	ctx := f.staffCtx(f.staffAccount)
	date := timezone.NewDate(2026, 9, 15) // a Tuesday
	seedWeekdayPickup(t, f, ctx, date, 15, 30)
	pickup := time.Date(2000, 1, 1, 14, 30, 0, 0, time.UTC)

	req, err := f.svc.CreatePickupChangeRequest(ctx, f.chain.StudentID, f.chain.AccountID, date, pickup, "Arzttermin")
	require.NoError(t, err)

	created := events.last(t)
	assert.Equal(t, "request_created", created.EventType)
	assert.Equal(t, "Abholzeit angefragt: 15.09.2026, 14:30 Uhr", created.Body)
	assert.Equal(t, "pickup_change", created.RequestType)
	assert.Equal(t, "schedule.care_schedule_change_requests", created.RefTable)
	require.NotNil(t, created.RefID)
	assert.Equal(t, req.ID, *created.RefID)
	assert.Equal(t, map[string]any{
		"date": "2026-09-15", "pickup_time": "14:30", "previous_pickup_time": "15:30",
	}, created.Payload)

	_, err = f.svc.Decide(ctx, carerequests.DecideInput{
		RequestID: req.ID, Approve: true, ReviewedBy: f.staffAccount,
	})
	require.NoError(t, err)

	confirmed := events.last(t)
	assert.Equal(t, "request_status", confirmed.EventType)
	assert.Equal(t, "erledigt", confirmed.RequestStatus)
	assert.Equal(t, "Abholzeit bestätigt: 15.09.2026, 14:30 Uhr", confirmed.Body)
	require.NotNil(t, confirmed.RefID)
	assert.Equal(t, req.ID, *confirmed.RefID, "the decision pill references the same request row")
	assert.Equal(t, "2026-09-15", confirmed.Payload["date"])
	assert.Equal(t, "14:30", confirmed.Payload["pickup_time"])

	// A second request for another day is rejected: the pill names THAT day
	// and keeps the staff reason.
	second, err := f.svc.CreatePickupChangeRequest(ctx, f.chain.StudentID, f.chain.AccountID, date.AddDays(1), pickup, "Oma holt ab")
	require.NoError(t, err)
	_, err = f.svc.Decide(ctx, carerequests.DecideInput{
		RequestID: second.ID, Approve: false, Reason: "passt nicht", ReviewedBy: f.staffAccount,
	})
	require.NoError(t, err)

	rejected := events.last(t)
	assert.Equal(t, "abgelehnt", rejected.RequestStatus)
	assert.Equal(t, "Abholzeit abgelehnt: 16.09.2026, 14:30 Uhr. Grund: passt nicht", rejected.Body)
	assert.Equal(t, "passt nicht", rejected.DecisionReason)
	require.NotNil(t, rejected.RefID)
	assert.Equal(t, second.ID, *rejected.RefID)
	assert.Equal(t, "2026-09-16", rejected.Payload["date"])
}

func TestWeeklyPlanPills_KeepPlainBodyWithoutPayload(t *testing.T) {
	t.Parallel()

	f, events := newPickupPillFixture(t)
	f.createPending(t, careWeekdays(map[string]any{"weekday": 1, "pickup": "16:00"}))

	created := events.last(t)
	assert.Equal(t, "Anfrage: Dauerhafte Betreuungszeiten ändern", created.Body)
	assert.Nil(t, created.Payload, "a weekly plan has no single day to name")
}

func TestGetForReview_OpensDecidedAndPendingPickupRequests(t *testing.T) {
	t.Parallel()

	f, _ := newPickupPillFixture(t)
	ctx := f.staffCtx(f.staffAccount)
	date := timezone.NewDate(2026, 9, 15)
	seedWeekdayPickup(t, f, ctx, date, 15, 30)
	pickup := time.Date(2000, 1, 1, 14, 30, 0, 0, time.UTC)

	decidedReq, err := f.svc.CreatePickupChangeRequest(ctx, f.chain.StudentID, f.chain.AccountID, date, pickup, "Arzttermin")
	require.NoError(t, err)
	_, err = f.svc.Decide(ctx, carerequests.DecideInput{
		RequestID: decidedReq.ID, Approve: false, Reason: "passt nicht", ReviewedBy: f.staffAccount,
	})
	require.NoError(t, err)
	pendingReq, err := f.svc.CreatePickupChangeRequest(ctx, f.chain.StudentID, f.chain.AccountID, date.AddDays(1), pickup, "Oma holt ab")
	require.NoError(t, err)

	// The live plan moves on AFTER the decision; the detail must not follow it.
	require.NoError(t, f.sf.PickupSchedule.UpsertStudentPickupSchedule(ctx, &careplan.PickupSchedule{
		StudentID:  f.chain.StudentID,
		Weekday:    int(date.Weekday()),
		PickupTime: timezone.NormalizeWallClock(time.Date(1, 1, 1, 17, 0, 0, 0, time.UTC)),
		CreatedBy:  f.staffID,
	}))

	decided, err := nativeCareReviews(t, f).GetForReview(ctx, decidedReq.ID)
	require.NoError(t, err)
	assert.Equal(t, decidedReq.ID, decided.Request.ID)
	assert.Equal(t, scheduleModels.CareRequestStatusRejected, decided.Request.Status)
	assert.Equal(t, "Felix", decided.FirstName)
	assert.Equal(t, "Schneider", decided.LastName)
	assert.Equal(t, "Clara Confirm", decided.ReviewerName)
	require.NotNil(t, decided.RequestReason)
	assert.Equal(t, "Arzttermin", *decided.RequestReason)
	require.NotNil(t, decided.Request.DecisionReason)
	assert.Equal(t, "passt nicht", *decided.Request.DecisionReason)
	require.Len(t, decided.Requested, 1)
	assert.Equal(t, "15.09.2026 · Abholzeit", decided.Requested[0].Label)
	assert.Equal(t, "14:30", decided.Requested[0].New)
	require.NotEmpty(t, decided.Diff, "a rejection freezes the alt → neu comparison")
	assert.Equal(t, "15:30", decided.Diff[0].Old, "the frozen previous time survives later plan changes")
	assert.Equal(t, "14:30", decided.Diff[0].New)

	pending, err := nativeCareReviews(t, f).GetForReview(ctx, pendingReq.ID)
	require.NoError(t, err)
	assert.Equal(t, pendingReq.ID, pending.Request.ID)
	assert.Equal(t, scheduleModels.CareRequestStatusPending, pending.Request.Status)
	assert.Empty(t, pending.ReviewerName)
	require.NotNil(t, pending.RequestReason)
	assert.Equal(t, "Oma holt ab", *pending.RequestReason)
	assert.Equal(t, "16.09.2026 · Abholzeit", pending.Requested[0].Label)
}

func TestGetForReview_RefusesUnauthorizedAndForeignReaders(t *testing.T) {
	t.Parallel()

	f, _ := newPickupPillFixture(t)
	ctx := f.staffCtx(f.staffAccount)
	date := timezone.NewDate(2026, 9, 15)
	req, err := f.svc.CreatePickupChangeRequest(ctx, f.chain.StudentID, f.chain.AccountID, date,
		time.Date(2000, 1, 1, 14, 30, 0, 0, time.UTC), "Arzttermin")
	require.NoError(t, err)

	// users:update without a staff record (the guardian's own account) is
	// outside the per-child review scope: forbidden, never a detail.
	_, err = nativeCareReviews(t, f).GetForReview(f.nonAdminCtx(f.chain.AccountID), req.ID)
	assert.ErrorIs(t, err, carerequests.ErrCareRequestForbidden)

	// A missing row is not found.
	_, err = nativeCareReviews(t, f).GetForReview(ctx, req.ID+1_000_000)
	assert.ErrorIs(t, err, careplan.ErrCareScheduleRequestNotFound)

	// Another school's admin does not learn that the row exists.
	foreignTenant, _ := testpkg.CreateTestTenant(t, f.db)
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignTenant)
	foreignCtx = context.WithValue(foreignCtx, jwt.CtxClaims, jwt.AppClaims{ID: int(f.staffAccount)})
	foreignCtx = context.WithValue(foreignCtx, jwt.CtxPermissions, []string{"admin:*"})
	_, err = nativeCareReviews(t, f).GetForReview(foreignCtx, req.ID)
	assert.ErrorIs(t, err, careplan.ErrCareScheduleRequestNotFound)
}

func nativeCareReviews(t *testing.T, f *careFixture) careplan.CareScheduleReviewQuery {
	t.Helper()
	query, err := requestreviewcompose.NewScheduleReviews(f.db, requestreviewcompose.ScheduleReviewDependencies{
		People: f.sf.PeopleDirectory,
		Scope: func(ctx context.Context) (requestreviewcompose.ReviewScope, error) {
			// Match testpkg.RequestReviewPolicy used by this fixture's commands.
			wide, _ := authorize.CanUpdateStudent(ctx, jwt.PermissionsFromCtx(ctx),
				careplan.ScheduleStudent{ID: f.chain.StudentID, TenantID: f.chain.TenantID}, f.sf.UserContext)
			return requestreviewcompose.ReviewScope{SchoolWide: wide}, nil
		},
		BookingsAuthoritative: func(context.Context) (bool, error) { return false, nil },
		Today:                 func() careplan.Date { return "2026-09-09" },
		ObserveCare:           func(requestreviewcompose.CareObservation) {},
		ObserveTimetable:      func(requestreviewcompose.TimetableObservation) {},
	})
	require.NoError(t, err)
	return query
}
