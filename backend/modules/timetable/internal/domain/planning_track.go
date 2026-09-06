package domain

import (
	"errors"
	"time"
)

var (
	ErrPlanningTrackNotFound     = errors.New("planning track not found")
	ErrPlanningTrackNameConflict = errors.New("planning track name conflict")
)

const PlanningTrackNameActiveIndex = "uniq_planning_tracks_tenant_active_name"

type PlanningTrack struct {
	ID         int64
	TenantID   int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Name       string
	Color      string
	SortOrder  int
	ArchivedAt *time.Time
}

type PlanningTrackFields struct {
	Name       string
	Color      string
	SortOrder  int
	ArchivedAt *time.Time
}

type PlanningTrackFilter struct {
	IDs     []int64
	Ordered bool
}
