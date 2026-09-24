package careplan

import (
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The vocabulary in this file is how the care-offering catalog reads the
// timetable a care offering materializes into: Timetable activity groups and
// schedules, School Calendar planning periods, timeframes and exceptions, and
// the Enrollment phase whose service window the offering covers. The owners
// describe their rows in their own words; the composition translates them.

// LinkedGroup is an activity group a care offering can link to.
type LinkedGroup struct {
	ID               int64
	IsTemplate       bool
	Archived         bool
	CalendarPeriodID *int64
	PlannedRoomID    *int64
}

// LinkedSchedule is one weekday rule of a linked group. Weekday is ISO
// (1=Monday .. 7=Sunday); nil validity bounds are open.
type LinkedSchedule struct {
	GroupID          int64
	Weekday          int
	TimeframeID      *int64
	CalendarPeriodID *int64
	WeekPattern      int
	ValidFrom        *calendar.Date
	ValidUntil       *calendar.Date
}

// LinkedPeriod is a School Calendar planning period.
type LinkedPeriod struct {
	ID              int64
	StartDate       calendar.Date
	EndDate         calendar.Date
	IsActive        bool
	WeekCycleLength int
	// WeekCycleAnchor is empty when no anchor is set.
	WeekCycleAnchor string
}

// LinkedTimeframe is a timeframe as wall-clock values; a nil end is open.
type LinkedTimeframe struct {
	ID        int64
	StartTime time.Time
	EndTime   *time.Time
}

// LinkedException is a cancellation or modification of one occurrence of a
// linked group.
type LinkedException struct {
	GroupID   int64
	Date      calendar.Date
	Cancelled bool
	Modified  bool
	RoomID    *int64
	StartTime *time.Time
	EndTime   *time.Time
}

// OfferingPhase is the service window of the enrollment phase an offering
// belongs to.
type OfferingPhase struct {
	ID           int64
	Name         string
	ServiceStart calendar.Date
	ServiceEnd   calendar.Date
}

// LinkedSegment is one live segment of a linked template's split series with
// the period and schedules it materializes from. Period is nil for a
// historical non-template link.
type LinkedSegment struct {
	Group     LinkedGroup
	Period    *LinkedPeriod
	Schedules []LinkedSchedule
}

// OfferingSources is the validated source set of an offering-sourced
// template (#2137): the surviving offerings in request order, their shared
// phase, and the ids that no longer resolve. Booking materialization (#3560)
// resyncs the template's roster from it.
type OfferingSources struct {
	Offerings []CareOffering
	Phase     *OfferingPhase
	Dropped   []int64
}

// SchedulesOverlapPhase reports whether any schedule can produce an
// occurrence inside the phase's service window.
func SchedulesOverlapPhase(schedules []LinkedSchedule, phase OfferingPhase) bool {
	for _, schedule := range schedules {
		startsAfterPhase := schedule.ValidFrom != nil && schedule.ValidFrom.After(phase.ServiceEnd)
		endsBeforeOrOnPhase := schedule.ValidUntil != nil && !schedule.ValidUntil.After(phase.ServiceStart)
		if !startsAfterPhase && !endsBeforeOrOnPhase {
			return true
		}
	}
	return false
}

// ValidatePhaseWithinPeriod refuses a phase whose service window leaves the
// template's planning period. A nil period passes.
func ValidatePhaseWithinPeriod(phase OfferingPhase, period *LinkedPeriod) error {
	if period == nil {
		return nil
	}
	if phase.ServiceStart.Before(period.StartDate) || phase.ServiceEnd.After(period.EndDate) {
		return fmt.Errorf("%w: %s: %w", ErrCareOfferingConfigInvalid,
			"linked timetable template period does not contain the enrollment phase",
			ErrCareOfferingTemplatePeriodMismatch)
	}
	return nil
}
