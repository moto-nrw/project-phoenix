package schoolcalendar

import "context"

// ClosingDayReads reads the school's calendar with only the closing days of
// its current Träger. The root binds the tenant-safe read projection.
type ClosingDayReads interface {
	ListClosingDays(context.Context, ClosingDayFilter) ([]ClosingDay, error)
	ListCareCalendarRanges(context.Context, string, string) ([]ClosingDay, []ClosingDay, error)
}
