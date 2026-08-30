package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestPlanningTracksSchemaRejectsCrossTenantTemplateReference(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	group := testpkg.CreateTestActivityGroup(t, db, "PlanningTrackTenantFK")

	otherScope := testpkg.NewTenantScope(t, db)
	defer testpkg.CleanupTenantTestData(t, db, otherScope.TenantID)
	var trackID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO schedule.planning_tracks (tenant_id, name, color, sort_order)
		VALUES (?, 'Andere Schule', '#5080D8', 0)
		RETURNING id
	`, otherScope.TenantID).Scan(context.Background(), &trackID))

	_, err := db.NewUpdate().Table("activities.groups").
		Set("planning_track_id = ?", trackID).
		Where("id = ?", group.ID).
		Exec(context.Background())
	require.Error(t, err)
}

func TestPlanningTracksSchemaEnforcesActiveNameAndColor(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	defer testpkg.CleanupTenantTestData(t, db, scope.TenantID)
	ctx := context.Background()

	insert := func(name, color string) error {
		_, err := db.NewRaw(`
			INSERT INTO schedule.planning_tracks (tenant_id, name, color, sort_order)
			VALUES (?, ?, ?, 0)
		`, scope.TenantID, name, color).Exec(ctx)
		return err
	}

	require.NoError(t, insert("Früh", "#5080D8"))
	require.Error(t, insert(" früh ", "#83CD2D"))
	require.Error(t, insert("Ungültig", "blue"))
}
