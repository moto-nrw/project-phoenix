package schedule_test

// Integration tests for the care-schedule change-request lifecycle
// (services/schedule/care_request_service.go): CreateRequest, Decide
// (approve/reject + apply), WithdrawRequest, GetPendingForStudent. These port
// the scenarios that previously lived on the chat-request path
// (services/messaging/requests*.go) onto the decoupled schedule-domain service
// — the apply/merge/canonicalize/validate business rules are unchanged, only
// their home moved (#1803).
//
// Hermetic: real test DB, CreateTest* fixtures with FK-cascade cleanup, no
// hardcoded IDs. The acting staff is resolved from the JWT account in the
// context, exactly as the request TenantTxMiddleware supplies it.

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// careFixture is the fully-wired care-request stack plus a parent→child chain
// and a staff account whose id the decide path resolves as the acting staff.
type careFixture struct {
	db           *bun.DB
	svc          schedule.CareScheduleRequestService
	sf           *services.Factory
	repos        *repositories.Factory
	chain        testpkg.ParentChain
	staffAccount int64
	staffID      int64
}

func newCareFixture(t *testing.T) *careFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	repos := repositories.NewFactory(db)
	sf, err := services.NewFactory(repos, db, slog.Default())
	require.NoError(t, err)

	svc := schedule.NewCareScheduleRequestService(
		repos.CareScheduleChangeRequest,
		repos.Student,
		repos.Person,
		sf.ArrivalSchedule,
		sf.PickupSchedule,
		sf.UserContext,
		nil, // emitter — pill emission is best-effort and after-commit; nil no-ops
		nil, // broadcaster — cache-invalidation fan-out; nil no-ops
		slog.Default(),
	)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() { testpkg.CleanupParentGuardianChain(t, db, chain) })
	staff, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Clara", "Confirm")
	t.Cleanup(func() { testpkg.CleanupStaffFixtures(t, db, staff.ID) })
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, staffAccount.ID) })

	return &careFixture{db: db, svc: svc, sf: sf, repos: repos, chain: chain, staffAccount: staffAccount.ID, staffID: staff.ID}
}

// staffCtx carries the fixture's tenant, admin permissions (so the per-child
// write check passes) and the given account id (resolved as the acting staff on
// approve).
func (f *careFixture) staffCtx(accountID int64) context.Context {
	ctx := tenant.WithTenantID(context.Background(), f.chain.TenantID)
	ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: int(accountID)})
	ctx = context.WithValue(ctx, jwt.CtxPermissions, []string{"admin:*"})
	return ctx
}

// nonAdminCtx carries a staff caller with users:update but NO admin wildcard and
// no supervised group, so the per-child write gate (CanUpdateStudent) denies —
// used to prove the queue/decide scope rejects a staffer who cannot edit the
// child.
func (f *careFixture) nonAdminCtx(accountID int64) context.Context {
	ctx := tenant.WithTenantID(context.Background(), f.chain.TenantID)
	ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: int(accountID)})
	ctx = context.WithValue(ctx, jwt.CtxPermissions, []string{"users:update"})
	return ctx
}

func careWeekdays(entries ...map[string]any) map[string]any {
	list := make([]any, 0, len(entries))
	for _, e := range entries {
		list = append(list, e)
	}
	return map[string]any{"weekdays": list}
}

// createPending stores a pending request submitted by the chain's guardian.
func (f *careFixture) createPending(t *testing.T, payload map[string]any) *scheduleModels.CareScheduleChangeRequest {
	t.Helper()
	req, err := f.svc.CreateRequest(f.staffCtx(f.chain.AccountID), f.chain.StudentID, f.chain.AccountID, payload)
	require.NoError(t, err)
	return req
}

