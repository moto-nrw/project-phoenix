package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

func TestCheckChildQuota(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		limit, before, after int
		refused              *schoolmembership.ChildQuotaReachedError
	}{
		"below the Kinderkontingent":         {limit: 3, before: 1, after: 2},
		"reaches it exactly":                 {limit: 3, before: 2, after: 3},
		"goes one past it":                   {limit: 3, before: 3, after: 4, refused: &schoolmembership.ChildQuotaReachedError{Booked: 3, Occupied: 3, Requested: 1}},
		"a batch that does not fit":          {limit: 3, before: 2, after: 4, refused: &schoolmembership.ChildQuotaReachedError{Booked: 3, Occupied: 2, Requested: 2}},
		"adds nobody while above it":         {limit: 3, before: 5, after: 5},
		"frees a place while above it":       {limit: 3, before: 5, after: 4},
		"adds one while above a lowered one": {limit: 3, before: 5, after: 6, refused: &schoolmembership.ChildQuotaReachedError{Booked: 3, Occupied: 5, Requested: 1}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := CheckChildQuota(tc.limit, tc.before, tc.after)
			if tc.refused == nil {
				if err != nil {
					t.Fatalf("unexpected refusal: %v", err)
				}
				return
			}
			var reached *schoolmembership.ChildQuotaReachedError
			if !errors.As(err, &reached) || *reached != *tc.refused {
				t.Fatalf("want %+v, got %v", tc.refused, err)
			}
		})
	}
}

func TestCountingWriteAfterClassGateRejectsCancellationBeforeQuota(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	// No store is bound: any quota-lock/read attempt would panic. Cancellation
	// must win before those operations, while an unbound quota retains priority.
	service := &Service{quota: &recordingStore{}}
	var stats domain.OperationStats
	err := service.countingWriteAfterClassGate(ctx, &stats, func(context.Context) error {
		t.Fatal("canceled command must not write")
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)
	require.EqualError(t, err, "context canceled")
	require.Zero(t, stats.Queries)
	service.quota = nil
	require.ErrorIs(t, service.countingWriteAfterClassGate(ctx, &stats, nil), errChildQuotaUnbound)
}
