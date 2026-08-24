package schedule_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// TestLockCareExceptionDay_RequiresTenant verifies the lock refuses to run
// without a tenant in context. The advisory key is tenant-scoped, so a missing
// tenant id is a programming error (the lock is always taken inside a tenant
// transaction) — failing fast here prevents two tenants from colliding on the
// same student_id+date key. The tenant check runs before any DB access, so this
// needs no database.
func TestLockCareExceptionDay_RequiresTenant(t *testing.T) {
	t.Parallel()

	err := scheduleService.LockCareExceptionDay(context.Background(), nil, 123, timezone.TodayDate())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant id is required")
}
