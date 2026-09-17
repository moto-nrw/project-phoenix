package compose

import (
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	activeRepo "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/repositories/active"
)

// The retained session and supervision repositories moved into this module
// with #3214. The legacy composition root and the retained integration tests
// still construct them over their own Bun record adapters; they reach the
// repositories through these names so no package outside Student Presence
// imports the retained Postgres package directly. Every entry goes with the
// last consumer of the retained repository contract.

// Retained repository contracts and records.
type (
	LegacyGroupRepository        = activeRepo.GroupRepository
	LegacyGroupRepositoryOption  = activeRepo.GroupRepositoryOption
	LegacyGroupRecords           = activeRepo.GroupRecords
	LegacyGroupRecordFilter      = activeRepo.GroupRecordFilter
	LegacyGroupSupervisionRecord = activeRepo.GroupSupervisionRecord
	LegacySupervisionRecords     = activeRepo.SupervisionRecords
	LegacySupervisionFilter      = activeRepo.SupervisionFilter
	LegacyStaffRoomSupervision   = activeRepo.StaffRoomSupervision
	LegacyDirectoryDevice        = activeRepo.DirectoryDevice
	LegacyDirectoryRoom          = activeRepo.DirectoryRoom
	LegacyRoomDirectory          = activeRepo.RoomDirectory
)

// NewLegacyGroupRepository builds the retained session repository over the
// caller's device directory, group records and activity directory.
func NewLegacyGroupRepository(devices activeRepo.DeviceDirectory, records activeRepo.GroupRecords, activities activeRepo.GroupActivityDirectory, options ...activeRepo.GroupRepositoryOption) activeModels.GroupRepository {
	return activeRepo.NewGroupRepository(devices, records, activities, options...)
}

// WithLegacyRoomDirectory installs the room directory the retained session
// repository resolves rooms through.
func WithLegacyRoomDirectory(rooms activeRepo.RoomDirectory) activeRepo.GroupRepositoryOption {
	return activeRepo.WithRoomDirectory(rooms)
}

// NewLegacyGroupSupervisorRepository builds the retained supervision
// repository over the caller's supervision records; the optional clock fixes
// the calendar date open supervisions are compared against.
func NewLegacyGroupSupervisorRepository(records activeRepo.SupervisionRecords, clocks ...func() time.Time) activeModels.GroupSupervisorRepository {
	return activeRepo.NewGroupSupervisorRepository(records, clocks...)
}
