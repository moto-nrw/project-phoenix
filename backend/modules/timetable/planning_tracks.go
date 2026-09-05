package timetable

import (
	"context"
	"time"
)

type PlanningTrack struct {
	ID         int64      `json:"id"`
	TenantID   int64      `json:"tenant_id"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	Name       string     `json:"name"`
	Color      string     `json:"color"`
	SortOrder  int        `json:"sort_order"`
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
}

func (p PlanningTrack) IsArchived() bool { return p.ArchivedAt != nil }

type PlanningTrackInput struct {
	Name       string
	Color      string
	SortOrder  int
	ArchivedAt *time.Time
}

type PlanningTrackFilter struct {
	IDs     []int64
	Ordered bool
}

type PlanningTrackQuery interface {
	FindPlanningTrack(context.Context, int64) (PlanningTrack, error)
	FindPlanningTrackForShare(context.Context, int64) (PlanningTrack, error)
	ListPlanningTracks(context.Context, PlanningTrackFilter) ([]PlanningTrack, error)
}

type PlanningTrackCommand interface {
	CreatePlanningTrack(context.Context, PlanningTrackInput) (PlanningTrack, error)
	UpdatePlanningTrack(context.Context, int64, PlanningTrackInput) (PlanningTrack, error)
	UpdateActivePlanningTrack(context.Context, int64, PlanningTrackInput) (PlanningTrack, bool, error)
	DeletePlanningTrack(context.Context, int64) error
	SetPlanningTrackArchivedAt(context.Context, int64, *time.Time) (PlanningTrack, bool, error)
	ReorderPlanningTracks(context.Context, []int64) error
	RestorePlanningTrackAtEnd(context.Context, int64) (PlanningTrack, bool, error)
}

type PlanningTrackCapability interface {
	PlanningTrackQuery
	PlanningTrackCommand
}
