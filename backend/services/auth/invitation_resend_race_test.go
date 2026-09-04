package auth_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Simulate acceptance/revocation committing after resend reads the invitation.
// The returned snapshot still has UsedAt=nil, exactly as in the racing request.
type consumedAfterReadInvitationRepository struct {
	authModels.InvitationTokenRepository
}

func (r consumedAfterReadInvitationRepository) FindByID(ctx context.Context, id any) (*authModels.InvitationToken, error) {
	invitation, err := r.InvitationTokenRepository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := r.MarkAsUsed(ctx, invitation.ID); err != nil {
		return nil, err
	}
	return invitation, nil
}

func TestInvitationResendCannotResurrectConsumedToken(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	creator := testpkg.CreateTestAccount(t, db, "resend-creator")
	role := testpkg.CreateTestRole(t, db, "resend-staff")
	invitation := &authModels.InvitationToken{
		Email: fmt.Sprintf("resend-%d@example.com", testpkg.Tenant(t)), RoleID: role.ID,
		Token: fmt.Sprintf("resend-%d", testpkg.Tenant(t)), ExpiresAt: time.Now().Add(time.Hour),
	}
	invitation.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.InvitationToken.Create(ctx, invitation))
	service := auth.NewInvitationService(auth.InvitationServiceConfig{
		InvitationRepo: consumedAfterReadInvitationRepository{repos.InvitationToken},
		RoleRepo:       repos.Role, SchoolRepo: repos.School, DB: db,
	})
	testpkg.SetTenantRuntime(t, service, db)
	require.NoError(t, service.ResendInvitation(ctx, invitation.ID, creator.ID))
	stored, err := repos.InvitationToken.FindByToken(ctx, invitation.Token)
	require.NoError(t, err)
	require.True(t, stored.IsUsed(), "resend must not clear the committed consumption")
	_, err = service.ValidateInvitation(context.Background(), invitation.Token)
	require.ErrorIs(t, err, auth.ErrInvitationUsed)
}
