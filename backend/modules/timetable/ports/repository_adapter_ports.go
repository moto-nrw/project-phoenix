package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// RepositoryDirectories are the consumer-owned ports through which the retained
// adapters name what other owners hold: School Structure's education
// groups, Facilities' rooms, Workforce's Schichtarten, School Membership's
// staff and People Directory's persons and students. The composition root
// binds them over those owners' public capabilities; this package never
// names the owners itself.
type RepositoryDirectories struct {
	// GroupNames names education groups by ID. The binding skips IDs <= 0
	// and repeats; a group it cannot see is missing from the map.
	GroupNames func(ctx context.Context, ids []int64) (map[int64]string, error)
	// RoomNames names rooms by ID; a room it cannot see is missing.
	RoomNames func(ctx context.Context, ids []int64) (map[int64]string, error)
	// ShiftTypes lists the Schichtarten with their name and colour.
	ShiftTypes func(ctx context.Context) ([]ShiftType, error)
	// StaffPersons maps staff IDs to their person IDs; includeDeleted keeps
	// offboarded staff members.
	StaffPersons func(ctx context.Context, staffIDs []int64, includeDeleted bool) (map[int64]int64, error)
	// PersonNames names persons by ID; IDs <= 0 are skipped.
	PersonNames func(ctx context.Context, ids []int64) (map[int64]PersonName, error)
	// StudentAlumni reports, per student it can see, whether the student is
	// an alumnus.
	StudentAlumni func(ctx context.Context, ids []int64) (map[int64]bool, error)
	// StudentNames names students by ID.
	StudentNames func(ctx context.Context, ids []int64) (map[int64]PersonName, error)
	// ActivePeriods lists School Calendar's active calendar periods.
	ActivePeriods func(ctx context.Context) ([]timetable.TemplateCapacityPeriod, error)
}

// ShiftType is the name and colour of one Schichtart.
type ShiftType struct {
	ID    int64
	Name  string
	Color string
}

// PersonName is the display name of a person or student.
type PersonName struct {
	FirstName string
	LastName  string
}
