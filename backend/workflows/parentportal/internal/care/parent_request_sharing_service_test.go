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

	"github.com/uptrace/bun"

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
	sharing := buildRequestSharing(db, repos, reposPeople)

	_, err := sharing.SetRequestSharing(ctx, author.AccountID, author.StudentID, parentportal.RequestShareMasterData, request.ID, []int64{other.GuardianProfileID})
	assert.ErrorIs(t, err, parentportal.ErrRequestSharingInvalid)
	_, err = sharing.SetRequestSharing(ctx, author.AccountID, author.StudentID, parentportal.RequestShareMasterData, request.ID, []int64{author.GuardianProfileID})
	assert.ErrorIs(t, err, parentportal.ErrRequestSharingInvalid)
	_, err = sharing.SetRequestSharing(ctx, author.AccountID, author.StudentID, parentportal.RequestShareMasterData, request.ID, []int64{author.GuardianProfileID, author.GuardianProfileID})
	assert.ErrorIs(t, err, parentportal.ErrRequestSharingInvalid)
	_, err = sharing.SetRequestSharing(ctx, other.AccountID, other.StudentID, parentportal.RequestShareMasterData, request.ID, nil)
	assert.Error(t, err)
}

func buildRequestSharing(db *bun.DB, repos *api.RepositoryTestFactory, reposPeople *api.PeopleRepositorySuiteFactory) parentportal.RequestSharingService {
	return parentportal.NewMessagingFlows(parentportal.MessagingConfig{
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
}

// TestSetRequestSharingRequiresTheRequestTypesSubmitPermission pins that
// changing who sees an open request is an edit of that request: once the
// school withdraws the submit permission of its type, the author can still
// read the sharing state but no longer change it, and no share event is
// written.
func TestSetRequestSharingRequiresTheRequestTypesSubmitPermission(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	author := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), author.TenantID)
	request := &parentportal.StudentDataChangeRequest{
		StudentID: author.StudentID, SubmittedBy: author.AccountID,
		Target: parentportal.DataChangeTargetPerson, FieldKey: "first_name",
		OldValue: json.RawMessage(`"Felix"`), NewValue: json.RawMessage(`"Feli"`),
		Status: parentportal.DataChangeStatusPending,
	}
	request.TenantID = author.TenantID
	require.NoError(t, api.CreateParentDataRequestFixture(repos.CarePlan(), ctx, request))
	sharing := buildRequestSharing(db, repos, reposPeople)
	shareEvents := func() int64 {
		count, err := db.NewSelect().TableExpr("users.parent_request_share_events").
			Where("student_id = ? AND request_id = ?", author.StudentID, request.ID).Count(ctx)
		require.NoError(t, err)
		return count
	}

	_, err := sharing.SetRequestSharing(ctx, author.AccountID, author.StudentID, parentportal.RequestShareMasterData, request.ID, nil)
	require.NoError(t, err, "the author with the submit permission changes the sharing")
	require.EqualValues(t, 1, shareEvents())

	var linkID int64
	require.NoError(t, db.NewSelect().TableExpr("users.student_guardian_relationships").Column("id").
		Where("student_id = ? AND guardian_profile_id = ?", author.StudentID, author.GuardianProfileID).Scan(ctx, &linkID))
	require.NoError(t, testpkg.SetTestStudentGuardianPermissions(ctx, db, linkID, map[string]interface{}{"parent_portal.access": true}))

	_, err = sharing.SetRequestSharing(ctx, author.AccountID, author.StudentID, parentportal.RequestShareMasterData, request.ID, nil)
	require.ErrorIs(t, err, parentportal.ErrGuardianPermissionDenied)
	assert.EqualValues(t, 1, shareEvents(), "a refused change writes no share event")

	_, err = sharing.GetRequestSharing(ctx, author.AccountID, author.StudentID, parentportal.RequestShareMasterData, request.ID)
	assert.NoError(t, err, "reading the sharing state stays on parent_portal.access")
}