// TestDecide_ForbiddenWithoutWriteAccess proves the per-child write gate: a
// staffer with users:update but neither admin nor supervision of the child's
// group cannot see the request in the scoped queue and cannot decide it either
// way — while the admin path still decides the same request, proving only the
// scope blocked the non-admin.
func TestDecide_ForbiddenWithoutWriteAccess(t *testing.T) {
	f := newCareFixture(t)
	req := f.createPending(t, careWeekdays(
		map[string]any{"weekday": 1, "mode": "pickup", "arrival": "08:00", "pickup": "16:00"},
	))

	denyCtx := f.nonAdminCtx(f.staffAccount)

	items, err := f.svc.ListPending(denyCtx)
	require.NoError(t, err)
	assert.Empty(t, items, "a non-writable child's request must not appear in the queue")

	_, err = f.svc.Decide(denyCtx, schedule.CareRequestDecideInput{RequestID: req.ID, Approve: true, ReviewedBy: f.staffAccount})
	assert.ErrorIs(t, err, schedule.ErrCareRequestForbidden)
	_, err = f.svc.Decide(denyCtx, schedule.CareRequestDecideInput{RequestID: req.ID, Approve: false, Reason: "nope", ReviewedBy: f.staffAccount})
	assert.ErrorIs(t, err, schedule.ErrCareRequestForbidden)

	item, err := f.svc.Decide(f.staffCtx(f.staffAccount), schedule.CareRequestDecideInput{RequestID: req.ID, Approve: false, Reason: "closing out", ReviewedBy: f.staffAccount})
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.CareRequestStatusRejected, item.Request.Status)
}

// --- Decide: approve applies the plan --------------------------------------

// TestDecide_ApproveAppliesModeArrivalPickup is the staff "Bestätigen" happy
// path: approving a pending request DURABLY applies the child's permanent
// weekly plan (departure mode + arrival + pickup for the requested weekday).
func TestDecide_ApproveAppliesModeArrivalPickup(t *testing.T) {
	f := newCareFixture(t)
	ctx := f.staffCtx(f.staffAccount)

	req := f.createPending(t, careWeekdays(
		map[string]any{"weekday": 1, "mode": "pickup", "arrival": "08:00", "pickup": "16:00"},
	))

	item, err := f.svc.Decide(ctx, schedule.CareRequestDecideInput{
		RequestID: req.ID, Approve: true, ReviewedBy: f.staffAccount,
	})
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.CareRequestStatusApproved, item.Request.Status)

	arrivals, err := f.sf.ArrivalSchedule.GetStudentArrivalSchedules(ctx, f.chain.StudentID)
	require.NoError(t, err)
	require.Len(t, arrivals, 1)
	assert.Equal(t, 1, arrivals[0].Weekday)
	assert.Equal(t, "08:00", arrivals[0].ExpectedArrival.Format("15:04"))

	pickups, err := f.sf.PickupSchedule.GetStudentPickupSchedules(ctx, f.chain.StudentID)
	require.NoError(t, err)
	require.Len(t, pickups, 1)
	assert.Equal(t, "16:00", pickups[0].PickupTime.Format("15:04"))

	student, err := f.sf.Users.GetStudentByID(ctx, f.chain.StudentID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.DeparturePickup, student.DepartureDays["mon"])
}

