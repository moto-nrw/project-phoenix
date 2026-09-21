package compose

import (
	"context"
	"errors"

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
// opens one for the tenant in context.
func NewStaffEmployment(db *bun.DB) (workforce.StaffEmployments, error) {
	if db == nil {
		return nil, errors.New("workforce staff employment: database is required")
	}
	return staffEmployment{store: postgres.New(databaseRuntime(db))}, nil
}

type staffEmployment struct{ store *postgres.Store }

func (e staffEmployment) read(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	if _, err := tenant.TenantFromContext(ctx); err == nil {
		return tenant.WithinCurrentTenant(ctx, callback)
	}
	return tenant.WithinAdmin(ctx, callback)
}

func (e staffEmployment) write(ctx context.Context, callback func(context.Context) error) error {
	var err error
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		err = callback(ctx)
	} else {
		err = tenant.WithinCurrentTenant(ctx, callback)
	}
	return mapStaffEmploymentError(err)
}

func (e staffEmployment) StaffEmployments(ctx context.Context, membershipIDs []int64) (map[int64]workforce.StaffEmployment, error) {
	result := make(map[int64]workforce.StaffEmployment, len(membershipIDs))
	if len(membershipIDs) == 0 {
		return result, nil
	}
	err := e.read(ctx, func(ctx context.Context) error {
		values, err := e.store.StaffEmployments(ctx, membershipIDs)
		for id, value := range values {
			result[id] = workforce.StaffEmployment(value)
		}
		return err
	})
	return result, err
}

func (e staffEmployment) StaffOnWorkTimeModel(ctx context.Context, workTimeModelID int64) (ids []int64, err error) {
	err = e.read(ctx, func(ctx context.Context) error {
		ids, err = e.store.StaffOnWorkTimeModel(ctx, workTimeModelID)
		return err
	})
	return ids, err
}

func (e staffEmployment) SaveStaffEmployment(ctx context.Context, value workforce.StaffEmployment) error {
	return e.write(ctx, func(ctx context.Context) error {
		return e.store.SaveStaffEmployment(ctx, domain.StaffEmployment(value))
	})
}

func (e staffEmployment) ClearStaffWorkTimeModel(ctx context.Context, membershipID int64) error {
	return e.write(ctx, func(ctx context.Context) error {
		return e.store.ClearStaffWorkTimeModel(ctx, membershipID)
	})
}

func (e staffEmployment) AppendStaffNotes(ctx context.Context, membershipID int64, notes string) (result workforce.StaffEmployment, err error) {
	err = e.write(ctx, func(ctx context.Context) error {
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
	return e.write(ctx, func(ctx context.Context) error {
		return e.store.SetStaffBirthdayDisplayOptOut(ctx, membershipID, optOut)
	})
}

func (e staffEmployment) RebaseStaffRotationAnchor(ctx context.Context, membershipIDs []int64, anchorDate string) error {
	if err := domain.ValidateDate(anchorDate, "rotation_anchor_date"); err != nil {
		return &workforce.InvalidWorkTimeError{Reason: err.Error()}
	}
	return e.write(ctx, func(ctx context.Context) error {
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
