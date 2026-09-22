package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewStaffEmployment composes the Workforce owner of the staff employment
// profile (#2753). It needs only the database, so School Membership can be
// bound to it before the rest of Workforce, which in turn depends on School
// Membership, is composed. Every operation joins the caller's transaction or
// opens one for the tenant in context. observe receives one observation per
// call; nil records nothing.
func NewStaffEmployment(db *bun.DB, observe func(Observation)) (workforce.StaffEmployments, error) {
	if db == nil {
		return nil, errors.New("workforce staff employment: database is required")
	}
	if observe == nil {
		observe = func(Observation) {}
	}
	return staffEmployment{store: postgres.New(databaseRuntime(db)), observe: observe}, nil
}

type staffEmployment struct {
	store   *postgres.Store
	observe func(Observation)
}

func (e staffEmployment) read(ctx context.Context, operation string, queries int64, callback func(context.Context) error) error {
	return e.observed(operation, queries, func() error {
		if _, ok := tenant.TransactionFromContext(ctx); ok {
			return callback(ctx)
		}
		if _, err := tenant.TenantFromContext(ctx); err == nil {
			return tenant.WithinCurrentTenant(ctx, callback)
		}
		return tenant.WithinAdmin(ctx, callback)
	})
}

func (e staffEmployment) write(ctx context.Context, operation string, queries int64, callback func(context.Context) error) error {
	return e.observed(operation, queries, func() error {
		if _, ok := tenant.TransactionFromContext(ctx); ok {
			return callback(ctx)
		}
		return tenant.WithinCurrentTenant(ctx, callback)
	})
}

func (e staffEmployment) observed(operation string, queries int64, run func() error) error {
	started := time.Now()
	err := mapStaffEmploymentError(run())
	e.observe(Observation{Operation: operation, Duration: time.Since(started), Stats: domain.OperationStats{Queries: queries}, Err: err})
	return err
}

func (e staffEmployment) StaffEmployments(ctx context.Context, membershipIDs []int64) (map[int64]workforce.StaffEmployment, error) {
	result := make(map[int64]workforce.StaffEmployment, len(membershipIDs))
	if len(membershipIDs) == 0 {
		return result, nil
	}
	err := e.read(ctx, "list_staff_employments", 1, func(ctx context.Context) error {
		values, err := e.store.StaffEmployments(ctx, membershipIDs)
		for id, value := range values {
			result[id] = workforce.StaffEmployment(value)
		}
		return err
	})
	return result, err
}

func (e staffEmployment) StaffOnWorkTimeModel(ctx context.Context, workTimeModelID int64) (ids []int64, err error) {
	err = e.read(ctx, "list_staff_on_work_time_model", 1, func(ctx context.Context) error {
		ids, err = e.store.StaffOnWorkTimeModel(ctx, workTimeModelID)
		return err
	})
	return ids, err
}

func (e staffEmployment) SaveStaffEmployment(ctx context.Context, value workforce.StaffEmployment) error {
	return e.write(ctx, "save_staff_employment", 1, func(ctx context.Context) error {
		return e.store.SaveStaffEmployment(ctx, domain.StaffEmployment(value))
	})
}

func (e staffEmployment) ClearStaffWorkTimeModel(ctx context.Context, membershipID int64) error {
	return e.write(ctx, "clear_staff_work_time_model", 1, func(ctx context.Context) error {
		return e.store.ClearStaffWorkTimeModel(ctx, membershipID)
	})
}

func (e staffEmployment) AppendStaffNotes(ctx context.Context, membershipID int64, notes string) (result workforce.StaffEmployment, err error) {
	err = e.write(ctx, "append_staff_notes", 2, func(ctx context.Context) error {
		current, err := e.store.LockStaffEmployment(ctx, membershipID)
		if err != nil {
			return err
		}
		current.StaffNotes = domain.AppendStaffNotes(current.StaffNotes, notes)
		if err := e.store.SetStaffNotes(ctx, membershipID, current.StaffNotes); err != nil {
			return err
		}
		result = workforce.StaffEmployment(current)
		return nil
	})
	return result, err
}

func (e staffEmployment) SetStaffBirthdayDisplayOptOut(ctx context.Context, membershipID int64, optOut bool) error {
	return e.write(ctx, "set_staff_birthday_display_opt_out", 1, func(ctx context.Context) error {
		return e.store.SetStaffBirthdayDisplayOptOut(ctx, membershipID, optOut)
	})
}

func (e staffEmployment) RebaseStaffRotationAnchor(ctx context.Context, membershipIDs []int64, anchorDate string) error {
	if err := domain.ValidateDate(anchorDate, "rotation_anchor_date"); err != nil {
		return &workforce.InvalidWorkTimeError{Reason: err.Error()}
	}
	return e.write(ctx, "rebase_staff_rotation_anchor", 1, func(ctx context.Context) error {
		return e.store.RebaseStaffRotationAnchor(ctx, membershipIDs, anchorDate)
	})
}

func mapStaffEmploymentError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrStaffEmploymentNotFound):
		return workforce.ErrStaffEmploymentNotFound
	case errors.Is(err, domain.ErrPersonnelNumberTaken):
		return workforce.ErrPersonnelNumberTaken
	}
	return err
}
