package ports

import (
	"context"
	"time"
)

// HolidayClosureNotifications processes the confirmed notices of changed
// school closing days of every Träger and follows their mails (ADR 0196).
// It opens its own units. The root binds the Holiday Closure Notification
// workflow; only a click confirms a notice, the job never does.
type HolidayClosureNotifications struct {
	ProcessDue func(ctx context.Context) error
	// Overdue counts stuck confirmed commands in an independent read-only unit.
	Overdue func(ctx context.Context, overdueFor time.Duration) (int, error)
}
