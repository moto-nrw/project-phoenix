package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrackingIndicatorsResolveTodaysActivityAndRoomNames(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	labels := []string{"VisitActivity", "VisitRoom"}
	result, err := svc.GetTrackingIndicators(ctx, nil, labels)
	require.NoError(t, err)
	require.Empty(t, result)

	data := newVisitProjectionFixture(t, db)
	testpkg.CreateTestVisit(t, db, data.Student1, data.GroupID, timezone.Today().Add(30*time.Minute), nil)
	result, err = svc.GetTrackingIndicators(ctx, []int64{data.Student1, data.Student1}, labels)
	require.NoError(t, err)
	require.Contains(t, result, data.Student1)
	assert.Equal(t, []bool{true, true}, result[data.Student1], "both activity and room names must resolve for today's visit")
}
