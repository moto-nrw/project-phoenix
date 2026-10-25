package test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Guardian fixtures (#2663). The guardian rows belong to the People
// Directory; tests that need a contact person with a specific name, or a
// link with a specific role, build it here instead of naming the owner's
// repositories.

// CreateTestGuardianProfileNamed creates a guardian profile with the given
// names in the test's tenant. email is made unique.
func CreateTestGuardianProfileNamed(tb testing.TB, db *bun.DB, firstName, lastName, email string) *GuardianProfile {
	tb.Helper()
	return CreateTestGuardianProfileForTenant(tb, db, fixtureTenantID(tb), firstName, lastName, email)
}

// CreateTestGuardianProfileForTenant creates a guardian profile in tenantID.
// email is made unique.
func CreateTestGuardianProfileForTenant(tb testing.TB, db *bun.DB, tenantID int64, firstName, lastName, email string) *GuardianProfile {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	uniqueEmail := fmt.Sprintf(testEmailFormat, email, uniqueFixtureSuffix())
	profile := &GuardianProfile{
		FirstName:              firstName,
		LastName:               lastName,
		Email:                  &uniqueEmail,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	profile.TenantID = tenantID
	err := db.NewInsert().Model(profile).ModelTableExpr(`users.guardian_profiles`).Scan(ctx)
	require.NoError(tb, err, "Failed to create test guardian profile")
	return profile
}

// CreateTestStudentGuardianLink links a guardian to a student in the test's
// tenant with the given role preset; the role decides the parents-portal
// permissions exactly as the production link path does.
func CreateTestStudentGuardianLink(tb testing.TB, db *bun.DB, studentID, guardianProfileID int64, role string) *StudentGuardian {
	tb.Helper()
	return CreateTestStudentGuardianLinkForTenant(tb, db, fixtureTenantID(tb), studentID, guardianProfileID, role)
}

// CreateTestStudentGuardianLinkForTenant links a guardian to a student in
// tenantID with the given role preset.
func CreateTestStudentGuardianLinkForTenant(tb testing.TB, db *bun.DB, tenantID, studentID, guardianProfileID int64, role string) *StudentGuardian {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	link := &StudentGuardian{
		StudentID:          studentID,
		GuardianProfileID:  guardianProfileID,
		RelationshipType:   "parent",
		IsEmergencyContact: true,
		CanPickup:          true,
		EmergencyPriority:  1,
	}
	ApplyStudentGuardianRole(link, role)
	link.TenantID = tenantID
	require.NoError(tb, InsertTestStudentGuardian(ctx, db, link), "Failed to create student guardian link")
	return link
}

// InsertTestStudentGuardian writes link as its three owner rows in one
// statement: People Directory's relationship, Care Plan's pickup permission
// and Identity & Access's portal access bound to the guardian's account. It
// fills the link's ID and timestamps. A zero ID draws from the relationship
// sequence.
func InsertTestStudentGuardian(ctx context.Context, db bun.IDB, link *StudentGuardian) error {
	permissions := []byte(`{}`)
	if link.Permissions != nil {
		var err error
		if permissions, err = json.Marshal(link.Permissions); err != nil {
			return err
		}
	}
	if link.GuardianRole == "" {
		link.GuardianRole = "custom"
	}
	return db.NewRaw(`
		WITH relationship AS (
			INSERT INTO users.student_guardian_relationships
				(id, tenant_id, student_id, guardian_profile_id, relationship_type, guardian_role,
				 is_primary, is_emergency_contact, emergency_priority, is_payer)
			VALUES (coalesce(nullif(?0, 0), nextval('users.student_guardian_relationships_id_seq')),
				?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9)
			RETURNING id, tenant_id, guardian_profile_id, created_at, updated_at
		), pickup AS (
			INSERT INTO users.student_guardian_pickup_permissions (tenant_id, relationship_id, can_pickup, pickup_notes)
			SELECT tenant_id, id, ?10, ?11 FROM relationship
		), access AS (
			INSERT INTO auth.guardian_student_access (tenant_id, relationship_id, account_id, permissions)
			SELECT r.tenant_id, r.id, g.account_id, ?12::jsonb
			FROM relationship AS r
			JOIN users.guardian_profiles AS g ON g.tenant_id = r.tenant_id AND g.id = r.guardian_profile_id
		)
		SELECT id, created_at, updated_at FROM relationship`,
		link.ID, link.TenantID, link.StudentID, link.GuardianProfileID, link.RelationshipType, link.GuardianRole,
		link.IsPrimary, link.IsEmergencyContact, link.EmergencyPriority, link.IsPayer,
		link.CanPickup, link.PickupNotes, string(permissions)).
		Scan(ctx, &link.ID, &link.CreatedAt, &link.UpdatedAt)
}

// studentGuardianLinkSource joins the three owners of a relationship into
// the StudentGuardian row shape, as the production guardian-link
// projection does. Test support may not import that module.
const studentGuardianLinkSource = `(SELECT r.id, r.tenant_id, r.student_id, r.guardian_profile_id, r.relationship_type,
	r.guardian_role, r.is_primary, r.is_emergency_contact, p.can_pickup, p.pickup_notes, p.pickup_weekdays,
	r.emergency_priority, r.is_payer, a.permissions, a.account_id AS access_account_id, r.created_at,
	GREATEST(r.updated_at, p.updated_at, a.updated_at) AS updated_at
	FROM users.student_guardian_relationships AS r
	JOIN users.student_guardian_pickup_permissions AS p
		ON p.tenant_id = r.tenant_id AND p.relationship_id = r.id
	JOIN auth.guardian_student_access AS a
		ON a.tenant_id = r.tenant_id AND a.relationship_id = r.id) AS "student_guardian"`

// StudentGuardianLinks selects every complete student-guardian link of every
// school as one row, joined from the three owner tables, for tests that assert
// what a write stored.
func StudentGuardianLinks(db bun.IDB) *bun.SelectQuery {
	return db.NewSelect().TableExpr(studentGuardianLinkSource)
}

// GuardianLinks stands in for the guardian link projection in suites that
// compose the retained People Directory repositories outside the root and
// cannot reach it, over the same three-owner row source.
type GuardianLinks struct{}

// GuardianLinkRows is the link row source, scoped to tenantID when positive.
func (GuardianLinks) GuardianLinkRows(db bun.IDB, tenantID int64) *bun.SelectQuery {
	query := StudentGuardianLinks(db)
	if tenantID <= 0 {
		return query.Err(errors.New("fixture projection: tenant is required"))
	}
	query = query.Where(`"student_guardian".tenant_id = ?`, tenantID)
	return query
}

// GuardianLinkModelRows is the link row source with BUN's model column
// selection.
func (GuardianLinks) GuardianLinkModelRows(db bun.IDB, tenantID int64, model any) *bun.SelectQuery {
	query := db.NewSelect().Model(model).ModelTableExpr(studentGuardianLinkSource)
	if tenantID <= 0 {
		return query.Err(errors.New("fixture projection: tenant is required"))
	}
	query = query.Where(`"student_guardian".tenant_id = ?`, tenantID)
	return query
}

// LoadTestStudentGuardian reads one link joined from its owners.
func LoadTestStudentGuardian(ctx context.Context, db bun.IDB, linkID int64) (*StudentGuardian, error) {
	var link StudentGuardian
	query := db.NewSelect().Model(&link).ModelTableExpr(studentGuardianLinkSource)
	if err := query.Where(`"student_guardian".id = ?`, linkID).Scan(ctx); err != nil {
		return nil, err
	}
	return &link, nil
}

// SetTestStudentGuardianPermissions replaces the portal permissions of a
// stored link on its Identity & Access row.
func SetTestStudentGuardianPermissions(ctx context.Context, db bun.IDB, linkID int64, permissions map[string]interface{}) error {
	encoded, err := json.Marshal(permissions)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `UPDATE auth.guardian_student_access SET permissions = ?::jsonb WHERE relationship_id = ?`, string(encoded), linkID)
	return err
}

// setTestStudentGuardianRole writes a link's role on its relationship.
func setTestStudentGuardianRole(ctx context.Context, db bun.IDB, linkID int64, role string) error {
	_, err := db.ExecContext(ctx, `UPDATE users.student_guardian_relationships SET guardian_role = ? WHERE id = ?`, role, linkID)
	return err
}

// SetTestStudentGuardianLinkRole re-files a stored link under the given role
// preset, rewriting its permissions exactly as the production link path does.
func SetTestStudentGuardianLinkRole(tb testing.TB, db *bun.DB, linkID int64, role string) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	link, err := LoadTestStudentGuardian(ctx, db, linkID)
	require.NoError(tb, err, "Failed to load student guardian link")
	ApplyStudentGuardianRole(link, role)
	err = db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := setTestStudentGuardianRole(ctx, tx, linkID, link.GuardianRole); err != nil {
			return err
		}
		return SetTestStudentGuardianPermissions(ctx, tx, linkID, link.Permissions)
	})
	require.NoError(tb, err, "Failed to update student guardian link role")
}

