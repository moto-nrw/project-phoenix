package compose

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// lastWallClockMinute is 23:59, the latest end a block of one day can have.
const lastWallClockMinute = 23*60 + 59

// spontaneousCompletionEnd is the end a spontaneous block records when it
// completes (#3921). Its planned end is a placeholder: the kiosk mirror
// writes start plus one hour and nothing ends the block at that time, so a
// finished block would keep an end that never happened. The real end is the
// completion's Berlin wall clock, at least one minute after the start and at
// most 23:59; a completion on a later day ends the block at 23:59. ok is
// false for planned blocks and when no end after the start fits the day.
func spontaneousCompletionEnd(instance *scheduleModel.ActivityInstance, completedAt time.Time) (end time.Time, ok bool) {
	if instance == nil || !instance.IsSpontaneous {
		return time.Time{}, false
	}
	startMinute := instance.StartTime.Hour()*60 + instance.StartTime.Minute()
	if startMinute >= lastWallClockMinute {
		return time.Time{}, false
	}
	berlin := completedAt.In(timezone.Berlin)
	endMinute := berlin.Hour()*60 + berlin.Minute()
	if timezone.DateFromTime(completedAt).String() != instance.Date.String() {
		endMinute = lastWallClockMinute
	}
	endMinute = min(max(endMinute, startMinute+1), lastWallClockMinute)
	clock := time.Date(1, time.January, 1, endMinute/60, endMinute%60, 0, 0, time.UTC)
	return timezone.NormalizeWallClock(clock), true
}
