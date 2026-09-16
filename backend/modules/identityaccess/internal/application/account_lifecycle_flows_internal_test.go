package application

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- parent accounts --------------------------------------------------------

func TestParentAccounts(t *testing.T) {
	t.Parallel()

	t.Run("create normalizes, hashes and stamps the tenant", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		created, err := f.lifecycle.CreateParentAccount(tenantContext(), " Parent@Example.TEST ", " eltern ", "Str0ngPass!")
		require.NoError(t, err)
		assert.Equal(t, "parent@example.test", created.Email)
		assert.Equal(t, "eltern", created.Username)
		assert.True(t, created.Active)
		assert.Equal(t, lifecycleTenant, created.TenantID)
		assert.Equal(t, "hash:Str0ngPass!", f.store.parents[created.ID].PasswordHash)
	})

	t.Run("create refuses weak passwords and taken identities", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		_, err := f.lifecycle.CreateParentAccount(tenantContext(), "a@example.test", "a", "short")
		require.Error(t, err)

		_, err = f.lifecycle.CreateParentAccount(tenantContext(), "a@example.test", "a", "Str0ngPass!")
		require.NoError(t, err)
		_, err = f.lifecycle.CreateParentAccount(tenantContext(), "A@example.test", "b", "Str0ngPass!")
		require.ErrorIs(t, err, domain.ErrEmailAlreadyExists)
		_, err = f.lifecycle.CreateParentAccount(tenantContext(), "b@example.test", "A", "Str0ngPass!")
		require.ErrorIs(t, err, domain.ErrUsernameAlreadyExists)
	})

	t.Run("lookups report a missing account", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		_, err := f.lifecycle.GetParentAccountByID(tenantContext(), 404)
		require.ErrorIs(t, err, domain.ErrParentAccountNotFound)
		_, err = f.lifecycle.GetParentAccountByEmail(tenantContext(), "nobody@example.test")
		require.ErrorIs(t, err, domain.ErrParentAccountNotFound)
		require.ErrorIs(t, f.lifecycle.UpdateParentAccount(tenantContext(), domain.ParentAccount{ID: 404}), domain.ErrParentAccountNotFound)
		require.ErrorIs(t, f.lifecycle.ActivateParentAccount(tenantContext(), 404), domain.ErrParentAccountNotFound)
		require.ErrorIs(t, f.lifecycle.DeactivateParentAccount(tenantContext(), 404), domain.ErrParentAccountNotFound)
	})

	t.Run("update keeps the credential and activation toggles", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		created, err := f.lifecycle.CreateParentAccount(tenantContext(), "a@example.test", "a", "Str0ngPass!")
		require.NoError(t, err)

		require.NoError(t, f.lifecycle.UpdateParentAccount(tenantContext(), domain.ParentAccount{ID: created.ID, Email: "new@example.test", Username: "neu", Active: true}))
		stored := f.store.parents[created.ID]
		assert.Equal(t, "new@example.test", stored.Email)
		assert.Equal(t, "hash:Str0ngPass!", stored.PasswordHash, "an update never clears the credential")

		require.NoError(t, f.lifecycle.DeactivateParentAccount(tenantContext(), created.ID))
		assert.False(t, f.store.parents[created.ID].Active)
		inactive := false
		listed, err := f.lifecycle.ListParentAccounts(tenantContext(), domain.ParentAccountFilter{Active: &inactive})
		require.NoError(t, err)
		require.Len(t, listed, 1)
		require.NoError(t, f.lifecycle.ActivateParentAccount(tenantContext(), created.ID))
		assert.True(t, f.store.parents[created.ID].Active)

		byEmail, err := f.lifecycle.GetParentAccountByEmail(tenantContext(), " NEW@example.test ")
		require.NoError(t, err)
		assert.Equal(t, created.ID, byEmail.ID)
	})
}

// --- staff preview ---------------------------------------------------------

const (
	previewAdmin  int64 = 11
	previewTarget int64 = 20
)

