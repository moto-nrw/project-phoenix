// Package models contains application specific entities.
package models

import (
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/uptrace/bun"
)

// AgCategory represents a category of activity groups
type AgCategory struct {
	ID        int64     `json:"id" bun:"id,pk,autoincrement"`
	Name      string    `json:"name" bun:"name,notnull,unique"`
	CreatedAt time.Time `json:"created_at" bun:"created_at,notnull"`
}

// BeforeInsert hook executed before database insert operation.
func (c *AgCategory) BeforeInsert(db *bun.DB) error {
	c.CreatedAt = time.Now()
	return c.Validate()
}

// Validate validates AgCategory struct and returns validation errors.
func (c *AgCategory) Validate() error {
	return validation.ValidateStruct(c,
		validation.Field(&c.Name, validation.Required),
	)
}

// Ag represents an activity group
type Ag struct {
	ID             int64                  `json:"id" bun:"id,pk,autoincrement"`
	Name           string                 `json:"name" bun:"name,notnull"`
	MaxParticipant int                    `json:"max_participant" bun:"max_participant,notnull"`
	IsOpenAg       bool                   `json:"is_open_ag" bun:"is_open_ags,notnull,default:false"`
	SupervisorID   int64                  `json:"supervisor_id" bun:"supervisor_id,notnull"`
	Supervisor     *PedagogicalSpecialist `json:"supervisor,omitempty" bun:"rel:belongs-to,join:supervisor_id=id"`
	AgCategoryID   int64                  `json:"ag_category_id" bun:"ag_categories_id,notnull"`
	AgCategory     *AgCategory            `json:"ag_category,omitempty" bun:"rel:belongs-to,join:ag_categories_id=id"`
	DatespanID     *int64                 `json:"datespan_id,omitempty" bun:"datespan_id"`
	Datespan       *Timespan              `json:"datespan,omitempty" bun:"rel:belongs-to,join:datespan_id=id"`
	CreatedAt      time.Time              `json:"created_at" bun:"created_at,notnull"`
	ModifiedAt     time.Time              `json:"updated_at" bun:"modified_at,notnull"`
	Times          []*AgTime              `json:"times,omitempty" bun:"rel:has-many,join:id=ag_id"`
	Students       []*Student             `json:"students,omitempty" bun:"m2m:student_ags,join:Ag=Student"`

	bun.BaseModel `bun:"table:ags"` // Specify the correct table name
}

// BeforeInsert hook executed before database insert operation.
func (ag *Ag) BeforeInsert(db *bun.DB) error {
	now := time.Now()
	ag.CreatedAt = now
	ag.ModifiedAt = now
	return ag.Validate()
}

// BeforeUpdate hook executed before database update operation.
func (ag *Ag) BeforeUpdate(db *bun.DB) error {
	ag.ModifiedAt = time.Now()
	return ag.Validate()
}

// Validate validates Ag struct and returns validation errors.
func (ag *Ag) Validate() error {
	return validation.ValidateStruct(ag,
		validation.Field(&ag.Name, validation.Required),
		validation.Field(&ag.MaxParticipant, validation.Required, validation.Min(1)),
		validation.Field(&ag.SupervisorID, validation.Required),
		validation.Field(&ag.AgCategoryID, validation.Required),
	)
}

// AgTime represents a time slot for an activity group
type AgTime struct {
	ID         int64     `json:"id" bun:"id,pk,autoincrement"`
	Weekday    string    `json:"weekday" bun:"weekday,notnull"`
	TimespanID int64     `json:"timespan_id" bun:"timespans_id,notnull"`
	Timespan   *Timespan `json:"timespan,omitempty" bun:"rel:belongs-to,join:timespans_id=id"`
	AgID       int64     `json:"ag_id" bun:"ag_id,nullzero"` // Use nullzero to allow zero values during insert
	Ag         *Ag       `json:"ag,omitempty" bun:"rel:belongs-to,join:ag_id=id"`
	CreatedAt  time.Time `json:"created_at" bun:"created_at,notnull"`
}

// BeforeInsert hook executed before database insert operation.
func (t *AgTime) BeforeInsert(db *bun.DB) error {
	t.CreatedAt = time.Now()
	return t.Validate()
}

// Validate validates AgTime struct and returns validation errors.
func (t *AgTime) Validate() error {
	return validation.ValidateStruct(t,
		validation.Field(&t.Weekday, validation.Required, validation.In("Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday")),
		validation.Field(&t.TimespanID, validation.Required),
		// Don't require AgID during initial creation - it will be set by CreateAg
	)
}

// StudentAg is the junction table for students and activity groups
type StudentAg struct {
	ID        int64     `bun:"id,pk,autoincrement"`
	StudentID int64     `bun:"student_id,notnull"`
	Student   *Student  `bun:"rel:belongs-to,join:student_id=id"`
	AgID      int64     `bun:"ag_id,notnull"`
	Ag        *Ag       `bun:"rel:belongs-to,join:ag_id=id"`
	CreatedAt time.Time `bun:"created_at,notnull"`

	bun.BaseModel `bun:"table:student_ags"`
}

// BeforeInsert hook executed before database insert operation.
func (sa *StudentAg) BeforeInsert(db *bun.DB) error {
	sa.CreatedAt = time.Now()
	return nil
}
