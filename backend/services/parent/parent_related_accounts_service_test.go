package parent_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// stubInvites records the calls the parent service makes into the guardian
// invitation service, so tests can assert on delegation (mode → RequireApproval,
// ByParent) without standing up the full invite machinery. Embeds the interface
// so the un-overridden methods exist but panic if unexpectedly called.
type stubInvites struct {
	authService.GuardianInvitationService
	lastInvite *authService.InviteToStudentRequest
	lastRevoke *authService.RevokeAccessRequest
}

func (s *stubInvites) InviteToStudent(_ context.Context, req authService.InviteToStudentRequest) (*authService.InviteToStudentResult, error) {
	s.lastInvite = &req
	return &authService.InviteToStudentResult{
		Outcome:           authService.InviteOutcomeInvited,
		GuardianProfileID: req.StudentID, // arbitrary non-zero
	}, nil
}

func (s *stubInvites) RevokeAccess(_ context.Context, req authService.RevokeAccessRequest) error {
	s.lastRevoke = &req
	return nil
}

// relAcctSettings is a configurable settings stub for the related-accounts gate.
type relAcctSettings struct {
	configService.SettingsService
	inviteMode string
	canRemove  bool
}

func (s relAcctSettings) ResolveStringForTenant(_ context.Context, _ int64, key string) (string, error) {
	if key == configModels.KeyGuardianParentInviteMode {
		return s.inviteMode, nil
	}
	return "", nil
}

func (s relAcctSettings) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	if key == configModels.KeyGuardianParentCanRemove {
		return s.canRemove, nil
	}
	return false, nil
}

func buildRelAcctService(t *testing.T, inviteMode string, canRemove bool) (parentService.Service, *stubInvites, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	invites := &stubInvites{}
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StatusDayRepo:       repos.StudentStatusDay,
		StudentRepo:         repos.Student,
		NoteRepo:            repos.StudentParentNote,
		Settings:            relAcctSettings{inviteMode: inviteMode, canRemove: canRemove},
		GuardianInvites:     invites,
		StudentGuardianRepo: repos.StudentGuardian,
		GuardianProfileRepo: repos.GuardianProfile,
		DB:                  db,
		Logger:              slog.Default(),
	})
	return svc, invites, db
}

func TestListRelatedAccounts_ReturnsLinkedWithStatus(t *testing.T) {
	svc, _, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)
	defer func() { _ = db.Close() }()

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	accounts, err := svc.ListRelatedAccounts(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	assert.Equal(t, chain.GuardianProfileID, accounts[0].GuardianProfileID)
	assert.True(t, accounts[0].IsPrimary)
	// The chain's guardian profile has an account linked → active.
	assert.Equal(t, parentService.RelatedAccountActive, accounts[0].Status)
}

func TestInviteRelatedAccount_DisabledIsRejected(t *testing.T) {
	svc, invites, db := buildRelAcctService(t, configModels.ParentInviteModeDisabled, false)
	defer func() { _ = db.Close() }()

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.InviteRelatedAccount(context.Background(), chain.AccountID, chain.StudentID, "x@example.test", "", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, parentService.ErrInviteDisabled))
	assert.Nil(t, invites.lastInvite, "invite service must not be called when disabled")
}

func TestInviteRelatedAccount_DirectDelegatesWithoutApproval(t *testing.T) {
	svc, invites, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)
	defer func() { _ = db.Close() }()

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.InviteRelatedAccount(context.Background(), chain.AccountID, chain.StudentID, "new@example.test", "Neue", "Person")
	require.NoError(t, err)
	require.NotNil(t, invites.lastInvite)
	assert.False(t, invites.lastInvite.RequireApproval, "direct mode must not require approval")
	require.NotNil(t, invites.lastInvite.RequestedByParentAccountID)
	assert.Equal(t, chain.AccountID, *invites.lastInvite.RequestedByParentAccountID)
	assert.Equal(t, chain.StudentID, invites.lastInvite.StudentID)
}

func TestInviteRelatedAccount_StaffApprovalRequiresApproval(t *testing.T) {
	svc, invites, db := buildRelAcctService(t, configModels.ParentInviteModeStaffApproval, false)
	defer func() { _ = db.Close() }()

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.InviteRelatedAccount(context.Background(), chain.AccountID, chain.StudentID, "new@example.test", "", "")
	require.NoError(t, err)
	require.NotNil(t, invites.lastInvite)
	assert.True(t, invites.lastInvite.RequireApproval, "staff_approval mode must queue for approval")
}

func TestInviteRelatedAccount_UnownedChildIsRejected(t *testing.T) {
	svc, invites, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)
	defer func() { _ = db.Close() }()

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	// A student the account is NOT a guardian of.
	other := testpkg.CreateTestStudent(t, db, "Not", "Mine", "9z")
	defer func() {
		_, _ = db.NewDelete().TableExpr("users.students").Where("id = ?", other.ID).Exec(context.Background())
	}()

	_, err := svc.InviteRelatedAccount(context.Background(), chain.AccountID, other.ID, "x@example.test", "", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, parentService.ErrChildNotLinked))
	assert.Nil(t, invites.lastInvite, "invite must not fire for an unowned child")
}

func TestRemoveRelatedAccount_DisabledIsRejected(t *testing.T) {
	svc, invites, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)
	defer func() { _ = db.Close() }()

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	err := svc.RemoveRelatedAccount(context.Background(), chain.AccountID, chain.StudentID, chain.GuardianProfileID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, parentService.ErrRemoveDisabled))
	assert.Nil(t, invites.lastRevoke, "revoke must not be called when removal is disabled")
}

func TestRemoveRelatedAccount_EnabledDelegatesAsParent(t *testing.T) {
	svc, invites, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, true)
	defer func() { _ = db.Close() }()

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	err := svc.RemoveRelatedAccount(context.Background(), chain.AccountID, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)
	require.NotNil(t, invites.lastRevoke)
	assert.True(t, invites.lastRevoke.ByParent, "parent removals must set ByParent (primary protection)")
	assert.Equal(t, chain.GuardianProfileID, invites.lastRevoke.GuardianProfileID)
}

func TestChildFeatures_ExposesRelatedAccountsFlags(t *testing.T) {
	t.Run("invite enabled + remove on", func(t *testing.T) {
		svc, _, db := buildRelAcctService(t, configModels.ParentInviteModeStaffApproval, true)
		defer func() { _ = db.Close() }()
		chain := testpkg.CreateTestParentGuardianChain(t, db)
		defer testpkg.CleanupParentGuardianChain(t, db, chain)

		flags, err := svc.ChildFeatures(context.Background(), chain.AccountID, chain.StudentID)
		require.NoError(t, err)
		assert.True(t, flags.RelatedAccountsInviteEnabled, "non-disabled invite mode → invite enabled")
		assert.True(t, flags.RelatedAccountsRemoveEnabled)
	})

	t.Run("invite disabled + remove off", func(t *testing.T) {
		svc, _, db := buildRelAcctService(t, configModels.ParentInviteModeDisabled, false)
		defer func() { _ = db.Close() }()
		chain := testpkg.CreateTestParentGuardianChain(t, db)
		defer testpkg.CleanupParentGuardianChain(t, db, chain)

		flags, err := svc.ChildFeatures(context.Background(), chain.AccountID, chain.StudentID)
		require.NoError(t, err)
		assert.False(t, flags.RelatedAccountsInviteEnabled, "disabled mode → invite hidden")
		assert.False(t, flags.RelatedAccountsRemoveEnabled)
	})
}
