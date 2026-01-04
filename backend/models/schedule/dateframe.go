package schedule

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// tableScheduleDateframes is the schema-qualified table name for dateframes
const tableScheduleDateframes = "schedule.dateframes"

// Dateframe represents a date range for scheduling
type Dateframe struct {
	base.Model  `bun:"schema:schedule,table:dateframes"`
	StartDate   time.Time `bun:"start_date,notnull" json:"start_date"`
	EndDate     time.Time `bun:"end_date,notnull" json:"end_date"`
	Name        string    `bun:"name" json:"name,omitempty"`
	Description string    `bun:"description" json:"description,omitempty"`
}

func (d *Dateframe) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(tableScheduleDateframes)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableScheduleDateframes)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableScheduleDateframes)
	}
	return nil
}

// TableName returns the database table name
func (d *Dateframe) TableName() string {
	return tableScheduleDateframes
}

// Validate ensures dateframe data is valid
func (d *Dateframe) Validate() error {
	if d.StartDate.IsZero() {
		return errors.New("start date is required")
	}

	if d.EndDate.IsZero() {
		return errors.New("end date is required")
	}

	if !d.EndDate.After(d.StartDate) && !d.EndDate.Equal(d.StartDate) {
		return errors.New("end date must be on or after start date")
	}

	return nil
}

// Duration returns the duration of the dateframe
func (d *Dateframe) Duration() time.Duration {
	return d.EndDate.Sub(d.StartDate)
}

// DaysCount returns the number of days in the dateframe
func (d *Dateframe) DaysCount() int {
	hours := d.Duration().Hours()
	return int(hours/24) + 1 // +1 to include both start and end dates
}

// Contains checks if the given date is within the dateframe
func (d *Dateframe) Contains(checkDate time.Time) bool {
	// Normalize times to ignore time component
	normalizedCheck := time.Date(checkDate.Year(), checkDate.Month(), checkDate.Day(), 0, 0, 0, 0, checkDate.Location())
	normalizedStart := time.Date(d.StartDate.Year(), d.StartDate.Month(), d.StartDate.Day(), 0, 0, 0, 0, d.StartDate.Location())
	normalizedEnd := time.Date(d.EndDate.Year(), d.EndDate.Month(), d.EndDate.Day(), 0, 0, 0, 0, d.EndDate.Location())

	return (normalizedCheck.Equal(normalizedStart) || normalizedCheck.After(normalizedStart)) &&
		(normalizedCheck.Equal(normalizedEnd) || normalizedCheck.Before(normalizedEnd))
}

// Overlaps checks if this dateframe overlaps with another dateframe
func (d *Dateframe) Overlaps(other *Dateframe) bool {
	return (other.StartDate.Before(d.EndDate) || other.StartDate.Equal(d.EndDate)) &&
		(d.StartDate.Before(other.EndDate) || d.StartDate.Equal(other.EndDate))
}

// GetID implements the Entity interface
func (d *Dateframe) GetID() interface{} {
	return d.ID
}

// GetCreatedAt implements the Entity interface
func (d *Dateframe) GetCreatedAt() time.Time {
	return d.CreatedAt
}

// GetUpdatedAt implements the Entity interface
func (d *Dateframe) GetUpdatedAt() time.Time {
	return d.UpdatedAt
}
