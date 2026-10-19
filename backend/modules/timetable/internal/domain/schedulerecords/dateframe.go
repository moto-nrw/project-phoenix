package schedulerecords

import (
	"time"
)

// Dateframe represents a date range for scheduling
type Dateframe struct {
	Model `bun:"schema:schedule,table:dateframes"`
	TenantModel
	StartDate   time.Time `bun:"start_date,notnull" json:"start_date"`
	EndDate     time.Time `bun:"end_date,notnull" json:"end_date"`
	Name        string    `bun:"name" json:"name,omitempty"`
	Description string    `bun:"description" json:"description,omitempty"`
}
