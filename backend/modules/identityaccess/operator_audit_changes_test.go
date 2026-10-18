package identityaccess_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func auditJSON(t *testing.T, encode func() (json.RawMessage, error)) string {
	t.Helper()
	raw, err := encode()
	require.NoError(t, err)
	return string(raw)
}

func TestOperatorAuditChangesKeepTheStoredKeys(t *testing.T) {
	t.Parallel()
	change := &identityaccess.OperatorAccessChange{SchoolID: 21, Email: "a@b.test", RoleID: 31, RoleName: "admin", RemovedRoles: []string{"user"}, AccountDeactivated: true}
	entry := func(action string) identityaccess.OperatorAuditEntry {
		return identityaccess.OperatorAuditEntry{Action: action, AccessChange: change}
	}
	assert.JSONEq(t, `{"schoolID":21,"email":"a@b.test","roleID":31,"roleName":"admin"}`,
		auditJSON(t, entry(identityaccess.OperatorAuditActionCreate).AuditChanges))
	assert.JSONEq(t, `{"schoolID":21,"email":"a@b.test","roleID":31,"roleName":"admin","removedRoles":["user"]}`,
		auditJSON(t, entry(identityaccess.OperatorAuditActionUpdate).AuditChanges))
	assert.JSONEq(t, `{"schoolID":21,"email":"a@b.test","accountDeactivated":true}`,
		auditJSON(t, entry(identityaccess.OperatorAuditActionDelete).AuditChanges))
	revoked := identityaccess.OperatorAuditEntry{RevokedSessions: &identityaccess.RevokedSessionsEvidence{PortalScope: "school", FamilyFingerprint: "f", Reason: "r", Count: 2}, AccessChange: change}
	assert.JSONEq(t, `{"portal_scope":"school","family_fingerprint":"f","reason":"r","revoked_token_count":2}`,
		auditJSON(t, revoked.AuditChanges), "revoked sessions win over an access change")
	assert.Empty(t, auditJSON(t, identityaccess.OperatorAuditEntry{}.AuditChanges))
}

func TestOperatorMFAAuditChangesKeepTheStoredKeys(t *testing.T) {
	t.Parallel()
	locked := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	override := &identityaccess.OperatorMFAOverrideEvidence{Action: "set", Scope: "platform", Override: "off", PreviousOverride: "on", Reason: "Wartung"}
	mfa := func(evidence *identityaccess.OperatorMFAEvidence) func() (json.RawMessage, error) {
		return identityaccess.OperatorAuditEntry{MFA: evidence}.MFAAuditChanges
	}
	assert.JSONEq(t, `{"action":"set","scope":"platform","override":"off","previous_override":"on","reason":"Wartung"}`,
		auditJSON(t, mfa(&identityaccess.OperatorMFAEvidence{Override: override, Reason: "ignored"})))
	assert.JSONEq(t, `{"reason":"zu viele Versuche"}`, auditJSON(t, mfa(&identityaccess.OperatorMFAEvidence{Reason: "zu viele Versuche", LockedUntil: &locked})))
	assert.JSONEq(t, `{"locked_until":"2026-10-19T12:00:00Z"}`, auditJSON(t, mfa(&identityaccess.OperatorMFAEvidence{LockedUntil: &locked})))
	assert.Empty(t, auditJSON(t, mfa(&identityaccess.OperatorMFAEvidence{})))
	assert.Empty(t, auditJSON(t, mfa(nil)))
}

func TestPayerRemovedAuditNoteKeepsItsText(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Erziehungsberechtigte Person vom Kind entfernt", identityaccess.PayerRemovedAuditNote)
}
