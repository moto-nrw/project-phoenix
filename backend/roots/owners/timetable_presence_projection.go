package owners

import (
	"context"
	"time"

	projection "github.com/moto-nrw/project-phoenix/modules/presenceprojection/compose"
	presenceContract "github.com/moto-nrw/project-phoenix/modules/presenceprojection/presencecontract"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func NewTimetablePresenceProjection(db transactionruntime.Database) ports.PresenceReader {
	if db.Pool() == nil {
		return nil
	}
	return timetablePresenceReader{projection.New().TimetableReads(timetableCompose.PresenceDatabase(db.Pool()))}
}

// timetablePresenceSource names the projection's public reads in the root,
// where its rows are translated into the consumer's activity vocabulary.
// Scalar reads need no translation and are promoted unchanged.
type timetablePresenceSource interface {
	CountNonAbsentParticipants(context.Context, []int64) (map[int64]int, error)
	ListParallelPresence(context.Context, int64, calendar.Date, []int64) ([]presenceContract.ParallelPresence, error)
	ListPartialAbsenceBlocks(context.Context, int64, calendar.Date, time.Time, bool, []int64) ([]presenceContract.PartialAbsenceBlock, error)
	ListScheduledInstancesForStudent(context.Context, int64, calendar.Date, calendar.Date) ([]presenceContract.ScheduledInstance, error)
	HasPlannedSlotsInRange(context.Context, calendar.Date, calendar.Date) (bool, error)
	ListLegacyInstances(context.Context, presenceContract.LegacyInstanceFilter) ([]presenceContract.Instance, error)
	ListLegacyParticipants(context.Context, presenceContract.LegacyParticipantFilter) ([]presenceContract.Participant, error)
}

type timetablePresenceReader struct{ timetablePresenceSource }

func (r timetablePresenceReader) ListParallelPresence(ctx context.Context, excludedID int64, day calendar.Date, studentIDs []int64) ([]ports.ParallelPresence, error) {
	rows, err := r.timetablePresenceSource.ListParallelPresence(ctx, excludedID, day, studentIDs)
	return mapTimetablePresenceRows(rows, func(row presenceContract.ParallelPresence) ports.ParallelPresence {
		return ports.ParallelPresence(row)
	}), err
}

func (r timetablePresenceReader) ListPartialAbsenceBlocks(ctx context.Context, studentID int64, day calendar.Date, from time.Time, enrolled bool, exceptions []int64) ([]ports.PartialAbsenceBlock, error) {
	rows, err := r.timetablePresenceSource.ListPartialAbsenceBlocks(ctx, studentID, day, from, enrolled, exceptions)
	return mapTimetablePresenceRows(rows, func(row presenceContract.PartialAbsenceBlock) ports.PartialAbsenceBlock {
		return ports.PartialAbsenceBlock(row)
	}), err
}

func (r timetablePresenceReader) ListScheduledInstancesForStudent(ctx context.Context, studentID int64, from, to calendar.Date) ([]ports.ScheduledInstance, error) {
	rows, err := r.timetablePresenceSource.ListScheduledInstancesForStudent(ctx, studentID, from, to)
	return mapTimetablePresenceRows(rows, func(row presenceContract.ScheduledInstance) ports.ScheduledInstance {
		return ports.ScheduledInstance{Instance: ports.ActivityInstance(row.Instance), Participant: ports.ParticipantAttendance(row.Participant)}
	}), err
}

func (r timetablePresenceReader) ListInstances(ctx context.Context, filter ports.LegacyInstanceFilter) ([]ports.ActivityInstance, error) {
	rows, err := r.ListLegacyInstances(ctx, presenceContract.LegacyInstanceFilter(filter))
	return mapTimetablePresenceRows(rows, func(row presenceContract.Instance) ports.ActivityInstance {
		return ports.ActivityInstance(row)
	}), err
}

func (r timetablePresenceReader) ListParticipants(ctx context.Context, filter ports.LegacyParticipantFilter) ([]ports.ParticipantAttendance, error) {
	rows, err := r.ListLegacyParticipants(ctx, presenceContract.LegacyParticipantFilter(filter))
	return mapTimetablePresenceRows(rows, func(row presenceContract.Participant) ports.ParticipantAttendance {
		return ports.ParticipantAttendance(row)
	}), err
}

// Preserve nil on failed reads and non-nil empty results on successful reads.
func mapTimetablePresenceRows[T, U any](rows []T, convert func(T) U) []U {
	if rows == nil {
		return nil
	}
	result := make([]U, 0, len(rows))
	for _, row := range rows {
		result = append(result, convert(row))
	}
	return result
}
