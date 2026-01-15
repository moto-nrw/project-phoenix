package database

import (
	activitiesModels "github.com/moto-nrw/project-phoenix/internal/core/domain/activities"
	authModels "github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	educationModels "github.com/moto-nrw/project-phoenix/internal/core/domain/education"
	facilitiesModels "github.com/moto-nrw/project-phoenix/internal/core/domain/facilities"
	iotModels "github.com/moto-nrw/project-phoenix/internal/core/domain/iot"
	userModels "github.com/moto-nrw/project-phoenix/internal/core/domain/users"
)

// Repositories groups the interfaces needed by the database service.
type Repositories struct {
	Student       userModels.StudentRepository
	Teacher       userModels.TeacherRepository
	Room          facilitiesModels.RoomRepository
	ActivityGroup activitiesModels.GroupRepository
	Group         educationModels.GroupRepository
	Role          authModels.RoleRepository
	Device        iotModels.DeviceRepository
	Permission    authModels.PermissionRepository
}
