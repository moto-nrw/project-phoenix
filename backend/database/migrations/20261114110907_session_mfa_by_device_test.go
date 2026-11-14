package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// TestSessionMFAByDeviceBackfillFailsClosed checks the backfill of #6955: a
// verified Träger session of an account that remembers a device at that
// Träger counts as device-proven; one without such a device, an unverified
// one and a school session stay unmarked. It runs the backfill in a
// transaction that rolls back, so the shared database keeps its rows.
func TestSessionMFAByDeviceBackfillFailsClosed(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	school := testpkg.Tenant(t)
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Gerätemerkmal", Slug: fmt.Sprintf("traeger-geraetemerkmal-%d", testpkg.UniqueTestTenantID(t)),
	})
	withDevice := testpkg.CreateTestAccount(t, db, "session-by-device").ID
	withoutDevice := testpkg.CreateTestAccount(t, db, "session-by-code").ID

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `INSERT INTO auth.mfa_trusted_devices (account_id, organization_id, token_hash, expires_at)
		VALUES (?, ?, 'session-by-device-backfill', NOW() + INTERVAL '1 day')`, withDevice, organization.ID)
	require.NoError(t, err)
	session := func(accountID int64, scope string, tenantID, organizationID any, verified bool) int64 {
		var id int64
		require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO auth.tokens (account_id, token, expiry, portal_scope, tenant_id, organization_id, mfa_verified)
			VALUES (?, encode(sha256(gen_random_uuid()::text::bytea), 'hex'), NOW() + INTERVAL '1 hour', ?, ?, ?, ?) RETURNING id`,
			accountID, scope, tenantID, organizationID, verified).Scan(&id))
		return id
	}
	deviceSession := session(withDevice, "org", nil, organization.ID, true)
	unverified := session(withDevice, "org", nil, organization.ID, false)
	schoolSession := session(withDevice, "tenant", school, nil, true)
	codeSession := session(withoutDevice, "org", nil, organization.ID, true)

	_, err = tx.ExecContext(ctx, sessionMFAByDeviceBackfillSQL)
	require.NoError(t, err)
	mark := func(id int64) bool {
		var byDevice bool
		require.NoError(t, tx.QueryRowContext(ctx, `SELECT mfa_by_device FROM auth.tokens WHERE id = ?`, id).Scan(&byDevice))
		return byDevice
	}
	assert.True(t, mark(deviceSession), "a verified Träger session of an account with a device there counts as device-proven")
	assert.False(t, mark(unverified), "an unverified session proved nothing")
	assert.False(t, mark(schoolSession), "a school session passed its own school's setting at sign-in")
	assert.False(t, mark(codeSession), "without a device at the Träger the code proved the factor")
}
