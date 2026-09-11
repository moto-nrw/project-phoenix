package domain

import "time"

// School is the tenant fact the dashboard needs to decide whether a display
// link is still alive and which name to render.
type School struct {
	Name    string
	Active  bool
	Deleted bool
}

// ActiveSession is one running room session. TemplateID is set only for
// template-backed sessions; spontaneous sessions carry nil.
type ActiveSession struct {
	ID         int64
	RoomID     int64
	TemplateID *int64
	Running    bool
}

// ActivityTemplate is one planned activity definition with its category.
type ActivityTemplate struct {
	ID               int64
	Name             string
	CategoryName     *string
	ParticipantLimit *int
}

// PlannedActivity is one of today's activity instances.
type PlannedActivity struct {
	ID              int64
	Title           string
	RoomID          int64
	ActivityGroupID *int64
	ActiveGroupID   *int64
	Planned         bool
	// StartWallClock is the instance's TIME column normalised to a wall
	// clock by the composition root, so this module never re-derives it.
	StartWallClock time.Time
}

// Room is the Facilities projection the occupancy panel renders.
type Room struct {
	ID       int64
	TenantID int64
	Name     string
	Capacity *int
}

// PresentVisit is one open room visit of a present student.
type PresentVisit struct {
	ActiveGroupID int64
	StudentID     int64
}

// PickupTime is one present student's effective pickup wall clock today.
type PickupTime struct {
	StudentID      int64
	PickupWallTime *time.Time
}

// DashboardFacts is everything one dashboard render reads, already mapped
// into this owner's values by the composition root.
type DashboardFacts struct {
	Rooms          []Room
	ActiveSessions []ActiveSession
	Templates      []ActivityTemplate
	Planned        []PlannedActivity
	OpenVisits     []PresentVisit
	PresentStudent []int64
}
