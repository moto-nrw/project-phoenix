package traegerview

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The occupancy of the schools of a Träger (#4676): how many children each
// school, each OGS group and each care form holds today, so the Träger-Büro
// can plan. It carries counts and names of groups and offers, never a child.
//
// The binding definitions:
//
//   - Children are the actively managed children of the Stichtagszahl rule
//     (#2791), counted on today's day, the same roster as ChildrenTotal of
//     SchoolFigures.
//   - A group counts the children of that roster whose school membership
//     names the group. WithoutGroup counts the others.
//   - A care form is a care offering of the school that counts as care
//     (counts_as_care) and runs today or holds a booking today. It runs today
//     when it is active and its enrollment phase is active with a service
//     period that holds today, named by Enrollment (#5977). It counts the
//     children of the roster with a booking of the offering valid today, so an
//     offering without one shows 0 children and all its places free.
//     Capacity is the offering's limit, nil without one. WithoutCareForm counts
//     the children of the roster without such a booking.

// OccupancyGroup is one OGS group of a school with its children.
type OccupancyGroup struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Children int    `json:"children"`
}

// OccupancyCareForm is one care offering of a school with its booked children
// and its limit.
type OccupancyCareForm struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Children int    `json:"children"`
	Capacity *int   `json:"capacity"`
}

// SchoolOccupancy is the occupancy of one school of the Träger. Groups and
// CareForms are ordered by name.
type SchoolOccupancy struct {
	School
	Children        int                 `json:"children"`
	WithoutGroup    int                 `json:"without_group"`
	Groups          []OccupancyGroup    `json:"groups"`
	WithoutCareForm int                 `json:"without_care_form"`
	CareForms       []OccupancyCareForm `json:"care_forms"`
}

// Occupancy is the occupancy of every school of the Träger, ordered by name,
// on Day, read at AsOf.
type Occupancy struct {
	Day     calendar.Date     `json:"day"`
	AsOf    time.Time         `json:"as_of"`
	Schools []SchoolOccupancy `json:"schools"`
}

// GroupMembership is the OGS group of each school membership, at School
// Membership: membership ID to group ID, for the memberships with a group.
// It joins the projection's administrative transaction and reads only the
// given schools.
type GroupMembership interface {
	MembershipGroupIDsByTenant(ctx context.Context, schools []int64) (map[int64]map[int64]int64, error)
}

// NamedGroup is an OGS group with its name.
type NamedGroup struct {
	ID   int64
	Name string
}

// GroupNames names the OGS groups of the schools, ordered by name, at School
// Structure. It
// joins the projection's administrative transaction and reads only the given
// schools.
type GroupNames interface {
	GroupsByTenant(ctx context.Context, schools []int64) (map[int64][]NamedGroup, error)
}

// CareOfferingBookings is one care offering that counts as care, with the
// children of the online enrollment (request child IDs) booked on the day,
// none for a running offering without a booking.
type CareOfferingBookings struct {
	ID              int64
	Name            string
	Capacity        *int
	RequestChildIDs []int64
}

// CareBookings reads the care offerings of the schools that count as care and
// hold a booking valid on the day or belong to one of the running phases,
// ordered by name, at Care Plan. It joins the projection's administrative
// transaction and reads only the given schools.
type CareBookings interface {
	CareOfferingBookingsByTenant(ctx context.Context, schools []int64, day calendar.Date, runningPhases []int64) (map[int64][]CareOfferingBookings, error)
}

// BookedChildren ties each approved child of the online enrollment to the
// child it became, at Enrollment: request child ID to student ID. It also
// names the running phases: active, with a service period that holds the
// day (#5977). Enrollment owns that period, so Care Plan takes the phases
// instead of reading them. It joins the projection's administrative
// transaction and reads only the given schools.
type BookedChildren interface {
	BookedChildrenAndRunningPhases(ctx context.Context, schools []int64, day calendar.Date) (children map[int64]map[int64]int64, runningPhases []int64, err error)
}

// OccupancyPorts are the owners' reads behind the occupancy (#4676).
type OccupancyPorts struct {
	Groups     GroupMembership
	GroupNames GroupNames
	Bookings   CareBookings
	Children   BookedChildren
}
