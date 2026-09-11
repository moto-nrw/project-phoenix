package domain

import (
	"errors"
	"time"
)

var ErrTimeframeNotFound = errors.New("timeframe not found")

type Timeframe struct {
	ID          int64
	TenantID    int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	StartTime   string
	EndTime     *string
	IsActive    bool
	Description string
}

type TimeframeFields struct {
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
