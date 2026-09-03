package repositories

import (
	"context"
	"fmt"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	iotRepo "github.com/moto-nrw/project-phoenix/database/repositories/iot"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	facilitiesRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/facilities/compose/repositoryadapter"
	"github.com/uptrace/bun"
)

// NewFacilities composes the room owner behind the legacy composition seam
// for graphs that do not record observations. Production roots compose the
// module themselves (api/base.go) so runtime evidence is kept.
func NewFacilities(db *bun.DB) (facilitiesModule.Capability, error) {
	return facilitiesCompose.New(facilitiesCompose.Dependencies{
		DB:           db,
		DeletionLock: func(context.Context) error { return nil },
		DeletionGuard: func(context.Context, int64) error {
			return facilitiesModule.ErrRoomDeletionGuardUnavailable
		},
		Observe: func(facilitiesCompose.Observation) {},
	})
}

// bindDefaultFacilities hands an unobserved room owner to every legacy
// repository that used to join facilities.rooms itself (#2665), while those
// repositories are still raw. The binders are kept so BindFacilities can
// swap in the observed capability after the projections wrapped them.
func (f *Factory) bindDefaultFacilities(db *bun.DB) {
	rooms, err := NewFacilities(db)
	if err != nil {
		panic(fmt.Sprintf("repository factory: compose facilities: %v", err))
	}
	f.roomBinders = f.roomBinders[:0]
	f.registerFacilitiesRoomBinder()
	f.registerActiveRoomBinders()
	f.registerRemainingRoomBinders()
	f.bindRoomDirectories(rooms)
}

func (f *Factory) registerFacilitiesRoomBinder() {
	if repo, ok := f.Room.(*facilitiesRepositoryAdapter.Repository); ok {
		f.roomBinders = append(f.roomBinders, func(rooms facilitiesModule.Query) {
			capability, ok := rooms.(facilitiesModule.Capability)
			if !ok {
				panic("repository factory: Facilities query does not support room commands")
			}
			repo.Bind(capability)
		})
	}
}

func (f *Factory) registerActiveRoomBinders() {
	if repo, ok := f.ActiveGroup.(*activeRepo.GroupRepository); ok {
		f.roomBinders = append(f.roomBinders, func(rooms facilitiesModule.Query) {
			repo.BindRoomDirectory(activeRoomDirectory{rooms})
		})
	}
	if repo, ok := f.ActiveVisit.(*activeRepo.VisitRepository); ok {
		f.roomBinders = append(f.roomBinders, func(rooms facilitiesModule.Query) {
			repo.BindRoomDirectory(activeRoomDirectory{rooms})
		})
	}
}

func (f *Factory) registerRemainingRoomBinders() {
	if repo, ok := f.Group.(*educationRepo.GroupRepository); ok {
		f.roomBinders = append(f.roomBinders, func(rooms facilitiesModule.Query) {
			repo.BindRoomDirectory(educationRoomDirectory{rooms})
		})
	}
	if repo, ok := f.Device.(*iotRepo.DeviceRepository); ok {
		f.roomBinders = append(f.roomBinders, func(rooms facilitiesModule.Query) {
			repo.BindRoomDirectory(iotRoomDirectory{rooms})
		})
	}
	if repo, ok := f.InstanceStudent.(*scheduleRepo.InstanceStudentRepository); ok {
		f.roomBinders = append(f.roomBinders, func(rooms facilitiesModule.Query) {
			repo.BindRoomDirectory(scheduleRoomDirectory{rooms})
		})
	}
	if repo, ok := f.CareExitCleanup.(*usersRepo.CareExitCleanupRepository); ok {
		f.roomBinders = append(f.roomBinders, func(rooms facilitiesModule.Query) {
			repo.BindRoomDirectory(usersRoomDirectory{rooms})
		})
	}
}

// BindFacilities replaces the default room owner with the observed
// Facilities capability, so every legacy repository that used to join
// facilities.rooms reads through the same owner the runtime evidence
// records (#2665).
func (f *Factory) BindFacilities(capability facilitiesModule.Capability) {
	if capability == nil {
		panic("repository factory: facilities capability is required")
	}
	if f.facilitiesBound {
		return
	}
	f.facilitiesBound = true
	f.bindRoomDirectories(capability)
}

func (f *Factory) bindRoomDirectories(rooms facilitiesModule.Query) {
	for _, bind := range f.roomBinders {
		bind(rooms)
	}
}

type activeRoomDirectory struct{ rooms facilitiesModule.Query }

func (d activeRoomDirectory) ListRoomsByID(ctx context.Context, ids []int64) ([]activeRepo.DirectoryRoom, error) {
	rooms, err := d.rooms.ListRoomsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]activeRepo.DirectoryRoom, 0, len(rooms))
	for _, room := range rooms {
		result = append(result, activeRepo.DirectoryRoom{
			ID: room.ID, TenantID: room.TenantID, CreatedAt: room.CreatedAt, UpdatedAt: room.UpdatedAt,
			Name: room.Name, Building: room.Building, Floor: room.Floor, Capacity: room.Capacity,
			Category: room.Category, Color: room.Color, IsSystem: room.IsSystem,
		})
	}
	return result, nil
}

type educationRoomDirectory struct{ rooms facilitiesModule.Query }

func (d educationRoomDirectory) ListRoomsByID(ctx context.Context, ids []int64) ([]educationRepo.DirectoryRoom, error) {
	rooms, err := d.rooms.ListRoomsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]educationRepo.DirectoryRoom, 0, len(rooms))
	for _, room := range rooms {
		result = append(result, educationRepo.DirectoryRoom{
			ID: room.ID, TenantID: room.TenantID, CreatedAt: room.CreatedAt, UpdatedAt: room.UpdatedAt,
			Name: room.Name, Building: room.Building, Floor: room.Floor, Capacity: room.Capacity,
			Category: room.Category, Color: room.Color,
		})
	}
	return result, nil
}

type iotRoomDirectory struct{ rooms facilitiesModule.Query }

func (d iotRoomDirectory) ListRoomsByID(ctx context.Context, ids []int64) ([]iotRepo.DirectoryRoom, error) {
	rooms, err := d.rooms.ListRoomsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]iotRepo.DirectoryRoom, 0, len(rooms))
	for _, room := range rooms {
		result = append(result, iotRepo.DirectoryRoom{ID: room.ID, TenantID: room.TenantID, Name: room.Name})
	}
	return result, nil
}

type scheduleRoomDirectory struct{ rooms facilitiesModule.Query }

func (d scheduleRoomDirectory) LockRoomsByID(ctx context.Context, ids []int64) ([]scheduleRepo.DirectoryRoom, error) {
	rooms, err := d.rooms.LockRoomsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]scheduleRepo.DirectoryRoom, 0, len(rooms))
	for _, room := range rooms {
		result = append(result, scheduleRepo.DirectoryRoom{ID: room.ID, TenantID: room.TenantID})
	}
	return result, nil
}

type usersRoomDirectory struct{ rooms facilitiesModule.Query }

func (d usersRoomDirectory) LockRoomsByID(ctx context.Context, ids []int64) ([]usersRepo.DirectoryRoom, error) {
	rooms, err := d.rooms.LockRoomsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]usersRepo.DirectoryRoom, 0, len(rooms))
	for _, room := range rooms {
		result = append(result, usersRepo.DirectoryRoom{ID: room.ID, TenantID: room.TenantID})
	}
	return result, nil
}
