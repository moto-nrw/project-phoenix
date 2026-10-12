package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type trackingStudentChangeLogStore struct {
	countCalled bool
}

func (r *trackingStudentChangeLogStore) CountOlderThanByStudent(context.Context, time.Time) (map[int64]int, error) {
	r.countCalled = true
	return nil, nil
}

func (*trackingStudentChangeLogStore) DeleteOlderThan(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func TestStudentChangeLogCleanup_StopsWhenRetentionResolutionFails(t *testing.T) {
	t.Parallel()

	resolveErr := errors.New("settings unavailable")
	store := &trackingStudentChangeLogStore{}
	service := NewStudentChangeLogCleanupService(
		store,
		nil,
		func(context.Context) (int, error) { return 0, resolveErr },
		nil,
	)
	ctx := tenant.WithTenantID(context.Background(), 42)

	result, err := service.CleanupExpiredChangeLog(ctx)

	require.ErrorIs(t, err, resolveErr)
	assert.Nil(t, result)
	assert.False(t, store.countCalled, "cleanup must not inspect or delete rows without a retention value")
}

func TestStudentChangeLogCleanup_StopsWhenSettingsAreNotConfigured(t *testing.T) {
	t.Parallel()

	store := &trackingStudentChangeLogStore{}
	service := NewStudentChangeLogCleanupService(store, nil, nil, nil)
	ctx := tenant.WithTenantID(context.Background(), 42)

	result, err := service.CleanupExpiredChangeLog(ctx)

	require.ErrorContains(t, err, "settings service not configured")
	assert.Nil(t, result)
	assert.False(t, store.countCalled, "cleanup must not inspect or delete rows without configured settings")
}
