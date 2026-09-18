package legacy_test

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

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal/legacy"
)

// stubInvites records the calls the parent service makes into the guardian
// invitation service, so tests can assert on delegation (mode → RequireApproval,
// ByParent) without standing up the full invite machinery. Embeds the interface
// so the un-overridden methods exist but panic if unexpectedly called.
// guardianInvitationReads reads the owner's invitations straight from the
// table, which is what the serving root's port does through the Identity &
// Access capability.
type guardianInvitationReads struct{ db *bun.DB }

func (r guardianInvitationReads) ListByProfile(ctx context.Context, guardianProfileID int64) ([]parentService.GuardianInvitationRecord, error) {
	var invitations []*authModels.GuardianInvitation
	err := r.db.NewSelect().
		Model(&invitations).
		ModelTableExpr(`auth.guardian_invitations AS "guardian_invitation"`).
		Where(`"guardian_invitation".guardian_profile_id = ?`, guardianProfileID).
		OrderExpr(`"guardian_invitation".created_at DESC`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	records := make([]parentService.GuardianInvitationRecord, 0, len(invitations))
	for _, invitation := range invitations {
		records = append(records, parentService.GuardianInvitationRecord{
			ID: invitation.ID, GuardianProfileID: invitation.GuardianProfileID, StudentID: invitation.StudentID,
			ExpiresAt: invitation.ExpiresAt, AcceptedAt: invitation.AcceptedAt, ApprovalStatus: invitation.ApprovalStatus,
		})
	}
	return records, nil
}

// stubInvites stands in for the consumer-owned relative access port the
// composition root binds to Identity & Access (#3332).
type stubInvites struct {
	lastInvite *parentService.GuardianInviteRequest
	lastRevoke *parentService.GuardianAccessRevocation
}

func (s *stubInvites) InviteToStudent(_ context.Context, request parentService.GuardianInviteRequest) (parentService.GuardianInviteOutcome, error) {
	s.lastInvite = &request
	return parentService.GuardianInviteOutcome{
		Outcome:           "invited",
		GuardianProfileID: request.StudentID, // arbitrary non-zero
	}, nil
}

func (s *stubInvites) RevokeAccess(_ context.Context, revocation parentService.GuardianAccessRevocation) error {
	s.lastRevoke = &revocation
	return nil
}

func buildRelAcctService(t *testing.T, inviteMode string, canRemove bool) (parentService.Service, *stubInvites, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	invites := &stubInvites{}
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:     repos.ParentChild,
		StatusDayRepo: repos.StudentStatusDay,
		StudentRepo:   repos.Student,
		Settings: parentSettingsStub{
			boolValues:   map[string]bool{configModels.KeyGuardianParentCanRemove: canRemove},
			stringValues: map[string]string{configModels.KeyGuardianParentInviteMode: inviteMode},
		},
		MealPlan:            availableMealPlan(false),
		GuardianInvites:     invites,
		GuardianInvitations: guardianInvitationReads{db: db},
		StudentGuardianRepo: repos.StudentGuardian,
		GuardianProfileRepo: repos.GuardianProfile,
		DB:                  db,
		Logger:              slog.Default(),
	})
	return svc, invites, db
}

