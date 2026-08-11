package absence_test

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
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	absenceSvc "github.com/moto-nrw/project-phoenix/services/absence"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// messagingDisabledSettings makes the emitter fail closed (no thread created)
// while still exercising the after-commit pill-scheduling code path.
type messagingDisabledSettings struct{}

func (messagingDisabledSettings) ResolveBoolForTenant(_ context.Context, _ int64, _ string) (bool, error) {
	return false, nil
}

// countingBroadcaster records how often BroadcastToTenant fired (the staff wake)
// and which guardian accounts BroadcastToGuardian woke (the message-independent
// guardian fan-out), so both after-commit broadcasts can be asserted. It also
// counts BroadcastParentMessage calls to prove the guardian child-update fan-out
// no longer routes through the per-guardian staff-copy path (#1845 review).
type countingBroadcaster struct {
	tenantBroadcasts int
	tenantEvents     []realtime.EventType
	guardianWakeups  []int64
	parentMessages   int
}

func (b *countingBroadcaster) BroadcastToGroup(int64, string, realtime.Event) error { return nil }
func (b *countingBroadcaster) BroadcastToStaffAccounts(_ int64, _ []int64, _ realtime.Event) error {
	return nil
}

func (b *countingBroadcaster) BroadcastToTenant(_ int64, event realtime.Event) error {
	b.tenantBroadcasts++
	b.tenantEvents = append(b.tenantEvents, event.Type)
	return nil
}
func (b *countingBroadcaster) BroadcastToTenantAdmins(_ int64, _ realtime.Event) error {
	return nil
}
func (b *countingBroadcaster) BroadcastToAll(realtime.Event) error { return nil }
func (b *countingBroadcaster) BroadcastParentMessage(_, _ int64, _ realtime.Event) error {
	b.parentMessages++
	return nil
}
func (b *countingBroadcaster) BroadcastToGuardian(_, guardianAccountID int64, _ realtime.Event) error {
	b.guardianWakeups = append(b.guardianWakeups, guardianAccountID)
	return nil
}

// buildAbsenceService wires the excused-request service against the real test DB
// with a real (messaging-disabled) emitter and a counting broadcaster so every
// after-commit hook is exercised.
func buildAbsenceService(t *testing.T) (absenceSvc.ExcusedAbsenceRequestService, *countingBroadcaster, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	repos := repositories.NewFactory(db)
	bc := &countingBroadcaster{}
	emitter := parentmessaging.NewEmitter(
		db,
		repos.ParentMessageThread,
		repos.ParentMessage,
		messagingDisabledSettings{},
		bc,
		slog.Default(),
	)
	svc := absenceSvc.NewExcusedAbsenceRequestServiceWithPartialAbsences(
		repos.ExcusedAbsenceRequest,
		repos.StudentStatusDay,
		repos.StudentPickupException,
		repos.Student,
		repos.Person,
		nil, // userContext: admin:* perms in the ctx short-circuit the write gate
		emitter,
		bc,
		nil, // logger: nil-safe, falls back to slog.Default()
		db,
	)
	return svc, bc, db
}

// adminCtx carries wildcard-admin permissions so the staff decide/list gate
// authorizes without a wired userContext.
func adminCtx() context.Context {
	return context.WithValue(context.Background(), jwt.CtxPermissions, []string{"admin:*"})
}

// createPending stores a pending request for the chain's child inside a tenant
// transaction, mirroring how the parent write service calls CreateRequest.
func createPending(t *testing.T, svc absenceSvc.ExcusedAbsenceRequestService, db *bun.DB, chain testpkg.ParentChain, dates []timezone.Date, note string) *activeModels.ExcusedAbsenceRequest {
	t.Helper()
	var req *activeModels.ExcusedAbsenceRequest
	err := tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		var e error
		req, e = svc.CreateRequest(txCtx, chain.StudentID, chain.AccountID, dates, note)
		return e
	})
	require.NoError(t, err)
	require.NotNil(t, req)
	return req
}

