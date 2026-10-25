package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// CreateTestOperator inserts a platform operator with a unique email and
// registers cleanup via tb.Cleanup. Sites that need an int64 use the ID field.
func CreateTestOperator(tb testing.TB, db *bun.DB) *Operator {
	tb.Helper()
	return CreateTestOperatorWithEmail(tb, db,
		fmt.Sprintf("op-%d@test.local", uniqueFixtureSuffix()), "Test Operator")
}

// CreateTestOperatorWithEmail inserts a platform operator with the given email
// and display name and registers cleanup via tb.Cleanup.
func CreateTestOperatorWithEmail(tb testing.TB, db *bun.DB, email, displayName string) *Operator {
	tb.Helper()

	op := &Operator{
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: "$argon2id$v=19$m=65536,t=3,p=2$placeholder$placeholder",
		Active:       true,
	}

	_, err := db.NewInsert().
		Model(op).
		ModelTableExpr("platform.operators").
		Returning("*").
		Exec(context.Background())
	require.NoError(tb, err, "Failed to create test operator")

	OwnTestOperator(tb, db, op.ID)

	return op
}

// CreateTestOrganization inserts an organization fixture and registers exact-ID cleanup.
func CreateTestOrganization(tb testing.TB, db *bun.DB, organization *Organization) *Organization {
	tb.Helper()
	require.NotNil(tb, organization)
	_, err := db.NewInsert().
		Model(organization).
		ModelTableExpr("platform.organizations").
		Returning("*").
		Exec(context.Background())
	require.NoError(tb, err, "Failed to create test organization")
	tb.Cleanup(func() {
		ctx := context.Background()
		var tenantIDs []int64
		cleanupErr := db.NewSelect().
			TableExpr("platform.schools").
			Column("id").
			Where("organization_id = ?", organization.ID).
			Scan(ctx, &tenantIDs)
		require.NoError(tb, cleanupErr, "Failed to find test organization schools")
		cleanupTenantTestData(tb, db, tenantIDs...)
		_, cleanupErr = db.NewDelete().
			Model((*Organization)(nil)).
			ModelTableExpr("platform.organizations").
			Where("id = ?", organization.ID).
			Exec(ctx)
		require.NoError(tb, cleanupErr, "Failed to clean up test organization")
	})
	return organization
}

// OwnTestOperator registers exact-ID teardown for an operator created through
// a service or repository path.
func OwnTestOperator(tb testing.TB, db *bun.DB, operatorID int64) {
	tb.Helper()
	tb.Cleanup(func() { cleanupOperator(tb, db, operatorID) })
}

// OwnTestOperatorPasskeySession removes an operator passkey ceremony when
// the test ends. A discoverable login names no operator, so its session
// does not cascade with the operator fixture.
func OwnTestOperatorPasskeySession(tb testing.TB, db *bun.DB, sessionID string) {
	tb.Helper()
	tb.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM platform.operator_passkey_sessions WHERE id = ?`, sessionID)
		require.NoError(tb, err)
	})
}

// OwnTestAccountPasskeySession removes a school-portal passkey ceremony
// when the test ends. A discoverable login names no account, so its session
// does not cascade with an account fixture.
func OwnTestAccountPasskeySession(tb testing.TB, db *bun.DB, sessionID string) {
	tb.Helper()
	tb.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM auth.passkey_sessions WHERE id = ?`, sessionID)
		require.NoError(tb, err)
	})
}

// cleanupOperator removes an operator and its audit-log rows. All other
// operator-scoped tables (tokens, MFA, passkeys) cascade on delete; rows in
// domain tables referencing the operator (announcements)
// must be removed by the caller's own cleanup first.
func cleanupOperator(tb testing.TB, db *bun.DB, operatorID int64) {
	tb.Helper()
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(tb, err)
	defer func() { _ = tx.Rollback() }()

	// MFA audit writes run asynchronously. Lock the operator first so an audit
	// insert cannot land between deleting its logs and deleting the operator.
	_, err = tx.ExecContext(ctx, `SELECT id FROM platform.operators WHERE id = ? FOR UPDATE`, operatorID)
	require.NoError(tb, err)
	_, err = tx.ExecContext(ctx, `DELETE FROM platform.operator_audit_log WHERE operator_id = ?`, operatorID)
	require.NoError(tb, err)
	_, err = tx.ExecContext(ctx, `DELETE FROM platform.operators WHERE id = ?`, operatorID)
	require.NoError(tb, err)
	require.NoError(tb, tx.Commit())
}

