package database

import (
	"context"
	"log/slog"
)

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

// databaseService implements the DatabaseService interface
type databaseService struct {
	deps   StatsDependencies
	logger *slog.Logger
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (s *databaseService) getLogger() *slog.Logger {
	if s.logger == nil {
		return slog.Default()
	}
	return s.logger
}

// NewService creates a new DatabaseService instance
func NewService(deps StatsDependencies, logger *slog.Logger) DatabaseService {
	return &databaseService{
		deps:   deps,
		logger: logger,
	}
}

// GetStats returns aggregated counts of all database entities

func (s *databaseService) GetStats(ctx context.Context, capabilities StatsCapabilities) (*StatsResponse, error) {
	permissions := StatsPermissions{
		CanViewStudents: capabilities.ViewStudents(), CanViewTeachers: capabilities.ViewTeachers(),
		CanViewRooms: capabilities.ViewRooms(), CanViewActivities: capabilities.ViewActivities(),
		CanViewGroups: capabilities.ViewGroups(), CanViewRoles: capabilities.ViewRoles(),
		CanViewDevices: capabilities.ViewDevices(), CanViewPermissions: capabilities.ViewPermissionCatalog(),
		CanViewTimetables: capabilities.ViewTimetables(), CanViewGradeTransitions: capabilities.ViewGradeTransitions(),
	}
	response := &StatsResponse{
		Permissions: permissions,
	}

	// One collector per entity type: permission gate -> CanViewX flag ->
	// count via repo List. The flag flips even when counting fails, so a
	// transient List error still renders the card (with count 0).
	// collectTeacherStats historically counts Staff (not Teacher) to match
	// what the personal page actually shows.
	collectors := []struct {
		canView bool
		label   string
		count   func(context.Context) (int, error)
		target  *int
	}{
		{response.Permissions.CanViewStudents, "students", s.deps.Students, &response.Students},
		{response.Permissions.CanViewTeachers, "staff", s.deps.Teachers, &response.Teachers},
		{response.Permissions.CanViewRooms, "rooms", s.deps.Rooms, &response.Rooms},
		{response.Permissions.CanViewActivities, "activities", s.deps.Activities, &response.Activities},
		{response.Permissions.CanViewGroups, "groups", s.deps.Groups, &response.Groups},
		{response.Permissions.CanViewRoles, "roles", s.deps.Roles, &response.Roles},
		{response.Permissions.CanViewDevices, "devices", s.deps.Devices, &response.Devices},
		{response.Permissions.CanViewPermissions, "permissions", s.deps.PermissionCount, &response.PermissionCount},
	}
	for _, c := range collectors {
		if !c.canView || c.count == nil {
			continue
		}
		if n, err := c.count(ctx); err != nil {
			s.getLogger().ErrorContext(ctx, "Error counting "+c.label, slog.String("error", err.Error()))
		} else {
			*c.target = n
		}
	}
	return response, nil
}
