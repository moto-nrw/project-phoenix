package application

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

// #3873: the worker settles the meal orders of every school and keeps going
// when one school fails.
func TestMealOrdersSettleEverySchool(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	calls := 0
	s := unitScheduler(&unitSchedulerDependencies{
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
