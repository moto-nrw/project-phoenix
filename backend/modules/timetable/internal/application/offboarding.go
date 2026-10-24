package application

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	internalports "github.com/moto-nrw/project-phoenix/modules/timetable/internal/ports"
)

// Offboarding needs the owner store and transaction runtime, not the student,
// room or care-plan graph used by activity planning.
type Offboarding struct {
	store    internalports.Store
	sessions internalports.SessionFacts
	tx       internalports.Transaction
	observe  internalports.Observer
}

func NewOffboarding(store internalports.Store, tx internalports.Transaction, sessions internalports.SessionFacts, observe internalports.Observer) *Offboarding {
	if store == nil || tx == nil || sessions == nil || observe == nil {
		panic("timetable: offboarding dependencies are required")
	}
	return &Offboarding{store: store, tx: tx, sessions: sessions, observe: observe}
}

func (o *Offboarding) Preview(ctx context.Context, staffID int64, from string) (result timetable.OffboardingPreview, err error) {
	err = o.run(ctx, "preview_staff_offboarding", func(txCtx context.Context, stats *domain.OperationStats) error {
		result, _, err = o.snapshot(txCtx, staffID, from, stats)
		return err
	})
	return result, err
}

func (o *Offboarding) Execute(ctx context.Context, staffID int64, from, revision string) (result timetable.OffboardingCounts, err error) {
	err = o.run(ctx, "execute_staff_offboarding", func(txCtx context.Context, stats *domain.OperationStats) error {
		preview, started, err := o.snapshot(txCtx, staffID, from, stats)
		if err != nil {
			return err
		}
		if preview.Counts == (timetable.OffboardingCounts{}) {
			return nil
		}
		if preview.Revision != revision {
			return timetable.ErrOffboardingConflict
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
			return timetable.ErrOffboardingConflict
		}
		return nil
	})
	if err != nil {
		return timetable.OffboardingCounts{}, err
	}
	return result, nil
}

// snapshot lists the assignments the offboarding removes: upcoming blocks,
// and today's blocks that nobody started. A block Student Presence already
// runs or ended is history and keeps its staff (#2762). The started ids come
// back so the delete skips the same rows the preview skipped.
func (o *Offboarding) snapshot(ctx context.Context, staffID int64, from string, stats *domain.OperationStats) (timetable.OffboardingPreview, []int64, error) {
	instances, supervisors, readStats, err := o.store.PreviewStaffOffboarding(ctx, staffID, from)
	stats.Add(readStats)
	if err != nil {
		return timetable.OffboardingPreview{}, nil, err
	}
	instanceIDs := make([]int64, 0, len(instances))
	for _, row := range instances {
		instanceIDs = append(instanceIDs, row.InstanceID)
	}
	started, err := o.sessions.StartedInstanceIDs(ctx, sortedUniqueIDs(instanceIDs))
	if err != nil {
		return timetable.OffboardingPreview{}, nil, err
	}
	if len(started) > 0 {
		kept := make([]domain.OffboardingInstance, 0, len(instances))
		for _, row := range instances {
			if !slices.Contains(started, row.InstanceID) {
				kept = append(kept, row)
			}
		}
		instances = kept
	}
	type instanceVersion struct {
		ID                int64
		UpdatedAt         time.Time
		Date, Status      string
		InstanceUpdatedAt time.Time
	}
	// supervisorVersion keeps the untagged field names of the revision, so a
	// preview issued before a deploy still matches after it.
	type supervisorVersion struct {
		ID               int64
		TenantID         int64
		CreatedAt        time.Time
		UpdatedAt        time.Time
		StaffID          int64
		GroupID          int64
		IsPrimary        bool
		ValidFrom        string
		ValidUntil       *string
		CalendarPeriodID *int64
		Weekday          *int
	}
	versions := struct {
		StaffID     int64
		From        string
		Instances   []instanceVersion
		Supervisors []supervisorVersion
	}{StaffID: staffID, From: from}
	for _, row := range instances {
		versions.Instances = append(versions.Instances, instanceVersion{row.AssignmentID, row.AssignmentUpdatedAt, row.Date, row.Status, row.InstanceUpdatedAt})
	}
	for _, supervisor := range supervisors {
		versions.Supervisors = append(versions.Supervisors, supervisorVersion(supervisor))
	}
	payload, err := json.Marshal(versions)
	return timetable.OffboardingPreview{Counts: timetable.OffboardingCounts{InstanceAssignments: int64(len(instances)), PlannedSupervisors: int64(len(supervisors))}, Revision: string(payload)}, started, err
}

func (o *Offboarding) run(ctx context.Context, operation string, callback func(context.Context, *domain.OperationStats) error) error {
	started := time.Now()
	stats := domain.OperationStats{}
	err := o.tx.RunWrite(ctx, false, func(txCtx context.Context) error { return callback(txCtx, &stats) })
	if err != nil {
		stats.Rows = 0
	}
	o.observe(internalports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	return err
}
