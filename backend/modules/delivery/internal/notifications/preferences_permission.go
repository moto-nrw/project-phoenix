package notifications

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// AccountPermissions reads the effective permission names of accounts in one
// school. Identity & Access answers it.
type AccountPermissions interface {
	FindEffectivePermissionNamesByAccountIDsForTenant(ctx context.Context, accountIDs []int64, tenantID int64) (map[int64][]string, error)
}

// PreferenceOption configures optional collaborators of the preference
// service.
type PreferenceOption = func(*preferenceService)

// WithAccountPermissions lets the preference service offer the push types
// that need a permission to its holders (#4791).
func WithAccountPermissions(reader AccountPermissions) PreferenceOption {
	return func(s *preferenceService) { s.accountPermissions = reader }
}

// offeredTypes is the catalogue of one portal the account may decide: the
// push types with a Permission only when it holds it.
func (s *preferenceService) offeredTypes(ctx context.Context, accountID int64, portal string) ([]TypeDefinition, error) {
	defs := TypesForPortal(portal)
	var granted []string
	loaded := false
	out := make([]TypeDefinition, 0, len(defs))
	for _, def := range defs {
		if def.Permission == "" {
			out = append(out, def)
			continue
		}
		if !loaded {
			var err error
			if granted, err = s.grantedPermissions(ctx, accountID); err != nil {
				return nil, err
			}
			loaded = true
		}
		if s.permissions.permits(def.Permission, granted) {
			out = append(out, def)
		}
	}
	return out, nil
}

// offeredToAccount reports whether the account may decide the type.
func (s *preferenceService) offeredToAccount(ctx context.Context, accountID int64, def TypeDefinition) (bool, error) {
	if def.Permission == "" {
		return true, nil
	}
	granted, err := s.grantedPermissions(ctx, accountID)
	if err != nil {
		return false, err
	}
	return s.permissions.permits(def.Permission, granted), nil
}

// grantedPermissions reads the account's permissions in the current school;
// without a reader or a school it holds none.
func (s *preferenceService) grantedPermissions(ctx context.Context, accountID int64) ([]string, error) {
	tenantID := tenant.FromContext(ctx)
	if s.accountPermissions == nil || tenantID <= 0 {
		return nil, nil
	}
	names, err := s.accountPermissions.FindEffectivePermissionNamesByAccountIDsForTenant(ctx, []int64{accountID}, tenantID)
	if err != nil {
		return nil, fmt.Errorf("load notification permissions: %w", err)
	}
	return names[accountID], nil
}

// storedOrDefault is the person's stored decision, else the type's default.
func storedOrDefault(stored map[string]bool, def TypeDefinition) bool {
	if enabled, ok := stored[def.Key]; ok {
		return enabled
	}
	return def.DefaultOn
}

// declineDefaultOnTypes stores an explicit "off" for the portal's default-on
// types: switching everything off must not leave them on (#4791).
func (s *preferenceService) declineDefaultOnTypes(ctx context.Context, accountID int64, portal string) error {
	for _, def := range TypesForPortal(portal) {
		if !def.DefaultOn {
			continue
		}
		if err := s.consent.RecordConsent(ctx, accountID, def.Key, false); err != nil {
			return fmt.Errorf("disable notification preferences: %w", err)
		}
	}
	return nil
}
