package parentportal_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

func TestRequestShareVisibilityRequiresTheLedgerAndTheFamilyProtection(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	shares := owners.BindParentRequestShareRecords(repos.ParentRequestShare)
	protection := owners.BindParentFamilyProtectionReads(repos.FamilyProtection)
	const message = "parent: request share visibility needs the sharing ledger and the family protection"

	_, err := parentportalcompose.NewRequestShareVisibility(nil, protection)
	require.EqualError(t, err, message)
	_, err = parentportalcompose.NewRequestShareVisibility(shares, nil)
	require.EqualError(t, err, message)
}

// The standalone visibility the staff request queues read answers exactly
// what the parents portal answers over the same ledger, including the
// Familienschutz cut-off.
func TestRequestShareVisibilityAnswersLikeThePortal(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposCare := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	author := testpkg.CreateTestParentGuardianChain(t, db)
	recipient := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), author.TenantID)

	shares := owners.BindParentRequestShareRecords(repos.ParentRequestShare)
	protection := owners.BindParentFamilyProtectionReads(repos.FamilyProtection)
	portal := parentportalcompose.New(parentportalcompose.Dependencies{
		GuardianChanges: owners.ParentGuardianChanges(), DeparturePolicy: owners.ParentDeparturePolicy(),
		GuardianPolicy: owners.NewParentGuardianPolicy(), Fingerprints: owners.SecurityFingerprints{},
		Declarations: owners.ParentDeclarationPolicy(), Locales: owners.ParentPortalLocales(),
		Runtime: owners.ParentPortalRuntime{}, CarePlanPolicy: owners.NewParentCarePlanPolicy(),
		FamilyProtectionEvents: protection, ParentRequestShares: shares, Logger: slog.Default(),
	})
	visibility, err := parentportalcompose.NewRequestShareVisibility(shares, protection)
	require.NoError(t, err)

	request := &parentService.StudentDataChangeRequest{
		StudentID: author.StudentID, SubmittedBy: author.AccountID,
		Target: parentService.DataChangeTargetPerson, FieldKey: "first_name",
		OldValue: json.RawMessage(`"Felix"`), NewValue: json.RawMessage(`"Feli"`),
		Status: parentService.DataChangeStatusPending,
	}
	request.TenantID = author.TenantID
	require.NoError(t, api.CreateParentDataRequestFixture(reposCare.CarePlan(), ctx, request))
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repos.ParentRequestShare.AppendParentRequestShare, &peopledirectory.ParentRequestShareRecord{
		TenantID:  author.TenantID,
		StudentID: author.StudentID, RequestType: parentService.RequestShareMasterData, RequestID: request.ID,
		AuthorAccountID: author.AccountID, RecipientAccountIDs: []int64{recipient.AccountID},
	}))

	answers := func() []int64 {
		t.Helper()
		want, err := testpkg.TenantQuery3(t, ctx, db, portal.SharedRecipientAccountIDs, author.StudentID, parentService.RequestShareMasterData, request.ID)
		require.NoError(t, err)
		got, err := testpkg.TenantQuery3(t, ctx, db, visibility.SharedRecipientAccountIDs, author.StudentID, parentService.RequestShareMasterData, request.ID)
		require.NoError(t, err)
		assert.Equal(t, want, got)
		return got
	}
	assert.Equal(t, []int64{recipient.AccountID}, answers())

	protectionOn := &peopledirectory.FamilyProtectionFixture{
		StudentID: author.StudentID, Enabled: true, Reason: "Schutzfall", ActorAccountID: author.AccountID,
	}
	protectionOn.TenantID = author.TenantID
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repos.CreateFamilyProtectionFixture, protectionOn))
	assert.Empty(t, answers())

	protectionOff := &peopledirectory.FamilyProtectionFixture{
		StudentID: author.StudentID, Enabled: false, Reason: "Schutz aufgehoben", ActorAccountID: author.AccountID,
	}
	protectionOff.TenantID = author.TenantID
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repos.CreateFamilyProtectionFixture, protectionOff))
	assert.Empty(t, answers(), "disabling protection must not revive an old share")
}
