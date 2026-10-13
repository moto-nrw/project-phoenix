package repositories

import (
	"context"
	"fmt"
	"time"

	deliveryModels "github.com/moto-nrw/project-phoenix/models/delivery"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	devicefleetLegacy "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/legacy"
	devicefleetRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/repositoryadapter"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce/adapters/timerecords"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
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
		School: mustNewOrganizationTenancy(db), Person: newPersonRepository(db),
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
	rooms, err := NewFacilities(db)
	if err != nil {
		panic(fmt.Sprintf("session cleanup repositories: compose facilities: %v", err))
	}
	sessions := newPresenceSessionRecords(presenceCompose.SessionRecordDependencies{
		DB: db, Devices: activeDeviceDirectory{devices: fleet}, Rooms: &activeRoomDirectory{rooms: rooms},
		Activities: NewSessionActivities(timetableCompose.NewTimetableGroupReads(timetableCompose.NewActivityGroupRecords(timetableCapability))),
	})
	return SessionCleanupRepositories{
		Sessions: sessions, Device: device,
		TimetableBridge: timetableCompose.NewActivityInstanceRepository(db, timetableCapability, newStudentPresence(db)),
	}
}

// TimeTrackingCleanupRepositories are the Workforce records the time-tracking
// retention deletes; the deletion ledger comes from the Audit composition.
type TimeTrackingCleanupRepositories struct {
	Session timerecords.WorkSessionRepository
	Absence timerecords.StaffAbsenceRepository
}

func NewTimeTrackingCleanupRepositories(db *bun.DB) TimeTrackingCleanupRepositories {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		panic(fmt.Sprintf("time tracking cleanup repositories: compose school membership: %v", err))
	}
	workTime, err := NewWorkforce(db, membership)
	if err != nil {
		panic(fmt.Sprintf("time tracking cleanup repositories: compose workforce: %v", err))
	}
	return TimeTrackingCleanupRepositories{
		Session: workforceLegacy.NewWorkSessionRepository(workTime), Absence: workforceLegacy.NewStaffAbsenceRepository(workTime),
	}
}

func mustNewOrganizationTenancy(db *bun.DB) organizationtenancy.Capability {
	capability, err := NewOrganizationTenancy(db)
	if err != nil {
		panic(fmt.Sprintf("repository factory: organization tenancy: %v", err))
	}
	return capability
}