func userRole() domain.RoleAssignment {
	return domain.RoleAssignment{RoleID: 101, Name: "user", IsSystem: true}
}

func guardianRole() domain.RoleAssignment {
	return domain.RoleAssignment{RoleID: 102, Name: "guardian", IsSystem: true}
}

func (f *lifecycleFixture) seedAccount(id int64, roles ...domain.RoleAssignment) {
	f.store.addAccount(id, fmt.Sprintf("account%d@example.test", id), "hash:secret", true)
	f.store.addMapping(id, lifecycleTenant, roles...)
}

func TestStartStaffPreviewMintsAReadOnlyTokenAndOneStartEvent(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.seedAccount(previewTarget, userRole())
	f.store.permissions[mappingKey{previewTarget, lifecycleTenant}] = []string{"groups:read"}

	session, err := f.lifecycle.StartStaffPreview(context.Background(), previewAdmin, lifecycleTenant, previewTarget, "", "127.0.0.1", "go-test")
	require.NoError(t, err)
	assert.Equal(t, previewTarget, session.TargetAccountID)
	assert.EqualValues(t, 900, session.ExpiresIn)
	assert.Equal(t, "user20", session.TargetName, "without a person the username names the target")

	claims := decodeAccessToken(t, session.AccessToken)
	assert.Equal(t, previewTarget, claims.AccountID)
	assert.True(t, claims.ReadOnly)
	assert.Equal(t, previewAdmin, claims.ActingAdminID)
	assert.Equal(t, lifecycleTenant, claims.TenantID)
	assert.Equal(t, []string{"user"}, claims.Roles)
	assert.Equal(t, []string{"groups:read"}, claims.Permissions)
	assert.Len(t, claims.PreviewID, 32)
	assert.Empty(t, claims.FamilyID, "a preview never carries a refresh family")

	require.Len(t, f.audit.started, 1)
	assert.Equal(t, domain.StaffPreviewEvent{
		AdminAccountID: previewAdmin, TenantID: lifecycleTenant, TargetAccountID: previewTarget,
		PreviewID: claims.PreviewID, IPAddress: "127.0.0.1", UserAgent: "go-test",
	}, f.audit.started[0])
	assert.Contains(t, f.store.calls, "FindLoginAccount(forUpdate)")
	assert.Contains(t, f.store.calls, "LockActiveTenantMappingShared")
	assert.Contains(t, f.store.calls, "ListAccountRolesAtTenant(forShare)")
	assert.Contains(t, f.store.calls, "LockAccountPermissionSources")
}

func TestStartStaffPreviewRefusesTargetsWithoutATenantPortal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		arrange  func(f *lifecycleFixture)
		tenantID int64
		target   int64
		want     error
	}{
		{name: "no school", tenantID: 0, target: previewTarget, want: domain.ErrTenantNotFound},
		{name: "self", tenantID: lifecycleTenant, target: previewAdmin, want: domain.ErrPreviewSelf},
		{name: "unknown account", tenantID: lifecycleTenant, target: 404, want: domain.ErrAccountNotFound},
		{name: "inactive account", tenantID: lifecycleTenant, target: previewTarget, want: domain.ErrAccountInactive, arrange: func(f *lifecycleFixture) {
			account := f.store.accounts[previewTarget]
			account.Active = false
			f.store.accounts[previewTarget] = account
		}},
		{name: "no membership", tenantID: lifecycleTenant, target: previewTarget, want: domain.ErrTenantAccessDenied, arrange: func(f *lifecycleFixture) {
			f.store.inactive[mappingKey{previewTarget, lifecycleTenant}] = true
		}},
		{name: "no role", tenantID: lifecycleTenant, target: previewTarget, want: domain.ErrPreviewTargetNotStaff, arrange: func(f *lifecycleFixture) {
			delete(f.store.roles, mappingKey{previewTarget, lifecycleTenant})
		}},
		{name: "guardian only", tenantID: lifecycleTenant, target: previewTarget, want: domain.ErrPreviewTargetNotStaff, arrange: func(f *lifecycleFixture) {
			f.store.roles[mappingKey{previewTarget, lifecycleTenant}] = []domain.RoleAssignment{guardianRole()}
		}},
		{name: "lehrkraft only", tenantID: lifecycleTenant, target: previewTarget, want: domain.ErrMustUseSchoolPortal, arrange: func(f *lifecycleFixture) {
			f.store.roles[mappingKey{previewTarget, lifecycleTenant}] = []domain.RoleAssignment{lehrkraftRole()}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newLifecycleFixture(t)
			f.seedAccount(previewTarget, userRole())
			if tc.arrange != nil {
				tc.arrange(f)
			}
			_, err := f.lifecycle.StartStaffPreview(context.Background(), previewAdmin, tc.tenantID, tc.target, "", "", "")
			require.ErrorIs(t, err, tc.want)
			var operation *OperationError
			require.ErrorAs(t, err, &operation)
			assert.Equal(t, "start staff preview", operation.Op)
			assert.Empty(t, f.audit.started, "a refused preview writes no start event")
		})
	}
}

