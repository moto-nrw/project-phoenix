package timetable

import (
	"context"
	"errors"
	"slices"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Roster review (#5103): a Regeltermin whose children are kept by hand or
// through a Klasse/Jahrgang/Gruppe target can drift away from the bookings
// of a Betreuungsangebot. The review compares its future children with the
// offering's bookings, previews a switch to the offering as source and
// performs that switch for every future segment of the series.

// ErrRosterReviewInvalid rejects a roster review request: a date before
// today, an empty offering selection or a duty template.
var ErrRosterReviewInvalid = errors.New("invalid roster review request")

// TemplateRosterReview is the Timetable owner's roster review.
type TemplateRosterReview interface {
	// FutureRosters returns, per template, the children planned on each
	// schedule weekday from max(from, segment start) on: hand and sourced
	// enrollment rows plus the current members of the Klasse, Jahrgang or
	// Gruppe targets. A zero from means today. Unknown ids are absent.
	FutureRosters(ctx context.Context, templateIDs []int64, from calendar.Date) (map[int64]TemplateFutureRoster, error)
	// PlannedRosters returns, per template, who it plans on which dates:
	// its enrollment rows with their validity and the current members of
	// its Klasse, Jahrgang or Gruppe targets (#5343). Children who left the
	// school are left out; unknown ids are absent.
	PlannedRosters(ctx context.Context, templateIDs []int64) (map[int64]TemplatePlannedRoster, error)
	// SeriesSegments lists the segments of the template's series that still
	// have days on or after from, ordered by their start.
	SeriesSegments(ctx context.Context, templateID int64, from calendar.Date) ([]TemplateSeriesSegment, error)
	// NextSeriesOccurrence returns the date of the series' first planned,
	// non-spontaneous occurrence on or after from; nil when none exists.
	NextSeriesOccurrence(ctx context.Context, templateID int64, from calendar.Date) (*calendar.Date, error)
	// ConnectOfferingSource makes the offerings the children's source for
	// every series segment with days on or after the effective date, in
	// place and in one unit of work.
	ConnectOfferingSource(ctx context.Context, cmd ConnectOfferingSourceCommand) (ConnectOfferingSourceResult, error)
	// SetManualRosterConfirmed stores "Bleibt von Hand gepflegt" on the
	// series; ErrTemplateSegmentNotEditable when the template is unknown.
	SetManualRosterConfirmed(ctx context.Context, templateID int64, confirmed bool) error
}

// TemplateFutureRoster is one template's future children per weekday.
type TemplateFutureRoster struct {
	TemplateID int64
	From       calendar.Date
	// Weekdays maps every schedule weekday to its children, ascending.
	Weekdays map[int][]int64
	// TargetSchoolClasses / TargetGradeLevels name the dynamic targets when
	// every target is a Klasse respectively a Jahrgang; empty otherwise.
	TargetSchoolClasses []string
	TargetGradeLevels   []int
	// OfferingKind restricts which offerings may explain the roster; the
	// zero value admits every offering.
	OfferingKind RosterOfferingKind
}

// TemplatePlannedRoster is who a Regeltermin plans, as materialization
// fills its occurrences: the enrollment rows on the dates they apply, and
// the members of its Klasse, Jahrgang or Gruppe targets on every date.
type TemplatePlannedRoster struct {
	Entries          []TemplateRosterEntry
	TargetStudentIDs []int64
}

// TemplateRosterEntry is one enrollment row of a Regeltermin.
type TemplateRosterEntry struct {
	StudentID int64
	// ValidFrom is inclusive, ValidUntil exclusive; nil is open.
	ValidFrom  *calendar.Date
	ValidUntil *calendar.Date
	// CalendarPeriodID pins the row to one planning period; nil holds in
	// every period.
	CalendarPeriodID *int64
	// Weekday scopes the row to one ISO weekday; nil holds on every day.
	Weekday *int
	// SelectedWeekdays restricts the row to these ISO weekdays; empty holds
	// on every day.
	SelectedWeekdays []int
}

// Plans reports whether the roster puts the child on the template's
// occurrence on date in the planning period, with materialization's rule
// for enrollment rows (isEnrollmentValidOn). A periodID of 0 skips the
// period check.
func (r TemplatePlannedRoster) Plans(studentID int64, date calendar.Date, periodID int64) bool {
	if slices.Contains(r.TargetStudentIDs, studentID) {
		return true
	}
	return slices.ContainsFunc(r.Entries, func(entry TemplateRosterEntry) bool {
		return entry.StudentID == studentID && entry.appliesOn(date, periodID)
	})
}

func (e TemplateRosterEntry) appliesOn(date calendar.Date, periodID int64) bool {
	weekday := calendar.ISOWeekday(date)
	switch {
	case e.ValidFrom != nil && e.ValidFrom.After(date),
		e.ValidUntil != nil && !e.ValidUntil.After(date),
		periodID != 0 && e.CalendarPeriodID != nil && *e.CalendarPeriodID != periodID,
		e.Weekday != nil && *e.Weekday != weekday:
		return false
	}
	return len(e.SelectedWeekdays) == 0 || slices.Contains(e.SelectedWeekdays, weekday)
}

// TemplateSeriesSegment is one segment of a split series.
type TemplateSeriesSegment struct {
	TemplateID       int64
	CalendarPeriodID *int64
	ValidFrom        *calendar.Date
	// ValidUntil is exclusive; nil marks the living segment.
	ValidUntil *calendar.Date
}

// ConnectOfferingSourceCommand switches a series to offerings as source.
type ConnectOfferingSourceCommand struct {
	TemplateID    int64
	OfferingIDs   []int64
	GradeLevels   []int
	SchoolClasses []string
	EffectiveDate calendar.Date
}

// ConnectOfferingSourceResult names the segments that were switched.
type ConnectOfferingSourceResult struct {
	TemplateIDs   []int64
	EffectiveDate calendar.Date
}
