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
	"errors"
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
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
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
	autoExcusal  *schedule.PickupAutoExcusalSyncer
}

func newCareFixture(t *testing.T) *careFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	sf, err := services.NewFactory(repos, db, slog.Default())
	require.NoError(t, err)

	autoExcusal := schedule.NewPickupAutoExcusalSyncer(
		repos.StudentPickupException,
		repos.StudentPickupSchedule,
		repos.InstanceStudent,
		db,
	)
	svc := schedule.NewCareScheduleRequestServiceWithPickupChanges(
		repos.CareScheduleChangeRequest,
		repos.Student,
		repos.Person,
		sf.ArrivalSchedule,
		sf.PickupSchedule,
		repos.StudentPickupException,
		repos.Attendance,
		autoExcusal,
		sf.UserContext,
		nil, // emitter — pill emission is best-effort and after-commit; nil no-ops
		nil, // broadcaster — cache-invalidation fan-out; nil no-ops
		slog.Default(),
		sf.StudentAudit,
	)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	staff, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Clara", "Confirm")

	return &careFixture{
		db:           db,
		svc:          svc,
		sf:           sf,
		repos:        repos,
		chain:        chain,
		staffAccount: staffAccount.ID,
		staffID:      staff.ID,
		autoExcusal:  autoExcusal,
	}
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

func (f *careFixture) seedGuardianPickupAutoExcusal(t *testing.T, date timezone.Date, pickupTime time.Time) {
	t.Helper()
	ctx := f.staffCtx(f.staffAccount)
	require.NoError(t, tenant.WithTenantTx(ctx, f.db, f.chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		if err := schedule.LockCareExceptionDay(txCtx, f.db, f.chain.StudentID, date); err != nil {
			return err
		}
		exception := &scheduleModels.StudentPickupException{
			TenantModel:       modelBase.TenantModel{TenantID: f.chain.TenantID},
			StudentID:         f.chain.StudentID,
			ExceptionDate:     date,
			PickupTime:        &pickupTime,
			Source:            scheduleModels.ExceptionSourceGuardian,
			CreatedByGuardian: &f.chain.AccountID,
		}
		if err := f.repos.StudentPickupException.Create(txCtx, exception); err != nil {
			return err
		}
		_, err := f.autoExcusal.Sync(txCtx, exception.ID)
		return err
	}))
}

// nonAdminCtx carries a caller with users:update but NO admin wildcard. Whether
// the per-child write gate (CanUpdateStudent) admits them depends only on the
// account behind the id holding a staff record (#2329): the fixture's staff
// account passes, the guardian account does not.
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

type failingPickupDeleteService struct {
	schedule.PickupScheduleService
	err error
}

func (s failingPickupDeleteService) DeleteStudentPickupSchedule(context.Context, int64) error {
	return s.err
}

// createPending stores a pending request submitted by the chain's guardian.
func (f *careFixture) createPending(t *testing.T, payload map[string]any) *scheduleModels.CareScheduleChangeRequest {
	t.Helper()
	req, err := f.svc.CreateRequest(f.staffCtx(f.chain.AccountID), f.chain.StudentID, f.chain.AccountID, payload)
	require.NoError(t, err)
	return req
}

// TestDecide_ForbiddenWithoutStaffRecord proves the per-child write gate after
// #2329: a caller without a staff record (the guardian account) — even holding
// users:update — sees nothing in the queue and cannot decide either way, while
// a verified non-admin staffer with the same permission sees and decides the
// same request.
func TestDecide_ForbiddenWithoutStaffRecord(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	req := f.createPending(t, careWeekdays(
		map[string]any{"weekday": 1, "mode": "pickup", "arrival": "08:00", "pickup": "16:00"},
	))

	denyCtx := f.nonAdminCtx(f.chain.AccountID)

	items, _, err := f.svc.ListPending(denyCtx, modelBase.RequestQueueFilters{})
	require.NoError(t, err)
	assert.Empty(t, items, "a caller without a staff record must not see the queue")

	_, err = f.svc.Decide(denyCtx, schedule.CareRequestDecideInput{RequestID: req.ID, Approve: true, ReviewedBy: f.staffAccount})
	assert.ErrorIs(t, err, schedule.ErrCareRequestForbidden)
	_, err = f.svc.Decide(denyCtx, schedule.CareRequestDecideInput{RequestID: req.ID, Approve: false, Reason: "nope", ReviewedBy: f.staffAccount})
	assert.ErrorIs(t, err, schedule.ErrCareRequestForbidden)

	staffCtx := f.nonAdminCtx(f.staffAccount)
	items, _, err = f.svc.ListPending(staffCtx, modelBase.RequestQueueFilters{})
	require.NoError(t, err)
	require.Len(t, items, 1, "a verified staffer with users:update sees the request")

	item, err := f.svc.Decide(staffCtx, schedule.CareRequestDecideInput{RequestID: req.ID, Approve: false, Reason: "closing out", ReviewedBy: f.staffAccount})
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.CareRequestStatusRejected, item.Request.Status)
}

