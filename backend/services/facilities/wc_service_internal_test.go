package facilities

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

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
		db,
	)

	activityService, err := activitiesSvc.NewService(
		repoFactory.ActivityCategory,
		repoFactory.ActivityGroup,
		repoFactory.ActivitySchedule,
		repoFactory.ActivitySupervisor,
		repoFactory.StudentEnrollment,
		repoFactory.ActiveGroup,
		db,
	)
	require.NoError(t, err)

	return &wcService{
		facilityService: facilityService,
		activityService: activityService,
		logger:          slog.Default(),
	}
}

func cleanupWCArtifactsInternal(t *testing.T, db *bun.DB) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stmts := []string{
		`DELETE FROM active.groups WHERE room_id IN (SELECT id FROM facilities.rooms WHERE name = '` + constants.WCRoomName + `')`,
		`DELETE FROM activities.schedules WHERE activity_group_id IN (SELECT id FROM activities.groups WHERE name = '` + constants.WCActivityName + `')`,
		`DELETE FROM activities.student_enrollments WHERE activity_group_id IN (SELECT id FROM activities.groups WHERE name = '` + constants.WCActivityName + `')`,
		`DELETE FROM activities.groups WHERE name = '` + constants.WCActivityName + `'`,
		`DELETE FROM activities.categories WHERE name = '` + constants.WCCategoryName + `'`,
		`DELETE FROM facilities.rooms WHERE name = '` + constants.WCRoomName + `'`,
	}

	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Logf("wc cleanup: %v (stmt: %s)", err, stmt)
		}
	}
}

func TestWCService_findWCActivity_NotFoundSentinel(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupWCArtifactsInternal(t, db)
	defer cleanupWCArtifactsInternal(t, db)

	service := setupWCServiceInternal(t, db)

	activityGroup, err := service.findWCActivity(testpkg.TenantContext(1))

	require.Nil(t, activityGroup)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errWCActivityNotFound))
}

func TestWCService_EnsureInfrastructure_PropagatesActivityLookupErrors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupWCArtifactsInternal(t, db)
	defer cleanupWCArtifactsInternal(t, db)

	service := setupWCServiceInternal(t, db)

	ctx, cancel := context.WithCancel(testpkg.TenantContext(1))
	cancel()

	activityGroup, err := service.EnsureInfrastructure(ctx)

	require.Nil(t, activityGroup)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to look up WC activity")
}

func TestWCService_ensureWCRoom_PropagatesLookupErrors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupWCArtifactsInternal(t, db)
	defer cleanupWCArtifactsInternal(t, db)

	service := setupWCServiceInternal(t, db)

	ctx, cancel := context.WithCancel(testpkg.TenantContext(1))
	cancel()

	room, err := service.ensureWCRoom(ctx)

	require.Nil(t, room)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to look up WC room")
}
