package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type studentProfiles struct {
	store   *postgres.Store
	observe func(Observation)
}

func NewStudentProfiles(db *bun.DB, observe func(Observation)) (careplan.StudentProfileCommands, error) {
	if db == nil || observe == nil {
		return nil, errors.New("care plan: database and observer are required")
	}
	return studentProfiles{store: postgres.New(carePlanDatabase(db)), observe: observe}, nil
}

func (p studentProfiles) run(ctx context.Context, operation string, write func(context.Context) (domain.OperationStats, error)) error {
	started := time.Now()
	var stats domain.OperationStats
	run := func(txCtx context.Context) (err error) {
		stats, err = write(txCtx)
		return err
	}
	var err error
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		err = run(ctx)
	} else {
		err = tenant.WithinCurrentTenant(ctx, run)
	}
	p.observe(Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	return err
}

func (p studentProfiles) SaveStudentCareProfile(ctx context.Context, input careplan.StudentCareProfile, plan *careplan.StudentDeparturePlan) error {
	if input.MembershipID <= 0 {
		return errors.New("care plan: membership ID is required")
	}
	return p.run(ctx, "save_student_care_profile", func(txCtx context.Context) (domain.OperationStats, error) {
		var departure *domain.StudentDeparturePlan
		if plan != nil {
			value := domain.StudentDeparturePlan(*plan)
			departure = &value
		}
		return p.store.SaveStudentCareProfile(txCtx, domain.StudentCareProfile(input), departure)
	})
}

func (p studentProfiles) ClearStudentStatusFlags(ctx context.Context, ids []int64, status string) (changed int64, err error) {
	if status != "sick" && status != "excused" {
		return 0, errors.New("care plan: unsupported absence flag")
	}
	if len(ids) == 0 {
		return 0, nil
	}
	err = p.run(ctx, "clear_student_status_flags", func(txCtx context.Context) (domain.OperationStats, error) {
		var stats domain.OperationStats
		changed, stats, err = p.store.ClearStudentStatusFlags(txCtx, ids, status)
		return stats, err
	})
	return changed, err
}

func (p studentProfiles) SetStudentLiveStatus(ctx context.Context, input careplan.StudentLiveStatus) (changed int64, err error) {
	if input.StudentID <= 0 {
		return 0, errors.New("care plan: student ID is required")
	}
	err = p.run(ctx, "set_student_live_status", func(txCtx context.Context) (domain.OperationStats, error) {
		var stats domain.OperationStats
		changed, stats, err = p.store.SetStudentLiveStatus(txCtx, domain.StudentLiveStatus(input))
		return stats, err
	})
	return changed, err
}
