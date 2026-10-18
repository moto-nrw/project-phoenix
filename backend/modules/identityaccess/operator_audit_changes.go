package identityaccess

// AuditChanges renders the typed change summary of an operator action into
// the JSON keys the operator audit log has always stored for it: the
// revoked sessions, or the school access change with the role keys its
// action carries. Nil when the entry carries neither.
func (e OperatorAuditEntry) AuditChanges() map[string]any {
	if evidence := e.RevokedSessions; evidence != nil {
		return map[string]any{
			"portal_scope":        evidence.PortalScope,
			"family_fingerprint":  evidence.FamilyFingerprint,
			"reason":              evidence.Reason,
			"revoked_token_count": evidence.Count,
		}
	}
	change := e.AccessChange
	if change == nil {
		return nil
	}
	changes := map[string]any{"schoolID": change.SchoolID, "email": change.Email}
	switch e.Action {
	case OperatorAuditActionCreate:
		changes["roleID"] = change.RoleID
		changes["roleName"] = change.RoleName
	case OperatorAuditActionUpdate:
		changes["roleID"] = change.RoleID
		changes["roleName"] = change.RoleName
		changes["removedRoles"] = change.RemovedRoles
	case OperatorAuditActionDelete:
		changes["accountDeactivated"] = change.AccountDeactivated
	}
	return changes
}

// MFAAuditChanges renders the MFA evidence of an operator's own mfa_* entry
// or account-wide MFA override into the keys the operator audit log stores:
// the override transition, else the reason, else the lock. Nil without MFA
// evidence.
func (e OperatorAuditEntry) MFAAuditChanges() map[string]any {
	evidence := e.MFA
	if evidence == nil {
		return nil
	}
	if override := evidence.Override; override != nil {
		return map[string]any{
			"action":            override.Action,
			"scope":             override.Scope,
			"override":          override.Override,
			"previous_override": override.PreviousOverride,
			"reason":            override.Reason,
		}
	}
	switch {
	case evidence.Reason != "":
		return map[string]any{"reason": evidence.Reason}
	case evidence.LockedUntil != nil:
		return map[string]any{"locked_until": evidence.LockedUntil}
	}
	return nil
}
