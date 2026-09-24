package timetable

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

// Planning-track administration answers (#2383): the editor maps a missing
// track to 404, an invalid or archived one to 400 and a taken name to 409.
// ErrPlanningTrackNotFound and ErrInvalidPlanningTrack are the owner's
// sentinels shared with the capability above.
var (
	ErrPlanningTrackNameTaken = errors.New("a planning track with this name already exists")
	ErrPlanningTrackArchived  = errors.New("planning track is archived")
)

// PlanningTrackDraft is the editable part of a planning track.
type PlanningTrackDraft struct {
	Name      string
	Color     string
	SortOrder int
}

// PlanningTrackAdministration is the planning-track editor of the
// Betreuungsplan (#2383): it validates the name and colour, keeps names
// unique among active tracks, freezes archived tracks and decides whether a
// template may be assigned to a track.
type PlanningTrackAdministration interface {
	// ListAllPlanningTracks returns every track, archived ones included, in
	// the planner's order.
	ListAllPlanningTracks(ctx context.Context) ([]PlanningTrack, error)
	GetPlanningTrack(ctx context.Context, id int64) (PlanningTrack, error)
	AddPlanningTrack(ctx context.Context, draft PlanningTrackDraft) (PlanningTrack, error)
	EditPlanningTrack(ctx context.Context, id int64, draft PlanningTrackDraft) (PlanningTrack, error)
	// OrderPlanningTracks stores the given ids as the new order.
	OrderPlanningTracks(ctx context.Context, ids []int64) error
	ArchivePlanningTrack(ctx context.Context, id int64) (PlanningTrack, error)
	RestorePlanningTrack(ctx context.Context, id int64) (PlanningTrack, error)
	// ValidatePlanningTrackAssignment accepts a nil id, rejects a missing
	// track and an archived one unless it is allowedArchivedID (a template
	// may keep the archived track it already has). The track row is read
	// FOR SHARE so a concurrent archive waits for the template write.
	ValidatePlanningTrackAssignment(ctx context.Context, id, allowedArchivedID *int64) error
}

// Validate trims the name and checks the draft the way the editor reports
// it: a required name of at most 100 characters, a #RRGGBB colour and a
// non-negative sort order. The error wraps ErrInvalidPlanningTrack.
func (d PlanningTrackDraft) Validate() (PlanningTrackDraft, error) {
	d.Name = strings.TrimSpace(d.Name)
	reason := ""
	if d.Name == "" {
		reason = "planning track name is required"
	} else if len(d.Name) > 100 {
		reason = "planning track name cannot exceed 100 characters"
	} else if !planningTrackColorPattern.MatchString(d.Color) {
		reason = "planning track color must use #RRGGBB"
	} else if d.SortOrder < 0 {
		reason = "planning track sort order cannot be negative"
	}
	if reason != "" {
		return d, fmt.Errorf("%w: %s", ErrInvalidPlanningTrack, reason)
	}
	return d, nil
}
