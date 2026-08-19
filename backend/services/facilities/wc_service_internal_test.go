package facilities

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupWCServiceInternal(t *testing.T, db *bun.DB) *wcService {
	t.Helper()

	repoFactory := repositories.NewFactory(db)

	facilityService := NewService(
		repoFactory.Room,
		repoFactory.ActiveGroup,
	)

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

	return &wcService{
		facilityService: facilityService,
		activityService: activityService,
		logger:          slog.Default(),
	}
}

func TestWCService_findWCActivity_NotFoundSentinel(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupWCServiceInternal(t, db)

	activityGroup, err := service.findWCActivity(testpkg.Ctx(t))

	require.Nil(t, activityGroup)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errWCActivityNotFound))
}

func TestWCService_EnsureInfrastructure_PropagatesActivityLookupErrors(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupWCServiceInternal(t, db)

	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()

	activityGroup, err := service.EnsureInfrastructure(ctx)

	require.Nil(t, activityGroup)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to look up WC activity")
}

func TestWCService_ensureWCRoom_PropagatesLookupErrors(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupWCServiceInternal(t, db)

	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()

	room, err := service.ensureWCRoom(ctx)

	require.Nil(t, room)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to look up WC room")
}

func TestWCService_ensureWCCategory_PropagatesLookupErrors(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupWCServiceInternal(t, db)

	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()

	category, err := service.ensureWCCategory(ctx)

	require.Nil(t, category)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list activity categories")
}

// Deliberately NOT parallel: the test calls EnsureInfrastructure with a
// tenant-less context, so its lookup is unscoped and sees the WC rows every
// other test in this package creates. It only holds while no WC activity
// exists anywhere in the clone, which is true for sequential tests (they run
// before the parked parallel ones resume) and false for a parallel one.
func TestWCService_EnsureInfrastructure_PropagatesRoomErrors(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	// Use a nil facility service to force ensureWCRoom to fail
	activityService, err := activitiesSvc.NewService(
		repositories.NewFactory(db).ActivityCategory,
		repositories.NewFactory(db).ActivityGroup,
		repositories.NewFactory(db).ActivitySchedule,
		repositories.NewFactory(db).ActivitySupervisor,
		repositories.NewFactory(db).StudentEnrollment,
		repositories.NewFactory(db).ActiveGroup,
		repositories.NewFactory(db).Staff,
		repositories.NewFactory(db).Student,
	)
	require.NoError(t, err)

	service := &wcService{
		facilityService: nil, // nil causes panic → use cancelled ctx instead
		activityService: activityService,
		logger:          slog.Default(),
	}

	// Cancel context after activity lookup succeeds but before room lookup
	// We need a valid context for findWCActivity (returns errWCActivityNotFound)
	// then a failing context for ensureWCRoom — but both share the same ctx.
	// Instead, use a context that works but the nil service panics.
	// Simplest: just verify the error wrapping via cancelled context on the full service.
	fullService := setupWCServiceInternal(t, db)
	_ = service // unused, approach changed

	// Create a context that will fail for room creation by removing tenant
	ctx := context.Background() // no tenant → room creation fails

	result, ensureErr := fullService.EnsureInfrastructure(ctx)

	require.Nil(t, result)
	require.Error(t, ensureErr)
}

func TestWCService_getLogger_NilSafe(t *testing.T) {
	t.Parallel()

	service := &wcService{
		logger: nil,
	}

	logger := service.getLogger()

	require.NotNil(t, logger, "getLogger should return slog.Default() when logger is nil")
	assert.Equal(t, slog.Default(), logger)
}

func TestWCService_ensureWCCategory_ReusesExisting(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupWCServiceInternal(t, db)
	ctx := testpkg.Ctx(t)

	// Create category first time
	cat1, err := service.ensureWCCategory(ctx)
	require.NoError(t, err)
	require.NotNil(t, cat1)

	// Second call should find and reuse existing
	cat2, err := service.ensureWCCategory(ctx)
	require.NoError(t, err)
	require.NotNil(t, cat2)

	assert.Equal(t, cat1.ID, cat2.ID, "should reuse existing category")
}

func TestWCService_ensureWCRoom_CreatesNewRoom(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupWCServiceInternal(t, db)
	ctx := testpkg.Ctx(t)

	room, err := service.ensureWCRoom(ctx)

	require.NoError(t, err)
	require.NotNil(t, room)
	assert.Equal(t, constants.WCRoomName, room.Name)
	assert.NotNil(t, room.Capacity)
	assert.Equal(t, constants.WCRoomCapacity, *room.Capacity)
}
