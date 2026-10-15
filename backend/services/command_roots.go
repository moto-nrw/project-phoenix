package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	deliveryModels "github.com/moto-nrw/project-phoenix/models/delivery"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	devicefleetLegacy "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/legacy"
	devicefleetRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/repositoryadapter"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

// AuthCleanupRepositories are the retained repositories the auth cleanup
// CLI composes. The expired session sweep runs through Identity & Access,
// which the composition binds to the school and person lookups here (#3251).
// The auth event trail comes from the Audit composition (ADR 0049).
type AuthCleanupRepositories struct {
	School           organizationtenancy.Capability
	Person           userModels.PersonRepository
	PushSubscription deliveryModels.PushSubscriptionRepository
}

func NewAuthCleanupRepositories(db *bun.DB) AuthCleanupRepositories {
	return AuthCleanupRepositories{
		School: mustNewOrganizationTenancy(db), Person: NewPersonRepository(db),
		PushSubscription: deliveryCompose.NewPushSubscriptionRepository(db),
	}
}

type SessionCleanupRepositories struct {
	// Sessions are the Student Presence session and supervision records the
	// session cleanup runs on.
	Sessions        studentpresence.SessionRecords
	Device          devicefleetLegacy.SessionDeviceRecords
	TimetableBridge SessionCleanupTimetableBridge
}

// SessionCleanupTimetableBridge completes the planned activity instances of
// the sessions the cleanup ends.
type SessionCleanupTimetableBridge interface {
	CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error)
}

func NewSessionCleanupRepositories(db *bun.DB, timetableCapability timetable.Capability) SessionCleanupRepositories {
	if timetableCapability == nil {
		panic("session cleanup repositories: timetable capability is required")
	}
	fleet, err := NewDeviceFleet(db)
	if err != nil {
		panic(fmt.Sprintf("session cleanup repositories: compose device fleet: %v", err))
	}
	device := devicefleetRepositoryAdapter.NewDeviceRepository(fleet)
	rooms, err := facilitiesCompose.NewUnobserved(db)
	if err != nil {
		panic(fmt.Sprintf("session cleanup repositories: compose facilities: %v", err))
	}
	sessions := newPresenceSessionRecords(presenceCompose.SessionRecordDependencies{
		DB: db, Devices: sessionDeviceDirectory{devices: fleet}, Rooms: sessionRoomDirectory{rooms: rooms},
		Activities: sessionActivities{records: timetableCompose.NewTimetableGroupReads(timetableCompose.NewActivityGroupRecords(timetableCapability))},
	})
	return SessionCleanupRepositories{
		Sessions: sessions, Device: device,
		TimetableBridge: timetableCompose.NewActivityInstanceRepository(db, timetableCapability, newStudentPresence(db, presenceCompose.FailureLogger())),
	}
}

func mustNewOrganizationTenancy(db *bun.DB) organizationtenancy.Capability {
	capability, err := NewOrganizationTenancy(db)
	if err != nil {
		panic(fmt.Sprintf("repository factory: organization tenancy: %v", err))
	}
	return capability
}

// The session records of the session and retention cleanups project rooms,
// devices and activities through the owners. The typed root owns these
// directories (api/presence_*_directory.go, api/session_activities.go,
// #4010); the private copies stay with the cleanup roots until #4013 moves
// them.

func newPresenceSessionRecords(deps presenceCompose.SessionRecordDependencies) *presenceCompose.SessionRecords {
	deps.Observe = presenceCompose.FailureLogger()
	deps.Errors = peopleCompose.RetainedRecordErrors{}
	records, err := presenceCompose.NewSessionRecords(deps)
	if err != nil {
		panic(fmt.Sprintf("compose presence session records: %v", err))
	}
	return records
}

// sessionDeviceDirectory hands the device owner to the session records.
type sessionDeviceDirectory struct{ devices devicefleet.Query }

func (d sessionDeviceDirectory) ListDevicesByID(ctx context.Context, ids []int64) ([]presenceCompose.DirectoryDevice, error) {
	devices, err := d.devices.ListDevicesByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]presenceCompose.DirectoryDevice, 0, len(devices))
	for _, device := range devices {
		result = append(result, presenceCompose.DirectoryDevice{
			ID: device.ID, TenantID: device.TenantID, CreatedAt: device.CreatedAt,
			UpdatedAt: device.UpdatedAt, DeviceID: device.DeviceID, DeviceType: device.DeviceType,
			Name: device.Name, Status: string(device.Status), LastSeen: device.LastSeen,
		})
	}
	return result, nil
}

