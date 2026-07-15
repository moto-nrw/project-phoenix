package config

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/uptrace/bun"
)

const (
	tableWorkTimeModels       = "config.work_time_models"
	tableWorkTimeModelEntries = "config.work_time_model_entries"
	WorkTimeModelMaxRotation  = 4
	// WorkTimeModelMaxDailyMinutes caps a single day's target working time at
	// 12 hours (720 minutes). Shared with StaffWorkSchedule validation.
	WorkTimeModelMaxDailyMinutes = 720
)

// WorkTimeModel is a tenant-scoped, named template that captures a working-time
// pattern (e.g. "Vollzeit 40h Mo-Fr" or "Teilzeit 30h A/B"). Multiple staff
// members can share a model, and a model can span up to four rotation weeks.
type WorkTimeModel struct {
	bun.BaseModel `bun:"schema:config,table:work_time_models"`

	ID                 int64         `bun:"id,pk,autoincrement" json:"id"`
	TenantID           int64         `bun:"tenant_id,notnull" json:"tenant_id"`
	Name               string        `bun:"name,notnull" json:"name"`
	RotationLength     int           `bun:"rotation_length,notnull,default:1" json:"rotation_length"`
	RotationAnchorDate timezone.Date `bun:"rotation_anchor_date,notnull,type:date" json:"rotation_anchor_date"`
	CreatedAt          time.Time     `bun:"created_at,notnull,default:now()" json:"created_at"`
	UpdatedAt          time.Time     `bun:"updated_at,notnull,default:now()" json:"updated_at"`

	Entries []*WorkTimeModelEntry `bun:"rel:has-many,join:id=model_id" json:"entries,omitempty"`
}

func (m *WorkTimeModel) SetTenantID(tenantID int64) { m.TenantID = tenantID }

func (m *WorkTimeModel) Validate() error {
	if m.Name == "" {
		return errors.New("name is required")
	}
	if m.RotationLength < 1 || m.RotationLength > WorkTimeModelMaxRotation {
		return errors.New("rotation_length must be between 1 and 4")
	}
	if m.RotationAnchorDate.IsZero() {
		return errors.New("rotation_anchor_date is required")
	}
	return nil
}

// WorkTimeModelEntry stores the target minutes for one (week_index, day_of_week)
// slot inside a model. UNIQUE(model_id, week_index, day_of_week) is enforced at
// the schema level.
type WorkTimeModelEntry struct {
	bun.BaseModel `bun:"schema:config,table:work_time_model_entries"`

	ID            int64      `bun:"id,pk,autoincrement" json:"id"`
	ModelID       int64      `bun:"model_id,notnull" json:"model_id"`
	WeekIndex     int        `bun:"week_index,notnull" json:"week_index"`
	DayOfWeek     int        `bun:"day_of_week,notnull" json:"day_of_week"`
	TargetMinutes int        `bun:"target_minutes,notnull" json:"target_minutes"`
	StartTime     *time.Time `bun:"start_time" json:"start_time,omitempty"`
	CreatedAt     time.Time  `bun:"created_at,notnull,default:now()" json:"created_at"`
	UpdatedAt     time.Time  `bun:"updated_at,notnull,default:now()" json:"updated_at"`
}

func (e *WorkTimeModelEntry) Validate() error {
	// model_id is set right before insert by the repository, so validating
	// it here would always fire on freshly built entries. The DB-level
	// FK + NOT NULL on model_id enforces the relationship.
	if e.WeekIndex < 0 || e.WeekIndex >= WorkTimeModelMaxRotation {
		return errors.New("week_index must be between 0 and 3")
	}
	if e.DayOfWeek < 0 || e.DayOfWeek > 6 {
		return errors.New("day_of_week must be between 0 (Monday) and 6 (Sunday)")
	}
	if e.TargetMinutes < 0 || e.TargetMinutes > WorkTimeModelMaxDailyMinutes {
		return errors.New("target_minutes must be between 0 and 720 (12h)")
	}
	return nil
}

// ResolveWeekIndex returns the rotation week index for a given date relative to
// the model's anchor. Negative deltas (date before anchor) wrap forwards so the
// result is always in [0, rotation_length). Callers pass week-start dates, so
// the day difference is an exact multiple of 7 and DaysUntil/7 reproduces the
// old truncating division exactly (DST-proof by Date's UTC anchoring).
func ResolveWeekIndex(rotationLength int, anchor, date timezone.Date) int {
	if rotationLength <= 1 {
		return 0
	}
	delta := anchor.DaysUntil(date) / 7
	mod := delta % rotationLength
	if mod < 0 {
		mod += rotationLength
	}
	return mod
}

// WorkTimeModelRepository defines tenant-scoped CRUD for work-time templates.
type WorkTimeModelRepository interface {
	List(ctx context.Context) ([]*WorkTimeModel, error)
	FindByID(ctx context.Context, id int64) (*WorkTimeModel, error)
	// FindByIDs resolves the given templates with their entries in one batched
	// read (missing IDs are simply absent from the result).
	FindByIDs(ctx context.Context, ids []int64) ([]*WorkTimeModel, error)
	Create(ctx context.Context, model *WorkTimeModel, entries []*WorkTimeModelEntry) error
	Update(ctx context.Context, model *WorkTimeModel, entries []*WorkTimeModelEntry) error
	RefreshAssignedStaffSchedules(ctx context.Context, modelID int64) error
	Delete(ctx context.Context, id int64) error
}