// --- Decide: approve applies the plan --------------------------------------

// TestDecide_ApproveAppliesModeArrivalPickup is the staff "Bestätigen" happy
// path: approving a pending request DURABLY applies the child's permanent
// weekly plan (departure mode + arrival + pickup for the requested weekday).
func TestDecide_ApproveAppliesModeArrivalPickup(t *testing.T) {
	t.Parallel()

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

	history, err := f.sf.StudentAudit.GetChangeHistory(ctx, f.chain.StudentID)
	require.NoError(t, err)
	require.NotEmpty(t, history)
	var departureEditFound bool
	for _, edit := range history {
		if edit.FieldName == "departure_days" {
			departureEditFound = true
			assert.Equal(t, f.staffAccount, edit.EditedBy)
		}
	}
	assert.True(t, departureEditFound)
}

// TestDecide_ApproveMergesPreservingOtherDaysAndModes pins the read-modify-write
// merge: approving a request that only touches Monday must NOT wipe an unrelated
// Tuesday arrival saved directly, and must preserve a multi-mode (bus+pickup)
// Tuesday departure set — the collapse bug the apply guards against.
func TestDecide_ApproveMergesPreservingOtherDaysAndModes(t *testing.T) {
	t.Parallel()

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

func TestDecide_ApproveInactiveCareDayRemovesWeeklyPlan(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	ctx := f.staffCtx(f.staffAccount)
	seedCareDay(t, f, ctx, 2)
	req := f.createPending(t, careWeekdays(map[string]any{"weekday": 2, "scheduled": false}))

	err := tenant.WithTenantTx(ctx, f.db, f.chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, err := f.svc.Decide(txCtx, schedule.CareRequestDecideInput{
			RequestID: req.ID, Approve: true, ReviewedBy: f.staffAccount,
		})
		return err
	})
	require.NoError(t, err)

	arrivals, err := f.sf.ArrivalSchedule.GetStudentArrivalSchedules(ctx, f.chain.StudentID)
	require.NoError(t, err)
	assert.Empty(t, arrivals)
	pickups, err := f.sf.PickupSchedule.GetStudentPickupSchedules(ctx, f.chain.StudentID)
	require.NoError(t, err)
	assert.Empty(t, pickups)
	student, err := f.sf.Users.GetStudentByID(ctx, f.chain.StudentID)
	require.NoError(t, err)
	assert.NotContains(t, student.AllowedDepartureModes, "tue")
}

func TestDecide_InactiveCareDayRollsBackWhenPickupDeleteFails(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	ctx := f.staffCtx(f.staffAccount)
	seedCareDay(t, f, ctx, 2)
	req := f.createPending(t, careWeekdays(map[string]any{"weekday": 2, "scheduled": false}))
	wantErr := errors.New("pickup delete failed")
	failingService := schedule.NewCareScheduleRequestService(
		f.repos.CareScheduleChangeRequest,
		f.repos.Student,
		f.repos.Person,
		f.sf.ArrivalSchedule,
		failingPickupDeleteService{PickupScheduleService: f.sf.PickupSchedule, err: wantErr},
		f.sf.UserContext,
		nil,
		nil,
		slog.Default(),
		f.sf.StudentAudit,
	)

	err := tenant.WithTenantTx(ctx, f.db, f.chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, err := failingService.Decide(txCtx, schedule.CareRequestDecideInput{
			RequestID: req.ID, Approve: true, ReviewedBy: f.staffAccount,
		})
		return err
	})
	require.ErrorIs(t, err, wantErr)

	arrivals, err := f.sf.ArrivalSchedule.GetStudentArrivalSchedules(ctx, f.chain.StudentID)
	require.NoError(t, err)
	require.Len(t, arrivals, 1)
	pickups, err := f.sf.PickupSchedule.GetStudentPickupSchedules(ctx, f.chain.StudentID)
	require.NoError(t, err)
	require.Len(t, pickups, 1)
	student, err := f.sf.Users.GetStudentByID(ctx, f.chain.StudentID)
	require.NoError(t, err)
	assert.Equal(t, []usersModels.DepartureMode{usersModels.DeparturePickup}, student.AllowedDepartureModes["tue"])
	pending, _, err := f.svc.GetPendingForStudent(ctx, f.chain.StudentID)
	require.NoError(t, err)
	require.NotNil(t, pending)
	assert.Equal(t, req.ID, pending.ID)
}