// TestDecide_ApproveMergesPreservingOtherDaysAndModes pins the read-modify-write
// merge: approving a request that only touches Monday must NOT wipe an unrelated
// Tuesday arrival saved directly, and must preserve a multi-mode (bus+pickup)
// Tuesday departure set — the collapse bug the apply guards against.
func TestDecide_ApproveMergesPreservingOtherDaysAndModes(t *testing.T) {
	f := newCareFixture(t)
	ctx := f.staffCtx(f.staffAccount)

	// Seed a multi-mode Tuesday directly on the student (a request can only carry
	// one mode per day, so this can't be produced through CreateRequest).
	student, err := f.sf.Users.GetStudentByID(ctx, f.chain.StudentID)
	require.NoError(t, err)
	student.AllowedDepartureModes = usersModels.AllowedDepartureModes{
		"tue": []usersModels.DepartureMode{usersModels.DepartureBus, usersModels.DeparturePickup},
	}
	student.DepartureDays = nil
	student.BusDays = nil
	student.PickupDays = nil
	require.NoError(t, f.repos.Student.Update(ctx, student))

	// Seed a direct Wednesday arrival (not via a request).
	require.NoError(t, f.sf.ArrivalSchedule.UpsertStudentArrivalSchedule(ctx,
		&scheduleModels.StudentArrivalSchedule{
			StudentID:       f.chain.StudentID,
			Weekday:         3,
			ExpectedArrival: timezone.WallClock(time.Date(1, 1, 1, 7, 30, 0, 0, time.UTC)),
			CreatedBy:       f.staffID,
		}))

	// Approve a request that only touches Monday.
	req := f.createPending(t, careWeekdays(
		map[string]any{"weekday": 1, "mode": "alone", "arrival": "08:15"},
	))
	_, err = f.svc.Decide(ctx, schedule.CareRequestDecideInput{RequestID: req.ID, Approve: true, ReviewedBy: f.staffAccount})
	require.NoError(t, err)

	// Wednesday arrival survives untouched.
	arrivals, err := f.sf.ArrivalSchedule.GetStudentArrivalSchedules(ctx, f.chain.StudentID)
	require.NoError(t, err)
	byDay := map[int]string{}
	for _, a := range arrivals {
		byDay[a.Weekday] = a.ExpectedArrival.Format("15:04")
	}
	assert.Equal(t, "07:30", byDay[3], "an unrelated direct Wednesday edit must survive")
	assert.Equal(t, "08:15", byDay[1], "the requested Monday arrival is applied")

	// The multi-mode Tuesday survives the Monday-only confirm.
	reloaded, err := f.sf.Users.GetStudentByID(ctx, f.chain.StudentID)
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]usersModels.DepartureMode{usersModels.DepartureBus, usersModels.DeparturePickup},
		reloaded.AllowedDepartureModes["tue"],
		"a bus+pickup day must survive an unrelated Monday confirm")
}

// TestDecide_NonStaffConfirmerForbidden: an account without a users.staff row
// (here the guardian account) cannot be stamped as the confirming staff, so the
// apply returns ErrCareRequestForbidden rather than a 500.
func TestDecide_NonStaffConfirmerForbidden(t *testing.T) {
	f := newCareFixture(t)
	req := f.createPending(t, careWeekdays(map[string]any{"weekday": 1, "arrival": "08:00"}))

	_, err := f.svc.Decide(f.staffCtx(f.chain.AccountID), schedule.CareRequestDecideInput{
		RequestID: req.ID, Approve: true, ReviewedBy: f.chain.AccountID,
	})
	require.ErrorIs(t, err, schedule.ErrCareRequestForbidden)
}

