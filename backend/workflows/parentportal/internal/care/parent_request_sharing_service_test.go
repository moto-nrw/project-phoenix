package care_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestRequestSharingRejectsUnlinkedRecipientAndNonOwner(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	author := testpkg.CreateTestParentGuardianChain(t, db)
	other := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), author.TenantID)
	request := &parentportal.StudentDataChangeRequest{
		StudentID: author.StudentID, SubmittedBy: author.AccountID,
		Target: parentportal.DataChangeTargetPerson, FieldKey: "first_name",
		OldValue: json.RawMessage(`"Felix"`), NewValue: json.RawMessage(`"Feli"`),
		Status: parentportal.DataChangeStatusPending,
	}
	request.TenantID = author.TenantID
	require.NoError(t, api.CreateParentDataRequestFixture(repos.CarePlan(), ctx, request))
	svc := parentportal.NewMessagingFlows(parentportal.MessagingConfig{
		SharePolicy:    parentportalcompose.NewRequestSharePolicy(),
		GuardianPolicy: owners.NewParentGuardianPolicy(),
		Fingerprints:   owners.SecurityFingerprints{},
		Declarations:   owners.ParentDeclarationPolicy(),
		Runtime:        owners.ParentPortalRuntime{},
		Children:       childResolver(db, repos, reposPeople),
		ChildRepo:      repos.ParentChildReads(), GuardianProfileRepo: owners.NewParentGuardianProfileReads(reposPeople.GuardianProfile),
		StudentGuardianRepo: owners.NewParentStudentGuardianReads(reposPeople.StudentGuardian), StudentRepo: owners.NewParentStudentReads(reposPeople.Student),
		ChangeRequestRepo:      owners.NewParentDataRequestReads(repos.CarePlan()),
		FamilyProtectionEvents: owners.BindParentFamilyProtectionReads(reposPeople.FamilyProtection), ParentRequestShares: owners.BindParentRequestShareRecords(reposPeople.ParentRequestShare),
		Logger: slog.Default(),
	})
	sharing := parentportal.RequestSharingService(svc)

	_, err := sharing.SetRequestSharing(ctx, author.AccountID, author.StudentID, parentportal.RequestShareMasterData, request.ID, []int64{other.GuardianProfileID})
	assert.ErrorIs(t, err, parentportal.ErrRequestSharingInvalid)
	_, err = sharing.SetRequestSharing(ctx, author.AccountID, author.StudentID, parentportal.RequestShareMasterData, request.ID, []int64{author.GuardianProfileID})
	assert.ErrorIs(t, err, parentportal.ErrRequestSharingInvalid)
	_, err = sharing.SetRequestSharing(ctx, author.AccountID, author.StudentID, parentportal.RequestShareMasterData, request.ID, []int64{author.GuardianProfileID, author.GuardianProfileID})
	assert.ErrorIs(t, err, parentportal.ErrRequestSharingInvalid)
	_, err = sharing.SetRequestSharing(ctx, other.AccountID, other.StudentID, parentportal.RequestShareMasterData, request.ID, nil)
	assert.Error(t, err)
}
