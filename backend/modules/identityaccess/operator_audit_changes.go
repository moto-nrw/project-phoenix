package identityaccess

import "encoding/json"

// AuditChanges encodes the typed change summary of an operator action into
// the JSON the operator audit log has always stored for it: the revoked
// sessions, or the school access change with the role keys its action
// carries. Nil when the entry carries neither.
func (e OperatorAuditEntry) AuditChanges() (json.RawMessage, error) {
	if evidence := e.RevokedSessions; evidence != nil {
		return json.Marshal(map[string]any{
			"portal_scope":        evidence.PortalScope,
			"family_fingerprint":  evidence.FamilyFingerprint,
			"reason":              evidence.Reason,
			"revoked_token_count": evidence.Count,
		})
	}
	change := e.AccessChange
	if change == nil {
		return nil, nil
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
	return json.Marshal(changes)
}

// MFAAuditChanges encodes the MFA evidence of an operator's own mfa_* entry
// or account-wide MFA override into the JSON the operator audit log stores:
// the override transition, else the reason, else the lock. Nil without MFA
// evidence.
func (e OperatorAuditEntry) MFAAuditChanges() (json.RawMessage, error) {
	evidence := e.MFA
	switch {
	case evidence == nil:
		return nil, nil
	case evidence.Override != nil:
		override := evidence.Override
		return json.Marshal(map[string]any{
			"action":            override.Action,
			"scope":             override.Scope,
			"override":          override.Override,
			"previous_override": override.PreviousOverride,
			"reason":            override.Reason,
		})
	case evidence.Reason != "":
		return json.Marshal(map[string]any{"reason": evidence.Reason})
	case evidence.LockedUntil != nil:
		return json.Marshal(map[string]any{"locked_until": evidence.LockedUntil})
	}
	return nil, nil
}
