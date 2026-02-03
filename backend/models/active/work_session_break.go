package active

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const tableActiveWorkSessionBreaks = "active.work_session_breaks"

// WorkSessionBreak represents a single break period within a work session
type WorkSessionBreak struct {
	base.Model      `bun:"schema:active,table:work_session_breaks"`
	SessionID       int64      `bun:"session_id,notnull" json:"session_id"`
	StartedAt       time.Time  `bun:"started_at,notnull" json:"started_at"`
	EndedAt         *time.Time `bun:"ended_at" json:"ended_at,omitempty"`
	DurationMinutes int        `bun:"duration_minutes,notnull,default:0" json:"duration_minutes"`
	PlannedEndTime  *time.Time `bun:"planned_end_time" json:"planned_end_time,omitempty"`

	Session *WorkSession `bun:"rel:belongs-to,join:session_id=id" json:"session,omitempty"`
}

// BeforeAppendModel implements the model hook for schema-qualified queries
func (b *WorkSessionBreak) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(tableActiveWorkSessionBreaks)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableActiveWorkSessionBreaks)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableActiveWorkSessionBreaks)
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr(tableActiveWorkSessionBreaks)
	}
	return nil
}

func (b *WorkSessionBreak) GetID() any              { return b.ID }
func (b *WorkSessionBreak) GetCreatedAt() time.Time { return b.CreatedAt }
func (b *WorkSessionBreak) GetUpdatedAt() time.Time { return b.UpdatedAt }
func (b *WorkSessionBreak) TableName() string       { return tableActiveWorkSessionBreaks }

// Validate validates the break record
func (b *WorkSessionBreak) Validate() error {
	if b.SessionID <= 0 {
		return errors.New("session ID is required")
	}
	if b.StartedAt.IsZero() {
		return errors.New("started_at is required")
	}
	if b.EndedAt != nil && b.StartedAt.After(*b.EndedAt) {
		return errors.New("started_at must be before ended_at")
	}
	if b.DurationMinutes < 0 {
		return errors.New("duration_minutes cannot be negative")
	}
	return nil
}

// IsActive returns true if the break has not ended yet
func (b *WorkSessionBreak) IsActive() bool {
	return b.EndedAt == nil
}
