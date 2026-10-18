package careplan

import (
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// StatusDayOverview asks for one page of the status-day overview: the days
// in the range, optionally of one status, inside the children's windows.
type StatusDayOverview struct {
	From     calendar.Date
	To       calendar.Date
	Status   string
	Children []StatusDayWindow
	Page     int
	PageSize int
}

// StatusDayWindow bounds the overview rows of one child; a nil bound leaves
// that side open.
type StatusDayWindow struct {
	StudentID int64
	From      *calendar.Date
	Until     *calendar.Date
}

// StatusDayOverviewOptions renders the overview in the schedule query shape.
// The overview shows the days still active and those the end-of-day run
// archived; a day cleared by staff or a guardian is gone. No child matches
// nothing.
func StatusDayOverviewOptions(overview StatusDayOverview) *StudentScheduleQueryOptions {
	filter := StudentScheduleQueryFilter{Conditions: []StudentScheduleQueryCondition{
		{Field: "date", Operator: ">=", Value: overview.From},
		{Field: "date", Operator: "<=", Value: overview.To},
	}}
	if overview.Status != "" {
		filter.Conditions = append(filter.Conditions, StudentScheduleQueryCondition{Field: "status", Operator: "=", Value: overview.Status})
	}
	active := StudentScheduleQueryFilter{
		Conditions: []StudentScheduleQueryCondition{{Field: "cleared_at", Operator: "IS NULL"}},
		Or: []StudentScheduleQueryFilter{{Conditions: []StudentScheduleQueryCondition{
			{Field: "source", Operator: "=", Value: excusedrequests.StudentStatusSourceEndOfDay},
		}}},
	}
	filter.And = []StudentScheduleQueryFilter{statusDayWindowsFilter(overview.Children), active}
	return &StudentScheduleQueryOptions{Filter: &filter, Limit: overview.PageSize, Offset: (overview.Page - 1) * overview.PageSize}
}

// statusDayWindowsFilter matches a row inside any child's window; no child
// matches nothing.
func statusDayWindowsFilter(children []StatusDayWindow) StudentScheduleQueryFilter {
	if len(children) == 0 {
		return StudentScheduleQueryFilter{Conditions: []StudentScheduleQueryCondition{{Field: "student_id", Operator: "=", Value: int64(-1)}}}
	}
	windows := make([]StudentScheduleQueryFilter, 0, len(children))
	for _, child := range children {
		window := StudentScheduleQueryFilter{Conditions: []StudentScheduleQueryCondition{{Field: "student_id", Operator: "=", Value: child.StudentID}}}
		if child.From != nil {
			window.Conditions = append(window.Conditions, StudentScheduleQueryCondition{Field: "date", Operator: ">=", Value: *child.From})
		}
		if child.Until != nil {
			window.Conditions = append(window.Conditions, StudentScheduleQueryCondition{Field: "date", Operator: "<=", Value: *child.Until})
		}
		windows = append(windows, window)
	}
	first := windows[0]
	if len(windows) > 1 {
		first.Or = windows[1:]
	}
	return first
}

// EndOfDayStatusFlagArchive is the end-of-day archive of a live status flag:
// the sick flag becomes a sick day, the excused flag an excused day, both
// with the end-of-day source. Any other flag is refused.
func EndOfDayStatusFlagArchive(flagColumn, sinceColumn string, date Date, fallback time.Time) (StatusFlagArchive, error) {
	var status string
	switch flagColumn {
	case "sick":
		status = StudentStatusDaySick
	case "excused":
		status = StudentStatusDayExcused
	default:
		return StatusFlagArchive{}, fmt.Errorf("unsupported status flag column %q", flagColumn)
	}
	return StatusFlagArchive{
		FlagColumn: flagColumn, SinceColumn: sinceColumn, Status: status, Date: date,
		ReportedFallback: fallback, Source: excusedrequests.StudentStatusSourceEndOfDay,
	}, nil
}
