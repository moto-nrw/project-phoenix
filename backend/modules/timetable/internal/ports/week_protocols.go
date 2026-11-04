package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// WeekProtocolStore is the owner-private persistence of the weekly
// protocols (#4769). Every method runs in the tenant transaction of the
// caller.
type WeekProtocolStore interface {
	// EnsureWeekProtocol returns the id and status of the target's protocol of
	// the week, inserting a collecting one when it is missing. A known group
	// keeps the name it had when its protocol began.
	EnsureWeekProtocol(ctx context.Context, target timetable.WeekProtocolTarget, weekStart calendar.Date) (int64, string, error)
	// CaptureWeekProtocolDay records the day's facts once: a day already recorded keeps
	// its first capture, a day with only a note gains the facts. It reports
	// whether this call recorded the day.
	CaptureWeekProtocolDay(ctx context.Context, protocolID int64, day calendar.Date, facts timetable.WeekProtocolDayFacts, at time.Time) (bool, error)
	// AdvanceWeekProtocols closes the drafts and collecting protocols of weeks that
	// began before closeBefore, with the notes entered so far, and, when
	// draftUpTo is set, turns the collecting protocols of weeks up to and
	// including it into drafts. It answers how many it drafted and the ids
	// it closed, whose PDFs the caller stores.
	AdvanceWeekProtocols(ctx context.Context, closeBefore calendar.Date, draftUpTo *calendar.Date, at time.Time) (drafted int, closed []int64, err error)
	ListWeekProtocols(ctx context.Context, filter timetable.WeekProtocolFilter) ([]timetable.WeekProtocol, error)
	// FindWeekProtocol loads the protocol with its days; lock takes its row lock for a
	// write. A missing protocol answers found false.
	FindWeekProtocol(ctx context.Context, id int64, lock bool) (timetable.WeekProtocol, bool, error)
	SaveWeekProtocolNotes(ctx context.Context, id int64, notes timetable.WeekProtocolNotes) error
	CloseWeekProtocol(ctx context.Context, id int64, actor timetable.WeekProtocolActor, at time.Time) error
	// SaveWeekProtocolPDF stores the PDF of a closed protocol once.
	SaveWeekProtocolPDF(ctx context.Context, protocolID int64, file timetable.WeekProtocolReportFile) error
	// FindWeekProtocolPDF loads the stored PDF; a protocol without one answers
	// found false.
	FindWeekProtocolPDF(ctx context.Context, protocolID int64) (timetable.WeekProtocolReportFile, bool, error)
	// DeleteWeekProtocolsEndedBefore deletes the protocols, with their days, whose week
	// ended before cutoff.
	DeleteWeekProtocolsEndedBefore(ctx context.Context, cutoff calendar.Date) (int, error)
}
