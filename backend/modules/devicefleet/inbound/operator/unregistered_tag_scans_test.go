package operator_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/render"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/inbound/operator"
)

// The review routes answer in the operator surface's format; these cases pin
// what the handlers hand to that surface and how they narrow and label the
// Device Fleet scans, and how a failed resolution maps onto the surface's
// client and internal errors.

type fakeScans struct {
	listFilter    devicefleet.UnregisteredTagScanFilter
	listCalled    bool
	listResult    []devicefleet.UnregisteredTagScan
	listErr       error
	resolveInput  devicefleet.ResolveUnregisteredTagScan
	resolveResult devicefleet.UnregisteredTagScan
	resolveErr    error
}

func (s *fakeScans) ListUnregisteredTagScans(_ context.Context, filter devicefleet.UnregisteredTagScanFilter) ([]devicefleet.UnregisteredTagScan, error) {
	s.listCalled = true
	s.listFilter = filter
	return s.listResult, s.listErr
}

func (s *fakeScans) ResolveUnregisteredTagScan(_ context.Context, input devicefleet.ResolveUnregisteredTagScan) (devicefleet.UnregisteredTagScan, error) {
	s.resolveInput = input
	return s.resolveResult, s.resolveErr
}

type fakeDirectory struct {
	listSchoolsByIDFn           func(context.Context, []int64) ([]operator.School, error)
	listSchoolsByOrganizationFn func(context.Context, int64) ([]operator.School, error)
	listOrganizationsByIDFn     func(context.Context, []int64) ([]operator.Organization, error)
}

func (d *fakeDirectory) ListSchoolsByID(ctx context.Context, ids []int64) ([]operator.School, error) {
	if d.listSchoolsByIDFn != nil {
		return d.listSchoolsByIDFn(ctx, ids)
	}
	schools := make([]operator.School, 0, len(ids))
	for _, id := range ids {
		schools = append(schools, operator.School{ID: id, OrganizationID: id / 2})
	}
	return schools, nil
}

func (d *fakeDirectory) ListSchoolsByOrganization(ctx context.Context, id int64) ([]operator.School, error) {
	if d.listSchoolsByOrganizationFn != nil {
		return d.listSchoolsByOrganizationFn(ctx, id)
	}
	return []operator.School{{ID: id * 2, OrganizationID: id}}, nil
}

func (d *fakeDirectory) ListOrganizationsByID(ctx context.Context, ids []int64) ([]operator.Organization, error) {
	if d.listOrganizationsByIDFn != nil {
		return d.listOrganizationsByIDFn(ctx, ids)
	}
	organizations := make([]operator.Organization, 0, len(ids))
	for _, id := range ids {
		organizations = append(organizations, operator.Organization{ID: id})
	}
	return organizations, nil
}

// testRenderer records which surface body a handler chose.
type testRenderer struct {
	status  int
	kind    string
	message string
}

func (r *testRenderer) Render(_ http.ResponseWriter, req *http.Request) error {
	render.Status(req, r.status)
	return nil
}

type testSurface struct {
	adminRuns  int
	operatorID int64
}

func (s *testSurface) surface() operator.Surface {
	return operator.Surface{
		InvalidRequest: func(err error) render.Renderer {
			return &testRenderer{status: http.StatusBadRequest, kind: "invalid", message: err.Error()}
		},
		Internal: func(message string) render.Renderer {
			return &testRenderer{status: http.StatusInternalServerError, kind: "internal", message: message}
		},
		RenderError: func(w http.ResponseWriter, r *http.Request, renderer render.Renderer) {
			rendered := renderer.(*testRenderer)
			render.Status(r, rendered.status)
			render.JSON(w, r, map[string]string{"kind": rendered.kind, "message": rendered.message})
		},
		Respond: func(w http.ResponseWriter, r *http.Request, status int, data any, message string) {
			render.Status(r, status)
			render.JSON(w, r, map[string]any{"data": data, "message": message})
		},
		OperatorID: func(context.Context) int64 { return s.operatorID },
	}
}

func newReview(scans *fakeScans, directory *fakeDirectory, surface *testSurface) *operator.Resource {
	return operator.NewResource(operator.Config{
		Scans:     scans,
		Directory: directory,
		Surface:   surface.surface(),
		WithinAdmin: func(ctx context.Context, fn func(context.Context) error) error {
			surface.adminRuns++
			return fn(ctx)
		},
	})
}

type errorBody struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type listBody struct {
	Data    []map[string]any `json:"data"`
	Message string           `json:"message"`
}

func serve(t *testing.T, resource *operator.Resource, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	resource.Router().ServeHTTP(rr, req)
	return rr
}

func decode[T any](t *testing.T, rr *httptest.ResponseRecorder) T {
	t.Helper()
	var body T
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), rr.Body.String())
	return body
}

