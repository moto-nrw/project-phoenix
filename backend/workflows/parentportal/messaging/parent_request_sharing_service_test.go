package messaging_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/messaging"
)

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
	svc := messaging.New(messaging.Config{
		Children:  childResolver(db, repos),
		ChildRepo: repos.ParentChild, GuardianProfileRepo: repos.GuardianProfile,
		StudentGuardianRepo: repos.StudentGuardian, StudentRepo: repos.Student,
		ChangeRequestRepo:      repos.StudentDataChangeRequest,
		FamilyProtectionEvents: repos.FamilyProtection, ParentRequestShares: repos.ParentRequestShare,
		DB: db, Logger: slog.Default(),
	})
	sharing := messaging.RequestSharingService(svc)

	_, err := sharing.SetRequestSharing(ctx, author.AccountID, author.StudentID, messaging.RequestShareMasterData, request.ID, []int64{other.GuardianProfileID})
	assert.ErrorIs(t, err, messaging.ErrRequestSharingInvalid)
	_, err = sharing.SetRequestSharing(ctx, author.AccountID, author.StudentID, messaging.RequestShareMasterData, request.ID, []int64{author.GuardianProfileID})
	assert.ErrorIs(t, err, messaging.ErrRequestSharingInvalid)
	_, err = sharing.SetRequestSharing(ctx, author.AccountID, author.StudentID, messaging.RequestShareMasterData, request.ID, []int64{author.GuardianProfileID, author.GuardianProfileID})
	assert.ErrorIs(t, err, messaging.ErrRequestSharingInvalid)
	_, err = sharing.SetRequestSharing(ctx, other.AccountID, other.StudentID, messaging.RequestShareMasterData, request.ID, nil)
	assert.Error(t, err)
}