// TestCreateRequest_Validation covers the three input guards that reject before
// any DB write, so no tenant transaction is needed.
func TestCreateRequest_Validation(t *testing.T) {
	svc, _, _ := buildAbsenceService(t)
	day := timezone.TodayDate().AddDays(2)

	_, err := svc.CreateRequest(context.Background(), 1, 1, nil, "note")
	assert.ErrorIs(t, err, absenceSvc.ErrExcusedRequestNoDates)

	_, err = svc.CreateRequest(context.Background(), 1, 1, []timezone.Date{day}, "   ")
	assert.ErrorIs(t, err, absenceSvc.ErrExcusedRequestEmptyNote)

	_, err = svc.CreateRequest(context.Background(), 1, 1, []timezone.Date{day}, strings.Repeat("x", 2001))
	assert.ErrorIs(t, err, absenceSvc.ErrExcusedRequestNoteTooLong)
}

// TestListPending_EnrichedAndScoped verifies the staff review queue returns
// pending requests newest-first with the child's name filled in.
func TestListPending_EnrichedAndScoped(t *testing.T) {
	svc, _, db := buildAbsenceService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	createPending(t, svc, db, chain, []timezone.Date{timezone.TodayDate().AddDays(2)}, "Arzttermin")
	createPending(t, svc, db, chain, []timezone.Date{timezone.TodayDate().AddDays(5)}, "Familienfeier")

	err := tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		items, e := svc.ListPending(txCtx)
		require.NoError(t, e)
		require.Len(t, items, 2, "both pending requests must surface in the staff queue")
		for _, it := range items {
			assert.Equal(t, chain.StudentID, it.Request.StudentID)
			assert.NotEmpty(t, it.FirstName, "child name must be enriched")
			assert.NotEmpty(t, it.LastName)
			assert.Equal(t, activeModels.ExcusedRequestStatusPending, it.Request.Status)
		}
		return nil
	})
	require.NoError(t, err)
}

// TestListPending_Empty covers the early return when the tenant has no pending
// requests.
func TestListPending_Empty(t *testing.T) {
	svc, _, db := buildAbsenceService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	err := tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		items, e := svc.ListPending(txCtx)
		require.NoError(t, e)
		assert.Empty(t, items)
		return nil
	})
	require.NoError(t, err)
}

// TestPendingByStudentForDate covers the inline planning-badge map: the newest
// pending request that covers the queried day is returned per student, and a day
// no request covers yields no entry.
func TestPendingByStudentForDate(t *testing.T) {
	svc, _, db := buildAbsenceService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	covered := timezone.TodayDate().AddDays(4)
	createPending(t, svc, db, chain, []timezone.Date{covered}, "Familienfeier")

	err := tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		hit, e := svc.PendingByStudentForDate(txCtx, covered)
		require.NoError(t, e)
		req, ok := hit[chain.StudentID]
		require.True(t, ok, "a request covering the day must appear")
		assert.Equal(t, activeModels.ExcusedRequestStatusPending, req.Status)

		miss, e := svc.PendingByStudentForDate(txCtx, covered.AddDays(30))
		require.NoError(t, e)
		assert.Empty(t, miss, "a day no request covers yields no badge")
		return nil
	})
	require.NoError(t, err)
}

