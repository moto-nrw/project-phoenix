package auth

import "net/http"

// ============================================================================
// EXPORTED HANDLER METHODS (for testing)
// ============================================================================

// GetAccountHandler returns the getAccount handler for testing
func (rs *Resource) GetAccountHandler() http.HandlerFunc { return rs.getAccount }

// ChangePasswordHandler returns the changePassword handler for testing
func (rs *Resource) ChangePasswordHandler() http.HandlerFunc { return rs.changePassword }

// ListRolesHandler returns the listRoles handler for testing
func (rs *Resource) ListRolesHandler() http.HandlerFunc { return rs.listRoles }

// CreateRoleHandler returns the createRole handler for testing
func (rs *Resource) CreateRoleHandler() http.HandlerFunc { return rs.createRole }

// GetRoleByIDHandler returns the getRoleByID handler for testing
func (rs *Resource) GetRoleByIDHandler() http.HandlerFunc { return rs.getRoleByID }

// DeleteRoleHandler returns the deleteRole handler for testing
func (rs *Resource) DeleteRoleHandler() http.HandlerFunc { return rs.deleteRole }

// ListPermissionsHandler returns the listPermissions handler for testing
func (rs *Resource) ListPermissionsHandler() http.HandlerFunc { return rs.listPermissions }

// CreatePermissionHandler returns the createPermission handler for testing
func (rs *Resource) CreatePermissionHandler() http.HandlerFunc { return rs.createPermission }

// GetPermissionByIDHandler returns the getPermissionByID handler for testing
func (rs *Resource) GetPermissionByIDHandler() http.HandlerFunc { return rs.getPermissionByID }

// ListAccountsHandler returns the listAccounts handler for testing
func (rs *Resource) ListAccountsHandler() http.HandlerFunc { return rs.listAccounts }

// UpdateRoleHandler returns the updateRole handler for testing
func (rs *Resource) UpdateRoleHandler() http.HandlerFunc { return rs.updateRole }

// UpdatePermissionHandler returns the updatePermission handler for testing
func (rs *Resource) UpdatePermissionHandler() http.HandlerFunc { return rs.updatePermission }

// DeletePermissionHandler returns the deletePermission handler for testing
func (rs *Resource) DeletePermissionHandler() http.HandlerFunc { return rs.deletePermission }

// AssignRoleToAccountHandler returns the assignRoleToAccount handler for testing
func (rs *Resource) AssignRoleToAccountHandler() http.HandlerFunc { return rs.assignRoleToAccount }

// RemoveRoleFromAccountHandler returns the removeRoleFromAccount handler for testing
func (rs *Resource) RemoveRoleFromAccountHandler() http.HandlerFunc { return rs.removeRoleFromAccount }

// GetAccountRolesHandler returns the getAccountRoles handler for testing
func (rs *Resource) GetAccountRolesHandler() http.HandlerFunc { return rs.getAccountRoles }

// AssignPermissionToRoleHandler returns the assignPermissionToRole handler for testing
func (rs *Resource) AssignPermissionToRoleHandler() http.HandlerFunc {
	return rs.assignPermissionToRole
}

// RemovePermissionFromRoleHandler returns the removePermissionFromRole handler for testing
func (rs *Resource) RemovePermissionFromRoleHandler() http.HandlerFunc {
	return rs.removePermissionFromRole
}

// GetRolePermissionsHandler returns the getRolePermissions handler for testing
func (rs *Resource) GetRolePermissionsHandler() http.HandlerFunc { return rs.getRolePermissions }

// GrantPermissionToAccountHandler returns the grantPermissionToAccount handler for testing
func (rs *Resource) GrantPermissionToAccountHandler() http.HandlerFunc {
	return rs.grantPermissionToAccount
}

// DenyPermissionToAccountHandler returns the denyPermissionToAccount handler for testing
func (rs *Resource) DenyPermissionToAccountHandler() http.HandlerFunc {
	return rs.denyPermissionToAccount
}

