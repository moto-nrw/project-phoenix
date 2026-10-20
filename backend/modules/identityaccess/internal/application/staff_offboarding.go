package application

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Staff offboarding access step: the preview snapshots the account's access
// at this school and the execution revokes it against that snapshot, inside
// the workflow's transaction. Role removals and the account deactivation run
// through the retained role and account management (#2721) so their session
// revocation invariants hold.

// PreviewStaffOffboarding locks the account before roles, permissions, tokens
// or membership are touched, matching the login/refresh lock order.
func (l *AccountLifecycle) PreviewStaffOffboarding(ctx context.Context, accountID int64) (result identityaccess.StaffOffboardingPreview, err error) {
	if l == nil {
		return identityaccess.StaffOffboardingPreview{}, errAccountLifecycleUnavailable
	}
	defer func() { err = lifecycleError(err) }()
	ctx = bindRuntime(l.auth.attach, ctx)
	if accountID <= 0 || l.runtime.TenantID(ctx) <= 0 {
		return result, errors.New("account and tenant are required for staff offboarding")
	}
	err = l.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		result, err = l.staffOffboardingSnapshot(txCtx, accountID)
		return err
	})
	return result, err
}

func (l *AccountLifecycle) ExecuteStaffOffboarding(ctx context.Context, accountID int64, revision string) (result identityaccess.StaffOffboardingResult, err error) {
	if l == nil {
		return identityaccess.StaffOffboardingResult{}, errAccountLifecycleUnavailable
	}
	defer func() { err = lifecycleError(err) }()
	ctx = bindRuntime(l.auth.attach, ctx)
	if accountID <= 0 || l.runtime.TenantID(ctx) <= 0 || revision == "" {
		return result, errors.New("account, tenant and preview revision are required for staff offboarding")
	}
	err = l.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		return l.offboardStaffInTx(txCtx, accountID, revision, &result)
	})
	if err != nil {
		return identityaccess.StaffOffboardingResult{}, err
	}
	return result, nil
}

// offboardStaffInTx revokes the staff grants the preview names and closes
// the school mapping inside the caller's transaction.
func (l *AccountLifecycle) offboardStaffInTx(txCtx context.Context, accountID int64, revision string, result *identityaccess.StaffOffboardingResult) error {
	preview, proceed, err := l.requireStaffOffboarding(txCtx, accountID, revision, result)
	if err != nil || !proceed {
		return err
	}
	*result, err = l.revokeStaffGrants(txCtx, accountID, preview)
	if err != nil {
		return err
	}
	tenantID := l.runtime.TenantID(txCtx)
	if preview.PreserveGuardian {
		return nil
	}
	if preview.DeactivateAccount {
		// Deactivation records the durable account-wide revocation intent
		// in this transaction; the existing worker retries post-commit cleanup.
		if err := l.admin.DeactivateAccount(txCtx, accountID); err != nil {
			return err
		}
		result.AccountDeactivated = true
	}
	_, err = l.store.DeactivateTenantMapping(txCtx, accountID, tenantID)
	return err
}

func (l *AccountLifecycle) staffOffboardingSnapshot(ctx context.Context, accountID int64) (identityaccess.StaffOffboardingPreview, error) {
	preview := identityaccess.StaffOffboardingPreview{AccountID: accountID}
	tenantID := l.runtime.TenantID(ctx)
	account, found, _, err := findLoginAccount(ctx, l.runtime, l.logins, accountID, true)
	if err != nil {
		return preview, err
	}
	if !found {
		return preview, domain.ErrAccountNotFound
	}
	active, _, err := l.logins.HasActiveAccountTenant(ctx, accountID, tenantID)
	if err != nil {
		return preview, err
	}
	preview.ActiveMembership = active
	if !active {
		preview.Revision = "inactive"
		return preview, nil
	}
	roles, _, err := l.logins.ListAccountRolesAtTenant(ctx, accountID, tenantID, false)
	if err != nil {
		return preview, err
	}
	for _, role := range roles {
		if strings.EqualFold(role.Name, domain.GuardianRoleName) {
			preview.PreserveGuardian = true
		} else {
			preview.RoleIDs = append(preview.RoleIDs, role.RoleID)
		}
	}
	slices.Sort(preview.RoleIDs)
	permissions, _, err := l.store.ListAccountPermissionGrants(ctx, accountID, tenantID)
	if err != nil {
		return preview, err
	}
	preview.Permissions = int64(len(permissions))
	tokenIDs, err := l.auth.ListSessionIDs(ctx, accountID)
	if err != nil {
		return preview, err
	}
	preview.Tokens = int64(len(tokenIDs))
	activeTenants, _, err := l.logins.ListActiveTenantIDs(ctx, accountID)
	if err != nil {
		return preview, err
	}
	preview.DeactivateAccount = !preview.PreserveGuardian && len(activeTenants) == 1 && activeTenants[0] == tenantID
	preview.Revision, err = staffOffboardingRevision(accountID, tenantID, account.UpdatedAt, preview, permissions, tokenIDs, activeTenants)
	return preview, err
}

