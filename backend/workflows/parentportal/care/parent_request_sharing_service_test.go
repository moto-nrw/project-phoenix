package care_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
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
		GuardianPolicy: api.NewParentGuardianPolicy(),
		Fingerprints:   api.SecurityFingerprints{},
		Declarations:   api.ParentDeclarationPolicy(),
		Runtime:        api.ParentPortalRuntime{},
		Children:       childResolver(db, repos, reposPeople),
		ChildRepo:      repos.ParentChildReads(), GuardianProfileRepo: api.NewParentGuardianProfileReads(reposPeople.GuardianProfile),
		StudentGuardianRepo: api.NewParentStudentGuardianReads(reposPeople.StudentGuardian), StudentRepo: api.NewParentStudentReads(reposPeople.Student),
		ChangeRequestRepo:      api.NewParentDataRequestReads(repos.CarePlan()),
		FamilyProtectionEvents: api.BindParentFamilyProtectionReads(reposPeople.FamilyProtection), ParentRequestShares: api.BindParentRequestShareRecords(reposPeople.ParentRequestShare),
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