// TestListForStudent_FiltersOutcomes verifies the parent view keeps pending and
// rejected requests but hides approved and withdrawn ones.
func TestListForStudent_FiltersOutcomes(t *testing.T) {
	svc, _, db := buildAbsenceService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	pending := createPending(t, svc, db, chain, []timezone.Date{timezone.TodayDate().AddDays(2)}, "bleibt sichtbar")
	rejected := createPending(t, svc, db, chain, []timezone.Date{timezone.TodayDate().AddDays(3)}, "wird abgelehnt")
	withdrawn := createPending(t, svc, db, chain, []timezone.Date{timezone.TodayDate().AddDays(6)}, "wird zurueckgezogen")

	// Reject one (staff) and withdraw one (guardian).
	err := tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		if _, e := svc.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{RequestID: rejected.ID, Approve: false, Reason: "telefonisch klaeren"}); e != nil {
			return e
		}
		_, e := svc.WithdrawRequest(txCtx, withdrawn.ID, chain.StudentID, chain.AccountID)
		return e
	})
	require.NoError(t, err)

	var got []*activeModels.ExcusedAbsenceRequest
	err = tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		var e error
		got, e = svc.ListForStudent(txCtx, chain.StudentID, time.Now().Add(-24*time.Hour))
		return e
	})
	require.NoError(t, err)

	ids := map[int64]string{}
	for _, r := range got {
		ids[r.ID] = r.Status
	}
	assert.Equal(t, activeModels.ExcusedRequestStatusPending, ids[pending.ID], "pending stays visible")
	assert.Equal(t, activeModels.ExcusedRequestStatusRejected, ids[rejected.ID], "rejected stays visible so the parent learns the outcome")
	_, hasWithdrawn := ids[withdrawn.ID]
	assert.False(t, hasWithdrawn, "a withdrawn request must not appear in the parent absence list")
}

// TestPendingByStudentForDate_DedupesPerStudent verifies only the newest
// pending request per student is kept when several cover the same day.
func TestPendingByStudentForDate_DedupesPerStudent(t *testing.T) {
	svc, _, db := buildAbsenceService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	day := timezone.TodayDate().AddDays(4)
	createPending(t, svc, db, chain, []timezone.Date{day}, "aeltere Anfrage")
	newer := createPending(t, svc, db, chain, []timezone.Date{day}, "neuere Anfrage")

	err := tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		hit, e := svc.PendingByStudentForDate(txCtx, day)
		require.NoError(t, e)
		require.Len(t, hit, 1, "only one entry per student")
		assert.Equal(t, newer.ID, hit[chain.StudentID].ID, "the newest pending request wins")
		return nil
	})
	require.NoError(t, err)
}

// TestDecide_ForbiddenWithoutWriteAccess verifies a caller who cannot write the
// child (no admin perms, no supervision) is refused even on a valid pending id.
func TestDecide_ForbiddenWithoutWriteAccess(t *testing.T) {
	svc, _, db := buildAbsenceService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	pending := createPending(t, svc, db, chain, []timezone.Date{timezone.TodayDate().AddDays(2)}, "note")

	// A plain context (no admin:* permissions, nil userContext) cannot write the
	// child, so the decision is forbidden.
	err := tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{RequestID: pending.ID, Approve: true})
		if e != absenceSvc.ErrExcusedRequestForbidden {
			t.Fatalf("expected forbidden, got %v", e)
		}
		return nil
	})
	require.NoError(t, err)
}

// TestDecide_ValidationErrors covers the pre-lock guards.
func TestDecide_ValidationErrors(t *testing.T) {
	svc, _, _ := buildAbsenceService(t)

	_, err := svc.Decide(context.Background(), absenceSvc.ExcusedRequestDecideInput{RequestID: 0, Approve: true})
	assert.ErrorIs(t, err, activeModels.ErrExcusedRequestNotFound)

	_, err = svc.Decide(context.Background(), absenceSvc.ExcusedRequestDecideInput{RequestID: 5, Approve: false, Reason: "  "})
	assert.ErrorIs(t, err, absenceSvc.ErrExcusedRequestRejectReasonRequired)

	_, err = svc.Decide(context.Background(), absenceSvc.ExcusedRequestDecideInput{RequestID: 5, Approve: false, Reason: strings.Repeat("x", 2001)})
	assert.ErrorIs(t, err, absenceSvc.ErrExcusedRequestRejectReasonTooLong)
}

