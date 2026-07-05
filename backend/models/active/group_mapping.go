package active

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Table name constants for BUN ORM schema qualification
const ()

// GroupMapping represents a mapping between a combined group and an active group
type GroupMapping struct {
	base.Model `bun:"schema:active,table:group_mappings"`
	base.TenantModel
	ActiveCombinedGroupID int64 `bun:"active_combined_group_id,notnull" json:"active_combined_group_id"`
	ActiveGroupID         int64 `bun:"active_group_id,notnull" json:"active_group_id"`

	// Relations - these would be populated when using the ORM's relations
	CombinedGroup *CombinedGroup `bun:"rel:belongs-to,join:active_combined_group_id=id" json:"combined_group,omitempty"`
	ActiveGroup   *Group         `bun:"rel:belongs-to,join:active_group_id=id" json:"active_group,omitempty"`
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