// CreateTestOperatorWithPassword inserts a platform operator whose password
// hash verifies the given password (cheap Argon2id parameters), for the
// operator login and password-change flows, and registers cleanup.
func CreateTestOperatorWithPassword(tb testing.TB, db *bun.DB, email, password string) *Operator {
	tb.Helper()

	hash, err := hashPassword(password)
	require.NoError(tb, err, "Failed to hash operator password")
	op := &Operator{
		Email:        email,
		DisplayName:  "Test Operator",
		PasswordHash: hash,
		Active:       true,
	}
	_, err = db.NewInsert().
		Model(op).
		ModelTableExpr("platform.operators").
		Returning("*").
		Exec(context.Background())
	require.NoError(tb, err, "Failed to create test operator with password")

	OwnTestOperator(tb, db, op.ID)

	return op
}

// CreateTestSchoolInOrganization inserts a live school of the organization
// and returns its ID (= tenant ID). The organization fixture's cleanup removes
// it with the organization's other schools.
func CreateTestSchoolInOrganization(tb testing.TB, db *bun.DB, organizationID int64, name string) int64 {
	tb.Helper()
	schoolID := uniqueJWTSafeTenantID()
	token := fmt.Sprintf("%d-%d", schoolID, uniqueFixtureSuffix())
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO platform.schools (id, organization_id, name, slug, subdomain, active)
		VALUES (?, ?, ?, ?, ?, true)`,
		schoolID, organizationID, name, "test-school-"+token, fmt.Sprintf("t%d", schoolID))
	require.NoError(tb, err, "Failed to create test school in organization")
	return schoolID
}

// CreateTestTraegerMember grants the account an active Träger membership
// with the given Träger roles (ADR 0084) and removes both when the test
// ends. The rows carry no tenant, and the tenant cleanup deletes an emptied
// organization with foreign-key triggers suspended, so they need their own
// exact-key teardown.
func CreateTestTraegerMember(tb testing.TB, db *bun.DB, organizationID, accountID int64, roles ...string) {
	tb.Helper()
	ctx := context.Background()
	tb.Cleanup(func() { cleanupTraegerMember(tb, db, organizationID, accountID) })
	_, err := db.ExecContext(ctx, `
		INSERT INTO platform.organization_accounts (organization_id, account_id, status, activated_at)
		VALUES (?, ?, 'active', NOW())`, organizationID, accountID)
	require.NoError(tb, err, "Failed to create test Träger membership")
	for _, role := range roles {
		_, err = db.ExecContext(ctx, `
			INSERT INTO auth.organization_account_roles (organization_id, account_id, role)
			VALUES (?, ?, ?)`, organizationID, accountID, role)
		require.NoError(tb, err, "Failed to create test Träger role")
	}
}

// cleanupTraegerMember removes one Träger membership with its roles.
func cleanupTraegerMember(tb testing.TB, db *bun.DB, organizationID, accountID int64) {
	tb.Helper()
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `DELETE FROM auth.organization_account_roles WHERE organization_id = ? AND account_id = ?`, organizationID, accountID)
	require.NoError(tb, err, "Failed to clean up test Träger roles")
	_, err = db.ExecContext(ctx, `DELETE FROM platform.organization_accounts WHERE organization_id = ? AND account_id = ?`, organizationID, accountID)
	require.NoError(tb, err, "Failed to clean up test Träger membership")
}

// OwnTraegerHolidayPeriods removes the holiday periods a test creates for the
// Träger, with their weeks and time models, when the test ends (#4659). The
// rows carry no tenant, and the tenant cleanup deletes an emptied
// organization with foreign-key triggers suspended, so they need their own
// exact-key teardown.
func OwnTraegerHolidayPeriods(tb testing.TB, db *bun.DB, organizationID int64) {
	tb.Helper()
	tb.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM enrollment.holiday_periods WHERE organization_id = ?`, organizationID)
		require.NoError(tb, err, "Failed to clean up test holiday periods")
	})
}

// SoftDeleteTestSchool marks a school deleted, as the operator's school
// deletion does; its rows stay with its tenant cleanup.
func SoftDeleteTestSchool(tb testing.TB, db *bun.DB, schoolID int64) {
	tb.Helper()
	_, err := db.ExecContext(context.Background(), `UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, schoolID)
	require.NoError(tb, err, "Failed to soft-delete test school")
}
