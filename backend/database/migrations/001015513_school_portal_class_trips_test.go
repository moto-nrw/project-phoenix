package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchoolPortalClassTripsGrantedToLehrkraftOnly pins #4799: the Lehrkraft
// role holds class_day:class_trip_write, the Betreuer role does not, and a
// second run changes nothing.
func TestSchoolPortalClassTripsGrantedToLehrkraftOnly(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	require.NoError(t, schoolPortalClassTripsUp(ctx, db))

	roleHolds := func(roleName string) int {
		var count int
		require.NoError(t, db.NewRaw(`
			SELECT COUNT(*)
			FROM auth.role_permissions rp
			JOIN auth.roles r ON r.id = rp.role_id
			JOIN auth.permissions p ON p.id = rp.permission_id
			WHERE r.name = ? AND p.name = 'class_day:class_trip_write'
		`, roleName).Scan(ctx, &count))
		return count
	}

	assert.Equal(t, 1, roleHolds("lehrkraft"), "the Lehrkraft role holds the permission exactly once")
	assert.Zero(t, roleHolds("user"), "the Betreuer role must not hold class_day:class_trip_write")
}
