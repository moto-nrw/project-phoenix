package facilities_test

import (
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/services/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupWCService creates a WC service with real database connection.
func setupWCService(t *testing.T, db *bun.DB) facilitiesSvc.WCService {
	t.Helper()

	repoFactory := repositories.NewFactory(db)

	facilityService := facilitiesSvc.NewServiceWithConfig(facilitiesSvc.ServiceConfig{
		RoomRepo: repoFactory.Room, ActiveGroupRepo: repoFactory.ActiveGroup,
	})

	activityService, err := activitiesSvc.NewService(
		repoFactory.ActivityCategory,
		repoFactory.ActivityGroup,
		repoFactory.ActivitySchedule,
		repoFactory.ActivitySupervisor,
		repoFactory.StudentEnrollment,
		repoFactory.ActiveGroup,
		repoFactory.Staff,
		repoFactory.Student,
	)
	require.NoError(t, err)

	return facilitiesSvc.NewWCService(
		facilityService,
		activityService,
		slog.Default(),
	)
}

// ============================================================================
// EnsureInfrastructure Tests
// ============================================================================

func TestWCService_EnsureInfrastructure_CreatesAll(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupWCService(t, db)
	ctx := testpkg.Ctx(t)

	// ACT
	activityGroup, err := service.EnsureInfrastructure(ctx)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, activityGroup)
	assert.Equal(t, constants.WCActivityName, activityGroup.Name)
	assert.True(t, activityGroup.IsOpen)
	assert.Equal(t, constants.WCMaxParticipants, activityGroup.MaxParticipants)
	assert.NotNil(t, activityGroup.PlannedRoomID)
	assert.Nil(t, activityGroup.CreatedBy, "system-created activity should have nil CreatedBy")
}

func TestWCService_EnsureInfrastructure_Idempotent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupWCService(t, db)
	ctx := testpkg.Ctx(t)

	// ACT - First call creates infrastructure
	group1, err := service.EnsureInfrastructure(ctx)
	require.NoError(t, err)

	// ACT - Second call returns existing infrastructure
	group2, err := service.EnsureInfrastructure(ctx)
	require.NoError(t, err)

	// ASSERT - Same activity group returned
	assert.Equal(t, group1.ID, group2.ID)
}

func TestWCService_EnsureInfrastructure_CreatesRoom(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupWCService(t, db)
	ctx := testpkg.Ctx(t)

	// ACT
	activityGroup, err := service.EnsureInfrastructure(ctx)
	require.NoError(t, err)
	require.NotNil(t, activityGroup.PlannedRoomID)

	// ASSERT - Room exists with correct properties
	var roomName string
	err = db.NewSelect().
		TableExpr("facilities.rooms").
		Column("name").
		Where("id = ?", *activityGroup.PlannedRoomID).
		Scan(ctx, &roomName)
	require.NoError(t, err, "WC room should exist in database")
	assert.Equal(t, constants.WCRoomName, roomName)
}

func TestWCService_EnsureInfrastructure_CreatesCategory(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupWCService(t, db)
	ctx := testpkg.Ctx(t)

	// ACT
	activityGroup, err := service.EnsureInfrastructure(ctx)
	require.NoError(t, err)

	// ASSERT - Category exists with correct properties
	var categoryName string
	err = db.NewSelect().
		TableExpr("activities.categories").
		Column("name").
		Where("id = ?", activityGroup.CategoryID).
		Scan(ctx, &categoryName)
	require.NoError(t, err, "WC category should exist in database")
	assert.Equal(t, constants.WCCategoryName, categoryName)
}

func TestWCService_EnsureInfrastructure_ReuseExistingRoom(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	// ARRANGE - Pre-create the WC room via raw SQL
	_, err := db.ExecContext(ctx,
		`INSERT INTO facilities.rooms (tenant_id, name, capacity, category, color) VALUES (?, ?, ?, ?, ?)`,
		testpkg.Tenant(t), constants.WCRoomName, constants.WCRoomCapacity, constants.WCCategoryName, constants.WCColor,
	)
	require.NoError(t, err)

	service := setupWCService(t, db)

	// ACT
	activityGroup, err := service.EnsureInfrastructure(ctx)

	// ASSERT - Should succeed and use the existing room
	require.NoError(t, err)
	require.NotNil(t, activityGroup)
	assert.Equal(t, constants.WCActivityName, activityGroup.Name)
}
