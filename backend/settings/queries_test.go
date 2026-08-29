package settings

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type enrollmentReaderStub struct {
	values map[int64]bool
	err    error
}

func (s enrollmentReaderStub) EnrollmentEnabledForTenants(_ context.Context, tenantIDs []int64) (map[int64]bool, error) {
	if s.err != nil {
		return nil, s.err
	}
	result := make(map[int64]bool, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		result[tenantID] = s.values[tenantID]
	}
	return result, nil
}

func TestQueriesEnrollmentEnabledPreservesResolvedTenantValues(t *testing.T) {
	t.Parallel()

	queries := NewQueries(enrollmentReaderStub{values: map[int64]bool{11: true, 22: false}})
	values, err := queries.EnrollmentEnabledForTenants(context.Background(), []int64{11, 22})
	require.NoError(t, err)
	assert.Equal(t, map[int64]bool{11: true, 22: false}, values)
}

func TestQueriesEnrollmentEnabledFailsForMissingProvider(t *testing.T) {
	t.Parallel()

	_, err := (*Queries)(nil).EnrollmentEnabledForTenants(context.Background(), []int64{11})
	assert.ErrorIs(t, err, ErrQueriesUnavailable)
}

func TestQueriesEnrollmentEnabledPreservesLookupFailure(t *testing.T) {
	t.Parallel()

	want := errors.New("lookup failed")
	queries := NewQueries(enrollmentReaderStub{err: want})
	_, err := queries.EnrollmentEnabledForTenants(context.Background(), []int64{11})
	assert.ErrorIs(t, err, want)
}
