package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	devicefleetRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/repositoryadapter"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// The cleanup command's roots its suites build too (#4013): the fail-closed
// Audit command and the records the session and retention sweeps run on.

// NewCleanupAuditCommand builds the same fail-closed Audit command used by
// the HTTP service graph. Cleanup producers must already be inside their
// authoritative transaction when they append an event.
func NewCleanupAuditCommand(logger *slog.Logger) (auditModels.Command, error) {
	if logger == nil {
		return nil, errors.New("cleanup audit command logger is required")
	}
	audits, err := auditCompose.NewHandoffPersistence()
	if err != nil {
		return nil, err
	}
	command, err := auditCompose.NewObservedCommand(audits.AppendStore, logger.With("component", "audit-command"), func(string, time.Duration, int, error) {})
	if err != nil {
		return nil, err
	}
	return command, nil
}

// RetentionCleanupRepositories are the Student Presence session and
// supervision records the stale-supervisor cleanup runs on and the deletion
// ledger it records into.
type RetentionCleanupRepositories struct {
	Sessions studentpresence.SessionRecords
	Deletion RetentionDeletions
}

// RetentionDeletions is the deletion ledger the retention cleanup records
// into, with the read its suites check the rollback through.
type RetentionDeletions interface {
	DataDeletionAppender
	FindByStudentID(ctx context.Context, studentID int64) ([]*auditModels.DataDeletion, error)
}

// NewRetentionCleanupRepositories composes the retention cleanup's records
// with the deletion ledger routed through command.
func NewRetentionCleanupRepositories(db transactionruntime.Database, command auditModels.Command) RetentionCleanupRepositories {
	return RetentionCleanupRepositories{
		Sessions: newPresenceSessionRecords(presenceCompose.SessionRecordDependencies{DB: db.Pool()}),
		Deletion: auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db.Pool()}).WithRoutedWrites(command).DataDeletion,
	}
}

// SessionCleanupTimetableBridge completes the planned activity instances of
// the sessions the cleanup ends.
type SessionCleanupTimetableBridge interface {
	CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error)
}

// NewSessionCleanupRepositories composes the records the session cleanup runs
// on over the unobserved Device Fleet and Facilities owners and the given
// Timetable: the Student Presence session and supervision records, the
// devices of the sessions and the Timetable completion of their planned
// instances.
func NewSessionCleanupRepositories(db transactionruntime.Database, timetableCapability timetable.Capability) (studentpresence.SessionRecords, devicefleet.SessionDeviceRecords, SessionCleanupTimetableBridge) {
	if timetableCapability == nil {
		panic("session cleanup repositories: timetable capability is required")
	}
	fleet, err := NewDeviceFleet(db)
	if err != nil {
		panic(fmt.Sprintf("session cleanup repositories: compose device fleet: %v", err))
	}
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		panic(fmt.Sprintf("session cleanup repositories: compose facilities: %v", err))
	}
	sessions := NewSessionRecords(db, nil, fleet, rooms, timetableCapability, nil, nil)
	devices := devicefleetRepositoryAdapter.NewDeviceRepository(fleet)
	completion := timetableCompose.NewActivityInstanceRepository(MustPresenceReads(db), timetableCapability, NewUnloggedStudentPresence(db))
	return sessions, devices, completion
}
