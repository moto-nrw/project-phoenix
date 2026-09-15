package api

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type activeStaffProjectionSource struct {
	lookup func(context.Context) (int64, int64, bool, error)
}

func (s activeStaffProjectionSource) GetCurrentStaff(ctx context.Context) (int64, int64, bool, error) {
	return s.lookup(ctx)
}

func (s activeStaffProjectionSource) HasCurrentStaff(ctx context.Context) (bool, error) {
	_, _, found, err := s.lookup(ctx)
	return found, err
}

func TestActiveStaffProjectionPreservesLookupResults(t *testing.T) {
	t.Parallel()
	lookupErr := errors.New("identity lookup failed")
	for _, withRow := range []bool{false, true} {
		for _, err := range []error{nil, lookupErr} {
			ctx, cancel := context.WithCancel(context.Background())
			adapter := activeStaffAccess{source: activeStaffProjectionSource{lookup: func(got context.Context) (int64, int64, bool, error) {
				assert.Same(t, ctx, got)
				return 42, 7, withRow, err
			}}}
			got, gotErr := adapter.GetCurrentStaff(ctx)
			assert.ErrorIs(t, gotErr, err)
			if withRow {
				require.NotNil(t, got)
				assert.EqualValues(t, 42, got.ID)
				assert.EqualValues(t, 7, got.TenantID)
			} else {
				assert.Nil(t, got)
			}
			found, gotErr := adapter.HasCurrentStaff(ctx)
			assert.Equal(t, withRow, found)
			assert.ErrorIs(t, gotErr, err)
			cancel()
		}
	}
}