// TestDecide_NotFoundAndNotPending covers the locked-row error mapping.
func TestDecide_NotFoundAndNotPending(t *testing.T) {
	svc, _, db := buildAbsenceService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	pending := createPending(t, svc, db, chain, []timezone.Date{timezone.TodayDate().AddDays(2)}, "note")

	err := tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		// Non-existent id under this tenant → not found.
		if _, e := svc.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{RequestID: pending.ID + 999999, Approve: true}); e != activeModels.ErrExcusedRequestNotFound {
			t.Fatalf("expected not-found, got %v", e)
		}
		// Approve once, then a second decide finds it non-pending.
		if _, e := svc.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{RequestID: pending.ID, Approve: true}); e != nil {
			return e
		}
		if _, e := svc.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{RequestID: pending.ID, Approve: true}); e != activeModels.ErrExcusedRequestNotPending {
			t.Fatalf("expected not-pending on second decide, got %v", e)
		}
		return nil
	})
	require.NoError(t, err)
}

// TestDecide_ApproveClearsLiveSickToday exercises applyExcusedRequest's
// today-branch: approving a request that includes today clears a stale live sick
// flag and broadcasts a student update.
func TestDecide_ApproveClearsLiveSickToday(t *testing.T) {
	svc, bc, db := buildAbsenceService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	// Mark the child live-sick so the approve path has something to clear.
	_, err := db.NewUpdate().
		Table("users.students").
		Set("sick = ?", true).
		Set("sick_since = ?", time.Now()).
		Where("id = ?", chain.StudentID).
		Exec(context.Background())
	require.NoError(t, err)

	today := timezone.TodayDate()
	pending := createPending(t, svc, db, chain, []timezone.Date{today}, "krank gemeldet, jetzt entschuldigt")

	err = tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		item, e := svc.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{RequestID: pending.ID, Approve: true, ReviewedBy: chain.AccountID})
		if e != nil {
			return e
		}
		assert.Equal(t, activeModels.ExcusedRequestStatusApproved, item.Request.Status)
		return nil
	})
	require.NoError(t, err)

	var sick *bool
	err = db.NewSelect().
		Table("users.students").
		Column("sick").
		Where("id = ?", chain.StudentID).
		Scan(context.Background(), &sick)
	require.NoError(t, err)
	require.NotNil(t, sick)
	assert.False(t, *sick, "approving an excused request that includes today clears the live sick flag")
	assert.Positive(t, bc.tenantBroadcasts, "an approval must broadcast a student update")
}

// TestTransition_WakesGuardiansIndependentOfMessaging pins the message-independent
// guardian invalidation (#1845 review): every excused-request transition must wake
// EVERY guardian of the child over the parent SSE fan-out even when parent
// messaging is DISABLED — buildAbsenceService wires messagingDisabledSettings, so a
// non-empty guardian wakeup here proves the wake does NOT depend on the messaging
// setting (nor on which guardian acted). The old submitter-only, messaging-gated
// pill path reached neither a co-guardian nor a messaging-off school, leaving an
// already-open parents-app tab stale until a focus refetch.
func TestTransition_WakesGuardiansIndependentOfMessaging(t *testing.T) {
	svc, bc, db := buildAbsenceService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	pending := createPending(t, svc, db, chain, []timezone.Date{timezone.TodayDate().AddDays(2)}, "Arzttermin")
	assert.Contains(t, bc.guardianWakeups, chain.AccountID,
		"creating a pending request must wake the child's guardian with messaging off")
	assert.Contains(t, bc.tenantEvents, realtime.EventChangeRequestsChanged,
		"creating a pending request must wake the staff review queue with messaging off")

	bc.guardianWakeups = nil
	bc.tenantEvents = nil
	err := tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{RequestID: pending.ID, Approve: true, ReviewedBy: chain.AccountID})
		return e
	})
	require.NoError(t, err)
	assert.Contains(t, bc.guardianWakeups, chain.AccountID,
		"deciding a request must wake the child's guardian with messaging off")
	assert.Contains(t, bc.tenantEvents, realtime.EventChangeRequestsChanged,
		"deciding a request must wake the staff review queue with messaging off")
	assert.Zero(t, bc.parentMessages,
		"the guardian child-update fan-out must use BroadcastToGuardian, never the per-guardian BroadcastParentMessage staff-copy path (#1845 review)")
}

