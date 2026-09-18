package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// A guardian invitation is a credential for exactly one school. This is the
// table-level check behind the flow tests: inside a phoenix_tenant
// transaction — what every request runs in — a school sees its own
// invitation row and not another school's, by listing, by id and by token.
func TestGuardianInvitationsAreIsolatedPerSchool(t *testing.T) {
	t.Parallel()
	db := SetupTestDB(t)

	tenantA := Tenant(t)
	tenantB, _ := CreateTestTenant(t, db)
	require.NotEqual(t, tenantA, tenantB)
	EnsureTestTenant(t, db, tenantA)
	EnsureTestTenant(t, db, tenantB)

	inviter := CreateTestAccount(t, db, "guardian-invitation-isolation")
	type invitation struct {
		id    int64
		token string
	}
	invite := func(tenantID int64, label string) invitation {
		profile := CreateTestGuardianProfileForTenant(t, db, tenantID, "Iso", label, "guardian-isolation-"+label)
		token := fmt.Sprintf("guardian-isolation-%s-%d", label, time.Now().UnixNano())
		var id int64
		require.NoError(t, db.NewRaw(`
			INSERT INTO auth.guardian_invitations (tenant_id, token, guardian_profile_id, created_by, expires_at)
			VALUES (?, ?, ?, ?, ?)
			RETURNING id`,
			tenantID, token, profile.ID, inviter.ID, time.Now().Add(48*time.Hour),
		).Scan(context.Background(), &id))
		t.Cleanup(func() {
			_, _ = db.NewDelete().TableExpr("auth.guardian_invitations").Where("id = ?", id).Exec(context.Background())
		})
		return invitation{id: id, token: token}
	}
	own := invite(tenantA, "a")
	foreign := invite(tenantB, "b")

	assertSeesOnly := func(t *testing.T, tenantID int64, own, foreign invitation) {
		t.Helper()
		require.NoError(t, WithTenantTx(t, context.Background(), db, tenantID, func(txCtx context.Context, tx bun.Tx) error {
			var visible []int64
			require.NoError(t, tx.NewRaw(`SELECT id FROM auth.guardian_invitations ORDER BY id`).Scan(txCtx, &visible))
			assert.Contains(t, visible, own.id, "a school must see its own invitation")
			assert.NotContains(t, visible, foreign.id, "another school's invitation leaked into the listing")

			var byID int64
			require.NoError(t, tx.NewRaw(`SELECT COUNT(*) FROM auth.guardian_invitations WHERE id = ?`, foreign.id).Scan(txCtx, &byID))
			assert.Zero(t, byID, "another school's invitation must not be readable by id")

			// The token is the credential, so the lookup behind the public
			// routes must not answer for a foreign school inside a tenant
			// session either.
			var byToken int64
			require.NoError(t, tx.NewRaw(`SELECT COUNT(*) FROM auth.guardian_invitations WHERE token = ?`, foreign.token).Scan(txCtx, &byToken))
			assert.Zero(t, byToken, "another school's invitation must not be readable by token")
			return nil
		}))
	}

	assertSeesOnly(t, tenantA, own, foreign)
	assertSeesOnly(t, tenantB, foreign, own)
}
