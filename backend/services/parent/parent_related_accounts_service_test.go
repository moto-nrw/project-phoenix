package parent_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
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

func buildRelAcctService(t *testing.T, inviteMode string, canRemove bool) (parentService.Service, *stubInvites, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	invites := &stubInvites{}
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:     repos.ParentChild,
		StatusDayRepo: repos.StudentStatusDay,
		StudentRepo:   repos.Student,
		Settings: parentSettingsStub{
			boolValues:   map[string]bool{configModels.KeyGuardianParentCanRemove: canRemove},
			stringValues: map[string]string{configModels.KeyGuardianParentInviteMode: inviteMode},
		},
		GuardianInvites:     invites,
		GuardianInviteRepo:  repos.GuardianInvitation,
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

func TestListRelatedAccounts_NoAccountWithoutInviteIsNotPending(t *testing.T) {
	svc, _, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)
	defer func() { _ = db.Close() }()

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	repos := repositories.NewFactory(db)
	ctx := testpkg.TenantContext(1)
	profile := testpkg.CreateTestGuardianProfile(t, db, "staff-contact")
	defer func() {
		_, _ = db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(context.Background())
	}()
	link := &userModels.StudentGuardian{
		StudentID:         chain.StudentID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "guardian",
		EmergencyPriority: 1,
	}
	link.SetTenantID(1)
	require.NoError(t, repos.StudentGuardian.Create(ctx, link))

	accounts, err := svc.ListRelatedAccounts(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)

	var found *parentService.RelatedAccount
	for _, account := range accounts {
		if account.GuardianProfileID == profile.ID {
			found = account
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, parentService.RelatedAccountNoAccount, found.Status)
}

func TestListRelatedAccounts_NoAccountWithOpenInviteIsPending(t *testing.T) {
	svc, _, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)
	defer func() { _ = db.Close() }()

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	repos := repositories.NewFactory(db)
	ctx := testpkg.TenantContext(1)
	profile := testpkg.CreateTestGuardianProfile(t, db, "pending-contact")
	defer func() {
		_, _ = db.NewDelete().TableExpr("auth.guardian_invitations").Where("guardian_profile_id = ?", profile.ID).Exec(context.Background())
		_, _ = db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(context.Background())
	}()
	link := &userModels.StudentGuardian{
		StudentID:         chain.StudentID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "guardian",
		EmergencyPriority: 1,
	}
	link.SetTenantID(1)
	require.NoError(t, repos.StudentGuardian.Create(ctx, link))
	studentID := chain.StudentID
	invitation := &authModels.GuardianInvitation{
		Token:             fmt.Sprintf("pending-parent-status-test-%d", time.Now().UnixNano()),
		GuardianProfileID: profile.ID,
		CreatedBy:         chain.AccountID,
		ExpiresAt:         time.Now().Add(time.Hour),
		StudentID:         &studentID,
		ApprovalStatus:    authModels.GuardianInvitationApprovalNotRequired,
	}
	invitation.SetTenantID(1)
	require.NoError(t, repos.GuardianInvitation.Create(ctx, invitation))

	accounts, err := svc.ListRelatedAccounts(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)

	var found *parentService.RelatedAccount
	for _, account := range accounts {
		if account.GuardianProfileID == profile.ID {
			found = account
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, parentService.RelatedAccountPending, found.Status)
}

func TestListRelatedAccounts_OpenInviteForAnotherChildIsNotPending(t *testing.T) {
	svc, _, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)
	defer func() { _ = db.Close() }()

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	repos := repositories.NewFactory(db)
	ctx := testpkg.TenantContext(1)
	profile := testpkg.CreateTestGuardianProfile(t, db, "sibling-pending-contact")
	otherStudent := testpkg.CreateTestStudent(t, db, "Other", "Child", "9z")
	defer func() {
		_, _ = db.NewDelete().TableExpr("auth.guardian_invitations").Where("guardian_profile_id = ?", profile.ID).Exec(context.Background())
		_, _ = db.NewDelete().TableExpr("users.students_guardians").Where("guardian_profile_id = ?", profile.ID).Exec(context.Background())
		_, _ = db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(context.Background())
		_, _ = db.NewDelete().TableExpr("users.students").Where("id = ?", otherStudent.ID).Exec(context.Background())
	}()
	link := &userModels.StudentGuardian{
		StudentID:         chain.StudentID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "guardian",
		EmergencyPriority: 1,
	}
	link.SetTenantID(1)
	require.NoError(t, repos.StudentGuardian.Create(ctx, link))
	otherStudentID := otherStudent.ID
	invitation := &authModels.GuardianInvitation{
		Token:             fmt.Sprintf("sibling-pending-parent-status-test-%d", time.Now().UnixNano()),
		GuardianProfileID: profile.ID,
		CreatedBy:         chain.AccountID,
		ExpiresAt:         time.Now().Add(time.Hour),
		StudentID:         &otherStudentID,
		ApprovalStatus:    authModels.GuardianInvitationApprovalNotRequired,
	}
	invitation.SetTenantID(1)
	require.NoError(t, repos.GuardianInvitation.Create(ctx, invitation))

	accounts, err := svc.ListRelatedAccounts(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)

	var found *parentService.RelatedAccount
	for _, account := range accounts {
		if account.GuardianProfileID == profile.ID {
			found = account
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, parentService.RelatedAccountNoAccount, found.Status)
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

func TestRemoveRelatedAccount_DisabledInviteModeRejectsStaleRemoveFlag(t *testing.T) {
	svc, invites, db := buildRelAcctService(t, configModels.ParentInviteModeDisabled, true)
	defer func() { _ = db.Close() }()

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	err := svc.RemoveRelatedAccount(context.Background(), chain.AccountID, chain.StudentID, chain.GuardianProfileID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, parentService.ErrRemoveDisabled))
	assert.Nil(t, invites.lastRevoke, "revoke must not be called when invite mode disables management")
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

	t.Run("invite disabled + stale remove on", func(t *testing.T) {
		svc, _, db := buildRelAcctService(t, configModels.ParentInviteModeDisabled, true)
		defer func() { _ = db.Close() }()
		chain := testpkg.CreateTestParentGuardianChain(t, db)
		defer testpkg.CleanupParentGuardianChain(t, db, chain)

		flags, err := svc.ChildFeatures(context.Background(), chain.AccountID, chain.StudentID)
		require.NoError(t, err)
		assert.False(t, flags.RelatedAccountsInviteEnabled)
		assert.False(t, flags.RelatedAccountsRemoveEnabled, "disabled invite mode must suppress stale remove flag")
	})
}

func buildRelAcctServiceWith(t *testing.T, settings configService.SettingsService) (parentService.Service, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StatusDayRepo:       repos.StudentStatusDay,
		StudentRepo:         repos.Student,
		Settings:            settings,
		GuardianInvites:     &stubInvites{},
		GuardianInviteRepo:  repos.GuardianInvitation,
		StudentGuardianRepo: repos.StudentGuardian,
		GuardianProfileRepo: repos.GuardianProfile,
		DB:                  db,
		Logger:              slog.Default(),
	})
	return svc, db
}

