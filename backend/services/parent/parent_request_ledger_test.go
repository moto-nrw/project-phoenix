package parent_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	absenceSvc "github.com/moto-nrw/project-phoenix/services/absence"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// buildLedgerServices wires the same parent + excused pair as the other
// absence-approval fixtures, but with the parent-request event recorder
// attached on both sides, so the ledger written inside the domain
// transactions is observable.
func buildLedgerServices(t *testing.T) (parentService.Service, absenceSvc.ExcusedAbsenceRequestService, usersSvc.ParentRequestEventRecorder, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	events := usersSvc.NewParentRequestEventRecorder(repos.ParentRequestEvent)
	excused := absenceSvc.NewExcusedAbsenceRequestServiceWithPolicy(
		repos.ExcusedAbsenceRequest,
		repos.StudentStatusDay,
		repos.StudentPickupException,
		repos.Student,
		repos.Person,
		nil, // userContext: admin perms in the ctx short-circuit the write gate
		nil, // emitter: pill is best-effort and nil-safe
		nil, // broadcaster
		testpkg.AbsenceRequestReviewPolicy{},
		events,
		slog.Default(),
		db,
	)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StatusDayRepo:       repos.StudentStatusDay,
		StudentRepo:         repos.Student,
		PickupExceptionRepo: repos.StudentPickupException,
		Settings: excusedApprovalSettings{
			sickRequiresApproval:    true,
			excusedRequiresApproval: true,
		},
		ExcusedRequests:     excused,
		ExcusedRequestRepo:  repos.ExcusedAbsenceRequest,
		ParentRequestEvents: events,
		DB:                  db,
		Logger:              slog.Default(),
	})
	return svc, excused, events, db
}

// TestParentRequestLedgerRecordsSubmitAndDecision pins the ledger wiring: a
// guardian submission and the staff decision each append exactly one event,
// inside the transaction of the write they describe, and the decision event
// carries the verdict and the reason staff typed.
func TestParentRequestLedgerRecordsSubmitAndDecision(t *testing.T) {
	t.Parallel()

	svc, requests, events, db := buildLedgerServices(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	day := timezone.TodayDate().AddDays(3)

	res, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]timezone.Date{day}, "Familienfeier", activeModels.StudentStatusDayExcused, nil)
	require.NoError(t, err)
	require.NotNil(t, res.PendingRequest)
	requestID := res.PendingRequest.ID

	rows := listLedger(t, db, events, chain.TenantID, requestID)
	require.Len(t, rows, 1, "the submission appends exactly one event")
	assert.Equal(t, usersModels.ParentRequestEventSubmitted, rows[0].EventType)
	assert.Equal(t, usersModels.ParentRequestTypeExcusedAbsence, rows[0].RequestType)
	require.NotNil(t, rows[0].ActorAccountID)
	assert.Equal(t, chain.AccountID, *rows[0].ActorAccountID, "the guardian is the actor of a submission")
	assert.Equal(t, chain.StudentID, rows[0].StudentID)

	err = testpkg.WithTenantTx(t, adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, decideErr := requests.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{
			RequestID:  requestID,
			Approve:    true,
			Reason:     "Passt so",
			ReviewedBy: chain.AccountID,
		})
		return decideErr
	})
	require.NoError(t, err)

	rows = listLedger(t, db, events, chain.TenantID, requestID)
	require.Len(t, rows, 2, "the decision appends one further event, oldest first")
	decided := rows[1]
	assert.Equal(t, usersModels.ParentRequestEventDecided, decided.EventType)
	assert.Equal(t, true, decided.Payload["approve"])
	assert.Equal(t, "Passt so", decided.Payload["reason"])
	assert.NotEmpty(t, decided.Version, "the decision event carries the version it produced")
	assert.NotEqual(t, rows[0].Version, decided.Version, "deciding bumps the request version")
}

// TestParentRequestLedgerRollsBackWithItsWrite pins that the event lives in
// the ambient transaction: a decision that fails leaves neither the decision
// nor an event behind.
func TestParentRequestLedgerRollsBackWithItsWrite(t *testing.T) {
	t.Parallel()

	svc, requests, events, db := buildLedgerServices(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	day := timezone.TodayDate().AddDays(4)

	res, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]timezone.Date{day}, "Familienfeier", activeModels.StudentStatusDayExcused, nil)
	require.NoError(t, err)
	requestID := res.PendingRequest.ID

	// A rejection without a reason is refused, so the transaction rolls back.
	err = testpkg.WithTenantTx(t, adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, decideErr := requests.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{
			RequestID:  requestID,
			Approve:    false,
			ReviewedBy: chain.AccountID,
		})
		return decideErr
	})
	require.Error(t, err)

	rows := listLedger(t, db, events, chain.TenantID, requestID)
	require.Len(t, rows, 1, "a refused decision appends no event")
	assert.Equal(t, usersModels.ParentRequestEventSubmitted, rows[0].EventType)
}

func listLedger(
	t *testing.T,
	db *bun.DB,
	events usersSvc.ParentRequestEventRecorder,
	tenantID, requestID int64,
) []*usersModels.ParentRequestEvent {
	t.Helper()
	var rows []*usersModels.ParentRequestEvent
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		rows, err = events.ListForRequest(txCtx, usersModels.ParentRequestTypeExcusedAbsence, requestID)
		return err
	}))
	return rows
}
