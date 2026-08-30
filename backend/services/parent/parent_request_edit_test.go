package parent_test

import (
	"context"
	"encoding/json"
	"errors"
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

// buildEditableMasterDataService is buildRequestService plus the event
// recorder, so the guardian_edited entry the edit writes is observable.
func buildEditableMasterDataService(t *testing.T) (parentService.Service, usersSvc.ParentRequestEventRecorder, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	events := usersSvc.NewParentRequestEventRecorder(repos.ParentRequestEvent)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StudentRepo:         repos.Student,
		GuardianProfileRepo: repos.GuardianProfile,
		PersonRepo:          repos.Person,
		GuardianPhoneRepo:   repos.GuardianPhoneNumber,
		ChangeRequestRepo:   repos.StudentDataChangeRequest,
		ParentRequestEvents: events,
		Settings:            masterDataSettings(false, true, false),
		Broadcaster:         testpkg.NewRecordingBroadcaster(),
		DB:                  db,
		Logger:              slog.Default(),
	})
	return svc, events, db
}

// TestEditExcusedRequestReplacesWithdrawal is the core of story 37: the
// guardian corrects their own open request, it keeps its id, the dates change
// and the ledger records the edit.
func TestEditExcusedRequestReplacesWithdrawal(t *testing.T) {
	t.Parallel()

	svc, _, events, db := buildLedgerServices(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	day := timezone.TodayDate().AddDays(5)

	res, err := svc.SubmitSickNote(ctx, chain.AccountID, chain.StudentID,
		[]timezone.Date{day}, "Familienfeier", activeModels.StudentStatusDayExcused, nil)
	require.NoError(t, err)
	original := res.PendingRequest
	version := usersSvc.ParentRequestVersion(original.UpdatedAt)

	corrected := day.AddDays(1)
	edited, err := svc.EditExcusedRequest(ctx, chain.AccountID, chain.StudentID, original.ID,
		[]timezone.Date{corrected}, "Doch einen Tag später", version)
	require.NoError(t, err)
	assert.Equal(t, original.ID, edited.ID, "an edit keeps the request id, so the share survives")
	require.Len(t, edited.Dates, 1)
	assert.Equal(t, corrected, edited.Dates[0])
	assert.Equal(t, "Doch einen Tag später", edited.Note)
	assert.Equal(t, activeModels.ExcusedRequestStatusPending, edited.Status)
	assert.NotEqual(t, version, usersSvc.ParentRequestVersion(edited.UpdatedAt), "an edit bumps the version")

	rows := listLedger(t, db, events, chain.TenantID, original.ID)
	require.Len(t, rows, 2)
	assert.Equal(t, usersModels.ParentRequestEventGuardianEdit, rows[1].EventType)
	require.NotNil(t, rows[1].ActorAccountID)
	assert.Equal(t, chain.AccountID, *rows[1].ActorAccountID)
}

// TestEditExcusedRequestRefusesStaleVersion pins that an edit taken on an
// outdated view never lands: the second edit carries the version the first one
// replaced.
func TestEditExcusedRequestRefusesStaleVersion(t *testing.T) {
	t.Parallel()

	svc, _, _, db := buildLedgerServices(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	day := timezone.TodayDate().AddDays(6)

	res, err := svc.SubmitSickNote(ctx, chain.AccountID, chain.StudentID,
		[]timezone.Date{day}, "Familienfeier", activeModels.StudentStatusDayExcused, nil)
	require.NoError(t, err)
	stale := usersSvc.ParentRequestVersion(res.PendingRequest.UpdatedAt)

	_, err = svc.EditExcusedRequest(ctx, chain.AccountID, chain.StudentID, res.PendingRequest.ID,
		[]timezone.Date{day.AddDays(1)}, "Erste Korrektur", stale)
	require.NoError(t, err)

	_, err = svc.EditExcusedRequest(ctx, chain.AccountID, chain.StudentID, res.PendingRequest.ID,
		[]timezone.Date{day.AddDays(2)}, "Zweite Korrektur", stale)
	require.ErrorIs(t, err, usersSvc.ErrParentRequestStale)
}

// TestEditExcusedRequestRevalidatesPayload pins that an edit runs the same
// validators as the create path — an empty note is refused there and here.
func TestEditExcusedRequestRevalidatesPayload(t *testing.T) {
	t.Parallel()

	svc, _, _, db := buildLedgerServices(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	day := timezone.TodayDate().AddDays(7)

	res, err := svc.SubmitSickNote(ctx, chain.AccountID, chain.StudentID,
		[]timezone.Date{day}, "Familienfeier", activeModels.StudentStatusDayExcused, nil)
	require.NoError(t, err)

	_, err = svc.EditExcusedRequest(ctx, chain.AccountID, chain.StudentID, res.PendingRequest.ID,
		[]timezone.Date{day}, "   ", "")
	require.Error(t, err, "an edit may not produce a request the create path would refuse")
}

// TestEditDecidedExcusedRequestIsRefused pins that the edit window closes with
// the decision: once staff decided, the guardian files a new request instead
// of rewriting the one that was judged.
func TestEditDecidedExcusedRequestIsRefused(t *testing.T) {
	t.Parallel()

	svc, requests, _, db := buildLedgerServices(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	day := timezone.TodayDate().AddDays(10)

	res, err := svc.SubmitSickNote(ctx, chain.AccountID, chain.StudentID,
		[]timezone.Date{day}, "Familienfeier", activeModels.StudentStatusDayExcused, nil)
	require.NoError(t, err)

	require.NoError(t, testpkg.WithTenantTx(t, adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, decideErr := requests.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{
			RequestID: res.PendingRequest.ID, Approve: true, ReviewedBy: chain.AccountID,
		})
		return decideErr
	}))

	_, err = svc.EditExcusedRequest(ctx, chain.AccountID, chain.StudentID, res.PendingRequest.ID,
		[]timezone.Date{day.AddDays(1)}, "Zu spät", "")
	require.ErrorIs(t, err, parentService.ErrExcusedRequestNotPending)
}

// TestEditExcusedRequestOfAnotherFamilyIsNotFound pins the probe guard: a
// foreign request is missing, never forbidden.
func TestEditExcusedRequestOfAnotherFamilyIsNotFound(t *testing.T) {
	t.Parallel()

	svc, _, _, db := buildLedgerServices(t)
	mine := testpkg.CreateTestParentGuardianChain(t, db)
	theirs := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	day := timezone.TodayDate().AddDays(8)

	res, err := svc.SubmitSickNote(ctx, theirs.AccountID, theirs.StudentID,
		[]timezone.Date{day}, "Familienfeier", activeModels.StudentStatusDayExcused, nil)
	require.NoError(t, err)

	_, err = svc.EditExcusedRequest(ctx, mine.AccountID, mine.StudentID, res.PendingRequest.ID,
		[]timezone.Date{day}, "Fremd", "")
	require.ErrorIs(t, err, parentService.ErrExcusedRequestNotFound)
}

// TestEditMasterDataRequestRewritesProposedValue covers the Stammdaten domain:
// the proposed value changes, the request keeps its id and target, and the
// ledger records the edit.
func TestEditMasterDataRequestRewritesProposedValue(t *testing.T) {
	t.Parallel()

	svc, events, db := buildEditableMasterDataService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	rows, err := svc.SubmitMasterDataChangeRequest(ctx, chain.AccountID, chain.StudentID,
		[]parentService.MasterDataFieldChange{
			{Target: usersModels.DataChangeTargetPerson, FieldKey: "first_name", Value: json.RawMessage(`"Maximilian"`)},
		}, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	version := usersSvc.ParentRequestVersion(rows[0].UpdatedAt)

	edited, err := svc.EditMasterDataRequest(ctx, chain.AccountID, chain.StudentID, rows[0].ID,
		json.RawMessage(`"Maxi"`), version)
	require.NoError(t, err)
	assert.Equal(t, rows[0].ID, edited.ID)
	assert.JSONEq(t, `"Maxi"`, string(edited.NewValue))
	assert.Equal(t, usersModels.DataChangeStatusPending, edited.Status)
	assert.Equal(t, usersModels.DataChangeTargetPerson, edited.Target)

	var ledger []*usersModels.ParentRequestEvent
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		var listErr error
		ledger, listErr = events.ListForRequest(txCtx, usersModels.ParentRequestTypeMasterData, rows[0].ID)
		return listErr
	}))
	require.Len(t, ledger, 2)
	assert.Equal(t, usersModels.ParentRequestEventSubmitted, ledger[0].EventType)
	assert.Equal(t, usersModels.ParentRequestEventGuardianEdit, ledger[1].EventType)
}

