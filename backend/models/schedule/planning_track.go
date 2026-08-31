package schedule

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

var planningTrackColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type PlanningTrack struct {
	base.Model `bun:"schema:schedule,table:planning_tracks"`
	base.TenantModel
	Name       string     `bun:"name,notnull" json:"name"`
	Color      string     `bun:"color,notnull" json:"color"`
	SortOrder  int        `bun:"sort_order,notnull" json:"sort_order"`
	ArchivedAt *time.Time `bun:"archived_at" json:"archived_at,omitempty"`
}

func (p *PlanningTrack) Validate() error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return errors.New("planning track name is required")
	}
	if len(p.Name) > 100 {
		return errors.New("planning track name cannot exceed 100 characters")
	}
	if !planningTrackColorPattern.MatchString(p.Color) {
		return errors.New("planning track color must use #RRGGBB")
	}
	if p.SortOrder < 0 {
		return errors.New("planning track sort order cannot be negative")
	}
	return nil
}

func (p *PlanningTrack) IsArchived() bool {
	return p.ArchivedAt != nil
}

type PlanningTrackRepository interface {
	base.CRUDRepository[*PlanningTrack]
	ListAll(ctx context.Context) ([]*PlanningTrack, error)
	// FindByIDs returns the tracks matching the given IDs in one
	// tenant-scoped IN query (missing IDs are simply absent). Archived
	// rows are included so colours for historical references still resolve.
	FindByIDs(ctx context.Context, ids []int64) ([]*PlanningTrack, error)
	FindByIDForShare(ctx context.Context, id int64) (*PlanningTrack, error)
	UpdateIfActive(ctx context.Context, track *PlanningTrack) (bool, error)
	UpdateSortOrders(ctx context.Context, ids []int64) error
	RestoreAtEnd(ctx context.Context, track *PlanningTrack) (bool, error)
	UpdateColumns(ctx context.Context, track *PlanningTrack, columns ...string) (int64, error)
}
