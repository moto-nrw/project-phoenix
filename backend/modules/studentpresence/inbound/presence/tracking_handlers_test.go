package presence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"

	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock SettingsService ---

// newTrackingMockSettingsService builds a configtest.Mock wired the same way
// as the previous hand-rolled stub: ResolveBool/ResolveString delegate to the
// given funcs (nil → zero value), and the ForTenant variants delegate to the
// same funcs.
func newTrackingMockSettingsService(resolveBoolFunc func(ctx context.Context, key string) (bool, error), resolveStringFunc func(ctx context.Context, key string) (string, error)) *configtest.Mock {
	resolveBool := func(ctx context.Context, key string) (bool, error) {
		if resolveBoolFunc != nil {
			return resolveBoolFunc(ctx, key)
		}
		return false, nil
	}
	resolveString := func(ctx context.Context, key string) (string, error) {
		if resolveStringFunc != nil {
			return resolveStringFunc(ctx, key)
		}
		return "", nil
	}
	return &configtest.Mock{
		ResolveBoolFn:   resolveBool,
		ResolveStringFn: resolveString,
		ResolveBoolForTenantFn: func(ctx context.Context, _ int64, key string) (bool, error) {
			return resolveBool(ctx, key)
		},
	}
}

// --- Test Helpers ---

func createTrackingRequest(t *testing.T, body TrackingIndicatorsRequest) *testutil.Request {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	return httptest.NewRequest(testutil.MethodPost, "/tracking-indicators", bytes.NewReader(b))
}

// --- Tests ---

func TestGetTrackingIndicators_InvalidBody(t *testing.T) {
	t.Parallel()

	rs := resourceForTest(Resource{})

	req := httptest.NewRequest(testutil.MethodPost, "/tracking-indicators", bytes.NewReader([]byte("not json")))
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, testutil.StatusBadRequest, rr.Code)
}

func TestGetTrackingIndicators_EmptyStudentIDs(t *testing.T) {
	t.Parallel()

	rs := resourceForTest(Resource{})

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, testutil.StatusOK, rr.Code)

	var resp struct {
		Data TrackingIndicatorsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data.Labels)
	assert.Empty(t, resp.Data.Results)
}

func TestGetTrackingIndicators_InvalidStudentID(t *testing.T) {
	t.Parallel()

	rs := resourceForTest(Resource{})

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{10, 0}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, testutil.StatusBadRequest, rr.Code)
}

func TestGetTrackingIndicators_NegativeStudentID(t *testing.T) {
	t.Parallel()

	rs := resourceForTest(Resource{})

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{10, -5}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, testutil.StatusBadRequest, rr.Code)
}

func TestGetTrackingIndicators_SettingsError(t *testing.T) {
	t.Parallel()

	settings := newTrackingMockSettingsService(func(ctx context.Context, key string) (bool, error) {
		return false, errors.New("settings service error")
	}, nil)

	rs := resourceForTest(Resource{SettingsService: settings})

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{10}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	// Returns 200 with empty data on settings error (graceful degradation)
	assert.Equal(t, testutil.StatusOK, rr.Code)
	var resp struct {
		Data TrackingIndicatorsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data.Labels)
}

func TestGetTrackingIndicators_Disabled(t *testing.T) {
	t.Parallel()

	settings := newTrackingMockSettingsService(func(ctx context.Context, key string) (bool, error) {
		return false, nil
	}, nil)

	rs := resourceForTest(Resource{SettingsService: settings})

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{10}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, testutil.StatusOK, rr.Code)
	var resp struct {
		Data TrackingIndicatorsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data.Labels)
}

func TestGetTrackingIndicators_NoLabelsConfigured(t *testing.T) {
	t.Parallel()

	settings := newTrackingMockSettingsService(
		func(ctx context.Context, key string) (bool, error) {
			return true, nil
		},
		func(ctx context.Context, key string) (string, error) {
			return "", nil
		},
	)

	rs := resourceForTest(Resource{SettingsService: settings})

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{10}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, testutil.StatusOK, rr.Code)
	var resp struct {
		Data TrackingIndicatorsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data.Labels)
}

