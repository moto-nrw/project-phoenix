package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

type StudentOwners interface {
	LockClassWrites(context.Context, bool) error
	Enroll(context.Context, peopledirectory.StudentRecord) (int64, error)
	Renew(context.Context, peopledirectory.StudentRecord) (int64, error)
	SaveCare(context.Context, int64, peopledirectory.StudentRecord, peopledirectory.StudentPlan, *string, bool) error
}

type studentOwners struct{ owners StudentOwners }

func (s studentOwners) LockClassWrites(ctx context.Context, exclusive bool) error {
	if s.owners == nil {
		return errors.New("people directory: student owners are not bound")
	}
	return s.owners.LockClassWrites(ctx, exclusive)
}

func (s studentOwners) Enroll(ctx context.Context, record domain.StudentRecord) (int64, error) {
	if s.owners == nil {
		return 0, errors.New("people directory: student owners are not bound")
	}
	return s.owners.Enroll(ctx, peopledirectory.StudentRecord(record))
}

func (s studentOwners) Renew(ctx context.Context, record domain.StudentRecord) (int64, error) {
	if s.owners == nil {
		return 0, errors.New("people directory: student owners are not bound")
	}
	return s.owners.Renew(ctx, peopledirectory.StudentRecord(record))
}

func (s studentOwners) SaveCare(ctx context.Context, membershipID int64, record domain.StudentRecord, plan domain.DeparturePlan, note *string, touched bool) error {
	if s.owners == nil {
		return errors.New("people directory: student owners are not bound")
	}
	return s.owners.SaveCare(ctx, membershipID, peopledirectory.StudentRecord(record), peopledirectory.StudentPlan{
		AllowedDepartureModes: plan.AllowedDepartureModes, DepartureDays: plan.DepartureDays, BusDays: plan.BusDays, PickupDays: plan.PickupDays,
	}, note, touched)
}
