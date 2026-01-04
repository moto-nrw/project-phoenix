package database

import (
	"context"
	"log"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
)

// databaseService implements the DatabaseService interface
type databaseService struct {
	repos *repositories.Factory
}

// NewService creates a new DatabaseService instance
func NewService(repos *repositories.Factory) DatabaseService {
	return &databaseService{
		repos: repos,
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

	return response, nil
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
		log.Printf("Error counting students: %v", err)
	} else {
		response.Students = len(students)
	}
}

// collectTeacherStats collects teacher statistics
func collectTeacherStats(ctx context.Context, s *databaseService, claims jwt.AppClaims, response *StatsResponse) {
	if !checkUserPermission(claims, permissions.UsersRead, permissions.UsersList) {
		return
	}

	response.Permissions.CanViewTeachers = true
	if teachers, err := s.repos.Teacher.List(ctx, nil); err != nil {
		log.Printf("Error counting teachers: %v", err)
	} else {
		response.Teachers = len(teachers)
	}
}

// collectRoomStats collects room statistics
func collectRoomStats(ctx context.Context, s *databaseService, claims jwt.AppClaims, response *StatsResponse) {
	if !checkUserPermission(claims, permissions.RoomsRead, permissions.RoomsList) {
		return
	}

	response.Permissions.CanViewRooms = true
	if rooms, err := s.repos.Room.List(ctx, nil); err != nil {
		log.Printf("Error counting rooms: %v", err)
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
		log.Printf("Error counting activities: %v", err)
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
		log.Printf("Error counting groups: %v", err)
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
		log.Printf("Error counting roles: %v", err)
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
		log.Printf("Error counting devices: %v", err)
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
		log.Printf("Error counting permissions: %v", err)
	} else {
		response.PermissionCount = len(perms)
	}
}
