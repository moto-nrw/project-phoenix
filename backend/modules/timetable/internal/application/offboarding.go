package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/ports"
)

// Offboarding needs the owner store and transaction runtime, not the student,
// room or care-plan graph used by activity planning.
type Offboarding struct {
	store   ports.Store
	tx      ports.Transaction
	observe ports.Observer
}

func NewOffboarding(store ports.Store, tx ports.Transaction, observe ports.Observer) *Offboarding {
	if store == nil || tx == nil || observe == nil {
		panic("timetable: offboarding dependencies are required")
	}
	return &Offboarding{store: store, tx: tx, observe: observe}
}

func (o *Offboarding) Preview(ctx context.Context, staffID int64, from string) (result domain.OffboardingPreview, err error) {
	err = o.run(ctx, "preview_staff_offboarding", func(txCtx context.Context, stats *domain.OperationStats) error {
		result, err = o.snapshot(txCtx, staffID, from, stats)
		return err
	})
	return result, err
}

func (o *Offboarding) Execute(ctx context.Context, staffID int64, from, revision string) (result domain.OffboardingCounts, err error) {
	err = o.run(ctx, "execute_staff_offboarding", func(txCtx context.Context, stats *domain.OperationStats) error {
		preview, err := o.snapshot(txCtx, staffID, from, stats)
		if err != nil {
			return err
		}
		if preview.Counts == (domain.OffboardingCounts{}) {
			return nil
		}
		if preview.Revision != revision {
			return domain.ErrOffboardingConflict
		}
		var changed domain.OperationStats
		result.PlannedSupervisors, changed, err = o.store.DeletePlannedSupervisorsByStaff(txCtx, staffID)
		stats.Add(changed)
		if err != nil {
			return err
		}
		result.InstanceAssignments, changed, err = o.store.DeleteUpcomingInstanceStaff(txCtx, staffID, from)
		stats.Add(changed)
		if err != nil {
			return err
		}
		if result != preview.Counts {
			return domain.ErrOffboardingConflict
		}
		return nil
	})
	if err != nil {
		return domain.OffboardingCounts{}, err
	}
	return result, nil
}

func (o *Offboarding) snapshot(ctx context.Context, staffID int64, from string, stats *domain.OperationStats) (domain.OffboardingPreview, error) {
	snapshot, readStats, err := o.store.PreviewStaffOffboarding(ctx, staffID, from)
	stats.Add(readStats)
	if err != nil {
		return domain.OffboardingPreview{}, err
	}
	type instanceVersion struct {
		ID                int64
		UpdatedAt         time.Time
		Date, Status      string
		InstanceUpdatedAt time.Time
	}
	versions := struct {
		StaffID     int64
		From        string
		Instances   []instanceVersion
		Supervisors []domain.PlannedSupervisor
	}{StaffID: staffID, From: from, Supervisors: snapshot.Supervisors}
	for _, row := range snapshot.Instances {
		versions.Instances = append(versions.Instances, instanceVersion{row.Assignment.ID, row.Assignment.UpdatedAt, row.Date, row.Status, row.InstanceUpdatedAt})
	}
	payload, err := json.Marshal(versions)
	return domain.OffboardingPreview{Counts: domain.OffboardingCounts{InstanceAssignments: int64(len(snapshot.Instances)), PlannedSupervisors: int64(len(snapshot.Supervisors))}, Revision: string(payload)}, err
}

func (o *Offboarding) run(ctx context.Context, operation string, callback func(context.Context, *domain.OperationStats) error) error {
	started := time.Now()
	stats := domain.OperationStats{}
	err := o.tx.RunWrite(ctx, false, func(txCtx context.Context) error { return callback(txCtx, &stats) })
	if err != nil {
		stats.Rows = 0
	}
	o.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	return err
}
