package application

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/ports"
)

// Offboarding needs the owner store and transaction runtime, not the student,
// room or care-plan graph used by activity planning.
type Offboarding struct {
	store    ports.Store
	sessions ports.SessionFacts
	tx       ports.Transaction
	observe  ports.Observer
}

func NewOffboarding(store ports.Store, tx ports.Transaction, sessions ports.SessionFacts, observe ports.Observer) *Offboarding {
	if store == nil || tx == nil || sessions == nil || observe == nil {
		panic("timetable: offboarding dependencies are required")
	}
	return &Offboarding{store: store, tx: tx, sessions: sessions, observe: observe}
}

func (o *Offboarding) Preview(ctx context.Context, staffID int64, from string) (result domain.OffboardingPreview, err error) {
	err = o.run(ctx, "preview_staff_offboarding", func(txCtx context.Context, stats *domain.OperationStats) error {
		result, _, err = o.snapshot(txCtx, staffID, from, stats)
		return err
	})
	return result, err
}

func (o *Offboarding) Execute(ctx context.Context, staffID int64, from, revision string) (result domain.OffboardingCounts, err error) {
	err = o.run(ctx, "execute_staff_offboarding", func(txCtx context.Context, stats *domain.OperationStats) error {
		preview, started, err := o.snapshot(txCtx, staffID, from, stats)
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
		result.InstanceAssignments, changed, err = o.store.DeleteUpcomingInstanceStaff(txCtx, staffID, from, started)
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

// snapshot lists the assignments the offboarding removes: upcoming blocks,
// and today's blocks that nobody started. A block Student Presence already
// runs or ended is history and keeps its staff (#2762). The started ids come
// back so the delete skips the same rows the preview skipped.
func (o *Offboarding) snapshot(ctx context.Context, staffID int64, from string, stats *domain.OperationStats) (domain.OffboardingPreview, []int64, error) {
	snapshot, readStats, err := o.store.PreviewStaffOffboarding(ctx, staffID, from)
	stats.Add(readStats)
	if err != nil {
		return domain.OffboardingPreview{}, nil, err
	}
	instanceIDs := make([]int64, 0, len(snapshot.Instances))
	for _, row := range snapshot.Instances {
		instanceIDs = append(instanceIDs, row.Assignment.InstanceID)
	}
	started, err := o.sessions.StartedInstanceIDs(ctx, sortedUniqueIDs(instanceIDs))
	if err != nil {
		return domain.OffboardingPreview{}, nil, err
	}
	if len(started) > 0 {
		kept := make([]domain.OffboardingInstance, 0, len(snapshot.Instances))
		for _, row := range snapshot.Instances {
			if !slices.Contains(started, row.Assignment.InstanceID) {
				kept = append(kept, row)
			}
		}
		snapshot.Instances = kept
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
	return domain.OffboardingPreview{Counts: domain.OffboardingCounts{InstanceAssignments: int64(len(snapshot.Instances)), PlannedSupervisors: int64(len(snapshot.Supervisors))}, Revision: string(payload)}, started, err
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