// TestEditMasterDataRequestRefusesUnchangedValue pins that an edit whose value
// meanwhile equals the live record is refused — it would be a request asking
// for nothing.
func TestEditMasterDataRequestRefusesUnchangedValue(t *testing.T) {
	t.Parallel()

	svc, _, db := buildEditableMasterDataService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	rows, err := svc.SubmitMasterDataChangeRequest(ctx, chain.AccountID, chain.StudentID,
		[]parentService.MasterDataFieldChange{
			{Target: usersModels.DataChangeTargetPerson, FieldKey: "first_name", Value: json.RawMessage(`"Maximilian"`)},
		}, nil)
	require.NoError(t, err)

	person, err := repositories.NewFactory(db).Person.FindByID(ctx, chain.PersonID)
	require.NoError(t, err)
	live, err := json.Marshal(person.FirstName)
	require.NoError(t, err)

	_, err = svc.EditMasterDataRequest(ctx, chain.AccountID, chain.StudentID, rows[0].ID, live, "")
	require.ErrorIs(t, err, parentService.ErrMasterDataNoChanges)
}

// TestListRequestEventsIsSubmitterOnly pins the read gate: the author sees the
// history, a request that is not theirs is missing.
func TestListRequestEventsIsSubmitterOnly(t *testing.T) {
	t.Parallel()

	svc, _, _, db := buildLedgerServices(t)
	mine := testpkg.CreateTestParentGuardianChain(t, db)
	theirs := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	day := timezone.TodayDate().AddDays(9)

	res, err := svc.SubmitSickNote(ctx, mine.AccountID, mine.StudentID,
		[]timezone.Date{day}, "Familienfeier", activeModels.StudentStatusDayExcused, nil)
	require.NoError(t, err)

	history, err := svc.ListRequestEvents(ctx, mine.AccountID, mine.StudentID,
		usersModels.ParentRequestTypeExcusedAbsence, res.PendingRequest.ID)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, usersModels.ParentRequestEventSubmitted, history[0].EventType)
	assert.False(t, history[0].CreatedAt.IsZero())

	_, err = svc.ListRequestEvents(ctx, theirs.AccountID, theirs.StudentID,
		usersModels.ParentRequestTypeExcusedAbsence, res.PendingRequest.ID)
	require.Error(t, err)
	assert.True(t,
		errors.Is(err, parentService.ErrRequestSharingNotFound) || errors.Is(err, parentService.ErrChildNotLinked),
		"a foreign request must not be readable: %v", err)
}