// StudentGuardianLinkGrantsPortalAccess reports whether the stored link grants
// parent_portal.access, read straight from the row so a test can pin what a
// write left behind.
func StudentGuardianLinkGrantsPortalAccess(tb testing.TB, db *bun.DB, linkID int64) bool {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	link, err := LoadTestStudentGuardian(ctx, db, linkID)
	require.NoError(tb, err, "Failed to load student guardian link")
	return GuardianPermissionsGrant(link.Permissions, securityruntime.GuardianPermissionPortalAccess)
}

// ApplyStudentGuardianRole sets a relationship's role to the preset and its
// parent-portal permissions to exactly the ones the preset grants, as the
// owner's role assignment stores them.
func ApplyStudentGuardianRole(link *StudentGuardian, role string) {
	if link == nil {
		return
	}
	normalized, granted := securityruntime.StudentGuardianRolePreset(role)
	permissions := make(map[string]interface{}, len(granted))
	for _, permission := range granted {
		permissions[permission] = true
	}
	link.GuardianRole = normalized
	link.Permissions = permissions
}

// GuardianPermissionsGrant reports whether a relationship's stored
// parent-portal permissions grant permission under the Security Runtime's
// stored-value rule.
func GuardianPermissionsGrant(permissions any, permission string) bool {
	if raw, ok := permissions.(json.RawMessage); ok {
		return securityruntime.GuardianPermissionGrantedJSON(raw, permission)
	}
	values, ok := permissions.(map[string]any)
	if !ok {
		return false
	}
	value, exists := values[permission]
	return exists && securityruntime.GuardianPermissionGranted(value)
}

// GuardianPermissionValueJSON reads the stored value without interpreting its grant.
func GuardianPermissionValueJSON(raw json.RawMessage, permission string) any {
	var values map[string]any
	if json.Unmarshal(raw, &values) != nil {
		return nil
	}
	return values[permission]
}

func (GuardianLinks) AllSchoolsGuardianLinkRows(db bun.IDB) *bun.SelectQuery {
	return StudentGuardianLinks(db)
}
func (GuardianLinks) AllSchoolsGuardianLinkModelRows(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(studentGuardianLinkSource)
}
