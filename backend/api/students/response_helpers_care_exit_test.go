package students

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	userService "github.com/moto-nrw/project-phoenix/services/users"
)

type failingRecordedExitService struct {
	userService.CareLifecycleService
	err error
}

func (s failingRecordedExitService) RecordedExitStudentIDs(_ context.Context, _ []int64) (map[int64]bool, error) {
	return nil, s.err
}

func TestEnrichWithCareExitFlag_PropagatesRecordedExitLookupFailure(t *testing.T) {
	t.Parallel()

	lookupErr := errors.New("care exit lookup unavailable")
	rs := &Resource{ResourceConfig: ResourceConfig{
		CareLifecycleService: failingRecordedExitService{err: lookupErr},
	}}
	responses := []StudentResponse{{ID: 42, CareEndsOn: "2026-08-23"}}

	err := rs.enrichWithCareExitFlag(context.Background(), responses)

	require.ErrorIs(t, err, lookupErr)
	require.False(t, responses[0].CareExitRecorded)
}
