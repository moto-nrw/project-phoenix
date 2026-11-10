package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// revokeRoleHolderSessions joins the role mutation's transaction, including
// the auth audit. A failed revocation rolls back the permission change.
func (r *RoleAdministration) revokeRoleHolderSessions(ctx context.Context, roleID int64) error {
	ids, err := r.store.ListRoleHolderAccountIDs(ctx, roleID)
	if err != nil {
		return failed("list role holders for session revocation", err)
	}
	for _, id := range ids {
		sessions, err := r.sessions.DeleteAccountSessionsWithAudit(ctx, id, "role_changed", "", "")
		if err != nil {
			return failed("revoke role holder sessions", err)
		}
		r.sessions.QueuePushCleanup(ctx, id, sessions, "role_changed")
	}
	return nil
}

// lockRoleHolders takes the same account locks as mint, refresh and assignment,
// in ID order and before the role lock. The inverse order deadlocks with an
// assignment's foreign-key lock on the role.
func (r *RoleAdministration) lockRoleHolders(ctx context.Context, roleID int64) (map[int64]struct{}, error) {
	ids, err := r.store.ListRoleHolderAccountIDs(ctx, roleID)
	if err != nil {
		return nil, failed("list role holders to lock", err)
	}
	locked := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, err := r.store.LockAccount(ctx, id); err != nil {
			return nil, failed("lock role holder", err)
		}
		locked[id] = struct{}{}
	}
	return locked, nil
}

// requireLockedRoleHolders closes the gap between the initial holder read and
// the role lock. A concurrent new assignment may have committed in that gap;
// refusing it avoids locking a new account in the inverse order.
func (r *RoleAdministration) requireLockedRoleHolders(ctx context.Context, roleID int64, locked map[int64]struct{}) error {
	ids, err := r.store.ListRoleHolderAccountIDs(ctx, roleID)
	if err != nil {
		return failed("recheck role holders", err)
	}
	for _, id := range ids {
		if _, found := locked[id]; !found {
			return failed("change role", identityaccess.ErrRoleHoldersChanged)
		}
	}
	return nil
}
