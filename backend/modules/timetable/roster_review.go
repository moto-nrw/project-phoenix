package timetable

import (
	"context"
	"errors"

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
