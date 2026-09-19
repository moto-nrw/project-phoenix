package presence

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mappingWriteQueries struct {
	mappingReadQueries
	write func(context.Context, int64, int64) error
}

func (q mappingWriteQueries) AddGroupToCombination(ctx context.Context, combinedID, groupID int64) error {
	return q.write(ctx, combinedID, groupID)
}

func (q mappingWriteQueries) RemoveGroupFromCombination(ctx context.Context, combinedID, groupID int64) error {
	return q.write(ctx, combinedID, groupID)
}

func TestMappingAddRoutePreservesPreflightAndVerification(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		status, writes int
	}{
		{"success", testutil.StatusOK, 1},
		{"duplicate", testutil.StatusBadRequest, 0},
		{"read failure", testutil.StatusInternalServerError, 0},
		{"write failure", testutil.StatusInternalServerError, 1},
		{"verification failure", testutil.StatusOK, 1},
		{"verification missing", testutil.StatusOK, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads, writes := 0, 0
			resource := resourceForTest(Resource{Presence: mappingWriteQueries{
				mappingReadQueries: mappingReadQueries{read: func(_ context.Context, filter studentpresence.GroupMappingFilter) ([]studentpresence.GroupMapping, error) {
					require.NotNil(t, filter.CombinedGroupID)
					assert.EqualValues(t, 23, *filter.CombinedGroupID)
					reads++
					if tc.name == "read failure" || (tc.name == "verification failure" && reads == 2) {
						return nil, errors.New("lookup unavailable")
					}
					if tc.name == "duplicate" || (tc.name == "success" && reads == 2) {
						return []studentpresence.GroupMapping{{ID: 17, ActiveGroupID: 42, ActiveCombinedGroupID: 23}}, nil
					}
					return nil, nil
				}},
				write: func(_ context.Context, combinedID, groupID int64) error {
					writes++
					assert.EqualValues(t, 23, combinedID)
					assert.EqualValues(t, 42, groupID)
					if tc.name == "write failure" {
						return errors.New("write unavailable")
					}
					return nil
				},
			}})
			request := httptest.NewRequest(testutil.MethodPost, "/", strings.NewReader(`{"active_group_id":42,"combined_group_id":23}`))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			resource.addGroupToCombination(response, request)
			assert.Equal(t, tc.status, response.Code)
			assert.Equal(t, tc.writes, writes)
			if tc.status == testutil.StatusOK {
				assert.Equal(t, 2, reads)
				assert.Contains(t, response.Body.String(), msgGroupAddedToCombination)
			}
		})
	}
}

func TestMappingRemoveRouteUsesNativeWrite(t *testing.T) {
	t.Parallel()
	for _, fail := range []bool{false, true} {
		called := false
		resource := resourceForTest(Resource{Presence: mappingWriteQueries{write: func(_ context.Context, combinedID, groupID int64) error {
			called = true
			assert.EqualValues(t, 23, combinedID)
			assert.EqualValues(t, 42, groupID)
			if fail {
				return errors.New("write unavailable")
			}
			return nil
		}}})
		request := httptest.NewRequest(testutil.MethodDelete, "/", strings.NewReader(`{"active_group_id":42,"combined_group_id":23}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		resource.removeGroupFromCombination(response, request)
		assert.True(t, called)
		if fail {
			assert.Equal(t, testutil.StatusInternalServerError, response.Code)
		} else {
			assert.Equal(t, testutil.StatusOK, response.Code)
		}
	}
}