func TestGetTrackingIndicators_LabelResolutionError(t *testing.T) {
	t.Parallel()

	settings := newTrackingMockSettingsService(
		func(ctx context.Context, key string) (bool, error) {
			return true, nil
		},
		func(ctx context.Context, key string) (string, error) {
			return "", errors.New("label read error")
		},
	)

	rs := resourceForTest(Resource{SettingsService: settings})

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{10}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, testutil.StatusOK, rr.Code)
	var resp struct {
		Data TrackingIndicatorsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data.Labels)
}

func TestGetTrackingIndicators_Success(t *testing.T) {
	t.Parallel()

	labelValues := map[string]string{
		config.KeyTrackingIndicator1: "Hausaufgaben",
		config.KeyTrackingIndicator2: "Mensa",
		config.KeyTrackingIndicator3: "",
	}

	settings := newTrackingMockSettingsService(
		func(ctx context.Context, key string) (bool, error) {
			return true, nil
		},
		func(ctx context.Context, key string) (string, error) {
			return labelValues[key], nil
		},
	)

	mockSvc := &stubPresenceOperations{
		trackingIndicators: func(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error) {
			assert.Equal(t, []int64{10, 20}, studentIDs)
			assert.Equal(t, []string{"Hausaufgaben", "Mensa"}, labels)
			return map[int64][]bool{
				10: {true, false},
				20: {false, true},
			}, nil
		},
	}

	rs := resourceForTest(Resource{
		SettingsService: settings,
		Operations:      mockSvc,
	})

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{10, 20}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, testutil.StatusOK, rr.Code)

	var resp struct {
		Data TrackingIndicatorsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, []string{"Hausaufgaben", "Mensa"}, resp.Data.Labels)
	assert.Equal(t, map[int64][]bool{10: {true, false}, 20: {false, true}}, resp.Data.Results)
}

func TestGetTrackingIndicators_ServiceError(t *testing.T) {
	t.Parallel()

	settings := newTrackingMockSettingsService(
		func(ctx context.Context, key string) (bool, error) {
			return true, nil
		},
		func(ctx context.Context, key string) (string, error) {
			return "Mensa", nil
		},
	)

	mockSvc := &stubPresenceOperations{
		trackingIndicators: func(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error) {
			return nil, errors.New("service error")
		},
	}

	rs := resourceForTest(Resource{
		SettingsService: settings,
		Operations:      mockSvc,
	})

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{10}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, testutil.StatusInternalServerError, rr.Code)
}

func TestGetTrackingIndicators_WhitespaceOnlyLabels(t *testing.T) {
	t.Parallel()

	settings := newTrackingMockSettingsService(
		func(ctx context.Context, key string) (bool, error) {
			return true, nil
		},
		func(ctx context.Context, key string) (string, error) {
			return "   ", nil
		},
	)

	rs := resourceForTest(Resource{SettingsService: settings})

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{10}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, testutil.StatusOK, rr.Code)
	var resp struct {
		Data TrackingIndicatorsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data.Labels)
}

func TestGetTrackingIndicators_PartialLabelsConfigured(t *testing.T) {
	t.Parallel()

	labelValues := map[string]string{
		config.KeyTrackingIndicator1: "Mensa",
		config.KeyTrackingIndicator2: "",
		config.KeyTrackingIndicator3: "Sport",
	}

	settings := newTrackingMockSettingsService(
		func(ctx context.Context, key string) (bool, error) {
			return true, nil
		},
		func(ctx context.Context, key string) (string, error) {
			return labelValues[key], nil
		},
	)

	mockSvc := &stubPresenceOperations{
		trackingIndicators: func(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error) {
			assert.Equal(t, []string{"Mensa", "Sport"}, labels)
			return map[int64][]bool{10: {true, true}}, nil
		},
	}

	rs := resourceForTest(Resource{
		SettingsService: settings,
		Operations:      mockSvc,
	})

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{10}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, testutil.StatusOK, rr.Code)
}