func TestStartStaffPreviewAcceptsASchoolRoleNamedLehrkraft(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	tenantID := lifecycleTenant
	f.seedAccount(previewTarget, domain.RoleAssignment{RoleID: 103, Name: "Lehrkraft", TenantID: &tenantID})

	_, err := f.lifecycle.StartStaffPreview(context.Background(), previewAdmin, lifecycleTenant, previewTarget, "", "", "")
	require.NoError(t, err)
}

// A renewal continues the running preview: same id, no second start event.
// A token of an ended preview, or one minted for somebody else, starts over.
func TestStartStaffPreviewRemint(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.seedAccount(previewTarget, userRole())
	f.seedAccount(previewTarget+1, userRole())
	ctx := context.Background()

	first, err := f.lifecycle.StartStaffPreview(ctx, previewAdmin, lifecycleTenant, previewTarget, "", "", "")
	require.NoError(t, err)
	firstID := decodeAccessToken(t, first.AccessToken).PreviewID

	renewed, err := f.lifecycle.StartStaffPreview(ctx, previewAdmin, lifecycleTenant, previewTarget, first.AccessToken, "", "")
	require.NoError(t, err)
	assert.Equal(t, firstID, decodeAccessToken(t, renewed.AccessToken).PreviewID)
	assert.Len(t, f.audit.started, 1, "a renewal is not a start")
	assert.Contains(t, f.audit.locks, "11:"+firstID, "the renewal holds the preview lock")

	for name, token := range map[string]string{
		"garbage":         "not-a-token",
		"other admin":     renewed.AccessToken,
		"other target":    renewed.AccessToken,
		"not read-only":   lifecycleCodec{}.encode("access:", domain.SessionClaims{AccountID: previewTarget, TenantID: lifecycleTenant, ActingAdminID: previewAdmin, PreviewID: firstID}),
		"missing preview": lifecycleCodec{}.encode("access:", domain.SessionClaims{AccountID: previewTarget, TenantID: lifecycleTenant, ActingAdminID: previewAdmin, ReadOnly: true}),
	} {
		admin, target := previewAdmin, previewTarget
		switch name {
		case "other admin":
			admin = previewAdmin + 1
		case "other target":
			target = previewTarget + 1
		}
		fresh, err := f.lifecycle.StartStaffPreview(ctx, admin, lifecycleTenant, target, token, "", "")
		require.NoError(t, err, name)
		assert.NotEqual(t, firstID, decodeAccessToken(t, fresh.AccessToken).PreviewID, name)
	}

	_, err = f.lifecycle.EndStaffPreview(ctx, renewed.AccessToken, "", "")
	require.NoError(t, err)
	started := len(f.audit.started)
	after, err := f.lifecycle.StartStaffPreview(ctx, previewAdmin, lifecycleTenant, previewTarget, renewed.AccessToken, "", "")
	require.NoError(t, err)
	assert.NotEqual(t, firstID, decodeAccessToken(t, after.AccessToken).PreviewID, "an ended preview is never revived")
	assert.Len(t, f.audit.started, started+1)
}

