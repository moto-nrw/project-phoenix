package users

import (
	"context"
	"errors"
	"testing"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingChangeLogSettings struct {
	configService.SettingsService
	err error
}

func (f failingChangeLogSettings) ResolveInt(context.Context, string) (int, error) {
	return 0, f.err
}

type trackingStudentFieldEditRepo struct {
	countCalled bool
}

func (*trackingStudentFieldEditRepo) CreateBatch(context.Context, []*auditModels.StudentFieldEdit) error {
	return nil
}

func (*trackingStudentFieldEditRepo) GetByStudentID(context.Context, int64) ([]*auditModels.StudentFieldEdit, error) {
	return nil, nil
}

func (r *trackingStudentFieldEditRepo) CountOlderThanByStudent(context.Context, time.Time) (map[int64]int, error) {
	r.countCalled = true
	return nil, nil
}

func (*trackingStudentFieldEditRepo) DeleteOlderThan(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func TestStudentChangeLogCleanup_StopsWhenRetentionResolutionFails(t *testing.T) {
	resolveErr := errors.New("settings unavailable")
	repo := &trackingStudentFieldEditRepo{}
	service := NewStudentChangeLogCleanupService(
		repo,
		nil,
		failingChangeLogSettings{err: resolveErr},
		nil,
	)
	ctx := tenant.WithTenantID(context.Background(), 42)

	result, err := service.CleanupExpiredChangeLog(ctx)

	require.ErrorIs(t, err, resolveErr)
	assert.Nil(t, result)
	assert.False(t, repo.countCalled, "cleanup must not inspect or delete rows without a retention value")
}
