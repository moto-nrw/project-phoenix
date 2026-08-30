package parent_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	absenceSvc "github.com/moto-nrw/project-phoenix/services/absence"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// excusedApprovalSettings is a settings stub with the sick-note feature on and
// the excused-approval gate (#1845) configurable, so the approval branch can be
// exercised both ways.
type excusedApprovalSettings struct {
	configService.SettingsService
	// requiresApproval keeps older error-path fixtures focused on the original
	// excused-only gate; new tests set the two explicit switches below.
	requiresApproval        bool
	sickRequiresApproval    bool
	excusedRequiresApproval bool
}

func (s excusedApprovalSettings) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	switch key {
	case configModels.KeyParentSickNoteEnabled:
		return true, nil
	case configModels.KeyParentSickRequiresApproval:
		return s.sickRequiresApproval, nil
	case configModels.KeyParentExcusedRequiresApproval:
		return s.requiresApproval || s.excusedRequiresApproval, nil
	default:
		return false, nil
	}
}

func (s excusedApprovalSettings) ResolveStringForTenant(_ context.Context, _ int64, key string) (string, error) {
	if key == configModels.KeyGuardianParentInviteMode {
		return configModels.ParentInviteModeDisabled, nil
	}
	return "", nil
}

// buildExcusedServices wires a parent service with the excused-request service
// attached and the approval gate set as requested. Returns both so tests can
// drive the parent submit path and the staff decide path.
func buildExcusedServices(t *testing.T, requiresApproval bool) (parentService.Service, absenceSvc.ExcusedAbsenceRequestService, *testpkg.RecordingBroadcaster, *bun.DB) {
	return buildAbsenceApprovalServices(t, false, requiresApproval)
}

func buildAbsenceApprovalServices(t *testing.T, sickRequiresApproval, excusedRequiresApproval bool) (parentService.Service, absenceSvc.ExcusedAbsenceRequestService, *testpkg.RecordingBroadcaster, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	bc := testpkg.NewRecordingBroadcaster()
	excused := absenceSvc.NewExcusedAbsenceRequestServiceWithPolicy(
		repos.ExcusedAbsenceRequest,
		repos.StudentStatusDay,
		repos.StudentPickupException,
		repos.Student,
		repos.Person,
		nil, // userContext: admin perms in the ctx short-circuit the write gate
		nil, // emitter: pill is best-effort and nil-safe
		bc,
		testpkg.AbsenceRequestReviewPolicy{},
		nil,
		slog.Default(),
		db,
	)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StatusDayRepo:       repos.StudentStatusDay,
		StudentRepo:         repos.Student,
		PickupExceptionRepo: repos.StudentPickupException,
		Settings: excusedApprovalSettings{
			sickRequiresApproval:    sickRequiresApproval,
			excusedRequiresApproval: excusedRequiresApproval,
		},
		Broadcaster:     bc,
		ExcusedRequests: excused,
		DB:              db,
		Logger:          slog.Default(),
	})
	return svc, excused, bc, db
}

// TestSubmitSick_ApprovalOn_CreatesPendingRequest verifies the new sick-note
// gate (#2449): no status day or live sick flag changes before staff approve.
func TestSubmitSick_ApprovalOn_CreatesPendingRequest(t *testing.T) {
	t.Parallel()

	svc, _, _, db := buildAbsenceApprovalServices(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	day := timezone.TodayDate()
	res, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]timezone.Date{day}, "Fieber", activeModels.StudentStatusDaySick, nil)
	require.NoError(t, err)
	require.NotNil(t, res.PendingRequest, "a sick report must become a pending request when approval is on")
	assert.Empty(t, res.StatusDays, "no status day is written while the sick request is pending")
	assert.Equal(t, activeModels.StudentStatusDaySick, res.PendingRequest.AbsenceStatus)

	var sick bool
	require.NoError(t, db.NewSelect().
		Table("users.students").
		Column("sick").
		Where("id = ?", chain.StudentID).
		Scan(testpkg.WithPackageTenantRuntime(context.Background()), &sick))
	assert.False(t, sick, "the child stays expected until the OGS approves the sick request")
}

func TestSubmitSick_ApprovalOff_WritesDirectly(t *testing.T) {
	t.Parallel()

	svc, _, _, db := buildAbsenceApprovalServices(t, false, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	day := timezone.TodayDate().AddDays(2)
	res, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]timezone.Date{day}, "Fieber", activeModels.StudentStatusDaySick, nil)
	require.NoError(t, err)
	require.Nil(t, res.PendingRequest)
	require.Len(t, res.StatusDays, 1)
	assert.Equal(t, activeModels.StudentStatusDaySick, res.StatusDays[0].Status)
}

