package operator_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/operator"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
)

type mockUnregisteredTagScanService struct {
	resolveFn func(ctx context.Context, id, operatorID int64, note *string) (*auditModels.UnregisteredTagScan, error)
}

func (m *mockUnregisteredTagScanService) Record(context.Context, string, *int64) error {
	return nil
}

func (m *mockUnregisteredTagScanService) ListForOperator(context.Context, auditModels.UnregisteredTagScanFilter) ([]*auditModels.UnregisteredTagScan, error) {
	return []*auditModels.UnregisteredTagScan{}, nil
}

func (m *mockUnregisteredTagScanService) Resolve(ctx context.Context, id, operatorID int64, note *string) (*auditModels.UnregisteredTagScan, error) {
	if m.resolveFn != nil {
		return m.resolveFn(ctx, id, operatorID, note)
	}
	return &auditModels.UnregisteredTagScan{}, nil
}

func (m *mockUnregisteredTagScanService) DeleteOlderThan(context.Context, int) (int, error) {
	return 0, nil
}

func TestResolveUnregisteredTagScanMapsDatabaseErrorToInternal(t *testing.T) {
	t.Parallel()

	rawErr := errors.New("pq: permission denied for audit.unregistered_tag_scans")
	service := &mockUnregisteredTagScanService{
		resolveFn: func(_ context.Context, id, operatorID int64, note *string) (*auditModels.UnregisteredTagScan, error) {
			require.Equal(t, int64(123), id)
			require.Equal(t, int64(42), operatorID)
			require.NotNil(t, note)
			require.Equal(t, "replacement issued", *note)
			return nil, &modelBase.DatabaseError{Op: "resolve unregistered tag scan", Err: rawErr}
		},
	}
	resource := operator.NewResource(operator.ResourceConfig{
		UnregisteredTagScanService: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/unregistered-tag-scans/123/resolve", bytes.NewBufferString(`{"note":" replacement issued "}`))
	req.Header.Set("Content-Type", "application/json")
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "123")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: 42})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.ResolveUnregisteredTagScan(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	var body struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Equal(t, "error", body.Status)
	require.Equal(t, "Failed to resolve unregistered RFID scan", body.Message)
	require.NotContains(t, rr.Body.String(), rawErr.Error())
}

func TestResolveUnregisteredTagScanKeepsValidationErrorsAsBadRequest(t *testing.T) {
	t.Parallel()

	service := &mockUnregisteredTagScanService{
		resolveFn: func(context.Context, int64, int64, *string) (*auditModels.UnregisteredTagScan, error) {
			return nil, errors.New("operator ID is required")
		},
	}
	resource := operator.NewResource(operator.ResourceConfig{
		UnregisteredTagScanService: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/unregistered-tag-scans/123/resolve", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "123")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.ResolveUnregisteredTagScan(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Contains(t, rr.Body.String(), "operator ID is required")
}

func TestResolveUnregisteredTagScanSuccess(t *testing.T) {
	t.Parallel()

	resolvedAt := time.Now()
	service := &mockUnregisteredTagScanService{
		resolveFn: func(context.Context, int64, int64, *string) (*auditModels.UnregisteredTagScan, error) {
			return &auditModels.UnregisteredTagScan{
				TagUID:     "ABC123",
				ResolvedAt: &resolvedAt,
			}, nil
		},
	}
	resource := operator.NewResource(operator.ResourceConfig{
		UnregisteredTagScanService: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/unregistered-tag-scans/123/resolve", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "123")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: 42})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.ResolveUnregisteredTagScan(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), "Unregistered RFID scan resolved successfully")
}