// TestDecide_ApproveRefusedWhenGuardianAccessRevoked pins the guardian-link gate
// (#1845 review follow-up): if the submitting guardian loses parent_portal.access
// after filing the request but before staff review, APPROVING is refused (it
// would write parent-sourced excused status days and notify an account the parent
// APIs now hide) and the request stays pending; REJECTING stays available so
// staff can wind it down. Mirrors the care-schedule request guard.
func TestDecide_ApproveRefusedWhenGuardianAccessRevoked(t *testing.T) {
	svc, _, db := buildAbsenceService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	pending := createPending(t, svc, db, chain, []timezone.Date{timezone.TodayDate().AddDays(2)}, "Arzttermin")

	// Revoke the guardian's portal access to the child (unlink / downgrade).
	_, err := db.ExecContext(context.Background(), `
		UPDATE users.students_guardians SET permissions = '{}'::jsonb
		WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?
	`, chain.TenantID, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)

	err = tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		// Approving a request whose submitter lost access is refused, and the row
		// stays pending so staff can still reject it.
		if _, e := svc.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{RequestID: pending.ID, Approve: true, ReviewedBy: chain.AccountID}); e != absenceSvc.ErrExcusedRequestGuardianAccessRevoked {
			t.Fatalf("expected guardian-access-revoked, got %v", e)
		}
		still, e := svc.ListForStudent(txCtx, chain.StudentID, time.Now().Add(-24*time.Hour))
		if e != nil {
			return e
		}
		require.Len(t, still, 1)
		assert.Equal(t, activeModels.ExcusedRequestStatusPending, still[0].Status, "a refused approval leaves the request pending")

		// Rejecting the same stale request is NOT gated on the guardian link.
		item, e := svc.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{RequestID: pending.ID, Approve: false, Reason: "Elternteil nicht mehr berechtigt", ReviewedBy: chain.AccountID})
		if e != nil {
			return e
		}
		assert.Equal(t, activeModels.ExcusedRequestStatusRejected, item.Request.Status, "a revoked guardian's request can still be rejected")
		return nil
	})
	require.NoError(t, err)
}

// TestWithdrawRequest covers success plus the ownership and status guards.
func TestWithdrawRequest(t *testing.T) {
	svc, _, db := buildAbsenceService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	pending := createPending(t, svc, db, chain, []timezone.Date{timezone.TodayDate().AddDays(2)}, "note")

	err := tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		// Wrong owner → not found (a stranger must not learn the id exists).
		if _, e := svc.WithdrawRequest(txCtx, pending.ID, chain.StudentID, chain.AccountID+424242); e != activeModels.ErrExcusedRequestNotFound {
			t.Fatalf("expected not-found for foreign submitter, got %v", e)
		}
		// Non-existent id → not found.
		if _, e := svc.WithdrawRequest(txCtx, pending.ID+999999, chain.StudentID, chain.AccountID); e != activeModels.ErrExcusedRequestNotFound {
			t.Fatalf("expected not-found for missing id, got %v", e)
		}
		// Owner withdraws successfully.
		out, e := svc.WithdrawRequest(txCtx, pending.ID, chain.StudentID, chain.AccountID)
		if e != nil {
			return e
		}
		assert.Equal(t, activeModels.ExcusedRequestStatusWithdrawn, out.Status)
		// Second withdraw of the now-terminal row → not pending.
		if _, e := svc.WithdrawRequest(txCtx, pending.ID, chain.StudentID, chain.AccountID); e != activeModels.ErrExcusedRequestNotPending {
			t.Fatalf("expected not-pending on second withdraw, got %v", e)
		}
		return nil
	})
	require.NoError(t, err)
}

