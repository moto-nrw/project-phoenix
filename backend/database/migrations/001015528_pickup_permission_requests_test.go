package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPickupPermissionRequestsSchema pins #4780 on the migrated database: the
// pickup permission's weekdays (NULL = every day, else 1 … 5, at least one),
// the request kind and one pending request per child and person.
func TestPickupPermissionRequestsSchema(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := t.Context()

	var columnType string
	require.NoError(t, db.NewRaw(`SELECT format_type(a.atttypid, a.atttypmod)
		FROM pg_attribute a WHERE a.attrelid = 'users.student_guardian_pickup_permissions'::regclass
		AND a.attname = 'pickup_weekdays' AND NOT a.attisdropped`).Scan(ctx, &columnType))
	assert.Equal(t, "smallint[]", columnType)

	var weekdays string
	require.NoError(t, db.NewRaw(`SELECT pg_get_constraintdef(oid) FROM pg_constraint
		WHERE conname = 'student_guardian_pickup_permissions_weekdays_check'`).Scan(ctx, &weekdays))
	assert.Contains(t, weekdays, "cardinality(pickup_weekdays)")
	assert.Contains(t, weekdays, "(5)::smallint")

	var kinds string
	require.NoError(t, db.NewRaw(`SELECT pg_get_constraintdef(oid) FROM pg_constraint
		WHERE conname = 'care_schedule_change_requests_request_kind_check'`).Scan(ctx, &kinds))
	for _, kind := range []string{"weekly_schedule", "pickup_change", "pickup_permission"} {
		assert.Contains(t, kinds, kind)
	}

	var index string
	require.NoError(t, db.NewRaw(`SELECT indexdef FROM pg_indexes WHERE schemaname = 'schedule'
		AND indexname = 'uniq_care_schedule_change_requests_pending_pickup_permission'`).Scan(ctx, &index))
	assert.Contains(t, index, "UNIQUE")
	assert.Contains(t, index, "guardian_profile_id")
	assert.Contains(t, index, "pickup_permission")
}