func TestSickRequest_ApproveWritesSickStatusAndLiveFlag(t *testing.T) {
	t.Parallel()

	svc, requests, _, db := buildAbsenceApprovalServices(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	today := timezone.TodayDate()

	res, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]timezone.Date{today}, "Fieber", activeModels.StudentStatusDaySick, nil)
	require.NoError(t, err)

	err = testpkg.WithTenantTx(t, adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, decideErr := requests.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{
			RequestID:  res.PendingRequest.ID,
			Approve:    true,
			ReviewedBy: chain.AccountID,
		})
		return decideErr
	})
	require.NoError(t, err)

	absences, err := svc.ListSickDays(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, today, today)
	require.NoError(t, err)
	require.Len(t, absences, 1)
	assert.Equal(t, activeModels.StudentStatusDaySick, absences[0].Status)

	var sick bool
	require.NoError(t, db.NewSelect().
		Table("users.students").
		Column("sick").
		Where("id = ?", chain.StudentID).
		Scan(testpkg.WithPackageTenantRuntime(context.Background()), &sick))
	assert.True(t, sick, "approving today's sick request must mark the child sick")
}

// adminCtx returns a context carrying wildcard-admin permissions so the staff
// decide gate authorizes without a wired userContext.
func adminCtx() context.Context {
	return context.WithValue(testpkg.WithPackageTenantRuntime(context.Background()), jwt.CtxPermissions, []string{"admin:*"})
}

// TestSubmitExcused_ApprovalOn_CreatesPendingRequest verifies that with the
// approval gate on an excused report becomes a pending request and does NOT
// write a status day — the child stays "expected" until staff decide.
func TestSubmitExcused_ApprovalOn_CreatesPendingRequest(t *testing.T) {
	t.Parallel()

	svc, _, _, db := buildExcusedServices(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	day := timezone.TodayDate().AddDays(3)
	res, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]timezone.Date{day}, "Familienfeier", activeModels.StudentStatusDayExcused, nil)
	require.NoError(t, err)
	require.NotNil(t, res.PendingRequest, "an excused report must become a pending request when approval is on")
	assert.Empty(t, res.StatusDays, "no status day is written while the request is pending")
	assert.Equal(t, activeModels.ExcusedRequestStatusPending, res.PendingRequest.Status)
	assert.Equal(t, activeModels.StudentStatusDayExcused, res.PendingRequest.AbsenceStatus)
	assert.Equal(t, "Familienfeier", res.PendingRequest.Note)

	// The absence list (status days) stays empty — the child is still expected.
	from := timezone.TodayDate()
	to := timezone.NewDate(from.Year, from.Month+1, from.Day)
	absences, err := svc.ListSickDays(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, from, to)
	require.NoError(t, err)
	assert.Empty(t, absences, "a pending excused request must not appear as a confirmed absence")

	// It DOES appear in the parent's excused-requests view.
	reqs, err := svc.ListExcusedRequests(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, reqs, 1)
	assert.Equal(t, activeModels.ExcusedRequestStatusPending, reqs[0].Status)
}

// TestSubmitExcused_ApprovalOff_WritesDirectly verifies the gate-off path is
// unchanged: an excused report is written straight to the status days.
func TestSubmitExcused_ApprovalOff_WritesDirectly(t *testing.T) {
	t.Parallel()

	svc, _, _, db := buildExcusedServices(t, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	day := timezone.TodayDate().AddDays(3)
	res, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]timezone.Date{day}, "Familienfeier", activeModels.StudentStatusDayExcused, nil)
	require.NoError(t, err)
	require.Nil(t, res.PendingRequest, "no request is created when the gate is off")
	require.Len(t, res.StatusDays, 1)
	assert.Equal(t, activeModels.StudentStatusDayExcused, res.StatusDays[0].Status)
}

// TestSubmitExcused_EmptyNoteRejected verifies AC2: a note is mandatory for an
// excused report, independent of the approval gate.
func TestSubmitExcused_EmptyNoteRejected(t *testing.T) {
	t.Parallel()

	for _, gate := range []bool{true, false} {
		svc, _, _, db := buildExcusedServices(t, gate)
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		day := timezone.TodayDate().AddDays(3)
		_, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
			[]timezone.Date{day}, "   ", activeModels.StudentStatusDayExcused, nil)
		assert.ErrorIs(t, err, parentService.ErrEmptyNote, "excused with blank note must be rejected (gate=%v)", gate)

	}
}

// TestExcusedRequest_ApproveWritesStatusDays drives the staff decide path: an
// approval writes the excused status days that the parent list then shows.
func TestExcusedRequest_ApproveWritesStatusDays(t *testing.T) {
	t.Parallel()

	svc, excused, _, db := buildExcusedServices(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	day := timezone.TodayDate().AddDays(3)
	res, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]timezone.Date{day}, "Familienfeier", activeModels.StudentStatusDayExcused, nil)
	require.NoError(t, err)
	require.NotNil(t, res.PendingRequest)
	requestID := res.PendingRequest.ID

	// Staff approve inside a tenant transaction (as the middleware would).
	err = testpkg.WithTenantTx(t, adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		item, derr := excused.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{
			RequestID: requestID,
			Approve:   true,
		})
		if derr != nil {
			return derr
		}
		require.Equal(t, activeModels.ExcusedRequestStatusApproved, item.Request.Status)
		return nil
	})
	require.NoError(t, err)

	// The approved absence now shows as a confirmed excused status day.
	from := timezone.TodayDate()
	to := timezone.NewDate(from.Year, from.Month+1, from.Day)
	absences, err := svc.ListSickDays(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, from, to)
	require.NoError(t, err)
	require.Len(t, absences, 1, "approving the request writes the excused status day")
	assert.Equal(t, activeModels.StudentStatusDayExcused, absences[0].Status)
	assert.Equal(t, day, absences[0].Date)
	assert.Equal(t, activeModels.StudentStatusSourceParent, absences[0].Source)
}

