package services

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
	"github.com/moto-nrw/project-phoenix/services/schedule"
)

// errShiftPlanSyncUnbound keeps the cascade fail-closed when the composition
// never bound the schedule side: a sick report must not commit half-applied.
var errShiftPlanSyncUnbound = errors.New("shift plan sync: schedule cascade is not bound")

// ShiftPlanSyncBridge binds the retained absence service's sick-cascade port to
// the schedule-owned cascade. The Workforce port and the schedule contract
// carry the same fields; neither package may import the other. The cascade is
// resolved on every call because the schedule services are assembled after
// the absence service.
func ShiftPlanSyncBridge(resolve func() schedule.ShiftPlanSyncer) timetracking.ShiftPlanSyncer {
	return shiftPlanSyncBridge{resolve: resolve}
}

type shiftPlanSyncBridge struct {
	resolve func() schedule.ShiftPlanSyncer
}

func (b shiftPlanSyncBridge) syncer() (schedule.ShiftPlanSyncer, error) {
	if b.resolve == nil {
		return nil, errShiftPlanSyncUnbound
	}
	syncer := b.resolve()
	if syncer == nil {
		return nil, errShiftPlanSyncUnbound
	}
	return syncer, nil
}

func (b shiftPlanSyncBridge) MarkSickForRange(ctx context.Context, in timetracking.SickCascadeInput) error {
	syncer, err := b.syncer()
	if err != nil {
		return err
	}
	return syncer.MarkSickForRange(ctx, scheduleSickCascade(in))
}

func (b shiftPlanSyncBridge) ClearSickForRange(ctx context.Context, in timetracking.SickCascadeInput) error {
	syncer, err := b.syncer()
	if err != nil {
		return err
	}
	return syncer.ClearSickForRange(ctx, scheduleSickCascade(in))
}

func (b shiftPlanSyncBridge) ReconcileSickRange(ctx context.Context, before, after timetracking.SickCascadeInput) error {
	syncer, err := b.syncer()
	if err != nil {
		return err
	}
	return syncer.ReconcileSickRange(ctx, scheduleSickCascade(before), scheduleSickCascade(after))
}

func (b shiftPlanSyncBridge) ReassignSickStamps(ctx context.Context, fromAbsenceID, toAbsenceID int64) error {
	syncer, err := b.syncer()
	if err != nil {
		return err
	}
	return syncer.ReassignSickStamps(ctx, fromAbsenceID, toAbsenceID)
}

func scheduleSickCascade(in timetracking.SickCascadeInput) schedule.SickCascadeInput {
	return schedule.SickCascadeInput{
		SubjectStaffID: in.SubjectStaffID,
		DateStart:      in.DateStart,
		DateEnd:        in.DateEnd,
		SkipStartDay:   in.SkipStartDay,
		SkipEndDay:     in.SkipEndDay,
		AbsenceID:      in.AbsenceID,
		ActorStaffID:   in.ActorStaffID,
		ActorAccountID: in.ActorAccountID,
	}
}
