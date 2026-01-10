package active

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Table name constants for BUN ORM schema qualification
const (
	tableActiveGroupMappings       = "active.group_mappings"
	tableExprGroupMappingsAsGM     = `active.group_mappings AS "group_mapping"`
)

// GroupMapping represents a mapping between a combined group and an active group
type GroupMapping struct {
	base.Model            `bun:"schema:active,table:group_mappings"`
	ActiveCombinedGroupID int64 `bun:"active_combined_group_id,notnull" json:"active_combined_group_id"`
	ActiveGroupID         int64 `bun:"active_group_id,notnull" json:"active_group_id"`

	// Relations - these would be populated when using the ORM's relations
	CombinedGroup *CombinedGroup `bun:"rel:belongs-to,join:active_combined_group_id=id" json:"combined_group,omitempty"`
	ActiveGroup   *Group         `bun:"rel:belongs-to,join:active_group_id=id" json:"active_group,omitempty"`
}

func (gm *GroupMapping) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(tableExprGroupMappingsAsGM)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableActiveGroupMappings)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableActiveGroupMappings)
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr(tableActiveGroupMappings)
	}
	return nil
}

// GetID returns the entity's ID
func (gm *GroupMapping) GetID() interface{} {
	return gm.ID
}

// GetCreatedAt returns the creation timestamp
func (gm *GroupMapping) GetCreatedAt() time.Time {
	return gm.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (gm *GroupMapping) GetUpdatedAt() time.Time {
	return gm.UpdatedAt
}

// TableName returns the database table name
func (gm *GroupMapping) TableName() string {
	return tableActiveGroupMappings
}

// Validate ensures group mapping data is valid
func (gm *GroupMapping) Validate() error {
	if gm.ActiveCombinedGroupID <= 0 {
		return errors.New("active combined group ID is required")
	}

	if gm.ActiveGroupID <= 0 {
		return errors.New("active group ID is required")
	}

	return nil
}
