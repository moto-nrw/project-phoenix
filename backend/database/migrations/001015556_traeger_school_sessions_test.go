package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTraegerSchoolSessionNamesSchoolAndTraeger pins the Absprung session of
// 1.15.556 (#4654): an OGS session may name the Träger it came through, but
// still needs its school.
func TestTraegerSchoolSessionNamesSchoolAndTraeger(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	school := testpkg.Tenant(t)
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Absprung", Slug: fmt.Sprintf("traeger-absprung-%d", testpkg.UniqueTestTenantID(t))})
	accountID := testpkg.CreateTestAccount(t, db, "traeger-absprung").ID
	insert := func(tenantID, organizationID any) error {
		_, err := db.ExecContext(ctx, `INSERT INTO auth.tokens (account_id, token, expiry, portal_scope, tenant_id, organization_id)
			VALUES (?, encode(sha256(gen_random_uuid()::text::bytea), 'hex'), NOW() + INTERVAL '1 hour', 'tenant', ?, ?)`, accountID, tenantID, organizationID)
		return err
	}
	assert.NoError(t, insert(school, organization.ID), "an OGS session names the Träger of its Absprung")
	assert.ErrorContains(t, insert(nil, organization.ID), "chk_tokens_session_owner", "an OGS session of the Träger still names its school")
	_, err := db.ExecContext(ctx, `DELETE FROM auth.tokens WHERE account_id = ?`, accountID)
	require.NoError(t, err)
}
