package behavior_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The suites pin the rows other owners persist for the Identity & Access
// flows by their stored values, read straight from the tables (#3446). What
// is listed here is the persisted contract those flows write into another
// owner's rows; none of it needs the owner's implementation package.

// audit.auth_events event types the flows write.
const (
	authEventLogout                   = "logout"
	authEventTenantSwitch             = "tenant_switch"
	authEventTokenRevoked             = "token_revoked"
	authEventAccountWideWipeCompleted = "account_wide_wipe_completed"
	authEventMFAEmailSent             = "mfa_email_sent"
	authEventMFAFailed                = "mfa_failed"
)

// authEventRow is an audit.auth_events row as the suites read it back.
type authEventRow struct {
	bun.BaseModel `bun:"table:audit.auth_events,alias:auth_event"`

	ID        int64          `bun:"id,pk"`
	TenantID  int64          `bun:"tenant_id"`
	AccountID int64          `bun:"account_id"`
	EventType string         `bun:"event_type"`
	Success   bool           `bun:"success"`
	IPAddress string         `bun:"ip_address"`
	UserAgent string         `bun:"user_agent"`
	Metadata  map[string]any `bun:"metadata,type:jsonb"`
	CreatedAt time.Time      `bun:"created_at"`
}

// Guardian link role presets of users.students_guardians.guardian_role.
const (
	guardianRoleLegalGuardian = "legal_guardian"
	guardianRoleEmergency     = "emergency_contact"
	guardianRolePickupOnly    = "pickup_only"
	guardianRoleSocialWorker  = "social_worker"
	guardianRoleCustom        = "custom"
)

// guardianPaymentFieldIsPayer is the audit.guardian_financial_changes
// field name of the payer mark.
const guardianPaymentFieldIsPayer = "is_payer"

// emailKindGuardianInvitation is the outbox kind of the guardian invitation
// mail.
const emailKindGuardianInvitation = "guardian_invitation"

// platform.operator_audit_log action and resource type of the operator MFA
// override.
const (
	operatorActionMFAAdminOverride = "mfa_admin_override"
	operatorResourceAccount        = "account"
)

// operatorAuditLogRow is a platform.operator_audit_log row as the suites
// read it back.
type operatorAuditLogRow struct {
	bun.BaseModel `bun:"table:platform.operator_audit_log,alias:operator_audit_log"`

	ID           int64           `bun:"id,pk"`
	OperatorID   int64           `bun:"operator_id"`
	Action       string          `bun:"action"`
	ResourceType string          `bun:"resource_type"`
	ResourceID   *int64          `bun:"resource_id"`
	Changes      json.RawMessage `bun:"changes,type:jsonb"`
	CreatedAt    time.Time       `bun:"created_at"`
}

// changes decodes the row's change record.
func (r *operatorAuditLogRow) changes(t *testing.T) map[string]any {
	t.Helper()
	var changes map[string]any
	require.NoError(t, json.Unmarshal(r.Changes, &changes))
	return changes
}

// Push subscription portals of iot.push_subscriptions.portal.
const (
	pushPortalStaff  = "staff"
	pushPortalParent = "parent"
)

// insertPushSubscription writes an iot.push_subscriptions row bound to the
// given token family ("" for an unbound subscription).
func insertPushSubscription(t *testing.T, db *bun.DB, accountID, tenantID int64, portal, endpoint, familyID string) {
	t.Helper()
	_, err := db.NewRaw(`INSERT INTO iot.push_subscriptions
		(tenant_id, account_id, portal, endpoint, p256dh, auth, token_family_id)
		VALUES (?, ?, ?, ?, 'p256dh-key', 'auth-key', ?)`,
		tenantID, accountID, portal, endpoint, familyID).Exec(context.Background())
	require.NoError(t, err)
}

// countPushSubscriptions counts the account's rows for a portal, narrowed to
// an endpoint when one is given.
func countPushSubscriptions(t *testing.T, db *bun.DB, accountID int64, portal, endpoint string) int {
	t.Helper()
	query := db.NewSelect().
		TableExpr("iot.push_subscriptions").
		Where("account_id = ?", accountID).
		Where("portal = ?", portal)
	if endpoint != "" {
		query = query.Where("endpoint = ?", endpoint)
	}
	count, err := query.Count(context.Background())
	require.NoError(t, err)
	return count
}

// Tenant setting keys and values the flows resolve.
const (
	settingKeyMFAMode                  = "security.mfa_mode"
	settingKeyMFATrustedDeviceEnabled  = "security.mfa_trusted_device_enabled"
	settingKeyMFATrustedDeviceDays     = "security.mfa_trusted_device_days"
	settingKeyOperationalOverviewScope = "operations.operational_overview_scope"
	settingKeyGroupLeaderReviewEnabled = "operations.parent_request_group_leader_review_enabled"
	settingKeyParentAbsenceReviewScope = "operations.parent_absence_review_scope"

	mfaModeOff            = "off"
	mfaModeRequiredAdmins = "required_admins"
	mfaModeRequiredAll    = "required_all"

	overviewScopeOwn      = "own"
	overviewScopeAdmins   = "admins"
	overviewScopeAllStaff = "all_staff"

	parentAbsenceReviewScopeInherit      = "inherit"
	parentAbsenceReviewScopeGroupLeaders = "group_leaders"
	parentAbsenceReviewScopeAllStaff     = "all_staff"
)

// writeTenantSettingOverride stores a raw tenant override, bypassing the
// settings service's validation, so a suite can pin how a flow treats a value
// the registry would refuse.
func writeTenantSettingOverride(t *testing.T, db *bun.DB, tenantID int64, key string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, ?, ?::jsonb)
		ON CONFLICT (tenant_id, setting_key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		tenantID, key, string(raw)).Exec(context.Background())
	require.NoError(t, err)
}

// guardianLinkSpec describes a users.students_guardians row a suite writes
// directly, the way a staff member's contact maintenance leaves it.
type guardianLinkSpec struct {
	studentID, guardianProfileID int64
	relationshipType             string
	isPrimary, isPayer           bool
}

// insertGuardianLink writes the link in the test's tenant with the default
// custom role and returns its id.
func insertGuardianLink(t *testing.T, db *bun.DB, spec guardianLinkSpec) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO users.students_guardians
		(tenant_id, student_id, guardian_profile_id, relationship_type, is_primary, is_payer, emergency_priority)
		VALUES (?, ?, ?, ?, ?, ?, 1) RETURNING id`,
		testpkg.Tenant(t), spec.studentID, spec.guardianProfileID, spec.relationshipType, spec.isPrimary, spec.isPayer).
		Scan(context.Background(), &id))
	return id
}

// guardianLinkRow is a users.students_guardians row as the suites read it
// back, with the portal access the stored permissions grant.
type guardianLinkRow struct {
	ID           int64  `bun:"id"`
	GuardianRole string `bun:"guardian_role"`
	IsPayer      bool   `bun:"is_payer"`
	portalAccess bool
}

// readGuardianLink loads the current link of a student and a guardian.
func readGuardianLink(t *testing.T, db *bun.DB, studentID, guardianProfileID int64) *guardianLinkRow {
	t.Helper()
	var row guardianLinkRow
	require.NoError(t, db.NewRaw(`SELECT id, guardian_role, is_payer FROM users.students_guardians
		WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?`, testpkg.Tenant(t), studentID, guardianProfileID).
		Scan(context.Background(), &row))
	row.portalAccess = testpkg.StudentGuardianLinkGrantsPortalAccess(t, db, row.ID)
	return &row
}
