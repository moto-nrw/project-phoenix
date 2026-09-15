package iot

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSchoolName_Success(t *testing.T) {
	t.Parallel()

	query := schoolNameStub(func(context.Context) (string, error) {
		return "OGS Musterstadt", nil
	})
	rs := NewResource(nil, query, testRuntime())

	req := httptest.NewRequest("GET", "/school-name", nil)
	testutil.WithDeviceIdentity(9, "school-device")(req)

	w := httptest.NewRecorder()
	rs.getSchoolName(w, req)

	assert.Equal(t, 200, w.Code)

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "success", body["status"])
	assert.Equal(t, "School name retrieved", body["message"])

	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "OGS Musterstadt", data["name"])
}

func TestGetSchoolName_NoDeviceContext(t *testing.T) {
	t.Parallel()

	rs := NewResource(nil, nil, testRuntime())

	req := httptest.NewRequest("GET", "/school-name", nil)
	w := httptest.NewRecorder()
	rs.getSchoolName(w, req)

	assert.Equal(t, 401, w.Code)

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "error", body["status"])
	assert.Equal(t, devicescan.MessageDeviceAPIKeyRequired, body["error"])
}

func TestGetSchoolName_SchoolNotFound(t *testing.T) {
	t.Parallel()

	query := schoolNameStub(func(context.Context) (string, error) {
		return "", errors.New("sql: no rows in result set")
	})
	rs := NewResource(nil, query, testRuntime())

	req := httptest.NewRequest("GET", "/school-name", nil)
	testutil.WithDeviceIdentity(9, "school-device")(req)

	w := httptest.NewRecorder()
	rs.getSchoolName(w, req)

	assert.Equal(t, 500, w.Code)

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "error", body["status"])
}

func TestGetSchoolName_DatabaseError(t *testing.T) {
	t.Parallel()

	query := schoolNameStub(func(context.Context) (string, error) {
		return "", errors.New("connection refused")
	})
	rs := NewResource(nil, query, testRuntime())

	req := httptest.NewRequest("GET", "/school-name", nil)
	testutil.WithDeviceIdentity(9, "school-device")(req)

	w := httptest.NewRecorder()
	rs.getSchoolName(w, req)

	assert.Equal(t, 500, w.Code)
}

func TestGetSchoolName_EmptySchoolName(t *testing.T) {
	t.Parallel()

	query := schoolNameStub(func(context.Context) (string, error) {
		return "", nil
	})
	rs := NewResource(nil, query, testRuntime())

	req := httptest.NewRequest("GET", "/school-name", nil)
	testutil.WithDeviceIdentity(9, "school-device")(req)

	w := httptest.NewRecorder()
	rs.getSchoolName(w, req)

	assert.Equal(t, 200, w.Code)

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)

	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "", data["name"])
}

func TestGetSchoolName_ResponseStructure(t *testing.T) {
	t.Parallel()

	query := schoolNameStub(func(context.Context) (string, error) {
		return "Grundschule am Park", nil
	})
	rs := NewResource(nil, query, testRuntime())

	req := httptest.NewRequest("GET", "/school-name", nil)
	testutil.WithDeviceIdentity(9, "school-device")(req)

	w := httptest.NewRecorder()
	rs.getSchoolName(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)

	// Verify only expected top-level keys
	assert.Contains(t, body, "status")
	assert.Contains(t, body, "data")
	assert.Contains(t, body, "message")

	// Verify data only contains "name"
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Len(t, data, 1)
	assert.Contains(t, data, "name")
	assert.Equal(t, "Grundschule am Park", data["name"])
}

func TestSchoolNameResponse_JSONSerialization(t *testing.T) {
	t.Parallel()

	resp := schoolNameResponse{Name: "Test Schule"}
	b, err := json.Marshal(resp)
	require.NoError(t, err)

	var parsed map[string]string
	err = json.Unmarshal(b, &parsed)
	require.NoError(t, err)
	assert.Equal(t, "Test Schule", parsed["name"])
	assert.Len(t, parsed, 1)
}

type schoolNameStub func(context.Context) (string, error)

func (s schoolNameStub) DeviceSchoolName(ctx context.Context) (string, error) { return s(ctx) }
