package services

import (
	"context"
	"fmt"
	"time"

	deliveryModels "github.com/moto-nrw/project-phoenix/models/delivery"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	devicefleetLegacy "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/legacy"
	devicefleetRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/repositoryadapter"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
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
		DB: db, Devices: activeDeviceDirectory{devices: fleet}, Rooms: &activeRoomDirectory{rooms: rooms},
		Activities: NewSessionActivities(timetableCompose.NewTimetableGroupReads(timetableCompose.NewActivityGroupRecords(timetableCapability))),
	})
	return SessionCleanupRepositories{
		Sessions: sessions, Device: device,
		TimetableBridge: timetableCompose.NewActivityInstanceRepository(db, timetableCapability, NewUnloggedStudentPresence(db)),
	}
}

// TimeTrackingCleanupRepositories are the Workforce records the time-tracking
// retention deletes; the deletion ledger comes from the Audit composition.
type TimeTrackingCleanupRepositories struct {
	Workforce workforce.Capability
	Absence   workforceCompose.StaffAbsenceRepository
}

func NewTimeTrackingCleanupRepositories(db *bun.DB) TimeTrackingCleanupRepositories {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		panic(fmt.Sprintf("time tracking cleanup repositories: compose school membership: %v", err))
	}
	workTime, err := workforceCompose.New(workforceCompose.Dependencies{
		LockStaffAssignment: WorkforceStaffAssignmentLock(membership), DB: db,
		LiveStaffIDs: WorkforceLiveStaffIDs(membership), Observe: func(workforceCompose.Observation) {},
	})
	if err != nil {
		panic(fmt.Sprintf("time tracking cleanup repositories: compose workforce: %v", err))
	}
	return TimeTrackingCleanupRepositories{
		Workforce: workTime, Absence: workforceCompose.NewStaffAbsenceRepository(workTime),
	}
}

func mustNewOrganizationTenancy(db *bun.DB) organizationtenancy.Capability {
	capability, err := NewOrganizationTenancy(db)
	if err != nil {
		panic(fmt.Sprintf("repository factory: organization tenancy: %v", err))
	}
	return capability
}