// TestDecide_ApproveRefusedWhenGuardianAccessRevoked pins the P1 guardian-link
// gate (#1803 review follow-up): if the submitting guardian loses
// parent_portal.access after filing the request but before staff review,
// APPROVING is refused (it would overwrite the child's permanent weekly plan and
// notify an account the parent APIs now hide) and the request stays pending;
// REJECTING stays available so staff can wind it down. Mirrors the chat's
// ConfirmRequest/requireLinkedGuardian split this flow replaced.
func TestDecide_ApproveRefusedWhenGuardianAccessRevoked(t *testing.T) {
	f := newCareFixture(t)
	// The default fixture passes a nil emitter; wire one so the link gate is
	// live. Only the thread repo is consulted for the access check — the pill
	// path self-no-ops with the remaining deps nil.
	svc := schedule.NewCareScheduleRequestService(
		f.repos.CareScheduleChangeRequest,
		f.repos.Student,
		f.repos.Person,
		f.sf.ArrivalSchedule,
		f.sf.PickupSchedule,
		f.sf.UserContext,
		parentmessaging.NewEmitter(f.db, f.repos.ParentMessageThread, nil, nil, nil, slog.Default()),
		nil,
		slog.Default(),
	)

	req, err := svc.CreateRequest(f.staffCtx(f.chain.AccountID), f.chain.StudentID, f.chain.AccountID,
		careWeekdays(map[string]any{"weekday": 1, "mode": "pickup", "arrival": "08:00", "pickup": "16:00"}))
	require.NoError(t, err)

	// Revoke the guardian's portal access to the child (unlink / downgrade).
	_, err = f.db.ExecContext(context.Background(), `
		UPDATE users.students_guardians SET permissions = '{}'::jsonb
		WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?
	`, f.chain.TenantID, f.chain.StudentID, f.chain.GuardianProfileID)
	require.NoError(t, err)

	ctx := f.staffCtx(f.staffAccount)

	_, err = svc.Decide(ctx, schedule.CareRequestDecideInput{RequestID: req.ID, Approve: true, ReviewedBy: f.staffAccount})
	require.ErrorIs(t, err, schedule.ErrCareRequestGuardianAccessRevoked, "a revoked guardian's request must not be applied")

	still, err := f.repos.CareScheduleChangeRequest.FindByID(ctx, req.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.CareRequestStatusPending, still.Status, "a refused approval leaves the request pending")

	item, err := svc.Decide(ctx, schedule.CareRequestDecideInput{RequestID: req.ID, Approve: false, Reason: "Elternteil nicht mehr berechtigt", ReviewedBy: f.staffAccount})
	require.NoError(t, err, "reject must stay available after access is revoked")
	assert.Equal(t, scheduleModels.CareRequestStatusRejected, item.Request.Status)
}

// fakeDisabledSettings reports parent-OGS messaging as OFF for every tenant, so
// the approve-time messaging gate can be exercised without touching the settings
// store. Returns a nil error, so the gate sees a genuine disabled flag (not the
// fail-open transient-error path).
type fakeDisabledSettings struct{}

func (fakeDisabledSettings) ResolveBoolForTenant(context.Context, int64, string) (bool, error) {
	return false, nil
}

// toggleSettings is a flippable messaging gate: a request can be filed while
// messaging is ON (created pill written) and then closed after it is turned OFF,
// exercising the emitter's reconcile-while-disabled path.
type toggleSettings struct{ enabled bool }

func (s *toggleSettings) ResolveBoolForTenant(context.Context, int64, string) (bool, error) {
	return s.enabled, nil
}

