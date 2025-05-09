package active

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// CombinedGroup represents a combination of multiple active groups
type CombinedGroup struct {
	base.Model
	StartTime time.Time  `bun:"start_time,notnull" json:"start_time"`
	EndTime   *time.Time `bun:"end_time" json:"end_time,omitempty"`

	// Relations - these would be populated when using the ORM's relations
	GroupMappings []*GroupMapping `bun:"rel:has-many,join:id=active_combined_group_id" json:"group_mappings,omitempty"`
	ActiveGroups  []*Group        `bun:"-" json:"active_groups,omitempty"` // This would be loaded through GroupMappings
}

// GetID returns the entity's ID
func (cg *CombinedGroup) GetID() interface{} {
	return cg.ID
}

// GetCreatedAt returns the creation timestamp
func (cg *CombinedGroup) GetCreatedAt() time.Time {
	return cg.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (cg *CombinedGroup) GetUpdatedAt() time.Time {
	return cg.UpdatedAt
}

// TableName returns the database table name
func (cg *CombinedGroup) TableName() string {
	return "active.combined_groups"
}

// Validate ensures combined group data is valid
func (cg *CombinedGroup) Validate() error {
	if cg.StartTime.IsZero() {
		return errors.New("start time is required")
	}

	if cg.EndTime != nil && cg.StartTime.After(*cg.EndTime) {
		return errors.New("start time must be before end time")
	}

	return nil
}

// IsActive returns whether this combined group is currently active
func (cg *CombinedGroup) IsActive() bool {
	return cg.EndTime == nil
}

// EndCombination sets the end time to the current time
func (cg *CombinedGroup) EndCombination() {
	now := time.Now()
	cg.EndTime = &now
}

// SetEndTime explicitly sets the end time
func (cg *CombinedGroup) SetEndTime(endTime time.Time) error {
	if cg.StartTime.After(endTime) {
		return errors.New("end time cannot be before start time")
	}
	cg.EndTime = &endTime
	return nil
}

// GetDuration returns the duration of the combined group
func (cg *CombinedGroup) GetDuration() time.Duration {
	if cg.EndTime == nil {
		return time.Since(cg.StartTime)
	}
	return cg.EndTime.Sub(cg.StartTime)
}
