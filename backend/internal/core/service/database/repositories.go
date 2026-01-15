package database

import (
	activitiesPort "github.com/moto-nrw/project-phoenix/internal/core/port/activities"
	authPort "github.com/moto-nrw/project-phoenix/internal/core/port/auth"
	educationPort "github.com/moto-nrw/project-phoenix/internal/core/port/education"
	facilitiesPort "github.com/moto-nrw/project-phoenix/internal/core/port/facilities"
	iotPort "github.com/moto-nrw/project-phoenix/internal/core/port/iot"
	userPort "github.com/moto-nrw/project-phoenix/internal/core/port/users"
)

// Repositories groups the interfaces needed by the database service.
type Repositories struct {
	Student       userPort.StudentRepository
	Teacher       userPort.TeacherRepository
	Room          facilitiesPort.RoomRepository
	ActivityGroup activitiesPort.GroupRepository
	Group         educationPort.GroupRepository
	Role          authPort.RoleRepository
	Device        iotPort.DeviceRepository
	Permission    authPort.PermissionRepository
}
