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

// countingBroadcaster records how often BroadcastToTenant fired so the
// approve-path broadcast can be asserted.
type countingBroadcaster struct{ tenantBroadcasts int }

func (b *countingBroadcaster) BroadcastToGroup(int64, string, realtime.Event) error { return nil }
func (b *countingBroadcaster) BroadcastToTenant(int64, realtime.Event) error {
	b.tenantBroadcasts++
	return nil
}
func (b *countingBroadcaster) BroadcastToAll(realtime.Event) error                       { return nil }
func (b *countingBroadcaster) BroadcastParentMessage(int64, int64, realtime.Event) error { return nil }

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
	svc := absenceSvc.NewExcusedAbsenceRequestService(
		repos.ExcusedAbsenceRequest,
		repos.StudentStatusDay,
		repos.Student,
		repos.Person,
		nil, // userContext: admin:* perms in the ctx short-circuit the write gate
		emitter,
		bc,
		nil, // logger: nil-safe, falls back to slog.Default()
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

// TestExcusedAbsenceRequestModel covers the pure model helpers and the
// bun-hook table-expr branches.
func TestExcusedAbsenceRequestModel(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	req := &activeModels.ExcusedAbsenceRequest{}
	assert.Equal(t, "active.excused_absence_requests", req.TableName())

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

	// Each query kind sets the schema-qualified table expr; the non-matching
	// kind (Select) falls through untouched.
	require.NoError(t, req.BeforeAppendModel(db.NewInsert()))
	require.NoError(t, req.BeforeAppendModel(db.NewUpdate()))
	require.NoError(t, req.BeforeAppendModel(db.NewDelete()))
	require.NoError(t, req.BeforeAppendModel(db.NewSelect()))
}
