package schedule

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
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

// Validate ensures closing day data is valid
func (c *ClosingDay) Validate() error {
	trimmedReason := strings.TrimSpace(c.Reason)
	if trimmedReason == "" {
		return errors.New("reason is required")
	}
	if utf8.RuneCountInString(trimmedReason) > ClosingDayReasonMaxLength {
		return errors.New("reason cannot exceed 255 characters")
	}
	if c.StartDate.IsZero() {
		return errors.New("start_date is required")
	}
	if c.EndDate.IsZero() {
		return errors.New("end_date is required")
	}
	if c.EndDate.Before(c.StartDate) {
		return errors.New("end_date must not be before start_date")
	}
	return nil
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
