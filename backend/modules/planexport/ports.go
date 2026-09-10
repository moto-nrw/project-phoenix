package planexport

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/services/listexport"
)

// Consumer-owned ports. Every port reads the tenant from the context and
// returns plain records; none of them writes. The composition root binds them
// to the owners that hold the facts (Timetable & Activities for shifts,
// blocks and their staff, People Directory for names, Facilities for rooms,
// School Calendar for closing days and holidays). Clock values are wall-clock
// times of the day; calendar days are Date strings.

// StaffMember is one row of the Dienstplan: the person a shift belongs to.
// Empty names print as "Unbekannt", which is how a staff row without a
// person record still keeps its shifts on the sheet.
type StaffMember struct {
	ID        int64
	FirstName string
	LastName  string
}

// Shift is one planned presence window of a staff member.
type Shift struct {
	ID        int64
	StaffID   int64
	Date      Date
	StartTime time.Time
	EndTime   time.Time
	// ShiftTypeID names the Schichtart, nil for a shift without one.
	ShiftTypeID *int64
	// OriginShiftID links a replacement shift to the cancelled shift it
	// covers (#1841), nil for an ordinary shift.
	OriginShiftID *int64
	Cancelled     bool
	// ChangeReason is why the shift was cancelled; printed on the internal
	// sheet only.
	ChangeReason *string
	Notes        string
}

// Interval is a wall-clock window inside a day.
type Interval struct {
	StartTime time.Time
	EndTime   time.Time
}

// Assignment is one timetable block a staff member is planned on.
type Assignment struct {
	StaffID       int64
	Date          Date
	StartTime     time.Time
	EndTime       time.Time
	ActivityTitle string
	// ActivityGroupID identifies the Angebot the block was materialized from,
	// nil for a spontaneous block. Titles are not unique, so a row keyed by
	// title alone would merge two separately configured Angebote.
	ActivityGroupID *int64
	RoomName        string
	IsSubstitute    bool
	IsAbsent        bool
	// UncoveredIntervals are the parts of the block no shift covers;
	// printed on the internal sheet only.
	UncoveredIntervals []Interval
}

// StaffScheduleOverview is the staff week the Dienstplan screen renders
// from, reduced to what the printout needs.
type StaffScheduleOverview struct {
	Staff       []*StaffMember
	Shifts      []*Shift
	Assignments []Assignment
}

// ShiftType is a Schichtart's label and colour.
type ShiftType struct {
	ID    int64
	Name  string
	Color string
}

// Instance is one materialized Betreuungsblock.
type Instance struct {
	ID        int64
	Date      Date
	StartTime time.Time
	EndTime   time.Time
	Title     string
	// ActivityGroupID identifies the Angebot, nil for a spontaneous block.
	ActivityGroupID *int64
	RoomID          int64
	Cancelled       bool
	// CancelReason, Notes and UnderstaffedNote are printed on the internal
	// sheet only.
	CancelReason     *string
	Notes            *string
	UnderstaffedNote *string
}

// InstanceStaff is one staff member planned on a block.
type InstanceStaff struct {
	InstanceID int64
	StaffID    int64
	// RoomID overrides the block's room for this person, nil when they are
	// in the block's own room.
	RoomID       *int64
	IsSubstitute bool
	IsAbsent     bool
}

// Room is a room's printed name.
type Room struct {
	ID   int64
	Name string
}

// ActivityGroup is the Angebot a block was materialized from, reduced to the
// Planungsspur that colours it.
type ActivityGroup struct {
	ID              int64
	PlanningTrackID *int64
}

// PlanningTrack is a Planungsspur's colour.
type PlanningTrack struct {
	ID    int64
	Color string
}

// Holiday is a public holiday inside the printed range.
type Holiday struct {
	Date Date
	Name string
}

// ClosingPeriod is a tenant-declared closure, inclusive on both ends.
type ClosingPeriod struct {
	StartDate Date
	EndDate   Date
	Reason    string
}

// OverviewReader supplies the staff week for the Dienstplan. It is the one
// read the Dienstplan cannot exist without.
type OverviewReader interface {
	StaffScheduleOverview(ctx context.Context, from, to Date) (*StaffScheduleOverview, error)
}

// ShiftTypeReader resolves Schichtart names and colours for the shift cells.
type ShiftTypeReader interface {
	ListShiftTypes(ctx context.Context) ([]*ShiftType, error)
}

// InstanceReader supplies the blocks of the Betreuungsplan, cancelled ones
// included. It is the one read the Betreuungsplan cannot exist without.
type InstanceReader interface {
	InstancesInRange(ctx context.Context, from, to Date) ([]*Instance, error)
}

// InstanceStaffReader supplies the staff planned on the given blocks.
type InstanceStaffReader interface {
	InstanceStaffByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*InstanceStaff, error)
}

// InstanceStudentCountReader returns the number of children per block in
// one grouped query, so a Betreuungsplan week costs one count query rather
// than one per block. Blocks without a non-absent child are absent from the
// map.
type InstanceStudentCountReader interface {
	CountNonAbsentByInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]int, error)
}

// RoomReader resolves the printed room names.
type RoomReader interface {
	RoomsByIDs(ctx context.Context, ids []int64) ([]*Room, error)
}

// StaffNameReader resolves the names of the staff planned on blocks. A
// missing or nil entry prints as "Unbekannt".
type StaffNameReader interface {
	StaffByIDs(ctx context.Context, ids []int64) (map[int64]*StaffMember, error)
}

// ActivityGroupReader and PlanningTrackReader resolve the planning-track
// colour of a Betreuungsblock, matching the planner.
type ActivityGroupReader interface {
	ActivityGroupsByIDs(ctx context.Context, ids []int64) ([]*ActivityGroup, error)
}

// PlanningTrackReader lists the tenant's Planungsspuren.
type PlanningTrackReader interface {
	ListPlanningTracks(ctx context.Context) ([]*PlanningTrack, error)
}

// ClosingDayReader supplies the closures overlapping [from, to].
type ClosingDayReader interface {
	ClosingDaysInRange(ctx context.Context, from, to Date) ([]*ClosingPeriod, error)
}

// HolidayReader supplies the public holidays in [from, to].
type HolidayReader interface {
	HolidaysInRange(ctx context.Context, from, to Date) ([]Holiday, error)
}

// Renderer is the slice of listexport this capability needs.
type Renderer interface {
	Render(doc listexport.Document, format listexport.Format, filenameBase string) (listexport.File, error)
}

// Dependencies are the reads the two exports need. The instance-side
// readers are deliberately separate from the overview — the Betreuungsplan
// cannot reuse the staff schedule overview, because that projection drops
// cancelled blocks and only knows blocks that have staff assigned, while a
// care plan must print a block whether or not anyone is on it yet.
type Dependencies struct {
	Overview      OverviewReader
	ShiftTypes    ShiftTypeReader
	Instances     InstanceReader
	InstanceStaff InstanceStaffReader
	Students      InstanceStudentCountReader
	Rooms         RoomReader
	Staff         StaffNameReader
	// ActivityGroups and PlanningTracks drive the colour bar on the care plan.
	// Optional: without them a block simply prints without its colour.
	ActivityGroups ActivityGroupReader
	PlanningTracks PlanningTrackReader
	// ClosingDays and Holidays label the days nobody is expected to work.
	// Both are optional: without them a closed day simply prints as empty,
	// which is a worse sheet but never a wrong one.
	ClosingDays ClosingDayReader
	Holidays    HolidayReader
	Renderer    Renderer
}