func TestEndStaffPreview(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.seedAccount(previewTarget, userRole())
	ctx := context.Background()

	for _, token := range []string{
		"not-a-token",
		lifecycleCodec{}.encode("access:", domain.SessionClaims{AccountID: previewTarget, TenantID: lifecycleTenant, ActingAdminID: previewAdmin, PreviewID: "x"}),
		lifecycleCodec{}.encode("access:", domain.SessionClaims{AccountID: previewTarget, TenantID: lifecycleTenant, ReadOnly: true, PreviewID: "x"}),
		lifecycleCodec{}.encode("access:", domain.SessionClaims{AccountID: previewTarget, ActingAdminID: previewAdmin, ReadOnly: true, PreviewID: "x"}),
		lifecycleCodec{}.encode("access:", domain.SessionClaims{TenantID: lifecycleTenant, ActingAdminID: previewAdmin, ReadOnly: true, PreviewID: "x"}),
		lifecycleCodec{}.encode("access:", domain.SessionClaims{AccountID: previewTarget, TenantID: lifecycleTenant, ActingAdminID: previewAdmin, ReadOnly: true}),
	} {
		_, err := f.lifecycle.EndStaffPreview(ctx, token, "", "")
		require.ErrorIs(t, err, domain.ErrPreviewTokenInvalid, token)
	}
	assert.Empty(t, f.audit.ended)

	session, err := f.lifecycle.StartStaffPreview(ctx, previewAdmin, lifecycleTenant, previewTarget, "", "", "")
	require.NoError(t, err)
	previewID := decodeAccessToken(t, session.AccessToken).PreviewID

	target, err := f.lifecycle.EndStaffPreview(ctx, session.AccessToken, "10.0.0.1", "browser")
	require.NoError(t, err)
	assert.Equal(t, previewTarget, target)
	target, err = f.lifecycle.EndStaffPreview(ctx, session.AccessToken, "10.0.0.1", "browser")
	require.NoError(t, err, "a replayed end is not an error")
	assert.Equal(t, previewTarget, target)

	require.Len(t, f.audit.ended, 1, "ending is one-shot per preview")
	assert.Equal(t, domain.StaffPreviewEvent{
		AdminAccountID: previewAdmin, TenantID: lifecycleTenant, TargetAccountID: previewTarget,
		PreviewID: previewID, IPAddress: "10.0.0.1", UserAgent: "browser",
	}, f.audit.ended[0])
}

func TestListStaffPreviewCandidates(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	tenantID := lifecycleTenant
	f.seedAccount(previewAdmin, domain.RoleAssignment{RoleID: 104, Name: "admin", IsSystem: true})
	f.seedAccount(previewTarget, userRole())
	f.seedAccount(21, guardianRole())
	f.seedAccount(22, lehrkraftRole())
	f.seedAccount(23, domain.RoleAssignment{RoleID: 103, Name: "Lehrkraft", TenantID: &tenantID})
	f.seedAccount(24)
	f.seedAccount(25, userRole())
	f.store.inactive[mappingKey{25, lifecycleTenant}] = true
	f.seedAccount(26, userRole())
	inactive := f.store.accounts[26]
	inactive.Active = false
	f.store.accounts[26] = inactive
	target := previewTarget
	f.staff.addPerson(domain.PersonRecord{FirstName: "Wählbar", LastName: "Betreuung", AccountID: &target})

	candidates, err := f.lifecycle.ListStaffPreviewCandidates(tenantContext(), lifecycleTenant, previewAdmin)
	require.NoError(t, err)

	ids := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.AccountID)
	}
	assert.Equal(t, []int64{previewTarget, 23}, ids,
		"the caller, guardian-only, Lehrkraft-only, roleless and inactive accounts are not previewable")
	assert.Equal(t, "Wählbar", candidates[0].FirstName)
	assert.Equal(t, "Betreuung", candidates[0].LastName)
	assert.Equal(t, []string{"user"}, candidates[0].Roles)
}

