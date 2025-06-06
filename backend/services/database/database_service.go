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
	// Get claims from context to check permissions
	claims := jwt.ClaimsFromCtx(ctx)

	// Initialize response
	response := &StatsResponse{
		Permissions: StatsPermissions{},
	}

	// Helper function to check if user has permission
	hasPermission := func(permission string) bool {
		for _, p := range claims.Permissions {
			if p == permission || p == permissions.AdminWildcard || p == permissions.FullAccess {
				return true
			}
		}
		return false
	}

	// Check and get student count
	if hasPermission(permissions.UsersRead) || hasPermission(permissions.UsersList) {
		response.Permissions.CanViewStudents = true
		students, err := s.repos.Student.List(ctx, nil)
		if err != nil {
			log.Printf("Error counting students: %v", err)
			// Continue with other counts even if this fails
		} else {
			response.Students = len(students)
		}
	}

	// Check and get teacher count
	if hasPermission(permissions.UsersRead) || hasPermission(permissions.UsersList) {
		response.Permissions.CanViewTeachers = true
		teachers, err := s.repos.Teacher.List(ctx, nil)
		if err != nil {
			log.Printf("Error counting teachers: %v", err)
		} else {
			response.Teachers = len(teachers)
		}
	}

	// Check and get room count
	if hasPermission(permissions.RoomsRead) || hasPermission(permissions.RoomsList) {
		response.Permissions.CanViewRooms = true
		rooms, err := s.repos.Room.List(ctx, nil)
		if err != nil {
			log.Printf("Error counting rooms: %v", err)
		} else {
			response.Rooms = len(rooms)
		}
	}

	// Check and get activity count
	if hasPermission(permissions.ActivitiesRead) || hasPermission(permissions.ActivitiesList) {
		response.Permissions.CanViewActivities = true
		activities, err := s.repos.ActivityGroup.List(ctx, nil)
		if err != nil {
			log.Printf("Error counting activities: %v", err)
		} else {
			response.Activities = len(activities)
		}
	}

	// Check and get group count
	if hasPermission(permissions.GroupsRead) || hasPermission(permissions.GroupsList) {
		response.Permissions.CanViewGroups = true
		groups, err := s.repos.Group.List(ctx, nil)
		if err != nil {
			log.Printf("Error counting groups: %v", err)
		} else {
			response.Groups = len(groups)
		}
	}

	// Check and get role count - using AuthManage permission since there's no specific roles permission
	if hasPermission(permissions.AuthManage) {
		response.Permissions.CanViewRoles = true
		roles, err := s.repos.Role.List(ctx, nil)
		if err != nil {
			log.Printf("Error counting roles: %v", err)
		} else {
			response.Roles = len(roles)
		}
	}

	// Check and get permission count - using AuthManage permission
	if hasPermission(permissions.AuthManage) {
		response.Permissions.CanViewPermissions = true
		perms, err := s.repos.Permission.List(ctx, nil)
		if err != nil {
			log.Printf("Error counting permissions: %v", err)
		} else {
			response.PermissionCount = len(perms)
		}
	}

	return response, nil
}
