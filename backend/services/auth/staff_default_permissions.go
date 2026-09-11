package auth

import (
	"context"
	"log/slog"
)

// GrantStaffDefaultPermissions grants the default staff permission
// (groups:read) to the account of a newly created staff member.
//
// The Lehrkraft role (#1772) is deliberately class_day:read only: groups:read
// would open the tenant-wide group list and every group's student names, far
// beyond the classes assigned to that Lehrkraft. The decision is made on the
// account's actual roles, not on the teacher flag, and fails closed: an
// unreadable role set is no licence to widen access.
//
// permissionName is the permission to grant (the caller passes the
// users-domain default so this package needs no permission contract).
func GrantStaffDefaultPermissions(ctx context.Context, service AuthService, logger *slog.Logger, accountID int64, isTeacher bool, permissionName string) {
	role := "staff"
	if isTeacher {
		role = "teacher"
	}
	roles, err := service.GetAccountRoles(ctx, int(accountID))
	if err != nil {
		logger.Error("failed to resolve account roles, skipping default permission grant",
			slog.String("role", role),
			slog.Int64("account_id", accountID),
			slog.String("error", err.Error()))
		return
	}
	for _, accountRole := range roles {
		if IsLehrkraftSystemRole(accountRole) {
			logger.Info("skipping groups:read for lehrkraft account", slog.Int64("account_id", accountID))
			return
		}
	}
	perm, err := service.GetPermissionByName(ctx, permissionName)
	if err != nil || perm == nil {
		return
	}
	if err := service.GrantPermissionToAccount(ctx, int(accountID), int(perm.ID)); err != nil {
		logger.Error("failed to grant groups:read permission",
			slog.String("role", role),
			slog.Int64("account_id", accountID),
			slog.String("error", err.Error()))
	}
}
