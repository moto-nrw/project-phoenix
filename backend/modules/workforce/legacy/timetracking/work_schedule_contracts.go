package timetracking

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// This file holds the work-schedule vocabulary the retained work-session
// service owns. The schedule rows and the pure Soll-minute derivations over
// them live in the Workforce public package (#3259); the names below alias
// them for the retained callers. The rows themselves are persisted by the
// workforce repositories behind the ports below.

// Day-of-week constants (ISO: 0=Monday, 6=Sunday).
const (
	DayMonday    = 0
	DayTuesday   = 1
	DayWednesday = 2
	DayThursday  = 3
	DayFriday    = 4
	DaySaturday  = 5
	DaySunday    = 6
)

const (
	// MaxScheduleRotation caps a schedule at four rotation weeks.
	MaxScheduleRotation = 4
	// MaxDailyTargetMinutes caps a single day's target working time at 12
	// hours.
	MaxDailyTargetMinutes = 720
)

type (
	WorkScheduleRow       = workforce.ScheduleRow
	WorkTimeTemplate      = workforce.ScheduleTemplate
	WorkTimeTemplateEntry = workforce.ScheduleTemplateEntry
)

var (
	ISODayIndex               = workforce.ISODayIndex
	ScheduleRotationLength    = workforce.ScheduleRotationLength
	ResolveRotationWeek       = workforce.ResolveRotationWeek
	ResolveScheduleAnchor     = workforce.ResolveScheduleAnchor
	DailyTargetFromSchedule   = workforce.DailyTargetFromSchedule
	DailyTargetFromTemplate   = workforce.DailyTargetFromTemplate
	WeeklyTargetFromSchedule  = workforce.WeeklyTargetFromSchedule
	WeeklyTargetsFromTemplate = workforce.WeeklyTargetsFromTemplate
)

// WorkSessionSchedules is the schedule capability used by work-session
// enforcement, summaries, and effective-dated schedule replacement.
type WorkSessionSchedules interface {
	GetByStaffIDAndDate(context.Context, int64, timezone.Date) ([]*WorkScheduleRow, error)
	FindByStaffIDsValidInRange(context.Context, []int64, timezone.Date, timezone.Date) ([]*WorkScheduleRow, error)
	GetCurrentByStaffID(context.Context, int64) ([]*WorkScheduleRow, error)
	ReplaceSchedule(context.Context, int64, []*WorkScheduleRow, timezone.Date) error
}

// WorkSessionTimeModels is the work-time-template capability the work-session
// service still calls: reading one template and saving a custom schedule as a
// new one. Template administration itself runs through the Workforce facade.
type WorkSessionTimeModels interface {
	FindByID(context.Context, int64) (*WorkTimeTemplate, error)
	Create(context.Context, *WorkTimeTemplate, []*WorkTimeTemplateEntry) error
}
