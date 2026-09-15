package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func NewOffboarding(dependencies Dependencies, appendDeletion func(context.Context, workforce.StaffAbsence, int64) error) (*workforce.Offboarding, error) {
	if appendDeletion == nil {
		return nil, errors.New("workforce: absence deletion audit is required")
	}
	service, err := newApplication(dependencies)
	if err != nil {
		return nil, err
	}
	return workforce.NewOffboarding(
		func(ctx context.Context, staffID int64) error {
			if _, ok := tenant.TransactionFromContext(ctx); !ok {
				return errors.New("workforce: offboarding lock requires a transaction")
			}
			return service.LockStaffOffboarding(ctx, staffID)
		},
		func(ctx context.Context, staffID int64, from string) (workforce.OffboardingPreview, error) {
			result, err := service.PreviewStaffOffboarding(ctx, staffID, from)
			return workforce.OffboardingPreview{Counts: offboardingCountsToPublic(result.Counts), Revision: result.Revision, Blocked: result.Blocked}, mapError(err)
		},
		func(ctx context.Context, staffID, actorID int64, from, revision string) (workforce.OffboardingCounts, error) {
			result, err := service.ExecuteStaffOffboarding(ctx, staffID, actorID, from, revision, func(txCtx context.Context, absence domain.StaffAbsence, actorID int64) error {
				return appendDeletion(txCtx, absenceToPublic(absence), actorID)
			})
			return offboardingCountsToPublic(result), mapError(err)
		}), nil
}

func offboardingCountsToPublic(value domain.OffboardingCounts) workforce.OffboardingCounts {
	return workforce.OffboardingCounts{Absences: value.Absences, Shifts: value.Shifts, Series: value.Series, Substitutions: value.Substitutions}
}