// sessionRoomDirectory hands the room owner to the session records.
type sessionRoomDirectory struct{ rooms facilitiesModule.Query }

func (d sessionRoomDirectory) ListRoomsByID(ctx context.Context, ids []int64) ([]presenceCompose.DirectoryRoom, error) {
	if d.rooms == nil {
		return nil, errors.New("active room directory: no room owner bound")
	}
	rooms, err := d.rooms.ListRoomsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]presenceCompose.DirectoryRoom, 0, len(rooms))
	for _, room := range rooms {
		result = append(result, presenceCompose.DirectoryRoom{
			ID: room.ID, TenantID: room.TenantID, CreatedAt: room.CreatedAt, UpdatedAt: room.UpdatedAt,
			Name: room.Name, Building: room.Building, Floor: room.Floor, Capacity: room.Capacity,
			Category: room.Category, Color: room.Color, IsSystem: room.IsSystem,
			IsOpenRoom: room.IsOpenRoom,
		})
	}
	return result, nil
}

// sessionActivities hands the Timetable activity groups to the session
// records.
type sessionActivities struct {
	records timetableCompose.TimetableGroupReads
}

func (r sessionActivities) FindByID(ctx context.Context, id any) (*presenceCompose.SessionActivity, error) {
	row, err := r.records.FindByID(ctx, id)
	return sessionActivity(row), err
}

func (r sessionActivities) FindByIDs(ctx context.Context, ids []int64) ([]*presenceCompose.SessionActivity, error) {
	rows, err := r.records.FindByIDs(ctx, ids)
	if rows == nil {
		return nil, err
	}
	result := make([]*presenceCompose.SessionActivity, len(rows))
	for i, row := range rows {
		result[i] = sessionActivity(row)
	}
	return result, err
}

func (r sessionActivities) ListSessionActivities(ctx context.Context) ([]*presenceCompose.SessionActivity, error) {
	rows, err := r.records.ListWithCategory(ctx)
	if rows == nil {
		return nil, err
	}
	result := make([]*presenceCompose.SessionActivity, len(rows))
	for i, row := range rows {
		result[i] = sessionActivity(row)
	}
	return result, err
}

func sessionActivity(row *timetable.Group) *presenceCompose.SessionActivity {
	if row == nil {
		return nil
	}
	result := &presenceCompose.SessionActivity{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Name: row.Name, MaxParticipants: row.MaxParticipants, RequiredStaff: row.RequiredStaff, IsOpen: row.IsOpen,
		CategoryID: row.CategoryID, PlanningTrackID: row.PlanningTrackID, PlannedRoomID: row.PlannedRoomID,
		CreatedBy: row.CreatedBy, Type: row.Type, EducationGroupID: row.EducationGroupID, ListKind: row.ListKind,
		IsTemplate: row.IsTemplate, IsSystem: row.IsSystem, ArchivedAt: row.ArchivedAt, SeriesRootID: row.SeriesRootID,
		CalendarPeriodID: row.CalendarPeriodID, TargetGroupType: row.TargetGroupType, TargetGradeLevel: row.TargetGradeLevel,
		TargetSchoolClass: row.TargetSchoolClass, SourceCareOfferingIDs: row.SourceCareOfferingIDs,
		SourceGradeLevels: row.SourceGradeLevels, SourceSchoolClasses: row.SourceSchoolClasses, Notes: row.Notes,
	}
	if row.Category != nil {
		result.Category = &presenceCompose.SessionActivityCategory{
			ID: row.Category.ID, TenantID: row.Category.TenantID, CreatedAt: row.Category.CreatedAt,
			UpdatedAt: row.Category.UpdatedAt, Name: row.Category.Name, Description: row.Category.Description,
			Color: row.Category.Color, IsSystem: row.Category.IsSystem, ShiftTypeID: row.Category.ShiftTypeID,
			ArchivedAt: row.Category.ArchivedAt,
		}
	}
	return result
}
