package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type preparationFunc func(context.Context, time.Time, time.Time) (int, func(context.Context) error, error)

func (f preparationFunc) PrepareAppointmentReminders(ctx context.Context, from, to time.Time) (int, func(context.Context) error, error) {
	return f(ctx, from, to)
}

type transactionMarker struct{}

func TestCommandCommitsBeforeDispatchAndPreservesFailures(t *testing.T) {
	t.Parallel()
	failure := errors.New("injected reminder failure")
	for _, stage := range []string{"success", "prepare", "commit", "dispatch"} {
		t.Run(stage, func(t *testing.T) {
			var order []string
			var afterCommit func()
			runtime := ports.CommandRuntime{
				TenantID: func(context.Context) int64 { return 7 },
				Detached: func(ctx context.Context) context.Context {
					return context.WithValue(ctx, transactionMarker{}, false)
				},
				WithinTenant: func(ctx context.Context, id int64, fn func(context.Context) error) error {
					assert.EqualValues(t, 7, id)
					assert.Equal(t, false, ctx.Value(transactionMarker{}), "detach the outer scheduler transaction")
					order = append(order, "begin")
					if err := fn(context.WithValue(ctx, transactionMarker{}, true)); err != nil {
						order = append(order, "rollback")
						return err
					}
					if stage == "commit" {
						order = append(order, "commit_failed")
						return failure
					}
					order = append(order, "commit")
					if afterCommit != nil {
						afterCommit()
					}
					return nil
				},
				AfterCommit: func(ctx context.Context, fn func()) {
					assert.Equal(t, true, ctx.Value(transactionMarker{}))
					afterCommit = fn
				},
			}
			from := time.Date(2026, time.September, 4, 10, 0, 0, 0, time.UTC)
			to := from.Add(time.Hour)
			source := preparationFunc(func(ctx context.Context, gotFrom, gotTo time.Time) (int, func(context.Context) error, error) {
				assert.Equal(t, true, ctx.Value(transactionMarker{}), "prepare email and claims inside the same transaction")
				assert.Equal(t, from, gotFrom)
				assert.Equal(t, to, gotTo)
				order = append(order, "prepare")
				if stage == "prepare" {
					return 2, nil, failure
				}
				return 2, func(ctx context.Context) error {
					assert.Equal(t, false, ctx.Value(transactionMarker{}), "dispatch must not retain a committed transaction")
					order = append(order, "dispatch")
					if stage == "dispatch" {
						return failure
					}
					return nil
				}, nil
			})
			ctx := context.WithValue(context.Background(), transactionMarker{}, true)
			queued, err := NewCommand(runtime, source).EnqueueDueAppointmentReminders(ctx, from, to)
			assert.Equal(t, 2, queued, "preserve the historical count even when a later stage fails")
			if stage == "success" {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, failure)
			}
			switch stage {
			case "prepare":
				assert.Equal(t, []string{"begin", "prepare", "rollback"}, order)
			case "commit":
				assert.Equal(t, []string{"begin", "prepare", "commit_failed"}, order)
			default:
				assert.Equal(t, []string{"begin", "prepare", "commit", "dispatch"}, order)
			}
		})
	}
}
