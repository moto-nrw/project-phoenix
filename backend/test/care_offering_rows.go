package test

import (
	"time"

	"github.com/uptrace/bun"
)

// CareOffering is one enrollment.care_offerings row the fixtures write. Care
// Plan writes the catalog and keeps its row mapping internal; the legacy
// offering value in models/enrollment carries no mapping and the shared
// fixtures may not import it (#2748, #4014). So test support maps the columns
// it writes and its callers read. A caller that hands an offering to owner
// code reads it back through the owner.
type CareOffering struct {
	bun.BaseModel      `bun:"table:enrollment.care_offerings,alias:care_offering"`
	ID                 int64             `bun:"id,pk,autoincrement"`
	TenantID           int64             `bun:"tenant_id,notnull"`
	CreatedAt          time.Time         `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt          time.Time         `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	PhaseID            int64             `bun:"phase_id,notnull"`
	ActivityGroupID    *int64            `bun:"activity_group_id"`
	Name               string            `bun:"name,notnull"`
	DaysOfWeekMode     string            `bun:"days_of_week_mode,notnull"`
	AvailableDays      []string          `bun:"available_days,type:jsonb,notnull"`
	IsActive           bool              `bun:"is_active,notnull"`
	IsRequired         bool              `bun:"is_required,notnull"`
	CountsAsCare       bool              `bun:"counts_as_care,notnull"`
	AutoAddGradeLevels []int             `bun:"auto_add_grade_levels,type:jsonb,nullzero,notnull,default:'[]'"`
	SelectionRule      string            `bun:"selection_rule,nullzero,notnull,default:'optional'"`
	PickupTimes        map[string]string `bun:"pickup_times,type:jsonb,nullzero"`
}