func TestInviteRelatedAccount_EmptyEmailRejected(t *testing.T) {
	svc, invites, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.InviteRelatedAccount(context.Background(), chain.AccountID, chain.StudentID, "   ", "", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, parentService.ErrEmailRequired))
	assert.Nil(t, invites.lastInvite)
}

func TestInviteRelatedAccount_InvalidEmailRejected(t *testing.T) {
	svc, invites, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.InviteRelatedAccount(context.Background(), chain.AccountID, chain.StudentID, "not-an-email", "", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, parentService.ErrInvalidInviteInput))
	assert.Nil(t, invites.lastInvite)
}

func TestRelatedAccounts_SettingsErrorsAreSurfaced(t *testing.T) {
	svc, db := buildRelAcctServiceWith(t, parentSettingsStub{
		boolErr:   errors.New("settings unavailable"),
		stringErr: errors.New("settings unavailable"),
	})
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.InviteRelatedAccount(context.Background(), chain.AccountID, chain.StudentID, "x@example.test", "", "")
	require.Error(t, err, "invite-mode resolve failure must surface")

	err = svc.RemoveRelatedAccount(context.Background(), chain.AccountID, chain.StudentID, chain.GuardianProfileID)
	require.Error(t, err, "can-remove resolve failure must surface")
}

// failingInvites errors on delegation so the parent service's error-return
// branches around the guardian invitation service are exercised.
type failingInvites struct {
	authService.GuardianInvitationService
}

func (failingInvites) InviteToStudent(_ context.Context, _ authService.InviteToStudentRequest) (*authService.InviteToStudentResult, error) {
	return nil, errors.New("invite failed")
}
func (failingInvites) RevokeAccess(_ context.Context, _ authService.RevokeAccessRequest) error {
	return errors.New("revoke failed")
}

func buildRelAcctServiceInvites(t *testing.T, inviteMode string, canRemove bool, invites authService.GuardianInvitationService) (parentService.Service, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:     repos.ParentChild,
		StatusDayRepo: repos.StudentStatusDay,
		StudentRepo:   repos.Student,
		Settings: parentSettingsStub{
			boolValues:   map[string]bool{configModels.KeyGuardianParentCanRemove: canRemove},
			stringValues: map[string]string{configModels.KeyGuardianParentInviteMode: inviteMode},
		},
		GuardianInvites:     invites,
		GuardianInviteRepo:  repos.GuardianInvitation,
		StudentGuardianRepo: repos.StudentGuardian,
		GuardianProfileRepo: repos.GuardianProfile,
		DB:                  db,
		Logger:              slog.Default(),
	})
	return svc, db
}

func TestListRelatedAccounts_UnownedChildErrors(t *testing.T) {
	svc, _, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	other := testpkg.CreateTestStudent(t, db, "Not", "Owned", "9z")
	defer func() {
		_, _ = db.NewDelete().TableExpr("users.students").Where("id = ?", other.ID).Exec(context.Background())
	}()

	_, err := svc.ListRelatedAccounts(context.Background(), chain.AccountID, other.ID)
	require.Error(t, err, "listing an unowned child must be rejected")
}

func TestRelatedAccounts_DelegateErrorsSurface(t *testing.T) {
	t.Run("invite delegate error", func(t *testing.T) {
		svc, db := buildRelAcctServiceInvites(t, configModels.ParentInviteModeDirect, true, failingInvites{})
		defer func() { _ = db.Close() }()
		chain := testpkg.CreateTestParentGuardianChain(t, db)
		defer testpkg.CleanupParentGuardianChain(t, db, chain)

		_, err := svc.InviteRelatedAccount(context.Background(), chain.AccountID, chain.StudentID, "x@example.test", "", "")
		require.Error(t, err)
	})

	t.Run("revoke delegate error", func(t *testing.T) {
		svc, db := buildRelAcctServiceInvites(t, configModels.ParentInviteModeDirect, true, failingInvites{})
		defer func() { _ = db.Close() }()
		chain := testpkg.CreateTestParentGuardianChain(t, db)
		defer testpkg.CleanupParentGuardianChain(t, db, chain)

		err := svc.RemoveRelatedAccount(context.Background(), chain.AccountID, chain.StudentID, chain.GuardianProfileID)
		require.Error(t, err)
	})
}
