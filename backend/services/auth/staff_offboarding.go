package auth

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
)

var ErrStaffOffboardingConflict = errors.New("account access changed since offboarding preview")

// StaffOffboardingPreview contains no credential material. The revision is an
// exact encoding of the access decisions, not an authorization credential.
type StaffOffboardingPreview struct {
	AccountID         int64
	ActiveMembership  bool
	RoleIDs           []int64
	Permissions       int64
	Tokens            int64
	PreserveGuardian  bool
	DeactivateAccount bool
	Revision          string
}

type StaffOffboardingResult struct {
	RolesRevoked            int64
	PermissionsRevoked      int64
	TokensRevoked           int64
	GuardianAccessPreserved bool
	AccountDeactivated      bool
}

// PreviewStaffOffboarding locks the account before roles, permissions, tokens
// or membership are touched, matching the login/refresh lock order.
func (s *Service) PreviewStaffOffboarding(ctx context.Context, accountID int64) (result StaffOffboardingPreview, err error) {
	if accountID <= 0 || tenant.FromContext(ctx) <= 0 {
		return result, errors.New("account and tenant are required for staff offboarding")
	}
	err = s.runInTx(ctx, func(txCtx context.Context) error {
		result, err = s.staffOffboardingSnapshot(txCtx, accountID)
		return err
	})
	return result, err
}

func (s *Service) ExecuteStaffOffboarding(ctx context.Context, accountID int64, revision string) (result StaffOffboardingResult, err error) {
	if accountID <= 0 || tenant.FromContext(ctx) <= 0 || revision == "" {
		return result, errors.New("account, tenant and preview revision are required for staff offboarding")
	}
	err = s.runInTx(ctx, func(txCtx context.Context) error {
		if !tenant.HasAfterCommitHooks(txCtx) {
			return errors.New("commit hooks are required for staff offboarding cleanup")
		}
		preview, err := s.staffOffboardingSnapshot(txCtx, accountID)
		if err != nil {
			return err
		}
		if !preview.ActiveMembership {
			return nil
		}
		if preview.PreserveGuardian && len(preview.RoleIDs) == 0 && preview.Permissions == 0 && preview.Tokens == 0 {
			result.GuardianAccessPreserved = true
			return nil
		}
		if preview.Revision != revision {
			return ErrStaffOffboardingConflict
		}
		for _, roleID := range preview.RoleIDs {
			if err := s.RemoveRoleFromAccount(txCtx, int(accountID), int(roleID)); err != nil {
				return err
			}
		}
		result.PermissionsRevoked, err = s.repos.AccountPermission.DeleteByAccountID(txCtx, accountID)
		if err != nil {
			return err
		}
		if result.PermissionsRevoked != preview.Permissions {
			return ErrStaffOffboardingConflict
		}
		// Revoke even when direct permissions were the only staff grants. Role
		// removal already revoked any tokens it encountered in this transaction.
		if err := s.RevokeAllTokensWithReason(txCtx, int(accountID), "role_changed"); err != nil {
			return err
		}
		result.RolesRevoked, result.TokensRevoked = int64(len(preview.RoleIDs)), preview.Tokens
		result.GuardianAccessPreserved = preview.PreserveGuardian
		if preview.PreserveGuardian {
			return nil
		}
		if preview.DeactivateAccount {
			// Deactivation records the durable account-wide revocation intent
			// in this transaction; the existing worker retries post-commit cleanup.
			if err := s.DeactivateAccount(txCtx, int(accountID)); err != nil {
				return err
			}
			result.AccountDeactivated = true
		}
		return s.repos.AccountTenant.Deactivate(txCtx, accountID, tenant.FromContext(txCtx))
	})
	if err != nil {
		return StaffOffboardingResult{}, err
	}
	return result, nil
}

func (s *Service) staffOffboardingSnapshot(ctx context.Context, accountID int64) (StaffOffboardingPreview, error) {
	preview := StaffOffboardingPreview{AccountID: accountID}
	account, err := s.repos.Account.FindByIDForUpdate(ctx, accountID)
	if err != nil {
		return preview, err
	}
	active, err := s.repos.AccountTenant.ExistsByAccountAndTenant(ctx, accountID, tenant.FromContext(ctx))
	if err != nil {
		return preview, err
	}
	preview.ActiveMembership = active
	if !active {
		preview.Revision = "inactive"
		return preview, nil
	}
	roles, err := s.repos.Role.FindByAccountID(ctx, accountID)
	if err != nil {
		return preview, err
	}
	for _, role := range roles {
		if strings.EqualFold(role.Name, authModels.BaseRoleGuardian) {
			preview.PreserveGuardian = true
		} else {
			preview.RoleIDs = append(preview.RoleIDs, role.ID)
		}
	}
	slices.Sort(preview.RoleIDs)
	permissions, err := s.repos.AccountPermission.FindByAccountID(ctx, accountID)
	if err != nil {
		return preview, err
	}
	preview.Permissions = int64(len(permissions))
	tokens, err := s.repos.Token.FindByAccountID(ctx, accountID)
	if err != nil {
		return preview, err
	}
	preview.Tokens = int64(len(tokens))
	mappings, err := s.repos.AccountTenant.FindActiveByAccountID(ctx, accountID)
	if err != nil {
		return preview, err
	}
	preview.DeactivateAccount = !preview.PreserveGuardian && len(mappings) == 1 && mappings[0].TenantID == tenant.FromContext(ctx)
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
	}{AccountID: accountID, TenantID: tenant.FromContext(ctx), AccountUpdatedAt: account.UpdatedAt, Roles: preview.RoleIDs, Guardian: preview.PreserveGuardian}
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
	for _, token := range tokens {
		versions.Tokens = append(versions.Tokens, token.ID)
	}
	for _, mapping := range mappings {
		versions.ActiveTenants = append(versions.ActiveTenants, mapping.TenantID)
	}
	slices.Sort(versions.Tokens)
	slices.Sort(versions.ActiveTenants)
	payload, err := json.Marshal(versions)
	preview.Revision = string(payload)
	return preview, err
}