// TestExcusedAbsenceRequestModel covers the pure model helpers.
func TestExcusedAbsenceRequestModel(t *testing.T) {
	req := &activeModels.ExcusedAbsenceRequest{}

	req.Status = activeModels.ExcusedRequestStatusPending
	assert.False(t, req.IsTerminal(), "pending is not terminal")
	for _, s := range []string{
		activeModels.ExcusedRequestStatusApproved,
		activeModels.ExcusedRequestStatusRejected,
		activeModels.ExcusedRequestStatusWithdrawn,
	} {
		req.Status = s
		assert.True(t, req.IsTerminal(), "%s must be terminal", s)
	}
}

// insertStatusDay writes one active status day for the chain's child at the
// given reported_at, so the approval-conflict tests can position a status
// relative to a request's created_at.
func insertStatusDay(t *testing.T, db *bun.DB, chain testpkg.ParentChain, date timezone.Date, status string, reportedAt time.Time) {
	t.Helper()
	repos := repositories.NewFactory(db)
	err := tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		return repos.StudentStatusDay.UpsertReported(txCtx, &activeModels.StudentStatusDay{
			StudentID:  chain.StudentID,
			Date:       date,
			Status:     status,
			ReportedAt: reportedAt,
			Source:     activeModels.StudentStatusSourceManual,
		})
	})
	require.NoError(t, err)
}

// TestCreateRequest_IdempotentOverlapDisjoint pins the create-path guards
// (#1845 review): an identical resubmit returns the SAME pending row (idempotent
// retry, no duplicate), a partial overlap is refused, and a disjoint date set is
// allowed as a second request.
func TestCreateRequest_IdempotentOverlapDisjoint(t *testing.T) {
	svc, _, db := buildAbsenceService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	d1 := timezone.TodayDate().AddDays(2)
	d2 := timezone.TodayDate().AddDays(3)
	d3 := timezone.TodayDate().AddDays(5)

	first := createPending(t, svc, db, chain, []timezone.Date{d1, d2}, "Arzttermin")

	// Identical resubmit (dates in a different order) → same row, no duplicate.
	var second *activeModels.ExcusedAbsenceRequest
	err := tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		var e error
		second, e = svc.CreateRequest(txCtx, chain.StudentID, chain.AccountID, []timezone.Date{d2, d1}, "nochmal")
		return e
	})
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "an identical resubmit must be idempotent, not a new row")

	// Partial overlap (d2 already covered) → refused.
	err = tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.CreateRequest(txCtx, chain.StudentID, chain.AccountID, []timezone.Date{d2, d3}, "ueberschneidung")
		return e
	})
	assert.ErrorIs(t, err, absenceSvc.ErrExcusedRequestOverlap)

	// Disjoint date set → allowed as a second request.
	var third *activeModels.ExcusedAbsenceRequest
	err = tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		var e error
		third, e = svc.CreateRequest(txCtx, chain.StudentID, chain.AccountID, []timezone.Date{d3}, "anderer Tag")
		return e
	})
	require.NoError(t, err)
	assert.NotEqual(t, first.ID, third.ID, "a disjoint request must create a new row")

	// Exactly two pending rows exist (first + third; the overlap never inserted).
	err = tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		items, e := svc.ListPending(txCtx)
		require.NoError(t, e)
		assert.Len(t, items, 2, "only the idempotent-deduped and disjoint requests should remain pending")
		return nil
	})
	require.NoError(t, err)
}

