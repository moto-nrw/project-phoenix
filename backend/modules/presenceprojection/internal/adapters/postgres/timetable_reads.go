package presenceprojection

import (
	"context"
	"time"

	presenceContract "github.com/moto-nrw/project-phoenix/modules/presenceprojection/presencecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

type TimetableDatabase func(context.Context) (bun.IDB, int64, error)
type TimetableReads struct{ database TimetableDatabase }

func (Reader) TimetableReads(database TimetableDatabase) TimetableReads {
	return TimetableReads{database: database}
}
func (r TimetableReads) CountNonAbsentParticipants(ctx context.Context, ids []int64) (map[int64]int, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	return CountNonAbsentParticipants(ctx, db, tenantID, ids)
}

func (r TimetableReads) ListParallelPresence(ctx context.Context, excluded int64, date calendar.Date, ids []int64) ([]presenceContract.ParallelPresence, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	return ListParallelPresence(ctx, db, tenantID, excluded, date, ids)
}

func (r TimetableReads) ListPartialAbsenceBlocks(ctx context.Context, id int64, date calendar.Date, from time.Time, enrolled bool, exceptions []int64) ([]presenceContract.PartialAbsenceBlock, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	return ListPartialAbsenceBlocks(ctx, db, tenantID, id, date, from, enrolled, exceptions)
}

func (r TimetableReads) ListScheduledInstancesForStudent(ctx context.Context, id int64, from, to calendar.Date) ([]presenceContract.ScheduledInstance, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	return ListScheduledInstancesForStudent(ctx, db, tenantID, id, from, to)
}

func (r TimetableReads) HasPlannedSlotsInRange(ctx context.Context, from, to calendar.Date) (bool, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return false, err
	}
	return HasPlannedSlotsInRange(ctx, db, tenantID, from, to)
}

func (r TimetableReads) ListLegacyInstances(ctx context.Context, filter presenceContract.LegacyInstanceFilter) ([]presenceContract.Instance, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	return ListLegacyInstances(ctx, db, tenantID, filter)
}

func (r TimetableReads) ListLegacyParticipants(ctx context.Context, filter presenceContract.LegacyParticipantFilter) ([]presenceContract.Participant, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	return ListLegacyParticipants(ctx, db, tenantID, filter)
}