func seedCareDay(t *testing.T, f *careFixture, ctx context.Context, weekday int) {
	t.Helper()
	student, err := f.sf.Users.GetStudentByID(ctx, f.chain.StudentID)
	require.NoError(t, err)
	student.AllowedDepartureModes = usersModels.AllowedDepartureModes{
		usersModels.PickupDayOrder[weekday-1]: {usersModels.DeparturePickup},
	}
	student.DepartureDays = nil
	student.BusDays = nil
	student.PickupDays = nil
	require.NoError(t, f.repos.Student.Update(ctx, student))
	require.NoError(t, f.sf.ArrivalSchedule.UpsertStudentArrivalSchedule(ctx,
		&scheduleModels.StudentArrivalSchedule{
			StudentID:       f.chain.StudentID,
			Weekday:         weekday,
			ExpectedArrival: timezone.WallClock(time.Date(1, 1, 1, 8, 0, 0, 0, time.UTC)),
			CreatedBy:       f.staffID,
		}))
	require.NoError(t, f.sf.PickupSchedule.UpsertStudentPickupSchedule(ctx,
		&scheduleModels.StudentPickupSchedule{
			StudentID:  f.chain.StudentID,
			Weekday:    weekday,
			PickupTime: timezone.WallClock(time.Date(1, 1, 1, 15, 30, 0, 0, time.UTC)),
			CreatedBy:  f.staffID,
		}))
}

