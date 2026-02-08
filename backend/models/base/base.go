package base

import (
	"time"
)

// Model represents the common fields for all database models
type Model struct {
	ID        int64     `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

// StringIDModel represents models that use string as primary key (like RFID cards)
type StringIDModel struct {
	ID        string    `bun:"id,pk" json:"id"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

// BeforeAppendModel is a hook that can be embedded in models to set timestamps before insert/update
type BeforeAppendModel struct{}

// BeforeAppend sets the timestamp values before the model is saved
func (m *Model) BeforeAppend() error {
	now := time.Now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	return nil
}

// TimeRange represents a time range with start and end timestamps
type TimeRange struct {
	StartTime time.Time  `bun:"start_time,nullzero,notnull" json:"start_time"`
	EndTime   *time.Time `bun:"end_time,nullzero" json:"end_time,omitempty"`
}

// DateRange represents a date range with start and end dates
type DateRange struct {
	StartDate time.Time `bun:"start_date,nullzero,notnull" json:"start_date"`
	EndDate   time.Time `bun:"end_date,nullzero,notnull" json:"end_date"`
}

// Activatable represents models that can be activated or deactivated
type Activatable struct {
	IsActive bool `bun:"is_active,notnull,default:true" json:"is_active"`
}

// Nameable represents models with a name field
type Nameable struct {
	Name string `bun:"name,notnull" json:"name"`
}

// NameableUnique represents models with a unique name field
type NameableUnique struct {
	Name string `bun:"name,notnull,unique" json:"name"`
}

// Describable represents models with a description field
type Describable struct {
	Description string `bun:"description" json:"description,omitempty"`
}

// Schema constants for the database schema names
const (
	SchemaAuth       = "auth"
	SchemaUsers      = "users"
	SchemaEducation  = "education"
	SchemaSchedule   = "schedule"
	SchemaActivities = "activities"
	SchemaFacilities = "facilities"
	SchemaIoT        = "iot"
	SchemaFeedback   = "feedback"
	SchemaActive     = "active"
	SchemaConfig     = "config"
	SchemaMeta       = "meta"
	SchemaPlatform   = "platform"
)

// Pointer helper functions for creating pointers to primitive values in tests.
// These are placed here to avoid import cycles when used in model package tests.

// StringPtr returns a pointer to the given string.
func StringPtr(s string) *string { return &s }

// IntPtr returns a pointer to the given int.
func IntPtr(i int) *int { return &i }

// Int64Ptr returns a pointer to the given int64.
func Int64Ptr(i int64) *int64 { return &i }

// TimePtr returns a pointer to the given time.Time.
func TimePtr(t time.Time) *time.Time { return &t }
