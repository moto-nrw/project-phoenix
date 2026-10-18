package identityaccess_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/assert"
)

func TestOperatorAuditChangesKeepTheStoredKeys(t *testing.T) {
	t.Parallel()
	change := &identityaccess.OperatorAccessChange{SchoolID: 21, Email: "a@b.test", RoleID: 31, RoleName: "admin", RemovedRoles: []string{"user"}, AccountDeactivated: true}
	assert.Equal(t, map[string]any{"schoolID": int64(21), "email": "a@b.test", "roleID": int64(31), "roleName": "admin"},
		identityaccess.OperatorAuditEntry{Action: identityaccess.OperatorAuditActionCreate, AccessChange: change}.AuditChanges())
	assert.Equal(t, map[string]any{"schoolID": int64(21), "email": "a@b.test", "roleID": int64(31), "roleName": "admin", "removedRoles": []string{"user"}},
		identityaccess.OperatorAuditEntry{Action: identityaccess.OperatorAuditActionUpdate, AccessChange: change}.AuditChanges())
	assert.Equal(t, map[string]any{"schoolID": int64(21), "email": "a@b.test", "accountDeactivated": true},
		identityaccess.OperatorAuditEntry{Action: identityaccess.OperatorAuditActionDelete, AccessChange: change}.AuditChanges())
	assert.Equal(t, map[string]any{"portal_scope": "school", "family_fingerprint": "f", "reason": "r", "revoked_token_count": 2},
		identityaccess.OperatorAuditEntry{RevokedSessions: &identityaccess.RevokedSessionsEvidence{PortalScope: "school", FamilyFingerprint: "f", Reason: "r", Count: 2}, AccessChange: change}.AuditChanges(),
		"revoked sessions win over an access change")
	assert.Nil(t, identityaccess.OperatorAuditEntry{}.AuditChanges())
}

func TestOperatorMFAAuditChangesKeepTheStoredKeys(t *testing.T) {
	t.Parallel()
	locked := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	override := &identityaccess.OperatorMFAOverrideEvidence{Action: "set", Scope: "platform", Override: "off", PreviousOverride: "on", Reason: "Wartung"}
	assert.Equal(t, map[string]any{"action": "set", "scope": "platform", "override": "off", "previous_override": "on", "reason": "Wartung"},
		identityaccess.OperatorAuditEntry{MFA: &identityaccess.OperatorMFAEvidence{Override: override, Reason: "ignored"}}.MFAAuditChanges())
	assert.Equal(t, map[string]any{"reason": "zu viele Versuche"},
		identityaccess.OperatorAuditEntry{MFA: &identityaccess.OperatorMFAEvidence{Reason: "zu viele Versuche", LockedUntil: &locked}}.MFAAuditChanges())
	assert.Equal(t, map[string]any{"locked_until": &locked},
		identityaccess.OperatorAuditEntry{MFA: &identityaccess.OperatorMFAEvidence{LockedUntil: &locked}}.MFAAuditChanges())
	assert.Nil(t, identityaccess.OperatorAuditEntry{MFA: &identityaccess.OperatorMFAEvidence{}}.MFAAuditChanges())
	assert.Nil(t, identityaccess.OperatorAuditEntry{}.MFAAuditChanges())
}

func TestPayerRemovedAuditNoteKeepsItsText(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Erziehungsberechtigte Person vom Kind entfernt", identityaccess.PayerRemovedAuditNote)
}
