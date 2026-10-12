package presenceprojection

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func TestWeekendFollowsFridayIn(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.September, 7)
	t.Run("weekday window does not resolve the setting", func(t *testing.T) {
		calls := 0
		ctx := calendar.WithWeekendPlan(context.Background(), func(context.Context) (bool, error) {
			calls++
			return true, nil
		})

		follows, err := weekendFollowsFridayIn(ctx, monday, monday.AddDays(4))
		if err != nil || follows || calls != 0 {
			t.Fatalf("weekday window = %v, %v after %d setting reads; want false, nil without reads", follows, err, calls)
		}
	})

	t.Run("weekend window resolves the setting", func(t *testing.T) {
		ctx := calendar.WithWeekendPlan(context.Background(), func(context.Context) (bool, error) {
			return true, nil
		})

		follows, err := weekendFollowsFridayIn(ctx, monday, monday.AddDays(5))
		if err != nil || !follows {
			t.Fatalf("weekend window = %v, %v; want true, nil", follows, err)
		}
	})

	t.Run("returns setting failures", func(t *testing.T) {
		boom := errors.New("settings unavailable")
		ctx := calendar.WithWeekendPlan(context.Background(), func(context.Context) (bool, error) {
			return false, boom
		})

		_, err := weekendFollowsFridayIn(ctx, monday, monday.AddDays(5))
		if !errors.Is(err, boom) {
			t.Fatalf("weekend window error = %v; want %v", err, boom)
		}
	})
}