func TestListRelatedAccounts_ReturnsLinkedWithStatus(t *testing.T) {
	t.Parallel()

	svc, _, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	accounts, err := svc.ListRelatedAccounts(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	assert.Equal(t, chain.GuardianProfileID, accounts[0].GuardianProfileID)
	assert.True(t, accounts[0].IsPrimary)
	// The chain's guardian profile has an account linked → active.
	assert.Equal(t, parentService.RelatedAccountActive, accounts[0].Status)
}

func TestListRelatedAccounts_NoAccountWithoutInviteIsNotPending(t *testing.T) {
	t.Parallel()

	svc, _, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	ctx := testpkg.Ctx(t)
	profile := testpkg.CreateTestGuardianProfile(t, db, "staff-contact")
	defer func() {
		_, _ = db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(testpkg.WithPackageTenantRuntime(context.Background()))
	}()
	link := &userModels.StudentGuardian{
		StudentID:         chain.StudentID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "guardian",
		EmergencyPriority: 1,
	}
	link.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.StudentGuardian.Create(ctx, link))

	accounts, err := svc.ListRelatedAccounts(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
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
	t.Parallel()

	svc, _, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	ctx := testpkg.Ctx(t)
	profile := testpkg.CreateTestGuardianProfile(t, db, "pending-contact")
	defer func() {
		_, _ = db.NewDelete().TableExpr("auth.guardian_invitations").Where("guardian_profile_id = ?", profile.ID).Exec(testpkg.WithPackageTenantRuntime(context.Background()))
		_, _ = db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(testpkg.WithPackageTenantRuntime(context.Background()))
	}()
	link := &userModels.StudentGuardian{
		StudentID:         chain.StudentID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "guardian",
		EmergencyPriority: 1,
	}
	link.SetTenantID(testpkg.Tenant(t))
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
	invitation.SetTenantID(testpkg.Tenant(t))
	testpkg.InsertTestGuardianInvitation(t, db, invitation)

	accounts, err := svc.ListRelatedAccounts(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
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
	t.Parallel()

	svc, _, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	ctx := testpkg.Ctx(t)
	profile := testpkg.CreateTestGuardianProfile(t, db, "sibling-pending-contact")
	otherStudent := testpkg.CreateTestStudent(t, db, "Other", "Child", "9z")
	defer func() {
		_, _ = db.NewDelete().TableExpr("auth.guardian_invitations").Where("guardian_profile_id = ?", profile.ID).Exec(testpkg.WithPackageTenantRuntime(context.Background()))
		_, _ = db.NewDelete().TableExpr("users.students_guardians").Where("guardian_profile_id = ?", profile.ID).Exec(testpkg.WithPackageTenantRuntime(context.Background()))
		_, _ = db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(testpkg.WithPackageTenantRuntime(context.Background()))
		_, _ = db.NewDelete().TableExpr("users.students").Where("id = ?", otherStudent.ID).Exec(testpkg.WithPackageTenantRuntime(context.Background()))
	}()
	link := &userModels.StudentGuardian{
		StudentID:         chain.StudentID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "guardian",
		EmergencyPriority: 1,
	}
	link.SetTenantID(testpkg.Tenant(t))
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
	invitation.SetTenantID(testpkg.Tenant(t))
	testpkg.InsertTestGuardianInvitation(t, db, invitation)

	accounts, err := svc.ListRelatedAccounts(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
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
	t.Parallel()

	svc, invites, db := buildRelAcctService(t, configModels.ParentInviteModeDisabled, false)

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.InviteRelatedAccount(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, "x@example.test", "", "", false)
	require.Error(t, err)
	assert.True(t, errors.Is(err, parentService.ErrInviteDisabled))
	assert.Nil(t, invites.lastInvite, "invite service must not be called when disabled")
}

func TestInviteRelatedAccount_DirectDelegatesWithoutApproval(t *testing.T) {
	t.Parallel()

	svc, invites, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.InviteRelatedAccount(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, "new@example.test", "Neue", "Person", false)
	require.NoError(t, err)
	require.NotNil(t, invites.lastInvite)
	assert.False(t, invites.lastInvite.RequireApproval, "direct mode must not require approval")
	assert.Equal(t, chain.AccountID, invites.lastInvite.RequestedByAccountID)
	assert.Equal(t, chain.StudentID, invites.lastInvite.StudentID)
}

func TestInviteRelatedAccount_StaffApprovalRequiresApproval(t *testing.T) {
	t.Parallel()

	svc, invites, db := buildRelAcctService(t, configModels.ParentInviteModeStaffApproval, false)

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.InviteRelatedAccount(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, "new@example.test", "", "", false)
	require.NoError(t, err)
	require.NotNil(t, invites.lastInvite)
	assert.True(t, invites.lastInvite.RequireApproval, "staff_approval mode must queue for approval")
}

func TestInviteRelatedAccount_UnownedChildIsRejected(t *testing.T) {
	t.Parallel()

	svc, invites, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	// A student the account is NOT a guardian of.
	other := testpkg.CreateTestStudent(t, db, "Not", "Mine", "9z")
	defer func() {
		_, _ = db.NewDelete().TableExpr("users.students").Where("id = ?", other.ID).Exec(testpkg.WithPackageTenantRuntime(context.Background()))
	}()

	_, err := svc.InviteRelatedAccount(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, other.ID, "x@example.test", "", "", false)
	require.Error(t, err)
	assert.True(t, errors.Is(err, parentService.ErrChildNotLinked))
	assert.Nil(t, invites.lastInvite, "invite must not fire for an unowned child")
}

// After the child has left the OGS the family keeps read access and can change
// nothing (#2487). The feature flags already hide both buttons; these two pin
// the half that also holds for a direct API call.
func TestInviteRelatedAccount_AfterCareEndedIsRejected(t *testing.T) {
	t.Parallel()

	svc, invites, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	endCareFor(t, db, chain.StudentID)

	_, err := svc.InviteRelatedAccount(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, "new@example.test", "", "", false)
	require.Error(t, err)
	assert.True(t, errors.Is(err, parentService.ErrChildCareEnded))
	assert.Nil(t, invites.lastInvite, "no invite may go out for a departed child")
}

func TestRemoveRelatedAccount_AfterCareEndedIsRejected(t *testing.T) {
	t.Parallel()

	svc, invites, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, true)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	endCareFor(t, db, chain.StudentID)

	err := svc.RemoveRelatedAccount(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, chain.GuardianProfileID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, parentService.ErrChildCareEnded))
	assert.Nil(t, invites.lastRevoke, "no access may be revoked for a departed child")
}

func TestRemoveRelatedAccount_DisabledIsRejected(t *testing.T) {
	t.Parallel()

	svc, invites, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	err := svc.RemoveRelatedAccount(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, chain.GuardianProfileID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, parentService.ErrRemoveDisabled))
	assert.Nil(t, invites.lastRevoke, "revoke must not be called when removal is disabled")
}

