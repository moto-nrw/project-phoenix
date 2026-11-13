package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/require"
)

type countReviewReads struct {
	batchReviewReads
	failAt string
	calls  []string
	err    error
}

func (r *countReviewReads) read(step string) error {
	r.calls = append(r.calls, step)
	if r.failAt == step {
		return r.err
	}
	return nil
}

func (r *countReviewReads) ListCareScheduleRequests(ctx context.Context, filter careplan.CareScheduleRequestFilter) ([]careplan.CareScheduleChangeRequest, error) {
	if err := r.read("requests"); err != nil {
		return nil, err
	}
	return r.batchReviewReads.ListCareScheduleRequests(ctx, filter)
}

func (r *countReviewReads) FindStudents(ctx context.Context, ids []int64) (map[int64]careplan.ReviewStudent, error) {
	if err := r.read("students"); err != nil {
		return nil, err
	}
	return r.batchReviewReads.FindStudents(ctx, ids)
}

func (r *countReviewReads) PersonNames(ctx context.Context, ids []int64) (map[int64]careplan.PersonName, error) {
	if err := r.read("names"); err != nil {
		return nil, err
	}
	return r.batchReviewReads.PersonNames(ctx, ids)
}

func TestCareScheduleCountRetainsReadErrorPriority(t *testing.T) {
	t.Parallel()
	steps := []string{"requests", "students", "names", "scope"}
	for i, step := range steps {
		t.Run(step, func(t *testing.T) {
			for _, omit := range []bool{false, true} {
				reads := &countReviewReads{failAt: step, err: errors.New("read unavailable")}
				query := NewScheduleReviews(ScheduleReviewDependencies{
					RunInTx:  func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
					Requests: reads, People: reads,
					Scope: func(context.Context) (careplan.ReviewScope, error) {
						return careplan.ReviewScope{}, reads.read("scope")
					},
				})
				items, cursor, err := query.ListPending(context.Background(), careplan.RequestQueueFilter{OmitDiff: omit})
				require.ErrorIs(t, err, reads.err)
				require.Nil(t, items)
				require.Nil(t, cursor)
				require.Equal(t, steps[:i+1], reads.calls)
			}
		})
	}
}

func TestCareScheduleCountDoesNotReadUnusedDiffs(t *testing.T) {
	t.Parallel()
	reads := &countReviewReads{}
	query := NewScheduleReviews(ScheduleReviewDependencies{
		RunInTx:  func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
		Requests: reads, People: reads,
		Scope: func(context.Context) (careplan.ReviewScope, error) {
			return careplan.ReviewScope{SchoolWide: true}, nil
		},
		Today: func() careplan.Date { return "2026-11-05" },
		// No schedule, class or booking source: counting must not read them.
	})
	items, cursor, err := query.ListPending(context.Background(), careplan.RequestQueueFilter{OmitDiff: true})
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Nil(t, cursor)
	for _, item := range items {
		require.Empty(t, item.Diff)
	}
	require.Equal(t, []string{"requests", "students", "names"}, reads.calls)
}
