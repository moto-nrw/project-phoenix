package schedule

import (
	"context"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

func (r *ActivityInstanceRepository) OldestBefore(ctx context.Context, column string, cutoff *scheduleModels.Date) (*scheduleModels.Date, error) {
	return oldestDate(ctx, r.Repository.OldestBefore, column, cutoff)
}

func (r *ActivityExceptionRepository) OldestBefore(ctx context.Context, column string, cutoff *scheduleModels.Date) (*scheduleModels.Date, error) {
	return oldestDate(ctx, r.Repository.OldestBefore, column, cutoff)
}

func (r *ActivityInstanceRepository) DeleteOlderThan(ctx context.Context, column string, cutoff scheduleModels.Date) (int64, error) {
	return r.Repository.DeleteOlderThan(ctx, column, string(cutoff))
}

func (r *ActivityExceptionRepository) DeleteOlderThan(ctx context.Context, column string, cutoff scheduleModels.Date) (int64, error) {
	return r.Repository.DeleteOlderThan(ctx, column, string(cutoff))
}

func oldestDate(
	ctx context.Context,
	load func(context.Context, string, *string) (*string, error),
	column string,
	cutoff *scheduleModels.Date,
) (*scheduleModels.Date, error) {
	var storedCutoff *string
	if cutoff != nil {
		value := string(*cutoff)
		storedCutoff = &value
	}
	stored, err := load(ctx, column, storedCutoff)
	if err != nil || stored == nil {
		return nil, err
	}
	date, err := scheduleModels.ParseDate(*stored)
	return &date, err
}
