package ports

import (
	"context"
	"time"
)

// WeekProtocolEvenings runs the evening of the weekly protocols for the
// school in ctx (#4769): it records the day of every group, turns the week
// into drafts from Friday evening and closes the drafts nobody closed after
// the following week. A repeated run on the same evening records nothing
// twice. The root binds Timetable's evening run.
type WeekProtocolEvenings func(ctx context.Context, now time.Time) error
