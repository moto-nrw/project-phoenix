package active

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

func (r *WorkSessionRepository) OldestBefore(ctx context.Context, column string, cutoff *timezone.Date) (*timezone.Date, error) {
	return oldestDate(ctx, r.Repository.OldestBefore, column, cutoff)
}

func (r *StaffAbsenceRepository) OldestBefore(ctx context.Context, column string, cutoff *timezone.Date) (*timezone.Date, error) {
	return oldestDate(ctx, r.Repository.OldestBefore, column, cutoff)
}

func (r *WorkSessionRepository) DeleteOlderThan(ctx context.Context, column string, cutoff timezone.Date) (int64, error) {
	return r.Repository.DeleteOlderThan(ctx, column, string(cutoff))
}

func (r *StaffAbsenceRepository) DeleteOlderThan(ctx context.Context, column string, cutoff timezone.Date) (int64, error) {
	return r.Repository.DeleteOlderThan(ctx, column, string(cutoff))
}

func oldestDate(
	ctx context.Context,
	load func(context.Context, string, *string) (*string, error),
	column string,
	cutoff *timezone.Date,
) (*timezone.Date, error) {
	var storedCutoff *string
	if cutoff != nil {
		value := string(*cutoff)
		storedCutoff = &value
	}
	stored, err := load(ctx, column, storedCutoff)
	if err != nil || stored == nil {
		return nil, err
	}
	date, err := timezone.ParseDate(*stored)
	return &date, err
}