// RemovePermissionFromAccountHandler returns the removePermissionFromAccount handler for testing
func (rs *Resource) RemovePermissionFromAccountHandler() http.HandlerFunc {
	return rs.removePermissionFromAccount
}

// GetAccountPermissionsHandler returns the getAccountPermissions handler for testing
func (rs *Resource) GetAccountPermissionsHandler() http.HandlerFunc { return rs.getAccountPermissions }

// GetAccountDirectPermissionsHandler returns the getAccountDirectPermissions handler for testing
func (rs *Resource) GetAccountDirectPermissionsHandler() http.HandlerFunc {
	return rs.getAccountDirectPermissions
}

// ActivateAccountHandler returns the activateAccount handler for testing
func (rs *Resource) ActivateAccountHandler() http.HandlerFunc { return rs.activateAccount }

// DeactivateAccountHandler returns the deactivateAccount handler for testing
func (rs *Resource) DeactivateAccountHandler() http.HandlerFunc { return rs.deactivateAccount }

// UpdateAccountHandler returns the updateAccount handler for testing
func (rs *Resource) UpdateAccountHandler() http.HandlerFunc { return rs.updateAccount }

// GetAccountsByRoleHandler returns the getAccountsByRole handler for testing
func (rs *Resource) GetAccountsByRoleHandler() http.HandlerFunc { return rs.getAccountsByRole }

// GetActiveTokensHandler returns the getActiveTokens handler for testing
func (rs *Resource) GetActiveTokensHandler() http.HandlerFunc { return rs.getActiveTokens }

// RevokeAllTokensHandler returns the revokeAllTokens handler for testing
func (rs *Resource) RevokeAllTokensHandler() http.HandlerFunc { return rs.revokeAllTokens }

// CleanupExpiredTokensHandler returns the cleanupExpiredTokens handler for testing
func (rs *Resource) CleanupExpiredTokensHandler() http.HandlerFunc { return rs.cleanupExpiredTokens }

// CreateInvitationHandler returns the createInvitation handler for testing
func (rs *Resource) CreateInvitationHandler() http.HandlerFunc { return rs.createInvitation }

// ListPendingInvitationsHandler returns the listPendingInvitations handler for testing
func (rs *Resource) ListPendingInvitationsHandler() http.HandlerFunc {
	return rs.listPendingInvitations
}

// ResendInvitationHandler returns the resendInvitation handler for testing
func (rs *Resource) ResendInvitationHandler() http.HandlerFunc { return rs.resendInvitation }

// RevokeInvitationHandler returns the revokeInvitation handler for testing
func (rs *Resource) RevokeInvitationHandler() http.HandlerFunc { return rs.revokeInvitation }

// CreateParentAccountHandler returns the createParentAccount handler for testing
func (rs *Resource) CreateParentAccountHandler() http.HandlerFunc { return rs.createParentAccount }

// ListParentAccountsHandler returns the listParentAccounts handler for testing
func (rs *Resource) ListParentAccountsHandler() http.HandlerFunc { return rs.listParentAccounts }

// GetParentAccountByIDHandler returns the getParentAccountByID handler for testing
func (rs *Resource) GetParentAccountByIDHandler() http.HandlerFunc { return rs.getParentAccountByID }

// UpdateParentAccountHandler returns the updateParentAccount handler for testing
func (rs *Resource) UpdateParentAccountHandler() http.HandlerFunc { return rs.updateParentAccount }

// ActivateParentAccountHandler returns the activateParentAccount handler for testing
func (rs *Resource) ActivateParentAccountHandler() http.HandlerFunc { return rs.activateParentAccount }

// DeactivateParentAccountHandler returns the deactivateParentAccount handler for testing
func (rs *Resource) DeactivateParentAccountHandler() http.HandlerFunc {
	return rs.deactivateParentAccount
}
