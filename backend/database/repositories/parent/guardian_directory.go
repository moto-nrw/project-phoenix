package parent

import (
	"context"
	"errors"
	"fmt"
)

// DirectoryGuardianLink is the People Directory projection of one
// users.students_guardians row the parent-portal repositories read.
// users.guardian_profiles and users.students_guardians belong to that owner
// (#2663); the cross-tenant joins that used to read them are now an
// account-tenants read plus a directory lookup inside the same admin
// transaction. Permissions lists the granted parents-portal permissions.
type DirectoryGuardianLink struct {
	TenantID          int64
	StudentID         int64
	GuardianProfileID int64
	Permissions       []string
}

// HasPermission reports whether the link grants the named permission.
func (l DirectoryGuardianLink) HasPermission(name string) bool {
	for _, granted := range l.Permissions {
		if granted == name {
			return true
		}
	}
	return false
}

// PermissionMap renders the granted permissions in the shape the legacy
// child summary carries.
func (l DirectoryGuardianLink) PermissionMap() map[string]any {
	result := make(map[string]any, len(l.Permissions))
	for _, name := range l.Permissions {
		result[name] = true
	}
	return result
}

// GuardianDirectory is bound by the composition root; it fails while
// unbound instead of falling back to a foreign query.
type GuardianDirectory interface {
	// ListGuardianLinksByAccount returns every link of the account's
	// guardian profiles across every tenant the admin transaction can see,
	// ordered by tenant then student.
	ListGuardianLinksByAccount(ctx context.Context, accountID int64) ([]DirectoryGuardianLink, error)
}

var errGuardianDirectoryRequired = errors.New("parent repositories: guardian directory is not bound")

func guardianLinksByAccount(ctx context.Context, directory GuardianDirectory, accountID int64) ([]DirectoryGuardianLink, error) {
	if directory == nil {
		return nil, errGuardianDirectoryRequired
	}
	return directory.ListGuardianLinksByAccount(ctx, accountID)
}

// activeMappingTenants returns the tenants the account holds an ACTIVE
// auth.account_tenants mapping at. The membership is the only safe scope
// for a cross-tenant parent read; a deactivated mapping hides the school's
// rows even when guardian links linger.
func activeMappingTenants(ctx context.Context, runtime Runtime, accountID int64) (map[int64]struct{}, error) {
	var tenantIDs []int64
	err := runtimeDB(ctx, runtime).NewRaw(`
		SELECT at.tenant_id AS tenant_id
		FROM auth.account_tenants AS at
		WHERE at.account_id = ?
		  AND at.status     = 'active'
	`, accountID).Scan(ctx, &tenantIDs)
	if err != nil {
		return nil, fmt.Errorf("list active memberships: %w", err)
	}
	result := make(map[int64]struct{}, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		result[tenantID] = struct{}{}
	}
	return result, nil
}

// activeGuardianLinks returns the account's guardian links at the tenants
// where it holds an ACTIVE mapping, in directory order (tenant, student).
func activeGuardianLinks(ctx context.Context, runtime Runtime, directory GuardianDirectory, accountID int64) ([]DirectoryGuardianLink, error) {
	tenants, err := activeMappingTenants(ctx, runtime, accountID)
	if err != nil {
		return nil, err
	}
	links, err := guardianLinksByAccount(ctx, directory, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]DirectoryGuardianLink, 0, len(links))
	for _, link := range links {
		if _, active := tenants[link.TenantID]; active {
			out = append(out, link)
		}
	}
	return out, nil
}
