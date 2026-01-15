package database

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
}

// StatsGetter defines operations for database statistics and management
// Named following Go single-method interface conventions (method name + er suffix)
type StatsGetter interface {
	// GetStats returns aggregated counts of all database entities
	// Counts are filtered based on user permissions - returns 0 for entities user cannot access
	GetStats(ctx context.Context) (*StatsResponse, error)
}

// DatabaseService is an alias for backward compatibility (deprecated - use StatsGetter)
type DatabaseService = StatsGetter