// TestDecide_RejectClosesRequestPillEvenWhenMessagingDisabled pins the #1803
// review follow-up (finding #2): when a request was filed while messaging was ON
// (so its request_created pill + thread exist) and the school then turns
// parent_notes OFF, REJECTING must still write the closing request_status pill.
// The staff thread timeline decides "is this request still actionable?" by
// looking for a later status pill with the same ref_id, so dropping it would show
// a request the review queue has already resolved forever. The reconcile write
// only appends to the EXISTING thread + created pill — it never creates a thread.
func TestDecide_RejectClosesRequestPillEvenWhenMessagingDisabled(t *testing.T) {
	f := newCareFixture(t)
	// This test actually writes pills (created + status) from the separate staff
	// account, whose sender_account_id FKs auth.accounts without cascade. Register
	// the clear LAST so it runs FIRST (LIFO) ahead of CleanupAuthFixtures(staff).
	t.Cleanup(func() { testpkg.CleanupParentMessagingForAccount(t, f.db, f.staffAccount) })
	settings := &toggleSettings{enabled: true}
	svc := schedule.NewCareScheduleRequestService(
		f.repos.CareScheduleChangeRequest,
		f.repos.Student,
		f.repos.Person,
		f.sf.ArrivalSchedule,
		f.sf.PickupSchedule,
		f.sf.UserContext,
		parentmessaging.NewEmitter(f.db, f.repos.ParentMessageThread, f.repos.ParentMessage, settings, nil, slog.Default()),
		nil,
		slog.Default(),
	)

	// Filed while messaging is ON: the request_created pill opens the thread.
	req, err := svc.CreateRequest(f.staffCtx(f.chain.AccountID), f.chain.StudentID, f.chain.AccountID,
		careWeekdays(map[string]any{"weekday": 1, "mode": "pickup", "arrival": "08:00", "pickup": "16:00"}))
	require.NoError(t, err)

	ctx := f.staffCtx(f.staffAccount)
	thread, err := f.repos.ParentMessageThread.FindByStudentGuardian(ctx, f.chain.StudentID, f.chain.AccountID)
	require.NoError(t, err)
	require.NotNil(t, thread, "the request_created pill must have opened the thread while messaging was on")
	created, err := f.repos.ParentMessage.FindEventByRef(ctx, thread.ID, usersModels.ParentMessageEventRequestCreated, "schedule.care_schedule_change_requests", req.ID)
	require.NoError(t, err)
	require.NotNil(t, created, "request_created pill exists while messaging is on")

	// School turns messaging OFF, then staff rejects the pending request.
	settings.enabled = false
	item, err := svc.Decide(ctx, schedule.CareRequestDecideInput{RequestID: req.ID, Approve: false, Reason: "telefonisch geklärt", ReviewedBy: f.staffAccount})
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.CareRequestStatusRejected, item.Request.Status)

	// The closing pill must still be written so the timeline and the queue agree.
	status, err := f.repos.ParentMessage.FindEventByRef(ctx, thread.ID, usersModels.ParentMessageEventRequestStatus, "schedule.care_schedule_change_requests", req.ID)
	require.NoError(t, err)
	require.NotNil(t, status, "the closing request_status pill must be written even while messaging is disabled")
	assert.Equal(t, usersModels.ParentMessageRequestStatusRejected, status.RequestStatus)
}

// TestDecide_NoReconcilePillWhenRequestFiledWhileDisabled pins the other half of
// the reconcile gate: if the request was filed while messaging was already OFF,
// no thread / request_created pill was ever emitted, so REJECTING must NOT
// conjure a dangling request_status pill (or create a thread). Only an
// already-open notice is reconciled.
func TestDecide_NoReconcilePillWhenRequestFiledWhileDisabled(t *testing.T) {
	f := newCareFixture(t)
	svc := schedule.NewCareScheduleRequestService(
		f.repos.CareScheduleChangeRequest,
		f.repos.Student,
		f.repos.Person,
		f.sf.ArrivalSchedule,
		f.sf.PickupSchedule,
		f.sf.UserContext,
		parentmessaging.NewEmitter(f.db, f.repos.ParentMessageThread, f.repos.ParentMessage, fakeDisabledSettings{}, nil, slog.Default()),
		nil,
		slog.Default(),
	)

	req, err := svc.CreateRequest(f.staffCtx(f.chain.AccountID), f.chain.StudentID, f.chain.AccountID,
		careWeekdays(map[string]any{"weekday": 1, "mode": "pickup", "arrival": "08:00", "pickup": "16:00"}))
	require.NoError(t, err)

	ctx := f.staffCtx(f.staffAccount)
	item, err := svc.Decide(ctx, schedule.CareRequestDecideInput{RequestID: req.ID, Approve: false, Reason: "kein Kanal", ReviewedBy: f.staffAccount})
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.CareRequestStatusRejected, item.Request.Status)

	// No thread was ever opened (created pill dropped while disabled), so there is
	// nothing to reconcile and no dangling status pill.
	thread, err := f.repos.ParentMessageThread.FindByStudentGuardian(ctx, f.chain.StudentID, f.chain.AccountID)
	require.NoError(t, err)
	assert.Nil(t, thread, "a request filed while disabled leaves no thread, so nothing to reconcile")
}

