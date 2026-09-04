package timetable

import (
	"context"
	"time"
)

// Timeframe is the owner view of one schedule.timeframes row. Clock values
// stay timezone-free HH:MM:SS strings at the module boundary.
type Timeframe struct {
	ID          int64     `json:"id"`
	TenantID    int64     `json:"tenant_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	StartTime   string    `json:"start_time"`
	EndTime     *string   `json:"end_time,omitempty"`
	IsActive    bool      `json:"is_active"`
	Description string    `json:"description,omitempty"`
}

type TimeframeInput struct {
	StartTime   string
	EndTime     *string
	IsActive    bool
	Description string
}

type TimeframeFilter struct {
	DescriptionContains string
	ActiveOnly          bool
	OverlapsStart       *string
	OverlapsEnd         *string
	Limit               int
	Offset              int
}

type TimeframeQuery interface {
	FindTimeframe(context.Context, int64) (Timeframe, error)
	ListTimeframes(context.Context, TimeframeFilter) ([]Timeframe, error)
}

type TimeframeCommand interface {
	CreateTimeframe(context.Context, TimeframeInput) (Timeframe, error)
	UpdateTimeframe(context.Context, int64, TimeframeInput) (Timeframe, error)
	DeleteTimeframe(context.Context, int64) error
}

type TimeframeCapability interface {
	TimeframeQuery
	TimeframeCommand
}
