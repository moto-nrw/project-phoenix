package schedulerecords

import (
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

	StartDate calendar.Date `bun:"start_date,notnull" json:"start_date"`
	EndDate   calendar.Date `bun:"end_date,notnull" json:"end_date"`
	Reason    string        `bun:"reason,notnull" json:"reason"`
}

// ports.ClosingDayRecords defines operations for managing closing days