// TestDecide_ApproveRefusedWhenNewerStatusExists pins the overwrite guard
// (#1845 review): approving must not clobber an absence record created AFTER the
// request was filed. A newer status blocks approval with a conflict; rejecting
// stays available so staff can wind the stale request down.
func TestDecide_ApproveRefusedWhenNewerStatusExists(t *testing.T) {
	svc, _, db := buildAbsenceService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	day := timezone.TodayDate().AddDays(3)
	pending := createPending(t, svc, db, chain, []timezone.Date{day}, "Arzttermin")

	// A newer sick status for the same day, reported AFTER the request was filed.
	insertStatusDay(t, db, chain, day, activeModels.StudentStatusDaySick, time.Now().Add(time.Hour))

	err := tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{RequestID: pending.ID, Approve: true, ReviewedBy: chain.AccountID})
		return e
	})
	assert.ErrorIs(t, err, absenceSvc.ErrExcusedRequestStatusConflict)

	// The request stays pending, and rejecting it still works.
	err = tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		item, e := svc.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{RequestID: pending.ID, Approve: false, Reason: "bitte klaeren", ReviewedBy: chain.AccountID})
		if e != nil {
			return e
		}
		assert.Equal(t, activeModels.ExcusedRequestStatusRejected, item.Request.Status)
		return nil
	})
	require.NoError(t, err)
}

func TestDecide_ApproveRefusedWhenPartialAbsenceExists(t *testing.T) {
	svc, _, db := buildAbsenceService(t)
	repos := repositories.NewFactory(db)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Partial", "Author")
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, chain)
		testpkg.CleanupStaffFixtures(t, db, staff.ID)
	})

	day := timezone.TodayDate().AddDays(3)
	pending := createPending(t, svc, db, chain, []timezone.Date{day}, "Arzttermin")
	from := timezone.WallClock(time.Date(2000, time.January, 1, 13, 30, 0, 0, time.UTC))
	staffID := staff.ID
	pickup := &scheduleModels.StudentPickupException{
		StudentID:             chain.StudentID,
		ExceptionDate:         day,
		PickupTime:            &from,
		ExcusedFrom:           &from,
		ExcusedCreatedBy:      &staffID,
		ExcusedOwnsPickupTime: true,
		Source:                scheduleModels.ExceptionSourceStaff,
		CreatedBy:             staff.ID,
	}
	pickup.SetTenantID(chain.TenantID)
	require.NoError(t, repos.StudentPickupException.Create(testpkg.TenantContext(chain.TenantID), pickup))

	err := tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, decideErr := svc.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{
			RequestID:  pending.ID,
			Approve:    true,
			ReviewedBy: chain.AccountID,
		})
		return decideErr
	})
	require.ErrorIs(t, err, absenceSvc.ErrExcusedRequestStatusConflict)

	rows, findErr := repos.StudentStatusDay.FindActiveByStudentAndDateRange(
		testpkg.TenantContext(chain.TenantID), chain.StudentID, day, day,
	)
	require.NoError(t, findErr)
	assert.Empty(t, rows)
}

// TestDecide_ApproveOverwritesOlderStatus is the complement: an OLDER status (set
// BEFORE the request) is exactly what the parent's request supersedes, so
// approval proceeds and writes the excused day.
func TestDecide_ApproveOverwritesOlderStatus(t *testing.T) {
	svc, _, db := buildAbsenceService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	day := timezone.TodayDate().AddDays(3)
	// An older sick status, reported a day BEFORE the request is filed.
	insertStatusDay(t, db, chain, day, activeModels.StudentStatusDaySick, time.Now().Add(-24*time.Hour))
	pending := createPending(t, svc, db, chain, []timezone.Date{day}, "Arzttermin")

	err := tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		item, e := svc.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{RequestID: pending.ID, Approve: true, ReviewedBy: chain.AccountID})
		if e != nil {
			return e
		}
		assert.Equal(t, activeModels.ExcusedRequestStatusApproved, item.Request.Status)
		return nil
	})
	require.NoError(t, err, "approving over an OLDER status the request supersedes must succeed")
}