func TestListUnregisteredTagScansNarrowsAndLabels(t *testing.T) {
	t.Parallel()

	schoolID := int64(10)
	orgID := schoolID / 2
	deviceName := "Eingang"
	scans := &fakeScans{listResult: []devicefleet.UnregisteredTagScan{{ID: 300, TenantID: schoolID, TagUID: "ABC123", DeviceName: &deviceName}}}
	directory := &fakeDirectory{
		listSchoolsByOrganizationFn: func(_ context.Context, id int64) ([]operator.School, error) {
			require.Equal(t, orgID, id)
			return []operator.School{{ID: schoolID, OrganizationID: orgID}, {ID: schoolID + 1, OrganizationID: orgID}}, nil
		},
		listSchoolsByIDFn: func(_ context.Context, ids []int64) ([]operator.School, error) {
			require.Equal(t, []int64{schoolID}, ids)
			return []operator.School{{ID: schoolID, Name: "School", OrganizationID: orgID}}, nil
		},
		listOrganizationsByIDFn: func(_ context.Context, ids []int64) ([]operator.Organization, error) {
			require.Equal(t, []int64{orgID}, ids)
			return []operator.Organization{{ID: orgID, Name: "Organization"}}, nil
		},
	}
	surface := &testSurface{}

	rr := serve(t, newReview(scans, directory, surface), http.MethodGet, "/?school_id=10&organization_id=5", "")

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	body := decode[listBody](t, rr)
	require.Equal(t, "Unregistered RFID scans retrieved successfully", body.Message)
	require.Len(t, body.Data, 1)
	require.Equal(t, map[string]any{
		"id":                float64(300),
		"created_at":        "0001-01-01T00:00:00Z",
		"updated_at":        "0001-01-01T00:00:00Z",
		"tenant_id":         float64(schoolID),
		"tag_uid":           "ABC123",
		"scanned_at":        "0001-01-01T00:00:00Z",
		"school_id":         float64(schoolID),
		"school_name":       "School",
		"organization_id":   float64(orgID),
		"organization_name": "Organization",
		"device_name":       "Eingang",
	}, body.Data[0])
	require.Equal(t, []int64{schoolID}, scans.listFilter.TenantIDs)
	require.True(t, scans.listFilter.UnresolvedOnly)
	require.Equal(t, 1, surface.adminRuns)
}

func TestListUnregisteredTagScansSchoolOutsideOrganizationMatchesNothing(t *testing.T) {
	t.Parallel()

	scans := &fakeScans{listResult: []devicefleet.UnregisteredTagScan{}}
	surface := &testSurface{}

	rr := serve(t, newReview(scans, &fakeDirectory{}, surface), http.MethodGet, "/?school_id=77&organization_id=10&resolved=all", "")

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.NotNil(t, scans.listFilter.TenantIDs)
	require.Empty(t, scans.listFilter.TenantIDs)
	require.False(t, scans.listFilter.UnresolvedOnly)
	require.Empty(t, decode[listBody](t, rr).Data)
}

func TestListUnregisteredTagScansWithoutFilterReadsEverySchool(t *testing.T) {
	t.Parallel()

	scans := &fakeScans{listResult: []devicefleet.UnregisteredTagScan{}}

	rr := serve(t, newReview(scans, &fakeDirectory{}, &testSurface{}), http.MethodGet, "/", "")

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Nil(t, scans.listFilter.TenantIDs)
	require.True(t, scans.listFilter.UnresolvedOnly)
	require.Equal(t, "[]", string(mustRaw(t, rr)["data"]))
}

func TestListUnregisteredTagScansOrganizationFilterKeepsDeletedSchoolHistory(t *testing.T) {
	t.Parallel()

	organizationID := time.Now().UnixNano()
	schoolID := organizationID + 1
	scans := &fakeScans{listResult: []devicefleet.UnregisteredTagScan{}}
	directory := &fakeDirectory{
		listSchoolsByOrganizationFn: func(_ context.Context, id int64) ([]operator.School, error) {
			require.Equal(t, organizationID, id)
			// The directory lists deleted schools too; the review keeps them.
			return []operator.School{{ID: schoolID, OrganizationID: id}}, nil
		},
	}

	rr := serve(t, newReview(scans, directory, &testSurface{}), http.MethodGet, "/?organization_id="+formatID(organizationID), "")

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Equal(t, []int64{schoolID}, scans.listFilter.TenantIDs)
}

