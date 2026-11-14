package workforce

import "context"

// ShiftTypeRemoval names what removing a shift type did (#6845).
type ShiftTypeRemoval string

const (
	// ShiftTypeRemovalDeleted means no planned shift or series carried the
	// type, so its row is gone.
	ShiftTypeRemovalDeleted ShiftTypeRemoval = "deleted"
	// ShiftTypeRemovalArchived means planned shifts or series still carry
	// the type: it keeps labelling them but is no longer offered.
	ShiftTypeRemovalArchived ShiftTypeRemoval = "archived"
)

// ShiftTypeReferences counts the planned rows that carry one shift type.
type ShiftTypeReferences struct {
	Shifts int
	Series int
}

// ShiftTypeRetirement removes shift types without erasing the
// classification of the shifts and series planned with them (#6845).
type ShiftTypeRetirement interface {
	// RemoveShiftType deletes an unreferenced type and archives a type that
	// planned shifts or series still carry, in one unit of work that holds
	// the type row so no concurrent plan write can slip in between.
	RemoveShiftType(context.Context, int64) (ShiftTypeRemoval, error)
	ShiftPlanReferenceQuery
}

// ShiftPlanReferences counts what the school's shift plan still references:
// the shifts and series per shift type, which the shift-type page names
// before a removal, and the series per calendar period, whose FK has no ON
// DELETE action, so a period with series cannot be deleted (#6845).
// Unreferenced types and periods are absent.
type ShiftPlanReferences struct {
	ByShiftType            map[int64]ShiftTypeReferences
	SeriesByCalendarPeriod map[int64]int
}

// ShiftPlanReferenceQuery serves the shift-type page and the School Calendar
// deletion preview.
type ShiftPlanReferenceQuery interface {
	// ShiftPlanReferences counts both in one read of the caller's school.
	ShiftPlanReferences(context.Context) (ShiftPlanReferences, error)
}