// TestDecide_ApproveRefusedWhenMessagingDisabled pins the P2 messaging gate
// (#1803 review follow-up): if the school turns operations.parent_notes_enabled
// OFF after a request is filed, APPROVING is refused — the guardian's "bestätigt"
// pill is the only notification channel and the emitter drops it while messaging
// is off, so approving would change the child's permanent plan with no parent
// notice. The request stays pending, the plan is untouched, and REJECTING stays
// available. Mirrors the chat's ConfirmRequest/requireEnabled gate this flow
// replaced.
func TestDecide_ApproveRefusedWhenMessagingDisabled(t *testing.T) {
	f := newCareFixture(t)
	// Wire an emitter whose settings report messaging OFF. The messaging gate
	// runs before the guardian-link check, so the guardian is left linked.
	svc := schedule.NewCareScheduleRequestService(
		f.repos.CareScheduleChangeRequest,
		f.repos.Student,
		f.repos.Person,
		f.sf.ArrivalSchedule,
		f.sf.PickupSchedule,
		f.sf.UserContext,
		parentmessaging.NewEmitter(f.db, f.repos.ParentMessageThread, f.repos.ParentMessage, fakeDisabledSettings{}, nil, slog.Default()),
		nil,
		slog.Default(),
	)

	req, err := svc.CreateRequest(f.staffCtx(f.chain.AccountID), f.chain.StudentID, f.chain.AccountID,
		careWeekdays(map[string]any{"weekday": 1, "mode": "pickup", "arrival": "08:00", "pickup": "16:00"}))
	require.NoError(t, err)

	ctx := f.staffCtx(f.staffAccount)

	_, err = svc.Decide(ctx, schedule.CareRequestDecideInput{RequestID: req.ID, Approve: true, ReviewedBy: f.staffAccount})
	require.ErrorIs(t, err, schedule.ErrCareRequestMessagingDisabled, "approving with messaging disabled must be refused")

	still, err := f.repos.CareScheduleChangeRequest.FindByID(ctx, req.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.CareRequestStatusPending, still.Status, "a refused approval leaves the request pending")

	arrivals, err := f.sf.ArrivalSchedule.GetStudentArrivalSchedules(ctx, f.chain.StudentID)
	require.NoError(t, err)
	assert.Empty(t, arrivals, "a refused approval must not apply the requested plan")

	item, err := svc.Decide(ctx, schedule.CareRequestDecideInput{RequestID: req.ID, Approve: false, Reason: "Nachrichten deaktiviert, bitte telefonisch klären", ReviewedBy: f.staffAccount})
	require.NoError(t, err, "reject must stay available while messaging is disabled")
	assert.Equal(t, scheduleModels.CareRequestStatusRejected, item.Request.Status)
}

// TestDecide_SecondDecideNotPending pins the idempotency/race guard: once a
// request is approved, a second decide (or a reject racing it) sees the terminal
// status under the FOR UPDATE re-read and returns ErrCareRequestNotPending.
func TestDecide_SecondDecideNotPending(t *testing.T) {
	f := newCareFixture(t)
	ctx := f.staffCtx(f.staffAccount)
	req := f.createPending(t, careWeekdays(map[string]any{"weekday": 2, "arrival": "07:45"}))

	_, err := f.svc.Decide(ctx, schedule.CareRequestDecideInput{RequestID: req.ID, Approve: true, ReviewedBy: f.staffAccount})
	require.NoError(t, err)

	_, err = f.svc.Decide(ctx, schedule.CareRequestDecideInput{RequestID: req.ID, Approve: true, ReviewedBy: f.staffAccount})
	require.ErrorIs(t, err, scheduleModels.ErrCareRequestNotPending, "a decided request cannot be approved twice")

	_, err = f.svc.Decide(ctx, schedule.CareRequestDecideInput{RequestID: req.ID, Approve: false, Reason: "zu spät", ReviewedBy: f.staffAccount})
	require.ErrorIs(t, err, scheduleModels.ErrCareRequestNotPending, "an applied request cannot then be rejected")
}