// TestExcusedRequest_RejectWritesNoStatusDay verifies a rejection leaves the
// child expected: no status day is written.
func TestExcusedRequest_RejectWritesNoStatusDay(t *testing.T) {
	t.Parallel()

	svc, excused, _, db := buildExcusedServices(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	day := timezone.TodayDate().AddDays(3)
	res, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]timezone.Date{day}, "Familienfeier", activeModels.StudentStatusDayExcused, nil)
	require.NoError(t, err)
	requestID := res.PendingRequest.ID

	err = testpkg.WithTenantTx(t, adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, derr := excused.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{
			RequestID: requestID,
			Approve:   false,
			Reason:    "Bitte telefonisch klären",
		})
		return derr
	})
	require.NoError(t, err)

	from := timezone.TodayDate()
	to := timezone.NewDate(from.Year, from.Month+1, from.Day)
	absences, err := svc.ListSickDays(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, from, to)
	require.NoError(t, err)
	assert.Empty(t, absences, "a rejected request must not write a status day")
}

// TestListExcusedRequests_ShowsRecentlyRejectedLongPending verifies a request
// that sat pending far longer than the 14-day recent window and was only just
// rejected still surfaces in the parent's outcome view. The recent filter keys
// on the DECISION time, not created_at; filtering on created_at would drop the
// row the instant it left the pending set, hiding the rejection from the parent
// (#1845 review).
func TestListExcusedRequests_ShowsRecentlyRejectedLongPending(t *testing.T) {
	t.Parallel()

	svc, excused, _, db := buildExcusedServices(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	day := timezone.TodayDate().AddDays(3)
	res, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]timezone.Date{day}, "Familienfeier", activeModels.StudentStatusDayExcused, nil)
	require.NoError(t, err)
	requestID := res.PendingRequest.ID

	// Backdate creation (and updated_at) well past the recent window while the
	// request is still pending, as if it had languished unreviewed for a month.
	old := time.Now().AddDate(0, 0, -30)
	_, err = db.NewUpdate().
		Table("active.excused_absence_requests").
		Set("created_at = ?", old).
		Set("updated_at = ?", old).
		Where("id = ?", requestID).
		Exec(testpkg.WithPackageTenantRuntime(context.Background()))
	require.NoError(t, err)

	// Staff reject it today — reviewed_at is stamped now, inside the window.
	err = testpkg.WithTenantTx(t, adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, derr := excused.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{
			RequestID: requestID,
			Approve:   false,
			Reason:    "Bitte telefonisch klären",
		})
		return derr
	})
	require.NoError(t, err)

	reqs, err := svc.ListExcusedRequests(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, reqs, 1, "a long-pending request rejected today must stay visible so the parent learns the outcome")
	assert.Equal(t, activeModels.ExcusedRequestStatusRejected, reqs[0].Status)
	assert.Equal(t, requestID, reqs[0].ID)
}

// TestListExcusedRequests_ShowsApprovedForOutOfWindowDates verifies an approved
// request stays visible in the parent's outcome list. Its confirmed excused
// status days live outside the parent's status-day fetch window (today..+2
// months) when the date is far in the future (or in the past for a delayed
// decision), so dropping approved requests from this list would make the
// confirmation vanish entirely — the pending row would just disappear (#1845
// review).
func TestListExcusedRequests_ShowsApprovedForOutOfWindowDates(t *testing.T) {
	t.Parallel()

	svc, excused, _, db := buildExcusedServices(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	// A date well beyond the parent's ~2-month status-day window.
	far := timezone.TodayDate().AddDays(120)
	res, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]timezone.Date{far}, "Urlaub", activeModels.StudentStatusDayExcused, nil)
	require.NoError(t, err)
	requestID := res.PendingRequest.ID

	err = testpkg.WithTenantTx(t, adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, derr := excused.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{
			RequestID: requestID,
			Approve:   true,
		})
		return derr
	})
	require.NoError(t, err)

	// The status-day view (today..+1 month here) does NOT include the far date.
	from := timezone.TodayDate()
	to := timezone.NewDate(from.Year, from.Month+1, from.Day)
	absences, err := svc.ListSickDays(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, from, to)
	require.NoError(t, err)
	assert.Empty(t, absences, "the confirmed day is outside the parent's status-day window")

	// But the approved request is still listed, so the parent sees the outcome.
	reqs, err := svc.ListExcusedRequests(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, reqs, 1, "an approved out-of-window request must stay visible so the parent sees the confirmation")
	assert.Equal(t, activeModels.ExcusedRequestStatusApproved, reqs[0].Status)
	assert.Equal(t, requestID, reqs[0].ID)
}
