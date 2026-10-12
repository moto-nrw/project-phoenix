package repositories

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

func presenceObservation(observation presenceCompose.Observation) {
	if observation.Err != nil {
		slog.Default().Warn("presence operation failed",
			"operation", observation.Operation,
			"error", observation.Err,
		)
	}
}

// NewPresenceSessionRecords builds the Student Presence session records
// without owner directories: the reads that project rooms, activities or
// devices fail loudly, and supervision reads carry no staff members. The
// optional clock fixes the supervision "today".
func NewPresenceSessionRecords(db *bun.DB, clocks ...func() time.Time) *presenceCompose.SessionRecords {
	var now func() time.Time
	if len(clocks) > 0 {
		now = clocks[0]
	}
	return newPresenceSessionRecords(presenceCompose.SessionRecordDependencies{DB: db, Now: now})
}

func newPresenceSessionRecords(deps presenceCompose.SessionRecordDependencies) *presenceCompose.SessionRecords {
	deps.Observe = presenceObservation
	deps.Errors = peopleCompose.RetainedRecordErrors{}
	records, err := presenceCompose.NewSessionRecords(deps)
	if err != nil {
		panic(fmt.Sprintf("repository factory: compose presence session records: %v", err))
	}
	return records
}

// SessionRecordOwners are the owners the Student Presence session records
// project their rooms, activities, devices and supervising staff through.
// Devices may be nil: device reads then fail loudly.
type SessionRecordOwners struct {
	// Now fixes the supervision "today"; nil is the Berlin clock.
	Now        func() time.Time
	Devices    devicefleet.Query
	Rooms      facilitiesModule.Query
	Timetable  timetable.Capability
	Membership schoolmembership.Query
	Persons    peopledirectory.Query
}

// NewSessionRecords builds the Student Presence session records with every
// owner directory bound at construction (#2743).
func NewSessionRecords(db *bun.DB, owners SessionRecordOwners) *presenceCompose.SessionRecords {
	deps := presenceCompose.SessionRecordDependencies{
		DB: db, Now: owners.Now, Rooms: activeRoomDirectory{rooms: owners.Rooms},
		Activities: NewSessionActivities(timetableCompose.NewTimetableGroupReads(timetableCompose.NewActivityGroupRecords(owners.Timetable))),
		Staff:      presenceSupervisionStaff{membership: owners.Membership, persons: owners.Persons},
	}
	if owners.Devices != nil {
		deps.Devices = activeDeviceDirectory{devices: owners.Devices}
	}
	return newPresenceSessionRecords(deps)
}

// presenceSupervisionStaff resolves the staff members behind supervision rows
// through School Membership and, when the People Directory is given, their
// person facts.
type presenceSupervisionStaff struct {
	membership staffLookup
	persons    peopledirectory.Query
}

func (d presenceSupervisionStaff) SupervisionStaff(ctx context.Context, staffIDs []int64) (map[int64]*presenceCompose.SessionStaff, error) {
	result := make(map[int64]*presenceCompose.SessionStaff)
	if d.membership == nil {
		return result, nil
	}
	members, err := staffByID(ctx, d.membership, staffIDs, true)
	if err != nil {
		return nil, err
	}
	for id, member := range members {
		result[id] = sessionStaff(member)
	}
	if d.persons == nil {
		return result, nil
	}
	return result, attachSupervisionPersons(ctx, d.persons, result)
}