// TestDecide_RejectRequiresReasonAndBounds pins the free-text guards: a blank
// reason is rejected, an over-2000-rune reason is rejected, and a valid reject
// flips the row to rejected WITHOUT applying the plan.
func TestDecide_RejectRequiresReasonAndBounds(t *testing.T) {
	f := newCareFixture(t)
	ctx := f.staffCtx(f.staffAccount)
	req := f.createPending(t, careWeekdays(map[string]any{"weekday": 1, "arrival": "08:00"}))

	_, err := f.svc.Decide(ctx, schedule.CareRequestDecideInput{RequestID: req.ID, Approve: false, Reason: "  \n\t ", ReviewedBy: f.staffAccount})
	require.ErrorIs(t, err, schedule.ErrCareRequestRejectReasonRequired)

	_, err = f.svc.Decide(ctx, schedule.CareRequestDecideInput{RequestID: req.ID, Approve: false, Reason: strings.Repeat("x", 2001), ReviewedBy: f.staffAccount})
	require.ErrorIs(t, err, schedule.ErrCareRequestRejectReasonTooLong)

	item, err := f.svc.Decide(ctx, schedule.CareRequestDecideInput{RequestID: req.ID, Approve: false, Reason: "Bitte mit Datum erneut", ReviewedBy: f.staffAccount})
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.CareRequestStatusRejected, item.Request.Status)
	require.NotNil(t, item.Request.DecisionReason)
	assert.Equal(t, "Bitte mit Datum erneut", *item.Request.DecisionReason)

	arrivals, err := f.sf.ArrivalSchedule.GetStudentArrivalSchedules(ctx, f.chain.StudentID)
	require.NoError(t, err)
	assert.Empty(t, arrivals, "rejecting must not apply the requested plan")
}

// --- CreateRequest: validation + canonicalization --------------------------

// TestCreateRequest_RejectsMidnight pins that 00:00 is rejected at create time
// (it normalizes to the zero time the schedule validators treat as "unset").
func TestCreateRequest_RejectsMidnight(t *testing.T) {
	f := newCareFixture(t)
	ctx := f.staffCtx(f.chain.AccountID)

	_, err := f.svc.CreateRequest(ctx, f.chain.StudentID, f.chain.AccountID,
		careWeekdays(map[string]any{"weekday": 1, "arrival": "00:00"}))
	require.ErrorIs(t, err, schedule.ErrInvalidCareRequestPayload)

	_, err = f.svc.CreateRequest(ctx, f.chain.StudentID, f.chain.AccountID,
		careWeekdays(map[string]any{"weekday": 1, "pickup": "00:00"}))
	require.ErrorIs(t, err, schedule.ErrInvalidCareRequestPayload)

	// One minute past midnight is a real wall-clock time and stays accepted.
	_, err = f.svc.CreateRequest(ctx, f.chain.StudentID, f.chain.AccountID,
		careWeekdays(map[string]any{"weekday": 1, "arrival": "00:01"}))
	require.NoError(t, err)
}

// TestCreateRequest_CanonicalizesPayload pins that the persisted payload is the
// sanitized canonical form: unknown keys dropped, duplicate weekday entries
// collapsed (last value per aspect wins).
func TestCreateRequest_CanonicalizesPayload(t *testing.T) {
	f := newCareFixture(t)
	ctx := f.staffCtx(f.chain.AccountID)

	req, err := f.svc.CreateRequest(ctx, f.chain.StudentID, f.chain.AccountID, map[string]any{
		"weekdays": []any{
			map[string]any{"weekday": 1, "arrival": "08:00"},
			map[string]any{"weekday": 1, "arrival": "09:00"}, // duplicate Monday, last wins
			map[string]any{"weekday": 1, "pickup": "15:00"},
		},
		"junk": "ignored",
	})
	require.NoError(t, err)

	_, hasJunk := req.Payload["junk"]
	assert.False(t, hasJunk, "unknown keys are dropped before persisting")
	weekdays, ok := req.Payload["weekdays"].([]any)
	require.True(t, ok)
	require.Len(t, weekdays, 1, "duplicate weekday entries collapse to one")
	monday, ok := weekdays[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "09:00", monday["arrival"], "last arrival value wins")
	assert.Equal(t, "15:00", monday["pickup"])
}

