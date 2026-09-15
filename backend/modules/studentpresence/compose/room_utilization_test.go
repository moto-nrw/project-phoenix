package compose_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestRoomUtilizationRequiresTenantAndValidWindows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	_, err = module.RoomUtilization(context.Background(), nil)
	require.Error(t, err, "even an empty report requires a tenant")
	rows, err := module.RoomUtilization(testpkg.Ctx(t), nil)
	require.NoError(t, err)
	require.Empty(t, rows)
	_, err = module.RoomUtilization(testpkg.Ctx(t), []studentpresence.StudentVisitWindow{{StudentID: 1}})
	require.Error(t, err, "invalid intervals must not reach the aggregate")
	student := testpkg.CreateTestStudent(t, db, "Visit", "Window", "3a")
	start := testpkg.TodayDate().BerlinMidnight()
	window := studentpresence.StudentVisitWindow{StudentID: student.ID, StartAt: start, EndAt: start.AddDate(0, 0, 1)}
	_, err = module.RoomUtilization(testpkg.Ctx(t), []studentpresence.StudentVisitWindow{window, window})
	require.ErrorContains(t, err, "overlapping", "duplicate windows must not double-count a visit")
}