func (l *AccountLifecycle) revokeStaffGrants(txCtx context.Context, accountID int64, preview identityaccess.StaffOffboardingPreview) (result identityaccess.StaffOffboardingResult, err error) {
	for _, roleID := range preview.RoleIDs {
		if err := l.admin.RemoveRoleFromAccount(txCtx, accountID, roleID); err != nil {
			return result, err
		}
	}
	tenantID := l.runtime.TenantID(txCtx)
	result.PermissionsRevoked, _, err = l.store.DeleteAccountPermissionGrants(txCtx, accountID, tenantID)
	if err != nil {
		return result, err
	}
	if result.PermissionsRevoked != preview.Permissions {
		return result, identityaccess.ErrStaffOffboardingConflict
	}
	// Revoke even when direct permissions were the only staff grants. Role
	// removal already revoked any tokens it encountered in this transaction.
	if err := l.auth.RevokeAllTokensWithReason(txCtx, accountID, "role_changed"); err != nil {
		return result, err
	}
	result.RolesRevoked, result.TokensRevoked = int64(len(preview.RoleIDs)), preview.Tokens
	result.GuardianAccessPreserved = preview.PreserveGuardian
	return result, nil
}

func staffOffboardingRevision(accountID, tenantID int64, updatedAt time.Time, preview identityaccess.StaffOffboardingPreview, permissions []domain.PermissionGrant, tokenIDs, activeTenants []int64) (string, error) {
	type permissionVersion struct {
		ID, PermissionID int64
		Granted          bool
	}
	versions := struct {
		AccountID, TenantID          int64
		AccountUpdatedAt             time.Time
		Roles, Tokens, ActiveTenants []int64
		Permissions                  []permissionVersion
		Guardian                     bool
	}{AccountID: accountID, TenantID: tenantID, AccountUpdatedAt: updatedAt, Roles: preview.RoleIDs, Guardian: preview.PreserveGuardian}
	for _, permission := range permissions {
		versions.Permissions = append(versions.Permissions, permissionVersion{permission.ID, permission.PermissionID, permission.Granted})
	}
	slices.SortFunc(versions.Permissions, func(a, b permissionVersion) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	versions.Tokens = append(versions.Tokens, tokenIDs...)
	versions.ActiveTenants = append(versions.ActiveTenants, activeTenants...)
	slices.Sort(versions.Tokens)
	slices.Sort(versions.ActiveTenants)
	payload, err := json.Marshal(versions)
	return string(payload), err
}

func (l *AccountLifecycle) requireStaffOffboarding(txCtx context.Context, accountID int64, revision string, result *identityaccess.StaffOffboardingResult) (identityaccess.StaffOffboardingPreview, bool, error) {
	if !l.runtime.HasAfterCommitHooks(txCtx) {
		return identityaccess.StaffOffboardingPreview{}, false, errors.New("commit hooks are required for staff offboarding cleanup")
	}
	preview, err := l.staffOffboardingSnapshot(txCtx, accountID)
	if err != nil {
		return identityaccess.StaffOffboardingPreview{}, false, err
	}
	if !preview.ActiveMembership {
		return preview, false, nil
	}
	if preview.PreserveGuardian && len(preview.RoleIDs) == 0 && preview.Permissions == 0 && preview.Tokens == 0 {
		result.GuardianAccessPreserved = true
		return preview, false, nil
	}
	if preview.Revision != revision {
		return preview, false, identityaccess.ErrStaffOffboardingConflict
	}
	return preview, true, nil
}

// HasLiveCaregiverProfile reports whether the account's identity at the
// school in context carries a live caregiver profile.
func (l *AccountLifecycle) HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error) {
	if l == nil {
		return false, errAccountLifecycleUnavailable
	}
	hasProfile, err := l.profiles.HasLiveCaregiverProfile(bindRuntime(l.auth.attach, ctx), accountID)
	return hasProfile, lifecycleError(err)
}
