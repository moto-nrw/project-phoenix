package importpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	importModels "github.com/moto-ogs/project-phoenix/modules/dataimport"
)

// Role resolution of the staff validation (#5887, ADR 0067 addendum of
// 2026-11-08, Umsetzungsschritt 2). The validation transaction reads the
// school's roles once in PreloadReferenceData and the permissions of the roles
// the file names once, at the first row that checks its role, instead of two
// reads per row. Each row
// still gets exactly the role the single read FindSchoolRoleByName would have
// chosen:
//
//   - The key is LOWER(name) from SQL (SchoolRole.NameKey), compared with the
//     trimmed, lowered input like the single read compares them.
//   - A role of the school ranks before a system role of the same key, like
//     the single read's ORDER BY tenant_id IS NULL.
//   - The exact name ranks before a display-name alias: validateRole asks
//     findRole for the alias only when the name itself has no candidate.
//   - Several candidates of the same rank leave the choice to the single read
//     itself, which chooses the smallest ID among them (#6234).
//
// Both reads stay inside the validation transaction; one snapshot per
// validation transaction is what the addendum allows (point 6).

// buildRoleIndex keeps, per name key, the candidates of the best rank. It
// returns nil when a role carries no key, so every row falls back to the
// single read.
func buildRoleIndex(roles []*importModels.SchoolRole) map[string][]*importModels.SchoolRole {
	index := make(map[string][]*importModels.SchoolRole, len(roles))
	for _, role := range roles {
		if role.NameKey == "" {
			return nil
		}
		candidates := index[role.NameKey]
		switch {
		case len(candidates) == 0 || roleRank(role) == roleRank(candidates[0]):
			index[role.NameKey] = append(candidates, role)
		case roleRank(role) < roleRank(candidates[0]):
			index[role.NameKey] = []*importModels.SchoolRole{role}
		}
	}
	return index
}

// roleRank orders a school's own role before a system role.
func roleRank(role *importModels.SchoolRole) int {
	if role.TenantID != nil {
		return 0
	}
	return 1
}

func roleNameKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// findRole resolves one role name like FindSchoolRoleByName.
func (c *StaffImportConfig) findRole(ctx context.Context, name string) (*importModels.SchoolRole, error) {
	if c.roleIndex == nil {
		return c.Roles.FindSchoolRoleByName(ctx, name)
	}
	switch candidates := c.roleIndex[roleNameKey(name)]; len(candidates) {
	case 0:
		return nil, importModels.ErrRoleNotFound
	case 1:
		return candidates[0], nil
	default:
		return c.Roles.FindSchoolRoleByName(ctx, name)
	}
}

// prepareRolePermissions notes, in ValidateBatch, the roles every row of the
// file can resolve to. Their permissions are read later, once, by the first
// row that checks its role (rolePermissionsOf): an update whose rows all
// exist checks no role and reads nothing, as before #5887.
func (c *StaffImportConfig) prepareRolePermissions(rows []importModels.StaffImportRow) {
	c.rolePermissions, c.rolePermissionsRead, c.namedRoles, c.rolePermissionsLost = nil, false, nil, nil
	if c.roleIndex == nil || c.FindRolePermissionsByRoleIDs == nil {
		return
	}
	c.namedRoles = c.namedRoleIDs(rows)
}

// readRolePermissions reads the permissions of the noted roles in one
// statement inside its own savepoint. When the read fails, the savepoint is
// rolled back and each row reads its role's permissions itself, with the same
// role_lookup_failed as before #5887. When a statement of the savepoint
// itself fails, nothing reads on: the validation ends the upload (E19).
func (c *StaffImportConfig) readRolePermissions(ctx context.Context) {
	c.rolePermissionsRead = true
	if len(c.namedRoles) == 0 {
		return
	}
	var read map[int64][]string
	if err := c.Transactions.Savepoint(ctx, func(ctx context.Context) error {
		var err error
		read, err = c.FindRolePermissionsByRoleIDs(ctx, c.namedRoles)
		return err
	}); err != nil {
		if errors.Is(err, importModels.ErrSavepointControl) {
			c.rolePermissionsLost = fmt.Errorf("%w: read role permissions: %v", errSavepointLost, err)
		}
		return
	}
	c.rolePermissions = make(map[int64][]string, len(c.namedRoles))
	for _, id := range c.namedRoles {
		c.rolePermissions[id] = read[id]
	}
}

// namedRoleIDs lists, without repetition, every candidate a row's role name or
// its alias resolves to in the index, including candidates of equal rank.
func (c *StaffImportConfig) namedRoleIDs(rows []importModels.StaffImportRow) []int64 {
	seen := make(map[int64]bool)
	var ids []int64
	add := func(candidates []*importModels.SchoolRole) {
		for _, role := range candidates {
			if !seen[role.ID] {
				seen[role.ID] = true
				ids = append(ids, role.ID)
			}
		}
	}
	for _, row := range rows {
		name := strings.TrimSpace(row.RoleName)
		if name == "" {
			continue
		}
		candidates := c.roleIndex[roleNameKey(name)]
		if len(candidates) == 0 {
			if raw, ok := displayNameToRawRole[strings.ToLower(name)]; ok && raw != name {
				candidates = c.roleIndex[roleNameKey(raw)]
			}
		}
		add(candidates)
	}
	return ids
}

// ValidationAbort is the failure that ends the upload after validation: a
// failed savepoint statement of the role read (E19).
func (c *StaffImportConfig) ValidationAbort() error { return c.rolePermissionsLost }

// rolePermissionsOf returns the role's permissions from the one read of the
// noted roles, or reads them for this row alone when that read has none.
func (c *StaffImportConfig) rolePermissionsOf(ctx context.Context, roleID int64) ([]string, error) {
	if !c.rolePermissionsRead {
		c.readRolePermissions(ctx)
	}
	if c.rolePermissionsLost != nil {
		return nil, c.rolePermissionsLost
	}
	if permissions, ok := c.rolePermissions[roleID]; ok {
		return permissions, nil
	}
	read, err := c.FindRolePermissionsByRoleIDs(ctx, []int64{roleID})
	if err != nil {
		return nil, err
	}
	return read[roleID], nil
}
