package api

import (
	"context"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Träger account has no session at the target school, so retirement
// cannot find it among the accounts it locks. The school lock must stop
// the hand-off even before retirement updates deleted_at (#6305).
func TestTraegerSchoolSwitchWhileItsSchoolEndsMintsNoSession(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	unit := testpkg.TenantRuntime(t, db)
	portal, identity := traegerPortalOn(t, db, unit)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Schulende", true)
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Schulende")
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleSchoolAccess)
	source := signInTraegerWithSecondFactor(t, portal, account.Email, password)
	family := traegerTokenClaims(t, source.AccessToken)["family_id"].(string)
	ctx, cancel := context.WithTimeout(tenant.WithUnitOfWork(context.Background(), unit), 10*time.Second)
	defer cancel()

	retirement, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = retirement.Rollback() }()
	var ended int
	require.NoError(t, retirement.NewRaw(`SELECT count(*) FROM auth.revoke_school_sessions(?)`, school).Scan(ctx, &ended))
	require.Zero(t, ended, "the switching account has no session at this school")

	done := make(chan error, 1)
	go func() {
		_, err := identity.StartTraegerSchoolSession(ctx, identityaccess.TraegerSchoolSessionRequest{
			AccountID: account.ID, OrganizationID: organization, SchoolID: school, TraegerFamilyID: family,
			IPAddress: "192.0.2.10", UserAgent: "test",
		})
		done <- err
	}()
	var switchErr error
	finished, waited := false, false
	require.Eventually(t, func() bool {
		select {
		case switchErr = <-done:
			finished = true
			return true
		default:
			var waiting int
			err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
				WHERE datname = current_database() AND pid <> pg_backend_pid() AND wait_event_type = 'Lock'`).Scan(ctx, &waiting)
			waited = err == nil && waiting > 0
			return waited
		}
	}, 5*time.Second, 10*time.Millisecond)

	_, err = retirement.NewRaw(`UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, school).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, retirement.Commit())
	if !finished {
		switchErr = <-done
	}
	assert.True(t, waited, "the switch waits for retirement before it can mint")
	assert.ErrorIs(t, switchErr, identityaccess.ErrTraegerSchoolNotFound)
	var remaining int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.tokens WHERE tenant_id = ?`, school).Scan(ctx, &remaining))
	assert.Zero(t, remaining, "no session survives retirement")
}
