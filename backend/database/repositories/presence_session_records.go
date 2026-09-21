package repositories

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/uptrace/bun"
)

// presenceRecordErrors keeps the retained repository error shape on the
// Student Presence session records: a missing row stays both a repository and
// an SQL not-found error.
type presenceRecordErrors struct{}

func (presenceRecordErrors) WrapRecordError(operation string, err error) error {
	return usersRepo.WrapError(operation, err)
}

func (presenceRecordErrors) MissingRecordError(operation string) error {
	return usersRepo.NotFoundError(operation)
}

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
	deps.Errors = presenceRecordErrors{}
	records, err := presenceCompose.NewSessionRecords(deps)
	if err != nil {
		panic(fmt.Sprintf("repository factory: compose presence session records: %v", err))
	}
	return records
}

// presenceSupervisionStaff resolves the staff members behind supervision rows
// through School Membership and, once the People Directory is bound, their
// person facts. It is a pointer so the factory bindings can install the
// owners behind records constructed earlier.
type presenceSupervisionStaff struct {
	membership staffLookup
	persons    peopledirectory.Query
}

func (d *presenceSupervisionStaff) SupervisionStaff(ctx context.Context, staffIDs []int64) (map[int64]*presenceCompose.SessionStaff, error) {
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

// supervisionStaffOf returns the rebindable staff directory of the factory's
// session records, or nil when the records were built without one.
func (f *Factory) supervisionStaffOf() *presenceSupervisionStaff {
	records, ok := f.ActiveGroup.(*presenceCompose.SessionRecords)
	if !ok {
		return nil
	}
	directory, _ := records.SupervisionStaff().(*presenceSupervisionStaff)
	return directory
}
