package parentportal_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// buildLedgerServices wires the same parent + excused pair as the other
// absence-approval fixtures, but with the parent-request event recorder
// attached on both sides, so the ledger written inside the domain
// transactions is observable.
func buildLedgerServices(t *testing.T) (*parentService.Portal, careplan.ExcusedAbsenceRequests, api.ParentRequestEventFixtureReads, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	events := reposPeople.ParentRequestEvent
	excused, err := api.NewTestExcusedAbsenceRequests(api.ExcusedRequestTestOptions{
		CarePlan: repos.CarePlan(), Logger: slog.Default(),
	}, api.ExcusedRequestTestPeople{Students: reposPeople.Student, Persons: reposPeople.Person, Events: events}, nil, nil)
	require.NoError(t, err)
	svc := parentportalcompose.New(parentportalcompose.Dependencies{
		GuardianChanges: owners.ParentGuardianChanges(),
		DeparturePolicy: owners.ParentDeparturePolicy(),
		GuardianPolicy:  owners.NewParentGuardianPolicy(),
		Fingerprints:    owners.SecurityFingerprints{},
		Declarations:    owners.ParentDeclarationPolicy(),
		Locales:         owners.ParentPortalLocales(),
		Runtime:         owners.ParentPortalRuntime{},
		CarePlan:        owners.NewParentCarePlanCommands(repos.CarePlan(), nil),
		People:          owners.MustNewPeopleDirectory(testpkg.MustDatabase(db)),
		CareProfiles:    careProfiles(t, db),
		ChildRepo:       repos.ParentChildReads(),
		StatusDayRepo:   repos.StudentStatusDay,
		StudentRepo:     owners.NewParentStudentReads(reposPeople.Student),
		CareExceptions:  owners.NewParentCareExceptions(repos.CarePlan()),
		Settings: excusedApprovalSettings{
			sickRequiresApproval:    true,
			excusedRequiresApproval: true,
		},
		ExcusedRequests:     owners.NewParentExcusedAbsenceRequests(excused),
		ExcusedRequestRepo:  repos.ExcusedAbsenceRequest,
		ParentRequestEvents: owners.NewParentRequestEvents(events),
		Logger:              slog.Default(),

		DataRequests: owners.NewParentDataRequestCommands(repos.CarePlan()),

		CarePlanPolicy: owners.NewParentCarePlanPolicy(),
	})
	return svc, excused, api.NewParentRequestEventFixtureReads(events), db
}

// TestParentRequestLedgerRecordsSubmitAndDecision pins the ledger wiring: a
// guardian submission and the staff decision each append exactly one event,
// inside the transaction of the write they describe, and the decision event
// carries the verdict and the reason staff typed.
func TestParentRequestLedgerRecordsSubmitAndDecision(t *testing.T) {
	t.Parallel()

	svc, requests, events, db := buildLedgerServices(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	day := calendar.TodayDate().AddDays(3)

	res, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]calendar.Date{day}, "Familienfeier", absencerecords.StudentStatusDayExcused, nil)
	require.NoError(t, err)
	require.NotNil(t, res.PendingRequest)
	requestID := res.PendingRequest.ID

	rows := listLedger(t, db, events, chain.TenantID, requestID)
	require.Len(t, rows, 1, "the submission appends exactly one event")
	assert.Equal(t, "submitted", rows[0].EventType)
	assert.Equal(t, "excused", rows[0].RequestType)
	require.NotNil(t, rows[0].ActorAccountID)
	assert.Equal(t, chain.AccountID, *rows[0].ActorAccountID, "the guardian is the actor of a submission")
	assert.Equal(t, chain.StudentID, rows[0].StudentID)

	err = testpkg.WithTenantTx(t, adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, decideErr := requests.Decide(txCtx, careplan.ExcusedRequestDecideInput{
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
	assert.Equal(t, "decided", decided.EventType)
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
	day := calendar.TodayDate().AddDays(4)

	res, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]calendar.Date{day}, "Familienfeier", absencerecords.StudentStatusDayExcused, nil)
	require.NoError(t, err)
	requestID := res.PendingRequest.ID

	// A rejection without a reason is refused, so the transaction rolls back.
	err = testpkg.WithTenantTx(t, adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, decideErr := requests.Decide(txCtx, careplan.ExcusedRequestDecideInput{
			RequestID:  requestID,
			Approve:    false,
			ReviewedBy: chain.AccountID,
		})
		return decideErr
	})
	require.Error(t, err)

	rows := listLedger(t, db, events, chain.TenantID, requestID)
	require.Len(t, rows, 1, "a refused decision appends no event")
	assert.Equal(t, "submitted", rows[0].EventType)
}

func listLedger(
	t *testing.T,
	db *bun.DB,
	events api.ParentRequestEventFixtureReads,
	tenantID, requestID int64,
) []*api.ParentRequestEventFixture {
	t.Helper()
	var rows []*api.ParentRequestEventFixture
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		rows, err = events.ListForRequest(txCtx, "excused", requestID)
		return err
	}))
	return rows
}