// --- staff offboarding -------------------------------------------------------

const offboardedAccount int64 = 30

func (f *lifecycleFixture) seedOffboarding(roles ...domain.RoleAssignment) {
	f.seedAccount(offboardedAccount, roles...)
	f.store.grants[mappingKey{offboardedAccount, lifecycleTenant}] = []domain.PermissionGrant{{ID: 2, PermissionID: 8, Granted: true}}
	f.sessions.sessions[70] = domain.AccountSession{ID: 70, AccountID: offboardedAccount, TenantID: lifecycleTenant, Token: "t70", FamilyID: "f70", PortalScope: "tenant"}
	f.sessions.nextSessionID = 70
}

func (f *lifecycleFixture) inTenantTx(t *testing.T, fn func(ctx context.Context) error) error {
	t.Helper()
	return f.runtime.WithTenantTx(tenantContext(), lifecycleTenant, fn)
}

func TestStaffOffboardingRequiresAccountTenantAndRevision(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)

	_, err := f.lifecycle.PreviewStaffOffboarding(context.Background(), offboardedAccount)
	require.Error(t, err)
	_, err = f.lifecycle.PreviewStaffOffboarding(tenantContext(), 0)
	require.Error(t, err)
	_, err = f.lifecycle.ExecuteStaffOffboarding(tenantContext(), offboardedAccount, "")
	require.Error(t, err)
}

