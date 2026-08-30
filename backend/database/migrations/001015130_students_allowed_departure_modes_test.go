package migrations

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func allowedDepartureModesColumnExists(t *testing.T, db *testpkg.DB) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.NewRaw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'users' AND table_name = 'students'
				AND column_name = 'allowed_departure_modes'
		)
	`).Scan(context.Background(), &exists))
	return exists
}

func TestStudentsAllowedDepartureModesMigration_BackfillsFromLegacyMaps(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	if !allowedDepartureModesColumnExists(t, db) {
		require.NoError(t, studentsAllowedDepartureModesUp(ctx, db))
	}

	tenantID := time.Now().UnixNano()
	testpkg.EnsureTestTenant(t, db, tenantID)
	t.Cleanup(func() {
		testpkg.CleanupTenantTestData(t, db, tenantID)
		if !allowedDepartureModesColumnExists(t, db) {
			require.NoError(t, studentsAllowedDepartureModesUp(ctx, db))
		}
	})

	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Allowed", "Modes", "3a")
	_, err := db.NewRaw(`
		UPDATE users.students
		SET bus_days = '{"mon":true,"tue":true}'::jsonb,
			pickup_days = '{"mon":true}'::jsonb,
			departure_days = '{"mon":"pickup","tue":"bus","wed":"alone"}'::jsonb,
			allowed_departure_modes = '{}'::jsonb
		WHERE id = ?
	`, student.ID).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, studentsAllowedDepartureModesUp(ctx, db))

	var row struct {
		AllowedDepartureModes json.RawMessage `bun:"allowed_departure_modes"`
	}
	require.NoError(t, db.NewRaw(`SELECT allowed_departure_modes FROM users.students WHERE id = ?`, student.ID).Scan(ctx, &row))
	assert.JSONEq(t, `{"mon":["bus","pickup"],"tue":["bus"],"wed":["alone"]}`, string(row.AllowedDepartureModes))
}