// TestDecide_NonStaffConfirmerForbidden: an account without a users.staff row
// (here the guardian account) cannot be stamped as the confirming staff, so the
// apply returns ErrCareRequestForbidden rather than a 500.
func TestDecide_NonStaffConfirmerForbidden(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	f := newCareFixture(t)
	// This test actually writes pills (created + status) from the separate staff
	// account, whose sender_account_id FKs auth.accounts without cascade. Register
	// the clear LAST so it runs FIRST (LIFO) ahead of CleanupAuthFixtures(staff).
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

// TestCreateRequest_RequestCreatedPillDoesNotAdvanceThreadPreview pins the #1803
// preview/badge alignment fix: a request_created pill is a QUEUE signal, not a
// chat message, so it must NOT become the thread's denormalized last-activity
// preview (last_message_*) — exactly as counterpartUnread excludes it from every
// unread count. Otherwise the Nachrichten badge (which flags an earlier staff
// decision) and the thread preview (the parent's own "Anfrage gestellt") point at
// different messages, so a parent's OWN submission reads as an unread message to
// themselves. A subsequent non-request_created pill (the reject decision) MUST
// advance the preview as before.
func TestCreateRequest_RequestCreatedPillDoesNotAdvanceThreadPreview(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	// Created pill (guardian) + status pill (staff) both reference auth.accounts
	// without cascade; clear them LIFO-first, ahead of the staff auth cleanup.
	svc := schedule.NewCareScheduleRequestService(
		f.repos.CareScheduleChangeRequest,
		f.repos.Student,
		f.repos.Person,
		f.sf.ArrivalSchedule,
		f.sf.PickupSchedule,
		f.sf.UserContext,
		parentmessaging.NewEmitter(f.db, f.repos.ParentMessageThread, f.repos.ParentMessage, &toggleSettings{enabled: true}, nil, slog.Default()),
		nil,
		slog.Default(),
	)

	// Filing a request opens the thread and writes the request_created pill.
	req, err := svc.CreateRequest(f.staffCtx(f.chain.AccountID), f.chain.StudentID, f.chain.AccountID,
		careWeekdays(map[string]any{"weekday": 1, "mode": "pickup", "arrival": "08:00", "pickup": "16:00"}))
	require.NoError(t, err)

	ctx := f.staffCtx(f.staffAccount)
	thread, err := f.repos.ParentMessageThread.FindByStudentGuardian(ctx, f.chain.StudentID, f.chain.AccountID)
	require.NoError(t, err)
	require.NotNil(t, thread, "the request_created pill opens the thread")
	created, err := f.repos.ParentMessage.FindEventByRef(ctx, thread.ID, usersModels.ParentMessageEventRequestCreated, "schedule.care_schedule_change_requests", req.ID)
	require.NoError(t, err)
	require.NotNil(t, created, "request_created pill exists")

	// The request_created pill must NOT have advanced the thread preview/sort.
	assert.Nil(t, thread.LastMessageAt, "a request_created pill must not set the thread's last_message_at")
	assert.Nil(t, thread.LastMessageID, "a request_created pill must not become the thread's last_message")
	assert.Empty(t, thread.LastMessageBody, "a request_created pill must not become the thread preview body")

	// A non-request_created pill (the reject decision) DOES advance the preview.
	_, err = svc.Decide(ctx, schedule.CareRequestDecideInput{RequestID: req.ID, Approve: false, Reason: "telefonisch geklärt", ReviewedBy: f.staffAccount})
	require.NoError(t, err)

	status, err := f.repos.ParentMessage.FindEventByRef(ctx, thread.ID, usersModels.ParentMessageEventRequestStatus, "schedule.care_schedule_change_requests", req.ID)
	require.NoError(t, err)
	require.NotNil(t, status, "the reject decision writes a request_status pill")

	decided, err := f.repos.ParentMessageThread.FindByStudentGuardian(ctx, f.chain.StudentID, f.chain.AccountID)
	require.NoError(t, err)
	require.NotNil(t, decided)
	require.NotNil(t, decided.LastMessageID, "a request_status decision advances the thread preview")
	assert.Equal(t, status.ID, *decided.LastMessageID, "the decision pill owns the preview")
	assert.NotEmpty(t, decided.LastMessageBody, "the decision pill sets the preview body")
}

// TestDecide_NoReconcilePillWhenRequestFiledWhileDisabled pins the other half of
// the reconcile gate: if the request was filed while messaging was already OFF,
// no thread / request_created pill was ever emitted, so REJECTING must NOT
// conjure a dangling request_status pill (or create a thread). Only an
// already-open notice is reconciled.
func TestDecide_NoReconcilePillWhenRequestFiledWhileDisabled(t *testing.T) {
	t.Parallel()

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

// TestDecide_ApproveAllowedWhenMessagingDisabled pins that request decisions
// are independent from the optional parent-message channel.
func TestDecide_ApproveAllowedWhenMessagingDisabled(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	// Wire an emitter whose settings report messaging OFF; notification pills
	// are dropped, but the request workflow remains available.
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

	item, err := svc.Decide(ctx, schedule.CareRequestDecideInput{RequestID: req.ID, Approve: true, ReviewedBy: f.staffAccount})
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.CareRequestStatusApproved, item.Request.Status)

	stored, err := f.repos.CareScheduleChangeRequest.FindByID(ctx, req.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.CareRequestStatusApproved, stored.Status)

	arrivals, err := f.sf.ArrivalSchedule.GetStudentArrivalSchedules(ctx, f.chain.StudentID)
	require.NoError(t, err)
	require.NotEmpty(t, arrivals, "approval applies the requested plan without messaging")
}

// TestDecide_SecondDecideNotPending pins the idempotency/race guard: once a
// request is approved, a second decide (or a reject racing it) sees the terminal
// status under the FOR UPDATE re-read and returns ErrCareRequestNotPending.
func TestDecide_SecondDecideNotPending(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

	// A foreign account probing the now-TERMINAL row must still get not-found,
	// not the not-pending the submitter gets: ownership is checked before the
	// pending-status distinction, so a decided request's id stays unprobeable.
	_, err = f.svc.WithdrawRequest(ctx, req.ID, f.chain.StudentID, other.ID)
	require.ErrorIs(t, err, scheduleModels.ErrCareRequestNotFound, "a foreign account cannot probe a decided request's id")
}

// --- GetPendingForStudent --------------------------------------------------

// TestGetPendingForStudent_NoneReturnsNil pins that a child with no open request
// yields (nil, nil, nil) — the read view shows no pending card.
func TestGetPendingForStudent_NoneReturnsNil(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	req, diff, err := f.svc.GetPendingForStudent(f.staffCtx(f.staffAccount), f.chain.StudentID)
	require.NoError(t, err)
	assert.Nil(t, req)
	assert.Nil(t, diff)
}

// TestGetPendingForStudent_ReturnsPendingWithDiff pins that an open request is
// returned with a live "current → requested" diff.
func TestGetPendingForStudent_ReturnsPendingWithDiff(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	ctx := f.staffCtx(f.chain.AccountID)
	f.createPending(t, careWeekdays(map[string]any{"weekday": 1, "arrival": "08:00"}))

	req, diff, err := f.svc.GetPendingForStudent(ctx, f.chain.StudentID)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.NotEmpty(t, diff, "an open request carries its current→requested diff")
	assert.Equal(t, "08:00", diff[0].New)
}

// TestListPending_ReturnsEnrichedItemWithDiff drives the staff-queue success
// path: a writable caller (admin) sees the open request enriched with the
// child's name and a live current→requested diff. This is the counterpart to
// the forbidden-scope test, which only exercises the empty-queue branch.
func TestListPending_ReturnsEnrichedItemWithDiff(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	f.createPending(t, careWeekdays(
		map[string]any{"weekday": 1, "mode": "pickup", "arrival": "08:00", "pickup": "16:00"},
	))

	items, _, err := f.svc.ListPending(f.staffCtx(f.staffAccount), modelBase.RequestQueueFilters{})
	require.NoError(t, err)
	require.Len(t, items, 1, "the admin queue must surface the open request")

	item := items[0]
	assert.NotEmpty(t, item.FirstName, "the queue item is enriched with the child's name")
	require.NotEmpty(t, item.Diff, "the queue item carries a current→requested diff")

	// The requested Monday values must appear in the diff's "New" column.
	news := make(map[string]bool)
	for _, d := range item.Diff {
		news[d.New] = true
	}
	assert.True(t, news["08:00"], "requested arrival should appear in the diff")
	assert.True(t, news["16:00"], "requested pickup should appear in the diff")
}

// TestDecide_ApproveBroadcastsCacheInvalidation proves that applying a care
// request fans out the cache-invalidation events (student_updated tenant-wide,
// arrival_schedule_changed globally) so every open kiosk/dashboard refetches the
// new plan. The shared fixture wires a nil broadcaster; this rebuilds the service
// with a capturing one against the SAME db/repos.
func TestDecide_ApproveBroadcastsCacheInvalidation(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	req := f.createPending(t, careWeekdays(
		map[string]any{"weekday": 1, "arrival": "08:00", "pickup": "16:00"},
	))

	bc := testpkg.NewRecordingBroadcaster()
	svc := schedule.NewCareScheduleRequestService(
		f.repos.CareScheduleChangeRequest, f.repos.Student, f.repos.Person,
		f.sf.ArrivalSchedule, f.sf.PickupSchedule, f.sf.UserContext,
		nil, bc, slog.Default(),
	)

	_, err := svc.Decide(f.staffCtx(f.staffAccount), schedule.CareRequestDecideInput{
		RequestID: req.ID, Approve: true, ReviewedBy: f.staffAccount,
	})
	require.NoError(t, err)

	assert.Positive(t, len(bc.CallsByMethod("tenant")), "approve must broadcast student_updated to the tenant")
	assert.Positive(t, len(bc.CallsByMethod("all")), "approve must broadcast arrival_schedule_changed globally")
}

// TestDecide_ApproveCompanionEventOnlyOnEffectiveChange pins WHO the approval
// wakes: student_companions_changed makes every open "läuft mit" editor in the
// school mark itself stale and refuse to save, so it may only fire when the
// approval actually reconciled a link. Carrying departure modes is not that
// question — a merge that drops no link (and a time-only request, which cannot
// drop one at all) has to stay silent, or routine parent requests cost unrelated
// staff their unsaved companion edits.
func TestDecide_ApproveCompanionEventOnlyOnEffectiveChange(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	bc := testpkg.NewRecordingBroadcaster()
	svc := schedule.NewCareScheduleRequestService(
		f.repos.CareScheduleChangeRequest, f.repos.Student, f.repos.Person,
		f.sf.ArrivalSchedule, f.sf.PickupSchedule, f.sf.UserContext,
		nil, bc, slog.Default(),
	)

	approve := func(t *testing.T, payload map[string]any) {
		t.Helper()
		req := f.createPending(t, payload)
		bc.Reset()
		_, err := svc.Decide(f.staffCtx(f.staffAccount), schedule.CareRequestDecideInput{
			RequestID: req.ID, Approve: true, ReviewedBy: f.staffAccount,
		})
		require.NoError(t, err)
	}

	t.Run("time-only approval stays silent", func(t *testing.T) {
		approve(t, careWeekdays(map[string]any{"weekday": 1, "arrival": "08:00", "pickup": "16:00"}))
		assert.False(t, bc.HasEventType(realtime.EventStudentCompanionsChanged),
			"a request that only moves times cannot touch a link and must not mark companion editors stale")
		assert.True(t, bc.HasEventType(realtime.EventStudentUpdated),
			"the ordinary student_updated invalidation still has to fire")
	})

	t.Run("departure-mode approval without links stays silent", func(t *testing.T) {
		approve(t, careWeekdays(map[string]any{"weekday": 3, "mode": "bus"}))
		assert.False(t, bc.HasEventType(realtime.EventStudentCompanionsChanged),
			"a plan merge that trims no link changed no Laufgemeinschaft to announce")
		assert.True(t, bc.HasEventType(realtime.EventStudentUpdated),
			"the ordinary student_updated invalidation still has to fire")
	})

	t.Run("departure-mode approval that drops a link announces it", func(t *testing.T) {
		f.linkCompanionOnTuesday(t)
		approve(t, careWeekdays(map[string]any{"weekday": 2, "mode": "bus"}))
		assert.True(t, bc.HasEventType(realtime.EventStudentCompanionsChanged),
			"Tuesday no longer allows 'Anderes Kind', so the link is gone from the partner's card too")
	})
}

// linkCompanionOnTuesday gives the chain's child a Laufgemeinschaft partner on
// Tuesday: both children get an accompanied Tuesday plus their own free-text
// "mit wem" note (so dropping the link later does not strand the partner, which
// is a different rule with its own test), then the undirected edge is written.
func (f *careFixture) linkCompanionOnTuesday(t *testing.T) {
	t.Helper()
	ctx := testpkg.TenantContext(f.chain.TenantID)

	partner := testpkg.CreateTestStudent(t, f.db, "Companion", "Partner", "1a")
	t.Cleanup(func() {
		_, err := f.db.NewDelete().
			TableExpr("users.student_companions").
			Where("student_low_id IN (?, ?) OR student_high_id IN (?, ?)",
				f.chain.StudentID, partner.ID, f.chain.StudentID, partner.ID).
			Exec(context.Background())
		if err != nil {
			t.Logf("Warning: failed to cleanup users.student_companions: %v", err)
		}
	})

	accompaniedTuesday := func(studentID int64) {
		student, err := f.repos.Student.FindByID(ctx, studentID)
		require.NoError(t, err)
		modes := usersModels.AllowedDepartureModes{}
		for day, allowed := range student.AllowedDepartureModes {
			modes[day] = allowed
		}
		modes["tue"] = []usersModels.DepartureMode{usersModels.DepartureAccompanied}
		student.AllowedDepartureModes = modes
		student.DepartureDays = nil
		student.BusDays = nil
		student.PickupDays = nil
		note := "Nachbarskind"
		student.DepartureCompanionNote = &note
		require.NoError(t, f.repos.Student.Update(ctx, student))
	}
	accompaniedTuesday(partner.ID)
	accompaniedTuesday(f.chain.StudentID)

	edge, err := usersModels.NewStudentCompanion(f.chain.StudentID, partner.ID, 2)
	require.NoError(t, err)
	require.NoError(t, f.repos.StudentCompanion.ReplaceForStudent(ctx, f.chain.StudentID, []*usersModels.StudentCompanion{edge}))
}

// TestWithdrawRequest_BogusIDNotFound covers the repository's no-rows lock
// branch: withdrawing an id that exists in no tenant returns not-found (never a
// panic or a leak of another child's row). The id is derived from a real
// fixture request, then offset past any real row, to stay hermetic.
func TestWithdrawRequest_BogusIDNotFound(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	req := f.createPending(t, careWeekdays(map[string]any{"weekday": 1, "arrival": "08:00"}))
	bogusID := req.ID + 1_000_000

	_, err := f.svc.WithdrawRequest(f.staffCtx(f.chain.AccountID), bogusID, f.chain.StudentID, f.chain.AccountID)
	require.ErrorIs(t, err, scheduleModels.ErrCareRequestNotFound,
		"withdrawing a non-existent request must be not-found")
}

// TestDecide_BogusIDNotFound covers the staff-decision lock on a missing row:
// the pending-row lookup returns not-found, so Decide surfaces it instead of
// dereferencing a nil request.
func TestDecide_BogusIDNotFound(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	req := f.createPending(t, careWeekdays(map[string]any{"weekday": 1, "arrival": "08:00"}))
	bogusID := req.ID + 1_000_000

	_, err := f.svc.Decide(f.staffCtx(f.staffAccount), schedule.CareRequestDecideInput{
		RequestID: bogusID, Approve: true, ReviewedBy: f.staffAccount,
	})
	require.ErrorIs(t, err, scheduleModels.ErrCareRequestNotFound,
		"deciding a non-existent request must be not-found")
}

// TestCareRequestLifecycle_WakesAllGuardians covers #1725 review finding #2: every
// care-schedule change-request lifecycle transition (create / withdraw / decide)
// posts its notification pill only to the SUBMITTING guardian's thread, and the
// approve-time staff broadcasts (student_updated / arrival_schedule_changed) never
// reach the parents SSE stream. So each transition must ALSO fan a message-
// independent parent_child_updated out to EVERY guardian of the child, or a
// co-guardian's open child-detail view keeps a stale pending badge (create/
// withdraw) or pickup time (approve) until reload. A RecordingBroadcaster wired
// into the emitter proves the guardian wake fires on each transition.
func TestCareRequestLifecycle_WakesAllGuardians(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	// The create/withdraw/decide pills reference the staff + guardian accounts
	// without cascade; clear them LIFO-first, ahead of the auth cleanup.

	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := schedule.NewCareScheduleRequestService(
		f.repos.CareScheduleChangeRequest,
		f.repos.Student,
		f.repos.Person,
		f.sf.ArrivalSchedule,
		f.sf.PickupSchedule,
		f.sf.UserContext,
		parentmessaging.NewEmitter(f.db, f.repos.ParentMessageThread, f.repos.ParentMessage, &toggleSettings{enabled: true}, broadcaster, slog.Default()),
		broadcaster,
		slog.Default(),
	)

	assertWoke := func(t *testing.T, label string) {
		t.Helper()
		woke := false
		for _, c := range broadcaster.CallsByMethod("guardian") {
			if c.GuardianID == f.chain.AccountID && c.Event.Type == realtime.EventParentChildUpdated {
				woke = true
				assert.Equal(t, f.chain.TenantID, c.TenantID)
			}
		}
		assert.Truef(t, woke, "%s must wake the child's guardian %d with parent_child_updated (#1725)", label, f.chain.AccountID)
	}

	// create → wake
	broadcaster.Reset()
	req, err := svc.CreateRequest(f.staffCtx(f.chain.AccountID), f.chain.StudentID, f.chain.AccountID,
		careWeekdays(map[string]any{"weekday": 1, "mode": "pickup", "arrival": "08:00", "pickup": "16:00"}))
	require.NoError(t, err)
	assertWoke(t, "creating a care request")

	// withdraw (submitter withdraws own pending request) → wake
	broadcaster.Reset()
	_, err = svc.WithdrawRequest(f.staffCtx(f.chain.AccountID), req.ID, f.chain.StudentID, f.chain.AccountID)
	require.NoError(t, err)
	assertWoke(t, "withdrawing a care request")

	// re-file, then approve (applies the weekly plan) → wake
	req2, err := svc.CreateRequest(f.staffCtx(f.chain.AccountID), f.chain.StudentID, f.chain.AccountID,
		careWeekdays(map[string]any{"weekday": 2, "pickup": "15:00"}))
	require.NoError(t, err)
	broadcaster.Reset()
	_, err = svc.Decide(f.staffCtx(f.staffAccount), schedule.CareRequestDecideInput{RequestID: req2.ID, Approve: true, ReviewedBy: f.staffAccount})
	require.NoError(t, err)
	assertWoke(t, "approving a care request")
}
