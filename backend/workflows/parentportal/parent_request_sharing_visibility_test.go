package parentportal_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// Kept with the retained parent services (#3227): the scenario reads the shared request
// through the master-data flows, which stay here.
func TestRequestSharingIsNamedAndFamilyProtectionDoesNotReviveOldShares(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposCare := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	author := testpkg.CreateTestParentGuardianChain(t, db)
	recipient := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), author.TenantID)

	_, err := db.NewUpdate().
		TableExpr(`users.student_guardian_relationships`).
		Set("student_id = ?", author.StudentID).
		// The author stays the child's one primary guardian (#2756).
		Set("is_primary = false").
		Where("guardian_profile_id = ?", recipient.GuardianProfileID).
		Exec(ctx)
	require.NoError(t, err)

	request := &parentService.StudentDataChangeRequest{
		StudentID: author.StudentID, SubmittedBy: author.AccountID,
		Target: parentService.DataChangeTargetPerson, FieldKey: "first_name",
		OldValue: json.RawMessage(`"Felix"`), NewValue: json.RawMessage(`"Feli"`),
		Status: parentService.DataChangeStatusPending,
	}
	request.TenantID = author.TenantID
	require.NoError(t, api.CreateParentDataRequestFixture(reposCare.CarePlan(), ctx, request))

	svc := parentportalcompose.New(parentportalcompose.Dependencies{
		GuardianChanges: owners.ParentGuardianChanges(),
		DeparturePolicy: owners.ParentDeparturePolicy(),
		GuardianPolicy:  owners.NewParentGuardianPolicy(),
		Fingerprints:    owners.SecurityFingerprints{},
		Declarations:    owners.ParentDeclarationPolicy(),
		Locales:         owners.ParentPortalLocales(),
		Runtime:         api.ParentPortalRuntime{},
		CarePlan:        owners.NewParentCarePlanCommands(reposCare.CarePlan(), nil),
		People:          owners.MustNewPeopleDirectory(testpkg.MustDatabase(db)),
		CareProfiles:    careProfiles(t, db),
		ChildRepo:       reposCare.ParentChildReads(), GuardianProfileRepo: owners.NewParentGuardianProfileReads(repos.GuardianProfile),
		GuardianPhoneRepo: owners.NewParentGuardianPhoneReads(repos.GuardianPhoneNumber), PersonRepo: owners.NewParentPersonReads(repos.Person),
		StudentGuardianRepo: owners.NewParentStudentGuardianReads(repos.StudentGuardian), StudentRepo: owners.NewParentStudentReads(repos.Student),
		ChangeRequestRepo:      owners.NewParentDataRequestReads(reposCare.CarePlan()),
		FamilyProtectionEvents: owners.BindParentFamilyProtectionReads(repos.FamilyProtection), ParentRequestShares: owners.BindParentRequestShareRecords(repos.ParentRequestShare),
		Logger: slog.Default(),

		DataRequests: owners.NewParentDataRequestCommands(reposCare.CarePlan()),

		CarePlanPolicy: owners.NewParentCarePlanPolicy(),
	})
	var sharing parentService.RequestSharingService = svc
	options, err := sharing.GetRequestSharingOptions(ctx, author.AccountID, author.StudentID)
	require.NoError(t, err)
	require.Len(t, options.Recipients, 1, "named recipients must be available before submission")
	assert.Equal(t, recipient.GuardianProfileID, options.Recipients[0].GuardianProfileID)
	assert.False(t, options.Recipients[0].Selected)

	recipientRows, err := svc.ListMyMasterDataRequests(ctx, recipient.AccountID, author.StudentID)
	require.NoError(t, err)
	assert.Empty(t, recipientRows)
	recipientMasterData, err := svc.GetChildMasterData(ctx, recipient.AccountID, author.StudentID)
	require.NoError(t, err)
	assert.Empty(t, recipientMasterData.PendingChanges, "the main child view must not bypass request privacy")

	state, err := sharing.SetRequestSharing(ctx, author.AccountID, author.StudentID, parentService.RequestShareMasterData, request.ID, []int64{recipient.GuardianProfileID})
	require.NoError(t, err)
	require.Len(t, state.Recipients, 1)
	assert.True(t, state.Recipients[0].Selected)

	recipientRows, err = svc.ListMyMasterDataRequests(ctx, recipient.AccountID, author.StudentID)
	require.NoError(t, err)
	require.Len(t, recipientRows, 1)
	assert.Equal(t, request.ID, recipientRows[0].ID)
	recipientMasterData, err = svc.GetChildMasterData(ctx, recipient.AccountID, author.StudentID)
	require.NoError(t, err)
	require.Len(t, recipientMasterData.PendingChanges, 1)
	assert.Equal(t, request.ID, recipientMasterData.PendingChanges[0].ID)

	protectionOn := &peopledirectory.FamilyProtectionFixture{
		StudentID: author.StudentID, Enabled: true, Reason: "Schutzfall", ActorAccountID: author.AccountID,
	}
	protectionOn.TenantID = author.TenantID
	require.NoError(t, repos.CreateFamilyProtectionFixture(ctx, protectionOn))
	recipientRows, err = svc.ListMyMasterDataRequests(ctx, recipient.AccountID, author.StudentID)
	require.NoError(t, err)
	assert.Empty(t, recipientRows)

	protectionOff := &peopledirectory.FamilyProtectionFixture{
		StudentID: author.StudentID, Enabled: false, Reason: "Schutz aufgehoben", ActorAccountID: author.AccountID,
	}
	protectionOff.TenantID = author.TenantID
	require.NoError(t, repos.CreateFamilyProtectionFixture(ctx, protectionOff))
	recipientRows, err = svc.ListMyMasterDataRequests(ctx, recipient.AccountID, author.StudentID)
	require.NoError(t, err)
	assert.Empty(t, recipientRows, "disabling protection must not revive an old share")

	_, err = sharing.SetRequestSharing(ctx, author.AccountID, author.StudentID, parentService.RequestShareMasterData, request.ID, []int64{recipient.GuardianProfileID})
	require.NoError(t, err)
	recipientRows, err = svc.ListMyMasterDataRequests(ctx, recipient.AccountID, author.StudentID)
	require.NoError(t, err)
	assert.Len(t, recipientRows, 1)
}
