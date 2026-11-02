package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPayrollDispatchTenantFailureIsReportedToTheScheduler(t *testing.T) {
	t.Parallel()

	failure := errors.New("payroll dispatch unavailable")
	scheduler := unitScheduler(&unitSchedulerDependencies{
		allTenantIDsLoader: func(context.Context) ([]int64, error) { return []int64{42}, nil },
		logger:             slog.Default(),
	})
	scheduler.payrollDispatch = func(context.Context) (int, error) { return 0, failure }

	err := scheduler.forEachTenantIncludingInactive(context.Background(), "payroll-dispatch", scheduler.runPayrollDispatchForTenant)

	require.ErrorIs(t, err, failure)
	_, advanced := scheduler.tenantBatchCursors.Load("payroll-dispatch")
	assert.False(t, advanced, "a failed tenant must not advance the scheduler cursor")
}
