package parent_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestRequestSharingIsNamedAndFamilyProtectionDoesNotReviveOldShares(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	author := testpkg.CreateTestParentGuardianChain(t, db)
	recipient := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), author.TenantID)

	_, err := db.NewUpdate().
		TableExpr(`users.students_guardians`).
		Set("student_id = ?", author.StudentID).
		Where("guardian_profile_id = ?", recipient.GuardianProfileID).
		Exec(ctx)
	require.NoError(t, err)

	request := &userModels.StudentDataChangeRequest{
		StudentID: author.StudentID, SubmittedBy: author.AccountID,
		Target: userModels.DataChangeTargetPerson, FieldKey: "first_name",
		OldValue: json.RawMessage(`"Felix"`), NewValue: json.RawMessage(`"Feli"`),
		Status: userModels.DataChangeStatusPending,
	}
	request.SetTenantID(author.TenantID)
	require.NoError(t, repos.StudentDataChangeRequest.Create(ctx, request))

	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo: repos.ParentChild, GuardianProfileRepo: repos.GuardianProfile,
		GuardianPhoneRepo: repos.GuardianPhoneNumber, PersonRepo: repos.Person,
		StudentGuardianRepo: repos.StudentGuardian, StudentRepo: repos.Student,
		ChangeRequestRepo:      repos.StudentDataChangeRequest,
		FamilyProtectionEvents: repos.FamilyProtection, ParentRequestShares: repos.ParentRequestShare,
		DB: db, Logger: slog.Default(),
	})
	sharing := svc.(parentService.RequestSharingService)
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

	protectionOn := &userModels.FamilyProtectionEvent{
		StudentID: author.StudentID, Enabled: true, Reason: "Schutzfall", ActorAccountID: author.AccountID,
	}
	protectionOn.SetTenantID(author.TenantID)
	require.NoError(t, repos.FamilyProtection.Create(ctx, protectionOn))
	recipientRows, err = svc.ListMyMasterDataRequests(ctx, recipient.AccountID, author.StudentID)
	require.NoError(t, err)
	assert.Empty(t, recipientRows)

	protectionOff := &userModels.FamilyProtectionEvent{
		StudentID: author.StudentID, Enabled: false, Reason: "Schutz aufgehoben", ActorAccountID: author.AccountID,
	}
	protectionOff.SetTenantID(author.TenantID)
	require.NoError(t, repos.FamilyProtection.Create(ctx, protectionOff))
	recipientRows, err = svc.ListMyMasterDataRequests(ctx, recipient.AccountID, author.StudentID)
	require.NoError(t, err)
	assert.Empty(t, recipientRows, "disabling protection must not revive an old share")

	_, err = sharing.SetRequestSharing(ctx, author.AccountID, author.StudentID, parentService.RequestShareMasterData, request.ID, []int64{recipient.GuardianProfileID})
	require.NoError(t, err)
	recipientRows, err = svc.ListMyMasterDataRequests(ctx, recipient.AccountID, author.StudentID)
	require.NoError(t, err)
	assert.Len(t, recipientRows, 1)
}

func TestRequestSharingRejectsUnlinkedRecipientAndNonOwner(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	author := testpkg.CreateTestParentGuardianChain(t, db)
	other := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), author.TenantID)
	request := &userModels.StudentDataChangeRequest{
		StudentID: author.StudentID, SubmittedBy: author.AccountID,
		Target: userModels.DataChangeTargetPerson, FieldKey: "first_name",
		OldValue: json.RawMessage(`"Felix"`), NewValue: json.RawMessage(`"Feli"`),
		Status: userModels.DataChangeStatusPending,
	}
	request.SetTenantID(author.TenantID)
	require.NoError(t, repos.StudentDataChangeRequest.Create(ctx, request))
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo: repos.ParentChild, GuardianProfileRepo: repos.GuardianProfile,
		StudentGuardianRepo: repos.StudentGuardian, StudentRepo: repos.Student,
		ChangeRequestRepo:      repos.StudentDataChangeRequest,
		FamilyProtectionEvents: repos.FamilyProtection, ParentRequestShares: repos.ParentRequestShare,
		DB: db, Logger: slog.Default(),
	})
	sharing := svc.(parentService.RequestSharingService)

	_, err := sharing.SetRequestSharing(ctx, author.AccountID, author.StudentID, parentService.RequestShareMasterData, request.ID, []int64{other.GuardianProfileID})
	assert.ErrorIs(t, err, parentService.ErrRequestSharingInvalid)
	_, err = sharing.SetRequestSharing(ctx, author.AccountID, author.StudentID, parentService.RequestShareMasterData, request.ID, []int64{author.GuardianProfileID})
	assert.ErrorIs(t, err, parentService.ErrRequestSharingInvalid)
	_, err = sharing.SetRequestSharing(ctx, author.AccountID, author.StudentID, parentService.RequestShareMasterData, request.ID, []int64{author.GuardianProfileID, author.GuardianProfileID})
	assert.ErrorIs(t, err, parentService.ErrRequestSharingInvalid)
	_, err = sharing.SetRequestSharing(ctx, other.AccountID, other.StudentID, parentService.RequestShareMasterData, request.ID, nil)
	assert.Error(t, err)
}