func TestStaffOffboardingSnapshotAndExecution(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.seedOffboarding(userRole(), domain.RoleAssignment{RoleID: 104, Name: "admin", IsSystem: true})

	preview, err := f.lifecycle.PreviewStaffOffboarding(tenantContext(), offboardedAccount)
	require.NoError(t, err)
	assert.True(t, preview.ActiveMembership)
	assert.Equal(t, []int64{101, 104}, preview.RoleIDs, "role ids are sorted for the revision")
	assert.EqualValues(t, 1, preview.Permissions)
	assert.EqualValues(t, 1, preview.Tokens)
	assert.False(t, preview.PreserveGuardian)
	assert.True(t, preview.DeactivateAccount, "the account's only school deactivates it")
	assert.NotEmpty(t, preview.Revision)
	assert.Contains(t, f.store.calls, "FindLoginAccount(forUpdate)", "the account is locked before its access is read")

	again, err := f.lifecycle.PreviewStaffOffboarding(tenantContext(), offboardedAccount)
	require.NoError(t, err)
	assert.Equal(t, preview.Revision, again.Revision, "the revision is deterministic")

	err = f.inTenantTx(t, func(ctx context.Context) error {
		result, executeErr := f.lifecycle.ExecuteStaffOffboarding(ctx, offboardedAccount, preview.Revision)
		require.NoError(t, executeErr)
		assert.Equal(t, domain.StaffOffboardingResult{RolesRevoked: 2, PermissionsRevoked: 1, TokensRevoked: 1, AccountDeactivated: true}, result)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []int64{101, 104}, f.admin.removed)
	assert.Equal(t, []int64{offboardedAccount}, f.admin.deactivated)
	assert.Empty(t, f.store.grants[mappingKey{offboardedAccount, lifecycleTenant}])
	assert.Empty(t, f.sessions.sessionsOf(offboardedAccount))
	assert.True(t, f.store.inactive[mappingKey{offboardedAccount, lifecycleTenant}])

	err = f.inTenantTx(t, func(ctx context.Context) error {
		result, executeErr := f.lifecycle.ExecuteStaffOffboarding(ctx, offboardedAccount, preview.Revision)
		require.NoError(t, executeErr, "an offboarded account is a no-op, not a conflict")
		assert.Zero(t, result)
		return nil
	})
	require.NoError(t, err)
}

func TestStaffOffboardingRefusesAChangedSnapshot(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.seedOffboarding(userRole())

	preview, err := f.lifecycle.PreviewStaffOffboarding(tenantContext(), offboardedAccount)
	require.NoError(t, err)
	f.store.grants[mappingKey{offboardedAccount, lifecycleTenant}] = append(f.store.grants[mappingKey{offboardedAccount, lifecycleTenant}],
		domain.PermissionGrant{ID: 3, PermissionID: 9, Granted: true})

	err = f.inTenantTx(t, func(ctx context.Context) error {
		_, executeErr := f.lifecycle.ExecuteStaffOffboarding(ctx, offboardedAccount, preview.Revision)
		return executeErr
	})
	require.ErrorIs(t, err, domain.ErrStaffOffboardingConflict)
	assert.Empty(t, f.admin.removed, "nothing is revoked against a stale preview")
	assert.False(t, f.store.inactive[mappingKey{offboardedAccount, lifecycleTenant}])
}

func TestStaffOffboardingNeedsCommitHooks(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.seedOffboarding(userRole())
	preview, err := f.lifecycle.PreviewStaffOffboarding(tenantContext(), offboardedAccount)
	require.NoError(t, err)

	ctx := context.WithValue(tenantContext(), fakeTxKey{}, true)
	_, err = f.lifecycle.ExecuteStaffOffboarding(ctx, offboardedAccount, preview.Revision)
	require.ErrorContains(t, err, "commit hooks are required")
	assert.Empty(t, f.admin.removed)
}

// Parent-portal access is a separate relationship: offboarding a staff
// member who is also a guardian keeps the guardian role and the membership.
func TestStaffOffboardingPreservesGuardianAccess(t *testing.T) {
	t.Parallel()

	t.Run("staff roles go, guardian access stays", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		f.seedOffboarding(userRole(), guardianRole())

		preview, err := f.lifecycle.PreviewStaffOffboarding(tenantContext(), offboardedAccount)
		require.NoError(t, err)
		assert.True(t, preview.PreserveGuardian)
		assert.False(t, preview.DeactivateAccount)
		assert.Equal(t, []int64{101}, preview.RoleIDs)

		err = f.inTenantTx(t, func(ctx context.Context) error {
			result, executeErr := f.lifecycle.ExecuteStaffOffboarding(ctx, offboardedAccount, preview.Revision)
			require.NoError(t, executeErr)
			assert.True(t, result.GuardianAccessPreserved)
			assert.False(t, result.AccountDeactivated)
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, []int64{101}, f.admin.removed)
		assert.Empty(t, f.admin.deactivated)
		assert.False(t, f.store.inactive[mappingKey{offboardedAccount, lifecycleTenant}])
	})

	t.Run("a guardian without staff access is untouched", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		f.seedAccount(offboardedAccount, guardianRole())

		err := f.inTenantTx(t, func(ctx context.Context) error {
			result, executeErr := f.lifecycle.ExecuteStaffOffboarding(ctx, offboardedAccount, "stale")
			require.NoError(t, executeErr, "nothing to revoke needs no matching revision")
			assert.Equal(t, domain.StaffOffboardingResult{GuardianAccessPreserved: true}, result)
			return nil
		})
		require.NoError(t, err)
		assert.Empty(t, f.admin.removed)
	})
}

func TestStaffOffboardingKeepsAccountsOfOtherSchools(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.seedOffboarding(userRole())
	f.store.addMapping(offboardedAccount, lifecycleTenant+1, userRole())

	preview, err := f.lifecycle.PreviewStaffOffboarding(tenantContext(), offboardedAccount)
	require.NoError(t, err)
	assert.False(t, preview.DeactivateAccount)

	err = f.inTenantTx(t, func(ctx context.Context) error {
		result, executeErr := f.lifecycle.ExecuteStaffOffboarding(ctx, offboardedAccount, preview.Revision)
		require.NoError(t, executeErr)
		assert.False(t, result.AccountDeactivated)
		return nil
	})
	require.NoError(t, err)
	assert.Empty(t, f.admin.deactivated)
	assert.True(t, f.store.inactive[mappingKey{offboardedAccount, lifecycleTenant}])
	assert.False(t, f.store.inactive[mappingKey{offboardedAccount, lifecycleTenant + 1}])
}
