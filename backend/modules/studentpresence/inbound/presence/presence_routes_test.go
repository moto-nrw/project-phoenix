// Package presence_test holds the live-group openness test and the route
// helpers the web attendance tests share.
package presence_test

import (
	"log/slog"
	"testing"
	"time"

	settingsFacts "github.com/moto-nrw/project-phoenix/modules/settings"

	rootAPI "github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/inbound/presence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Live Group Openness Tests
// =============================================================================

func TestLiveGroup_IsOpen(t *testing.T) {
	t.Parallel()

	t.Run("group with no end time is open", func(t *testing.T) {
		group := studentpresence.LiveGroup{
			RoomID: 1,
		}
		assert.True(t, group.IsOpen())
	})

	t.Run("group with end time is not open (regardless of time)", func(t *testing.T) {
		// IsOpen() returns true only when EndTime is nil
		futureTime := time.Now().Add(1 * time.Hour)
		group := studentpresence.LiveGroup{
			RoomID:  1,
			EndTime: &futureTime,
		}
		assert.False(t, group.IsOpen()) // EndTime is set, so not open
	})

	t.Run("group with past end time is not open", func(t *testing.T) {
		pastTime := time.Now().Add(-1 * time.Hour)
		group := studentpresence.LiveGroup{
			RoomID:  1,
			EndTime: &pastTime,
		}
		assert.False(t, group.IsOpen())
	})
}

// setupPresenceRoute builds the web attendance resource on the package test
// database.
func setupPresenceRoute(t *testing.T, db *testpkg.DB) *presence.Resource {
	t.Helper()

	_, serviceFactory, serviceFactoryPresence, serviceFactoryPeople, serviceFactoryEducation, serviceFactoryCaller, serviceFactoryCare := setupActiveModule(t)

	return presence.NewResource(serviceFactoryPresence.PresenceOperations(), activePeople{source: serviceFactoryPeople.AttendancePeople()}, serviceFactoryEducation.TeacherGroupIDs, activeStaffAccess{source: rootAPI.NewActiveTestAttendanceStaff(serviceFactoryCaller)}, serviceFactory.Settings, rootAPI.ProtectedTenantRoutes, slog.Default(), testPresenceQueries(t, db), requestRuntimeForTest(serviceFactoryCare.WithAttendanceStaff), authorizationForTest(), settingsFacts.NewDisplaySettings(serviceFactory.Settings))
}

func testPresenceQueries(t *testing.T, db *testpkg.DB) studentpresence.Owner {
	t.Helper()
	module, err := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: func(presenceCompose.Observation) {}})
	require.NoError(t, err)
	return module
}
