package enrollment_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentRepo "github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

func TestPhaseExpiryRepository_ListSnapshots_RequiresDatesAndTenant(t *testing.T) {
	t.Parallel()

	repo := enrollmentRepo.NewPhaseExpiryRepository(nil)
	_, err := repo.ListSnapshots(context.Background(), timezone.Date{}, timezone.Date{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dates are required")

	_, err = repo.ListSnapshots(
		context.Background(),
		timezone.NewDate(2027, 2, 1),
		timezone.NewDate(2027, 1, 31),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "horizon")

	_, err = repo.ListSnapshots(
		context.Background(),
		timezone.NewDate(2027, 1, 2),
		timezone.NewDate(2027, 2, 1),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant context")
}
