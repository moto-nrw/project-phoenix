package mealplan

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PastMealDay is a registered lunch on a finished demo care day.
type PastMealDay struct {
	StudentID int64
	Date      calendar.Date
}

// ParticipationHistory records past demo registrations without changing
// current schedules or replacing an existing decision. It is bound only by
// the dev-tools CLI, never by a production HTTP route.
type ParticipationHistory interface {
	RecordParticipationHistory(context.Context, []PastMealDay) error
}