func TestListUnregisteredTagScansOrganizationWithoutSchoolsReturnsEmpty(t *testing.T) {
	t.Parallel()

	scans := &fakeScans{listErr: errors.New("device fleet must not be called")}
	directory := &fakeDirectory{
		listSchoolsByOrganizationFn: func(context.Context, int64) ([]operator.School, error) {
			return []operator.School{}, nil
		},
	}

	rr := serve(t, newReview(scans, directory, &testSurface{}), http.MethodGet, "/?organization_id=12", "")

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.False(t, scans.listCalled)
	require.Equal(t, "[]", string(mustRaw(t, rr)["data"]))
}

func TestListUnregisteredTagScansRejectsInvalidFilters(t *testing.T) {
	t.Parallel()

	for target, message := range map[string]string{
		"/?school_id=abc":        "invalid school ID",
		"/?school_id=0":          "invalid school ID",
		"/?organization_id=-12":  "invalid organization ID",
		"/?organization_id=x12y": "invalid organization ID",
	} {
		scans := &fakeScans{}
		rr := serve(t, newReview(scans, &fakeDirectory{}, &testSurface{}), http.MethodGet, target, "")

		require.Equal(t, http.StatusBadRequest, rr.Code, target)
		require.Equal(t, errorBody{Kind: "invalid", Message: message}, decode[errorBody](t, rr), target)
		require.False(t, scans.listCalled, target)
	}
}

func TestListUnregisteredTagScansFailuresAreInternal(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		scans     *fakeScans
		directory *fakeDirectory
	}{
		"device fleet": {scans: &fakeScans{listErr: errors.New("list failed")}, directory: &fakeDirectory{}},
		"organization": {
			scans: &fakeScans{listResult: []devicefleet.UnregisteredTagScan{{TenantID: 84}}},
			directory: &fakeDirectory{listOrganizationsByIDFn: func(context.Context, []int64) ([]operator.Organization, error) {
				return nil, errors.New("organization query failed")
			}},
		},
		"missing school": {
			scans: &fakeScans{listResult: []devicefleet.UnregisteredTagScan{{TenantID: 84}}},
			directory: &fakeDirectory{listSchoolsByIDFn: func(context.Context, []int64) ([]operator.School, error) {
				return []operator.School{}, nil
			}},
		},
		"missing organization": {
			scans: &fakeScans{listResult: []devicefleet.UnregisteredTagScan{{TenantID: 84}}},
			directory: &fakeDirectory{listOrganizationsByIDFn: func(context.Context, []int64) ([]operator.Organization, error) {
				return []operator.Organization{}, nil
			}},
		},
	}
	for name, tc := range cases {
		rr := serve(t, newReview(tc.scans, tc.directory, &testSurface{}), http.MethodGet, "/", "")

		require.Equal(t, http.StatusInternalServerError, rr.Code, name)
		require.Equal(t, errorBody{Kind: "internal", Message: "Failed to list unregistered RFID scans"}, decode[errorBody](t, rr), name)
	}
}

