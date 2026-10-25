package application

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
)

// #3873: the worker settles the meal orders of every school and keeps going
// when one school fails.
func TestMealOrdersSettleEverySchool(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	calls := 0
	s := unitScheduler(&unitSchedulerDependencies{
		settings: bookedModules(),
		mealOrders: func(context.Context) error {
			calls++
			return errors.New("settle failed")
		},
		tasks:  make(map[string]*ScheduledTask),
		logger: slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}))})

	s.checkAndRunMealOrders(context.Background(), &ScheduledTask{Name: "meal-orders"})

	assert.Equal(t, 1, calls)
	assert.Contains(t, output.String(), `"msg":"tenant operation failed, continuing to next tenant"`)
	assert.Contains(t, output.String(), `"error":"settle failed"`)
}

func TestMealOrdersRespectModuleBooking(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		readErr error
		want    []int64
	}{
		{name: "only booked schools", want: []int64{41}},
		{name: "unreadable switch", readErr: errors.New("settings unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			settings := &moduleSwitches{off: map[int64][]string{42: {"module.mittagessen"}}, readErr: tc.readErr}
			s := moduleScheduler(settings, 41, 42)
			var ran []int64
			s.mealOrders = func(ctx context.Context) error {
				ran = append(ran, testpkg.TenantIDFromContext(ctx))
				return nil
			}

			s.checkAndRunMealOrders(context.Background(), &ScheduledTask{Name: "meal-orders"})

			assert.Equal(t, tc.want, ran)
		})
	}
}

func TestMealOrdersSkipAWhileRunning(t *testing.T) {
	t.Parallel()

	calls := 0
	s := unitScheduler(&unitSchedulerDependencies{
		mealOrders: func(context.Context) error { calls++; return nil },
		tasks:      make(map[string]*ScheduledTask),
		logger:     slog.New(slog.DiscardHandler)})
	task := &ScheduledTask{Name: "meal-orders", Running: true}

	s.checkAndRunMealOrders(context.Background(), task)

	assert.Zero(t, calls, "an overlapping tick waits for the running one")
}
