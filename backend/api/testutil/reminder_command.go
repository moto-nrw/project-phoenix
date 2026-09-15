package testutil

import (
	"github.com/moto-nrw/project-phoenix/services"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/uptrace/bun"
)

// ComposeCalendarReminderCommand uses the production binding with test-owned adapters.
func ComposeCalendarReminderCommand(db *bun.DB, source services.CalendarReminderSource) reminder.Command {
	return services.NewCalendarReminderCommand(db, source)
}