// TestCreateRequest_OnePendingPerStudent pins the partial unique index: a second
// pending request for the same child is ErrCareRequestAlreadyPending — the
// guardian withdraws and re-submits instead of stacking.
func TestCreateRequest_OnePendingPerStudent(t *testing.T) {
	f := newCareFixture(t)
	f.createPending(t, careWeekdays(map[string]any{"weekday": 1, "arrival": "08:00"}))

	_, err := f.svc.CreateRequest(f.staffCtx(f.chain.AccountID), f.chain.StudentID, f.chain.AccountID,
		careWeekdays(map[string]any{"weekday": 2, "arrival": "08:00"}))
	require.ErrorIs(t, err, schedule.ErrCareRequestAlreadyPending)
}

// --- Withdraw --------------------------------------------------------------

// TestWithdraw_BySubmitterAndGuards: the submitter withdraws their own pending
// request; a second withdraw of the now-terminal row fails; and a withdraw by a
// DIFFERENT guardian account is reported not-found (id space is not probeable).
func TestWithdraw_BySubmitterAndGuards(t *testing.T) {
	f := newCareFixture(t)
	ctx := f.staffCtx(f.chain.AccountID)
	req := f.createPending(t, careWeekdays(map[string]any{"weekday": 1, "arrival": "08:00"}))

	// A different guardian account cannot withdraw it.
	other := testpkg.CreateTestAccount(t, f.db, "other-guardian")
	t.Cleanup(func() {
		_, _ = f.db.ExecContext(context.Background(), `DELETE FROM auth.accounts WHERE id = ?`, other.ID)
	})
	_, err := f.svc.WithdrawRequest(ctx, req.ID, f.chain.StudentID, other.ID)
	require.ErrorIs(t, err, scheduleModels.ErrCareRequestNotFound, "a foreign account cannot withdraw the request")

	// The submitter withdraws it.
	withdrawn, err := f.svc.WithdrawRequest(ctx, req.ID, f.chain.StudentID, f.chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.CareRequestStatusWithdrawn, withdrawn.Status)

	// A second withdraw of the terminal row fails.
	_, err = f.svc.WithdrawRequest(ctx, req.ID, f.chain.StudentID, f.chain.AccountID)
	require.ErrorIs(t, err, scheduleModels.ErrCareRequestNotPending)
}

// --- GetPendingForStudent --------------------------------------------------

// TestGetPendingForStudent_NoneReturnsNil pins that a child with no open request
// yields (nil, nil, nil) — the read view shows no pending card.
func TestGetPendingForStudent_NoneReturnsNil(t *testing.T) {
	f := newCareFixture(t)
	req, diff, err := f.svc.GetPendingForStudent(f.staffCtx(f.staffAccount), f.chain.StudentID)
	require.NoError(t, err)
	assert.Nil(t, req)
	assert.Nil(t, diff)
}

// TestGetPendingForStudent_ReturnsPendingWithDiff pins that an open request is
// returned with a live "current → requested" diff.
func TestGetPendingForStudent_ReturnsPendingWithDiff(t *testing.T) {
	f := newCareFixture(t)
	ctx := f.staffCtx(f.chain.AccountID)
	f.createPending(t, careWeekdays(map[string]any{"weekday": 1, "arrival": "08:00"}))

	req, diff, err := f.svc.GetPendingForStudent(ctx, f.chain.StudentID)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.NotEmpty(t, diff, "an open request carries its current→requested diff")
	assert.Equal(t, "08:00", diff[0].New)
}