func TestRemoveRelatedAccount_DisabledInviteModeRejectsStaleRemoveFlag(t *testing.T) {
	t.Parallel()

	svc, invites, db := buildRelAcctService(t, configModels.ParentInviteModeDisabled, true)

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	err := svc.RemoveRelatedAccount(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, chain.GuardianProfileID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, parentService.ErrRemoveDisabled))
	assert.Nil(t, invites.lastRevoke, "revoke must not be called when invite mode disables management")
}

func TestRemoveRelatedAccount_EnabledDelegatesAsParent(t *testing.T) {
	t.Parallel()

	svc, invites, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, true)

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	err := svc.RemoveRelatedAccount(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)
	require.NotNil(t, invites.lastRevoke)
	// The port revokes as a parent by contract, so the portal passes only
	// who acts and on which link; the root proves the ByParent flag
	// (TestParentGuardianAccessRevokesAsParent).
	assert.Equal(t, chain.AccountID, invites.lastRevoke.ActorAccountID)
	assert.Equal(t, chain.StudentID, invites.lastRevoke.StudentID)
	assert.Equal(t, chain.GuardianProfileID, invites.lastRevoke.GuardianProfileID)
}

func TestChildFeatures_ExposesRelatedAccountsFlags(t *testing.T) {
	t.Parallel()

	t.Run("invite enabled + remove on", func(t *testing.T) {
		svc, _, db := buildRelAcctService(t, configModels.ParentInviteModeStaffApproval, true)
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		flags, err := svc.ChildFeatures(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
		require.NoError(t, err)
		assert.True(t, flags.RelatedAccountsInviteEnabled, "non-disabled invite mode → invite enabled")
		assert.True(t, flags.RelatedAccountsRemoveEnabled)
	})

	t.Run("invite disabled + remove off", func(t *testing.T) {
		svc, _, db := buildRelAcctService(t, configModels.ParentInviteModeDisabled, false)
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		flags, err := svc.ChildFeatures(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
		require.NoError(t, err)
		assert.False(t, flags.RelatedAccountsInviteEnabled, "disabled mode → invite hidden")
		assert.False(t, flags.RelatedAccountsRemoveEnabled)
	})

	t.Run("invite disabled + stale remove on", func(t *testing.T) {
		svc, _, db := buildRelAcctService(t, configModels.ParentInviteModeDisabled, true)
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		flags, err := svc.ChildFeatures(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
		require.NoError(t, err)
		assert.False(t, flags.RelatedAccountsInviteEnabled)
		assert.False(t, flags.RelatedAccountsRemoveEnabled, "disabled invite mode must suppress stale remove flag")
	})
}

func buildRelAcctServiceWith(t *testing.T, settings configService.SettingsService) (parentService.Service, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StatusDayRepo:       repos.StudentStatusDay,
		StudentRepo:         repos.Student,
		Settings:            settings,
		MealPlan:            availableMealPlan(false),
		GuardianInvites:     &stubInvites{},
		StudentGuardianRepo: repos.StudentGuardian,
		GuardianProfileRepo: repos.GuardianProfile,
		DB:                  db,
		Logger:              slog.Default(),
	})
	return svc, db
}

func TestInviteRelatedAccount_EmptyEmailRejected(t *testing.T) {
	t.Parallel()

	svc, invites, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.InviteRelatedAccount(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, "   ", "", "", false)
	require.Error(t, err)
	assert.True(t, errors.Is(err, parentService.ErrEmailRequired))
	assert.Nil(t, invites.lastInvite)
}

func TestInviteRelatedAccount_InvalidEmailRejected(t *testing.T) {
	t.Parallel()

	svc, invites, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.InviteRelatedAccount(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, "not-an-email", "", "", false)
	require.Error(t, err)
	assert.True(t, errors.Is(err, parentService.ErrInvalidInviteInput))
	assert.Nil(t, invites.lastInvite)
}

func TestRelatedAccounts_SettingsErrorsAreSurfaced(t *testing.T) {
	t.Parallel()

	svc, db := buildRelAcctServiceWith(t, parentSettingsStub{
		boolErr:   errors.New("settings unavailable"),
		stringErr: errors.New("settings unavailable"),
	})
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.InviteRelatedAccount(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, "x@example.test", "", "", false)
	require.Error(t, err, "invite-mode resolve failure must surface")

	err = svc.RemoveRelatedAccount(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, chain.GuardianProfileID)
	require.Error(t, err, "can-remove resolve failure must surface")
}

// failingInvites errors on delegation so the parent service's error-return
// branches around the relative access port are exercised.
type failingInvites struct{}

func (failingInvites) InviteToStudent(_ context.Context, _ parentService.GuardianInviteRequest) (parentService.GuardianInviteOutcome, error) {
	return parentService.GuardianInviteOutcome{}, errors.New("invite failed")
}
func (failingInvites) RevokeAccess(_ context.Context, _ parentService.GuardianAccessRevocation) error {
	return errors.New("revoke failed")
}

func buildRelAcctServiceInvites(t *testing.T, inviteMode string, canRemove bool, invites parentService.GuardianAccess) (parentService.Service, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:     repos.ParentChild,
		StatusDayRepo: repos.StudentStatusDay,
		StudentRepo:   repos.Student,
		Settings: parentSettingsStub{
			boolValues:   map[string]bool{configModels.KeyGuardianParentCanRemove: canRemove},
			stringValues: map[string]string{configModels.KeyGuardianParentInviteMode: inviteMode},
		},
		GuardianInvites:     invites,
		GuardianInvitations: guardianInvitationReads{db: db},
		StudentGuardianRepo: repos.StudentGuardian,
		GuardianProfileRepo: repos.GuardianProfile,
		DB:                  db,
		Logger:              slog.Default(),
	})
	return svc, db
}

