package ports

import (
	"context"
	"time"
)

// ReminderNotificationTick sends one school's due personal reminder
// notifications. onDutyOnly restricts the audience to staff members who are
// working right now (notifications.on_duty_only). The producer logs its own
// failures; the next tick tries again.
type ReminderNotificationTick func(ctx context.Context, tenantID int64, now time.Time, onDutyOnly bool)
