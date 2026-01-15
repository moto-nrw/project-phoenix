package fixed

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/logging"
	"github.com/moto-nrw/project-phoenix/models/auth"
)

// seedRolesAndPermissions creates the basic roles and permissions
func (s *Seeder) seedRolesAndPermissions(ctx context.Context) error {
	// Define roles
	roleData := []struct {
		name        string
		description string
	}{
		{"admin", "Full system administrator"},
		{"teacher", "Teacher with group management capabilities"},
		{"staff", "General staff member"},
		{"guardian", "Parent or guardian with limited access"},
	}

	// Create roles
	for _, data := range roleData {
		role := &auth.Role{
			Name:        data.name,
			Description: data.description,
		}
		role.CreatedAt = time.Now()
		role.UpdatedAt = time.Now()

		// Use raw query to avoid schema issues
		err := s.tx.NewRaw(`
			INSERT INTO auth.roles (name, description, created_at, updated_at) 
			VALUES (?, ?, ?, ?)
			ON CONFLICT (name) DO UPDATE SET updated_at = EXCLUDED.updated_at
			RETURNING id, created_at, updated_at
		`, role.Name, role.Description, role.CreatedAt, role.UpdatedAt).
			Scan(ctx, &role.ID, &role.CreatedAt, &role.UpdatedAt)
		if err != nil {
			return fmt.Errorf("failed to reload role %s: %w", data.name, err)
		}

		s.result.Roles = append(s.result.Roles, role)
	}

	// Create permissions - ALL permissions from constants.go
	allPermissions := []string{
		// Users
		permissions.UsersCreate,
		permissions.UsersRead,
		permissions.UsersUpdate,
		permissions.UsersDelete,
		permissions.UsersList,
		permissions.UsersManage,

		// Groups
		permissions.GroupsCreate,
		permissions.GroupsRead,
		permissions.GroupsUpdate,
		permissions.GroupsDelete,
		permissions.GroupsList,
		permissions.GroupsManage,
		permissions.GroupsAssign,

		// Activities
		permissions.ActivitiesCreate,
		permissions.ActivitiesRead,
		permissions.ActivitiesUpdate,
		permissions.ActivitiesDelete,
		permissions.ActivitiesList,
		permissions.ActivitiesManage,
		permissions.ActivitiesEnroll,
		permissions.ActivitiesAssign,

		// Visits
		permissions.VisitsCreate,
		permissions.VisitsRead,
		permissions.VisitsUpdate,
		permissions.VisitsDelete,
		permissions.VisitsList,
		permissions.VisitsManage,

		// Rooms
		permissions.RoomsCreate,
		permissions.RoomsRead,
		permissions.RoomsUpdate,
		permissions.RoomsDelete,
		permissions.RoomsList,
		permissions.RoomsManage,

		// Substitutions
		permissions.SubstitutionsCreate,
		permissions.SubstitutionsRead,
		permissions.SubstitutionsUpdate,
		permissions.SubstitutionsDelete,
		permissions.SubstitutionsList,
		permissions.SubstitutionsManage,

		// Schedules
		permissions.SchedulesCreate,
		permissions.SchedulesRead,
		permissions.SchedulesUpdate,
		permissions.SchedulesDelete,
		permissions.SchedulesList,
		permissions.SchedulesManage,

		// IoT
		permissions.IOTRead,
		permissions.IOTUpdate,
		permissions.IOTManage,

		// Feedback
		permissions.FeedbackCreate,
		permissions.FeedbackRead,
		permissions.FeedbackDelete,
		permissions.FeedbackList,
		permissions.FeedbackManage,

		// Config
		permissions.ConfigRead,
		permissions.ConfigUpdate,
		permissions.ConfigManage,

		// Auth
		permissions.AuthManage,
	}

	for _, permName := range allPermissions {
		// Parse resource and action from permission name
		parts := strings.Split(permName, ":")
		var resource, action string
		if len(parts) == 2 {
			resource = parts[0]
			action = parts[1]
		} else {
			// Fallback for malformed permission names
			resource = "unknown"
			action = permName
		}

		perm := &auth.Permission{
			Name:        permName,
			Description: fmt.Sprintf("Permission for %s", permName),
			Resource:    resource,
			Action:      action,
		}
		perm.CreatedAt = time.Now()
		perm.UpdatedAt = time.Now()

		// Use raw query to avoid schema issues
		err := s.tx.NewRaw(`
			INSERT INTO auth.permissions (name, description, resource, action, created_at, updated_at) 
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (name) DO UPDATE SET updated_at = EXCLUDED.updated_at
			RETURNING id
		`, perm.Name, perm.Description, perm.Resource, perm.Action, perm.CreatedAt, perm.UpdatedAt).
			Scan(ctx, &perm.ID)
		if err != nil {
			return fmt.Errorf("failed to create permission %s: %w", permName, err)
		}
	}

	// Assign permissions to roles
	rolePermissions := map[string][]string{
		"admin": allPermissions, // Admin gets everything

		"teacher": {
			// Users - read and update students in their groups
			permissions.UsersRead,
			permissions.UsersUpdate,
			permissions.UsersList,

			// Groups - full management of their groups
			permissions.GroupsCreate,
			permissions.GroupsRead,
			permissions.GroupsUpdate,
			permissions.GroupsDelete,
			permissions.GroupsList,
			permissions.GroupsManage,
			permissions.GroupsAssign,

			// Activities - full management
			permissions.ActivitiesCreate,
			permissions.ActivitiesRead,
			permissions.ActivitiesUpdate,
			permissions.ActivitiesDelete,
			permissions.ActivitiesList,
			permissions.ActivitiesManage,
			permissions.ActivitiesEnroll,
			permissions.ActivitiesAssign,

			// Visits - full management for their groups
			permissions.VisitsCreate,
			permissions.VisitsRead,
			permissions.VisitsUpdate,
			permissions.VisitsDelete,
			permissions.VisitsList,

			// Rooms - read access
			permissions.RoomsRead,
			permissions.RoomsList,

			// Substitutions - full management of their own
			permissions.SubstitutionsCreate,
			permissions.SubstitutionsRead,
			permissions.SubstitutionsUpdate,
			permissions.SubstitutionsDelete,
			permissions.SubstitutionsList,
			permissions.SubstitutionsManage,

			// Schedules - read access
			permissions.SchedulesRead,
			permissions.SchedulesList,

			// Feedback - read access
			permissions.FeedbackRead,
			permissions.FeedbackList,
		},

		"staff": {
			// Users - read only
			permissions.UsersRead,
			permissions.UsersList,

			// Groups - read only
			permissions.GroupsRead,
			permissions.GroupsList,

			// Activities - read only
			permissions.ActivitiesRead,
			permissions.ActivitiesList,

			// Visits - full management (staff handles check-ins/outs)
			permissions.VisitsCreate,
			permissions.VisitsRead,
			permissions.VisitsUpdate,
			permissions.VisitsDelete,
			permissions.VisitsList,

			// Rooms - read only
			permissions.RoomsRead,
			permissions.RoomsList,

			// Substitutions - read only
			permissions.SubstitutionsRead,
			permissions.SubstitutionsList,

			// Schedules - read only
			permissions.SchedulesRead,
			permissions.SchedulesList,
		},

		"guardian": {
			// Minimal access - limited to their children via policies
			permissions.UsersRead,
			permissions.GroupsRead,
			permissions.VisitsRead,
		},
	}

	// Map role names to IDs
	roleIDMap := make(map[string]int64)
	for _, role := range s.result.Roles {
		roleIDMap[role.Name] = role.ID
	}

	// Map permission names to IDs
	permIDMap := make(map[string]int64)
	for _, permName := range allPermissions {
		var permID int64
		err := s.tx.NewRaw(`
			SELECT id FROM auth.permissions WHERE name = ?
		`, permName).Scan(ctx, &permID)
		if err != nil {
			return fmt.Errorf("failed to load permission %s: %w", permName, err)
		}
		permIDMap[permName] = permID
	}

	// Create role-permission associations
	for roleName, permNames := range rolePermissions {
		roleID := roleIDMap[roleName]
		for _, permName := range permNames {
			permID := permIDMap[permName]

			rolePerm := &auth.RolePermission{
				RoleID:       roleID,
				PermissionID: permID,
			}
			rolePerm.CreatedAt = time.Now()
			rolePerm.UpdatedAt = time.Now()

			_, err := s.tx.NewRaw(`
				INSERT INTO auth.role_permissions (role_id, permission_id, created_at, updated_at)
				VALUES (?, ?, ?, ?)
				ON CONFLICT (role_id, permission_id) DO NOTHING
			`, rolePerm.RoleID, rolePerm.PermissionID, rolePerm.CreatedAt, rolePerm.UpdatedAt).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to assign permission %s to role %s: %w",
					permName, roleName, err)
			}
		}
	}

	if s.verbose && logging.Logger != nil {
		logging.Logger.WithField("count", len(s.result.Roles)).Info("Created roles and assigned permissions")
	}

	return nil
}