func TestListRelatedAccounts_UnownedChildErrors(t *testing.T) {
	t.Parallel()

	svc, _, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	other := testpkg.CreateTestStudent(t, db, "Not", "Owned", "9z")
	defer func() {
		_, _ = db.NewDelete().TableExpr("users.students").Where("id = ?", other.ID).Exec(testpkg.WithPackageTenantRuntime(context.Background()))
	}()

	_, err := svc.ListRelatedAccounts(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, other.ID)
	require.Error(t, err, "listing an unowned child must be rejected")
}

func TestRelatedAccounts_DelegateErrorsSurface(t *testing.T) {
	t.Parallel()

	t.Run("invite delegate error", func(t *testing.T) {
		svc, db := buildRelAcctServiceInvites(t, configModels.ParentInviteModeDirect, true, failingInvites{})
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		_, err := svc.InviteRelatedAccount(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, "x@example.test", "", "", false)
		require.Error(t, err)
	})

	t.Run("revoke delegate error", func(t *testing.T) {
		svc, db := buildRelAcctServiceInvites(t, configModels.ParentInviteModeDirect, true, failingInvites{})
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		err := svc.RemoveRelatedAccount(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, chain.GuardianProfileID)
		require.Error(t, err)
	})
}

func TestListRelatedAccounts_AccountWithoutAccessIsActiveNoAccess(t *testing.T) {
	t.Parallel()

	svc, _, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	ctx := testpkg.Ctx(t)
	profile := testpkg.CreateTestGuardianProfile(t, db, "active-no-access")
	_, account := testpkg.CreateTestPersonWithAccount(t, db, "NoAccess", "Account")
	defer func() {
		_, _ = db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(testpkg.WithPackageTenantRuntime(context.Background()))
	}()
	require.NoError(t, repos.GuardianProfile.LinkAccount(ctx, profile.ID, account.ID))
	// Restrictive contact role → empty permission set, no parent_portal.access.
	link := &userModels.StudentGuardian{
		StudentID:         chain.StudentID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "other",
		EmergencyPriority: 1,
	}
	authorize.ApplyStudentGuardianRole(link, authorize.GuardianRoleEmergency)
	link.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.StudentGuardian.Create(ctx, link))

	accounts, err := svc.ListRelatedAccounts(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)

	var found *parentService.RelatedAccount
	for _, account := range accounts {
		if account.GuardianProfileID == profile.ID {
			found = account
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, parentService.RelatedAccountActiveNoAccess, found.Status,
		"an account without parent_portal.access on this child must not report as plainly active")
}

