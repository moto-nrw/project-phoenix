package active

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mappingReadQueries struct {
	PresenceQueries
	read func(context.Context, studentpresence.GroupMappingFilter) ([]studentpresence.GroupMapping, error)
}

func (q mappingReadQueries) ListGroupMappings(ctx context.Context, filter studentpresence.GroupMappingFilter) ([]studentpresence.GroupMapping, error) {
	return q.read(ctx, filter)
}

func TestMappingReadRoutesUseNativeProjection(t *testing.T) {
	t.Parallel()
	for _, combined := range []bool{false, true} {
		for _, outcome := range []string{"rows", "empty", "error"} {
			t.Run(outcome+map[bool]string{true: " combined", false: " active"}[combined], func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				resource := resourceForTest(Resource{Presence: mappingReadQueries{read: func(received context.Context, filter studentpresence.GroupMappingFilter) ([]studentpresence.GroupMapping, error) {
					require.NoError(t, received.Err())
					if combined {
						require.NotNil(t, filter.CombinedGroupID)
						assert.EqualValues(t, 42, *filter.CombinedGroupID)
						assert.Nil(t, filter.ActiveGroupID)
					} else {
						require.NotNil(t, filter.ActiveGroupID)
						assert.EqualValues(t, 42, *filter.ActiveGroupID)
						assert.Nil(t, filter.CombinedGroupID)
					}
					switch outcome {
					case "error":
						return nil, errors.New("database unavailable")
					case "empty":
						return nil, nil
					default:
						return []studentpresence.GroupMapping{{ID: 17, ActiveGroupID: 42, ActiveCombinedGroupID: 42}}, nil
					}
				}}})
				router := testutil.NewRouter()
				if combined {
					router.Get("/{combinedId}", resource.getCombinedGroupMappings)
				} else {
					router.Get("/{groupId}", resource.getGroupMappings)
				}
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(testutil.MethodGet, "/42", nil).WithContext(ctx))
				if outcome == "error" {
					assert.Equal(t, testutil.StatusInternalServerError, response.Code)
					return
				}
				require.Equal(t, testutil.StatusOK, response.Code)
				var body struct {
					Data []GroupMappingResponse `json:"data"`
				}
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
				if outcome == "empty" {
					require.NotNil(t, body.Data)
					assert.Empty(t, body.Data)
				} else {
					assert.Equal(t, []GroupMappingResponse{{ID: 17, ActiveGroupID: 42, CombinedGroupID: 42}}, body.Data)
				}
			})
		}
	}
}
