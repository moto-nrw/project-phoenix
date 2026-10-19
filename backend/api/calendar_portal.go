package api

import (
	"context"
)

type CalendarPreferences interface {
	FilterNotOptedOut(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error)
	FilterOptedIn(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error)
}