func TestListRelatedAccounts_AccountWithoutAccessWithOpenInviteIsPending(t *testing.T) {
	t.Parallel()

	svc, _, db := buildRelAcctService(t, configModels.ParentInviteModeDirect, false)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	ctx := testpkg.Ctx(t)
	profile := testpkg.CreateTestGuardianProfile(t, db, "no-access-pending")
	_, account := testpkg.CreateTestPersonWithAccount(t, db, "NoAccessPending", "Account")
	defer func() {
		_, _ = db.NewDelete().TableExpr("auth.guardian_invitations").Where("guardian_profile_id = ?", profile.ID).Exec(testpkg.WithPackageTenantRuntime(context.Background()))
		_, _ = db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(testpkg.WithPackageTenantRuntime(context.Background()))
	}()
	require.NoError(t, repos.GuardianProfile.LinkAccount(ctx, profile.ID, account.ID))
	link := &userModels.StudentGuardian{
		StudentID:         chain.StudentID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "other",
		EmergencyPriority: 1,
	}
	authorize.ApplyStudentGuardianRole(link, authorize.GuardianRoleEmergency)
	link.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.StudentGuardian.Create(ctx, link))
	studentID := chain.StudentID
	invitation := &authModels.GuardianInvitation{
		Token:             fmt.Sprintf("no-access-pending-%d", time.Now().UnixNano()),
		GuardianProfileID: profile.ID,
		CreatedBy:         chain.AccountID,
		ExpiresAt:         time.Now().Add(time.Hour),
		StudentID:         &studentID,
		ApprovalStatus:    authModels.GuardianInvitationApprovalPending,
		RoleUpgrade:       true,
	}
	invitation.SetTenantID(testpkg.Tenant(t))
	testpkg.InsertTestGuardianInvitation(t, db, invitation)

	accounts, err := svc.ListRelatedAccounts(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)

	var found *parentService.RelatedAccount
	for _, account := range accounts {
		if account.GuardianProfileID == profile.ID {
			found = account
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, parentService.RelatedAccountPending, found.Status,
		"an in-flight upgrade approval must show as pending, not active_no_access")
}
