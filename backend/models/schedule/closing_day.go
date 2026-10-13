package schedule

import (
	"context"
)

// ClosingDayReasonMaxLength is the maximum length of the reason field.
const ClosingDayReasonMaxLength = 255

// ClosingDay represents a tenant-defined OGS closure period (#1418 3b) —
// pädagogische Tage, Weihnachtswoche, Sommerschließung. A row is a date
// range; a single closed day has start_date = end_date. On these days the
// staff Soll is 0, exactly like on public holidays.
type ClosingDay struct {
	Model `bun:"schema:schedule,table:closing_days"`
	TenantModel

	StartDate Date   `bun:"start_date,notnull" json:"start_date"`
	EndDate   Date   `bun:"end_date,notnull" json:"end_date"`
	Reason    string `bun:"reason,notnull" json:"reason"`
}

// ClosingDayRepository defines operations for managing closing days
type ClosingDayRepository interface {
	repository[*ClosingDay]

	// FindByTenantID finds all closing days for the current tenant,
	// ordered by start_date.
	FindByTenantID(ctx context.Context) ([]*ClosingDay, error)

	// FindOverlappingRange returns all closing days of the current tenant
	// whose [start_date, end_date] range overlaps [from, to] (inclusive on
	// both ends), ordered by start_date.
	FindOverlappingRange(ctx context.Context, from, to Date) ([]*ClosingDay, error)
}