func TestResolveUnregisteredTagScanPassesOperatorAndTrimmedNote(t *testing.T) {
	t.Parallel()

	resolvedAt := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
	operatorID := int64(42)
	scans := &fakeScans{resolveResult: devicefleet.UnregisteredTagScan{
		ID: 123, TenantID: 84, TagUID: "ABC123", ResolvedAt: &resolvedAt, ResolvedByOperatorID: &operatorID,
	}}
	surface := &testSurface{operatorID: operatorID}

	rr := serve(t, newReview(scans, &fakeDirectory{}, surface), http.MethodPost, "/123/resolve", `{"note":" replacement issued "}`)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Equal(t, int64(123), scans.resolveInput.ID)
	require.Equal(t, operatorID, scans.resolveInput.OperatorID)
	require.NotNil(t, scans.resolveInput.Note)
	require.Equal(t, "replacement issued", *scans.resolveInput.Note)
	require.Equal(t, 1, surface.adminRuns)

	var body struct {
		Data    map[string]any `json:"data"`
		Message string         `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Equal(t, "Unregistered RFID scan resolved successfully", body.Message)
	require.Equal(t, "ABC123", body.Data["tag_uid"])
	require.Equal(t, "2026-09-17T10:00:00Z", body.Data["resolved_at"])
	require.Equal(t, float64(42), body.Data["resolved_by_operator_id"])
	require.Equal(t, float64(84), body.Data["school_id"])
	require.Equal(t, float64(42), body.Data["organization_id"])
}

func TestResolveUnregisteredTagScanBlankNoteIsNoNote(t *testing.T) {
	t.Parallel()

	scans := &fakeScans{resolveResult: devicefleet.UnregisteredTagScan{ID: 123, TenantID: 84}}

	rr := serve(t, newReview(scans, &fakeDirectory{}, &testSurface{operatorID: 42}), http.MethodPost, "/123/resolve", `{"note":"   "}`)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Nil(t, scans.resolveInput.Note)
}

func TestResolveUnregisteredTagScanWithoutOperatorIsBadRequest(t *testing.T) {
	t.Parallel()

	scans := &fakeScans{}
	surface := &testSurface{}

	rr := serve(t, newReview(scans, &fakeDirectory{}, surface), http.MethodPost, "/123/resolve", `{}`)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Equal(t, errorBody{Kind: "invalid", Message: "operator ID is required"}, decode[errorBody](t, rr))
	require.Zero(t, scans.resolveInput.ID)
	require.Zero(t, surface.adminRuns)
}

// wrappedDriverError stands for a failure of the Device Fleet adapter: it
// wraps the raw driver text, which must never reach the operator.
type wrappedDriverError struct {
	op  string
	err error
}

func (e *wrappedDriverError) Error() string { return e.op + ": " + e.err.Error() }
func (e *wrappedDriverError) Unwrap() error { return e.err }

// storeFailureError stands for a retained repository error (StoreFailure),
// which stays internal even when it wraps a refusal text.
type storeFailureError struct{ err error }

func (e *storeFailureError) Error() string      { return "database error during resolve: " + e.err.Error() }
func (e *storeFailureError) Unwrap() error      { return e.err }
func (e *storeFailureError) StoreFailure() bool { return true }

func TestResolveUnregisteredTagScanPersistenceFailuresAreInternal(t *testing.T) {
	t.Parallel()

	rawErr := errors.New("pq: permission denied for audit.unregistered_tag_scans")
	for name, resolveErr := range map[string]error{
		"adapter error with unwrap":       &wrappedDriverError{op: "resolve unregistered tag scan", err: rawErr},
		"fmt wrapped adapter error":       fmt.Errorf("devicefleet postgres: resolve unregistered tag scan: %w", rawErr),
		"raw":                             errors.New("resolve failed"),
		"store failure with refusal text": &storeFailureError{err: errors.New("unregistered tag scan not found")},
	} {
		rr := serve(t, newReview(&fakeScans{resolveErr: resolveErr}, &fakeDirectory{}, &testSurface{operatorID: 42}), http.MethodPost, "/123/resolve", `{}`)

		require.Equal(t, http.StatusInternalServerError, rr.Code, name)
		require.Equal(t, errorBody{Kind: "internal", Message: "Failed to resolve unregistered RFID scan"}, decode[errorBody](t, rr), name)
		require.NotContains(t, rr.Body.String(), rawErr.Error(), name)
		require.NotContains(t, rr.Body.String(), "devicefleet postgres:", name)
	}
}

func TestResolveUnregisteredTagScanKeepsOwnerRefusalsAsBadRequest(t *testing.T) {
	t.Parallel()

	for _, resolveErr := range []error{
		errors.New("operator ID is required"),
		errors.New("scan ID is required"),
		errors.New("unregistered tag scan not found"),
		errors.New("unregistered tag scan already resolved"),
		errors.New("invalid unregistered tag scan"),
		devicefleet.ErrUnregisteredTagScanNotFound,
		devicefleet.ErrUnregisteredTagScanResolved,
		devicefleet.ErrInvalidUnregisteredTagScan,
		fmt.Errorf("resolve: %w", devicefleet.ErrUnregisteredTagScanResolved),
	} {
		scans := &fakeScans{resolveErr: resolveErr}

		rr := serve(t, newReview(scans, &fakeDirectory{}, &testSurface{operatorID: 42}), http.MethodPost, "/123/resolve", `{}`)

		require.Equal(t, http.StatusBadRequest, rr.Code, resolveErr.Error())
		require.Equal(t, errorBody{Kind: "invalid", Message: resolveErr.Error()}, decode[errorBody](t, rr), resolveErr.Error())
		require.Equal(t, int64(123), scans.resolveInput.ID, resolveErr.Error())
	}
}

func TestResolveUnregisteredTagScanRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		target, body, message string
	}{
		{target: "/abc/resolve", body: `{}`, message: "invalid scan ID"},
		{target: "/0/resolve", body: `{}`, message: "invalid scan ID"},
		{target: "/123/resolve", body: `{`, message: "unexpected EOF"},
	} {
		scans := &fakeScans{}
		rr := serve(t, newReview(scans, &fakeDirectory{}, &testSurface{operatorID: 42}), http.MethodPost, tc.target, tc.body)

		require.Equal(t, http.StatusBadRequest, rr.Code, tc.target)
		require.Equal(t, errorBody{Kind: "invalid", Message: tc.message}, decode[errorBody](t, rr), tc.target)
		require.Zero(t, scans.resolveInput.ID, tc.target)
	}
}

func mustRaw(t *testing.T, rr *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()
	return decode[map[string]json.RawMessage](t, rr)
}

func formatID(id int64) string {
	body, _ := json.Marshal(id)
	return string(body)
}
