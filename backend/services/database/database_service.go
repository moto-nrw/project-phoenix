package database

import (
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
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
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

	// Collect stats for each entity type
	collectStudentStats(ctx, s, claims, response)
	collectTeacherStats(ctx, s, claims, response)
	collectRoomStats(ctx, s, claims, response)
	collectActivityStats(ctx, s, claims, response)
	collectGroupStats(ctx, s, claims, response)
	collectRoleStats(ctx, s, claims, response)
	collectDeviceStats(ctx, s, claims, response)
	collectPermissionStats(ctx, s, claims, response)
	collectTimetableStats(claims, response)

	return response, nil
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

// collectStudentStats collects student statistics
func collectStudentStats(ctx context.Context, s *databaseService, claims jwt.AppClaims, response *StatsResponse) {
	if !checkUserPermission(claims, permissions.UsersRead, permissions.UsersList) {
		return
	}

	response.Permissions.CanViewStudents = true
	if students, err := s.repos.Student.List(ctx, nil); err != nil {
		s.getLogger().ErrorContext(ctx, "Error counting students", slog.String("error", err.Error()))
	} else {
		response.Students = len(students)
	}
}

// collectTeacherStats collects staff/personal statistics
// Uses Staff.List (not Teacher.List) to match what the personal page actually shows
func collectTeacherStats(ctx context.Context, s *databaseService, claims jwt.AppClaims, response *StatsResponse) {
	if !checkUserPermission(claims, permissions.UsersRead, permissions.UsersList) {
		return
	}

	response.Permissions.CanViewTeachers = true
	if staff, err := s.repos.Staff.List(ctx, nil); err != nil {
		s.getLogger().ErrorContext(ctx, "Error counting staff", slog.String("error", err.Error()))
	} else {
		response.Teachers = len(staff)
	}
}

// collectRoomStats collects room statistics
func collectRoomStats(ctx context.Context, s *databaseService, claims jwt.AppClaims, response *StatsResponse) {
	if !checkUserPermission(claims, permissions.RoomsRead, permissions.RoomsList) {
		return
	}

	response.Permissions.CanViewRooms = true
	if rooms, err := s.repos.Room.List(ctx, nil); err != nil {
		s.getLogger().ErrorContext(ctx, "Error counting rooms", slog.String("error", err.Error()))
	} else {
		response.Rooms = len(rooms)
	}
}

// collectActivityStats collects activity statistics
func collectActivityStats(ctx context.Context, s *databaseService, claims jwt.AppClaims, response *StatsResponse) {
	if !checkUserPermission(claims, permissions.ActivitiesRead, permissions.ActivitiesList) {
		return
	}

	response.Permissions.CanViewActivities = true
	if activities, err := s.repos.ActivityGroup.List(ctx, nil); err != nil {
		s.getLogger().ErrorContext(ctx, "Error counting activities", slog.String("error", err.Error()))
	} else {
		response.Activities = len(activities)
	}
}

// collectGroupStats collects group statistics
func collectGroupStats(ctx context.Context, s *databaseService, claims jwt.AppClaims, response *StatsResponse) {
	if !checkUserPermission(claims, permissions.GroupsRead, permissions.GroupsList) {
		return
	}

	response.Permissions.CanViewGroups = true
	if groups, err := s.repos.Group.List(ctx, nil); err != nil {
		s.getLogger().ErrorContext(ctx, "Error counting groups", slog.String("error", err.Error()))
	} else {
		response.Groups = len(groups)
	}
}

// collectRoleStats collects role statistics
func collectRoleStats(ctx context.Context, s *databaseService, claims jwt.AppClaims, response *StatsResponse) {
	if !checkUserPermission(claims, permissions.AuthManage) {
		return
	}

	response.Permissions.CanViewRoles = true
	if roles, err := s.repos.Role.List(ctx, nil); err != nil {
		s.getLogger().ErrorContext(ctx, "Error counting roles", slog.String("error", err.Error()))
	} else {
		response.Roles = len(roles)
	}
}

// collectDeviceStats collects device statistics
func collectDeviceStats(ctx context.Context, s *databaseService, claims jwt.AppClaims, response *StatsResponse) {
	if !checkUserPermission(claims, permissions.IOTRead, permissions.IOTManage) {
		return
	}

	response.Permissions.CanViewDevices = true
	if devices, err := s.repos.Device.List(ctx, nil); err != nil {
		s.getLogger().ErrorContext(ctx, "Error counting devices", slog.String("error", err.Error()))
	} else {
		response.Devices = len(devices)
	}
}

// collectPermissionStats collects permission statistics
func collectPermissionStats(ctx context.Context, s *databaseService, claims jwt.AppClaims, response *StatsResponse) {
	if !checkUserPermission(claims, permissions.AuthManage) {
		return
	}

	response.Permissions.CanViewPermissions = true
	if perms, err := s.repos.Permission.List(ctx, nil); err != nil {
		s.getLogger().ErrorContext(ctx, "Error counting permissions", slog.String("error", err.Error()))
	} else {
		response.PermissionCount = len(perms)
	}
}
