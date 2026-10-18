package stats

import (
	"context"
)

// StatsResponse represents the database statistics with counts and permissions
type StatsResponse struct {
	Students        int `json:"students"`
	Teachers        int `json:"teachers"`
	Rooms           int `json:"rooms"`
	Activities      int `json:"activities"`
	Groups          int `json:"groups"`
	Roles           int `json:"roles"`
	Devices         int `json:"devices"`
	PermissionCount int `json:"permissionCount"`

	// Permissions indicate which counts the user is allowed to see
	Permissions StatsPermissions `json:"permissions"`
}

// StatsPermissions indicates which statistics the user has permission to view
type StatsPermissions struct {
	CanViewStudents    bool `json:"canViewStudents"`
	CanViewTeachers    bool `json:"canViewTeachers"`
	CanViewRooms       bool `json:"canViewRooms"`
	CanViewActivities  bool `json:"canViewActivities"`
	CanViewGroups      bool `json:"canViewGroups"`
	CanViewRoles       bool `json:"canViewRoles"`
	CanViewDevices     bool `json:"canViewDevices"`
	CanViewPermissions bool `json:"canViewPermissions"`
	CanViewTimetables  bool `json:"canViewTimetables"`
	// CanViewGradeTransitions gates the Jahrgangswechsel card (#405); no
	// count is exposed because the card links into a workflow, not a list.
	CanViewGradeTransitions bool `json:"canViewGradeTransitions"`
}

type StatsCapabilities interface {
	ViewStudents() bool
	ViewTeachers() bool
	ViewRooms() bool
	ViewActivities() bool
	ViewGroups() bool
	ViewRoles() bool
	ViewDevices() bool
	ViewPermissionCatalog() bool
	ViewTimetables() bool
	ViewGradeTransitions() bool
}

// Service defines operations for database statistics and management
// Named following Go single-method interface conventions (method name + er suffix)
type Service interface {
	// GetStats returns aggregated counts of all database entities
	// Counts are filtered based on user permissions - returns 0 for entities user cannot access
	GetStats(ctx context.Context, capabilities StatsCapabilities) (*StatsResponse, error)
}

// StatsDependencies is the composition contract for the statistics service.
// Authorization and persistence stay in their owning adapters; this service
// only orchestrates the capabilities and counts it receives.
type StatsDependencies struct {
	Students        func(context.Context) (int, error)
	Teachers        func(context.Context) (int, error)
	Rooms           func(context.Context) (int, error)
	Activities      func(context.Context) (int, error)
	Groups          func(context.Context) (int, error)
	Roles           func(context.Context) (int, error)
	Devices         func(context.Context) (int, error)
	PermissionCount func(context.Context) (int, error)
}
