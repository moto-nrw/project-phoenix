package carecontract

import (
	"context"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The booked care of the contracts as the school reads it (#4683, ADR
// 0156): the Soll of Soll und Ist. A school reads, in its own tenant
// transaction, the contracts whose place of care it is and that left the
// draft; their versions say from when to when which modules are booked on
// which weekday and until when the child stays.

// BookedVersion is the content of one contract version that a day of care
// reads: from EffectiveFrom to EndsOn (nil: open) the services apply.
type BookedVersion struct {
	Number        int
	EffectiveFrom calendar.Date
	EndsOn        *calendar.Date
	Services      []Service
}

// BookedContract is a contract of a child at the reading school with every
// version that is neither rejected nor replaced, the newest first.
type BookedContract struct {
	ContractID int64
	StudentID  int64
	Versions   []BookedVersion
}

// BookedDay is what a contract books on one day.
type BookedDay struct {
	// InForce reports that a version of the contract covers the day. A day
	// outside every version leaves the Soll to the care plan.
	InForce bool
	// Booked reports that a module is booked on the weekday of the day; a
	// day in force without a module is a day without care.
	Booked bool
	// EndsAt is the latest end time of the day's modules: the child goes
	// home after the last module. Nil when a module of the day names no end
	// time, so the end is not known.
	EndsAt *calendar.WallClock
	// Modules names the day's modules, early care first.
	Modules []string
}

// Day answers what the contract books on date: the newest version that
// covers the date decides, and a module whose last day lies before the date
// no longer counts.
func (c BookedContract) Day(date calendar.Date) BookedDay {
	for _, version := range c.Versions {
		if date.Before(version.EffectiveFrom) || (version.EndsOn != nil && date.After(*version.EndsOn)) {
			continue
		}
		return version.day(date)
	}
	return BookedDay{}
}

func (v BookedVersion) day(date calendar.Date) BookedDay {
	result := BookedDay{InForce: true}
	known := true
	var latest *calendar.WallClock
	for _, service := range v.Services {
		weekday, ok := calendar.CanonicalDayToISOWeekday(service.Weekday)
		if !ok || weekday != calendar.ISOWeekday(date) || (service.LastDay != nil && date.After(*service.LastDay)) {
			continue
		}
		result.Booked = true
		if !slices.Contains(result.Modules, service.ModuleName) {
			result.Modules = append(result.Modules, service.ModuleName)
		}
		end, ok := wallClockOf(service.EndsAt)
		if !ok {
			known = false
			continue
		}
		if latest == nil || clockMinutes(end) > clockMinutes(*latest) {
			latest = &end
		}
	}
	if result.Booked && known {
		result.EndsAt = latest
	}
	return result
}

// wallClockOf parses an end time "HH:MM"; an empty or malformed one is not
// known.
func wallClockOf(value string) (calendar.WallClock, bool) {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return calendar.WallClock{}, false
	}
	clock, err := calendar.NewWallClock(parsed.Hour(), parsed.Minute(), 0, 0)
	return clock, err == nil
}

func clockMinutes(clock calendar.WallClock) int { return clock.Hour()*60 + clock.Minute() }

// BookedCare reads the booked care of the contracts at the school whose
// tenant transaction the caller holds. Drafts stay out: the school's policy
// does not show them, and they promise nothing yet.
type BookedCare interface {
	// BookedCare lists the contracts of the children with a version that
	// overlaps [from, to], by child. A child the school has no contract for
	// is missing from the map.
	BookedCare(ctx context.Context, studentIDs []int64, from, to calendar.Date) (map[int64]BookedContract, error)
}
