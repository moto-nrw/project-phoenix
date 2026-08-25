package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories/active"
)

func TestStatisticsRepository_RoomUtilizationRequiresTenantContext(t *testing.T) {
	t.Parallel()

	repo := active.NewStatisticsRepository(nil)
	_, err := repo.RoomUtilization(context.Background(), time.Now().Add(-time.Hour), time.Now(), nil)

	require.ErrorContains(t, err, "requires a tenant context")
}
