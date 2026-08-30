package test

import (
	"context"
	"strings"
	"sync/atomic"

	"github.com/uptrace/bun"
)

type settingValuesSelectCounter struct{ count atomic.Int32 }

func (c *settingValuesSelectCounter) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	query := strings.ToLower(event.Query)
	if strings.HasPrefix(strings.TrimSpace(query), "select") && strings.Contains(query, "config.setting_values") {
		c.count.Add(1)
	}
	return ctx
}

func (*settingValuesSelectCounter) AfterQuery(context.Context, *bun.QueryEvent) {}

func CaptureSettingValueSelects(db *bun.DB) func() int32 {
	counter := &settingValuesSelectCounter{}
	db.AddQueryHook(counter)
	return counter.count.Load
}
