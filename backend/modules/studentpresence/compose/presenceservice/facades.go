package presenceservice

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
)

type LockExceptionDay = func(context.Context, int64, string) error

func NewPresenceOperations(value studentpresence.Presence, atSchool presence.AtSchoolCounter, logger *slog.Logger) (presence.PresenceOperations, error) {
	return presence.NewPresenceOperations(value, atSchool, logger)
}
func NewStatusDays(repo StudentStatusDayRepository, pickupExceptions ManualPartialAbsenceReader, db studentpresence.DatabaseHandle, lockExceptionDay LockExceptionDay, clocks ...func() time.Time) studentpresence.StatusDays {
	return presence.NewStatusDays(repo, pickupExceptions, db, presence.LockExceptionDay(lockExceptionDay), clocks...)
}
func NewStatusDayOverviews(repo StudentStatusDayOverviewRepository, people StatusDayOverviewPeople) studentpresence.StatusDayOverviews {
	return presence.NewStatusDayOverviews(repo, people)
}
func NewStudentHistory(attendance AttendanceHistoryReader, rooms HistoryRoomReader, accessLog DataAccessAudit, slots HistorySlotReader) studentpresence.StudentHistory {
	return presence.NewStudentHistory(attendance, rooms, accessLog, slots)
}
