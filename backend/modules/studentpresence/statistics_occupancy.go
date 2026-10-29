package studentpresence

import "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

// Belegung nach Uhrzeit (#4764): how many children are there at which time of
// day, planned and actual, in a fixed raster over the care day. The section is
// part of the Statistik report so the Träger reports (#4713, #4714) can ask the
// same use case for it per school.

// StatisticsSectionOccupancy is the occupancy by time of day.
const StatisticsSectionOccupancy StatisticsSection = "occupancy"

// OccupancySlotMinutes is the raster width. A child counts in a slot when the
// care window overlaps it, so a child arriving at 07:45 counts from the 07:30
// slot on and a child picked up at 16:00 no longer counts at 16:00.
const OccupancySlotMinutes = 30

// PlannedCareDay is Care Plan's verdict for one child and day with the planned
// clock times, the input the composition root supplies. Expected is false for
// a day the child is not booked or was cancelled ("Kommt heute nicht");
// Arrival and Pickup are nil where the plan names no time.
type PlannedCareDay struct {
	Expected bool
	Arrival  *calendar.WallClock
	Pickup   *calendar.WallClock
}

// StatisticsOccupancySlot is one raster cell of one weekday. Minutes are Berlin
// wall-clock minutes after midnight.
type StatisticsOccupancySlot struct {
	StartMinute int
	// PlannedAverage is the mean planned head count over the weekday's care
	// days, PlannedMax the largest of them.
	PlannedAverage float64
	PlannedMax     int
	// ActualAverage and ActualMax are nil when no care day of the weekday lies
	// inside the attendance retention window.
	ActualAverage *float64
	ActualMax     *int
}

// StatisticsOccupancyWeekday is the raster for one ISO weekday (1 = Monday).
// A single-day report has exactly one, whose averages equal its maxima.
type StatisticsOccupancyWeekday struct {
	Weekday int
	// PlanDays is how many care days of this weekday the plan values cover,
	// ActualDays how many of them still have attendance data.
	PlanDays   int
	ActualDays int
	Slots      []StatisticsOccupancySlot
}

// StatisticsOccupancy is the occupancy section. Every weekday carries the same
// slot axis: from 07:00 to 17:00, widened to the first and last slot holding a
// child.
type StatisticsOccupancy struct {
	SlotMinutes int
	Weekdays    []StatisticsOccupancyWeekday
	// UntimedPlanDays counts child care days the plan expects without a usable
	// arrival and pickup time; those children are missing from the plan values.
	UntimedPlanDays int
	// ActualDataDays is gdpr.attendance_retention_days; attendance before
	// ActualDataFrom may already be deleted, so the actual values stop there.
	ActualDataDays int
	ActualDataFrom calendar.Date
}
