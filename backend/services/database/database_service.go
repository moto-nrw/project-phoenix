package database

import (
	"cmp"
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
)

// databaseService implements the DatabaseService interface
type databaseService struct {
	repos  *repositories.Factory
	logger *slog.Logger
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (s *databaseService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

// NewService creates a new DatabaseService instance
func NewService(repos *repositories.Factory, logger *slog.Logger) DatabaseService {
	return &databaseService{
		repos:  repos,
		logger: logger,
	}
}

// GetStats returns aggregated counts of all database entities
func (s *databaseService) GetStats(ctx context.Context) (*StatsResponse, error) {
	claims := jwt.ClaimsFromCtx(ctx)
	response := &StatsResponse{
		Permissions: StatsPermissions{},
	}

	// One collector per entity type: permission gate -> CanViewX flag ->
	// count via repo List. The flag flips even when counting fails, so a
	// transient List error still renders the card (with count 0).
	// collectTeacherStats historically counts Staff (not Teacher) to match
	// what the personal page actually shows.
	collectors := []struct {
		perms   []string
		canView *bool
		label   string
		count   func() (int, error)
		target  *int
	}{
		{[]string{permissions.UsersRead, permissions.UsersList}, &response.Permissions.CanViewStudents, "students",
			func() (int, error) { xs, err := s.repos.Student.List(ctx, nil); return len(xs), err }, &response.Students},
		{[]string{permissions.UsersRead, permissions.UsersList}, &response.Permissions.CanViewTeachers, "staff",
			func() (int, error) { xs, err := s.repos.Staff.List(ctx, nil); return len(xs), err }, &response.Teachers},
		{[]string{permissions.RoomsRead, permissions.RoomsList}, &response.Permissions.CanViewRooms, "rooms",
			func() (int, error) { xs, err := s.repos.Room.List(ctx, nil); return len(xs), err }, &response.Rooms},
		{[]string{permissions.ActivitiesRead, permissions.ActivitiesList}, &response.Permissions.CanViewActivities, "activities",
			func() (int, error) { xs, err := s.repos.ActivityGroup.List(ctx, nil); return len(xs), err }, &response.Activities},
		{[]string{permissions.GroupsRead, permissions.GroupsList}, &response.Permissions.CanViewGroups, "groups",
			func() (int, error) { xs, err := s.repos.Group.List(ctx, nil); return len(xs), err }, &response.Groups},
		{[]string{permissions.AuthManage}, &response.Permissions.CanViewRoles, "roles",
			func() (int, error) { xs, err := s.repos.Role.List(ctx, nil); return len(xs), err }, &response.Roles},
		{[]string{permissions.IOTRead, permissions.IOTManage}, &response.Permissions.CanViewDevices, "devices",
			func() (int, error) { xs, err := s.repos.Device.List(ctx, nil); return len(xs), err }, &response.Devices},
		{[]string{permissions.AuthManage}, &response.Permissions.CanViewPermissions, "permissions",
			func() (int, error) { xs, err := s.repos.Permission.List(ctx, nil); return len(xs), err }, &response.PermissionCount},
	}
	for _, c := range collectors {
		if !checkUserPermission(claims, c.perms...) {
			continue
		}
		*c.canView = true
		if n, err := c.count(); err != nil {
			s.getLogger().ErrorContext(ctx, "Error counting "+c.label, slog.String("error", err.Error()))
		} else {
			*c.target = n
		}
	}
	collectTimetableStats(claims, response)
	collectGradeTransitionStats(claims, response)

	return response, nil
}

// collectGradeTransitionStats flips CanViewGradeTransitions when the user has
// the grade_transitions:read permission. Flag-only like timetables — the
// Jahrgangswechsel card links into a workflow, not a flat list.
func collectGradeTransitionStats(claims jwt.AppClaims, response *StatsResponse) {
	if !checkUserPermission(claims, permissions.GradeTransitionsRead) {
		return
	}
	response.Permissions.CanViewGradeTransitions = true
}

// collectTimetableStats flips CanViewTimetables when the user has the
// schedules:read or schedules:list permission. No count is exposed because
// the timetable hub card links into a per-week view, not a flat list.
func collectTimetableStats(claims jwt.AppClaims, response *StatsResponse) {
	if !checkUserPermission(claims, permissions.SchedulesRead, permissions.SchedulesList) {
		return
	}
	response.Permissions.CanViewTimetables = true
}

// checkUserPermission checks if user has any of the given permissions
func checkUserPermission(claims jwt.AppClaims, requiredPerms ...string) bool {
	for _, userPerm := range claims.Permissions {
		if userPerm == permissions.AdminWildcard || userPerm == permissions.FullAccess {
			return true
		}
		for _, required := range requiredPerms {
			if userPerm == required {
				return true
			}
		}
	}
	return false
}
