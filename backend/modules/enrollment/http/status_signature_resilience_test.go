package enrollmenthttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errStatusSignatureUnavailable = errors.New("signature ledger unavailable")

type unavailableStatusSignatureService struct {
	RequestService
	statusSignatureCalls int
	signatureProofCalls  int
}

func (s *unavailableStatusSignatureService) GetByStatusToken(context.Context, string) (*Request, []*RequestChild, error) {
	return &Request{}, nil, nil
}

func (s *unavailableStatusSignatureService) EditModeForStatus(context.Context, *Request, []*RequestChild) (string, error) {
	return "", nil
}

func (s *unavailableStatusSignatureService) GuardiansByStatusToken(context.Context, string) ([]*capability.RequestGuardian, error) {
	return nil, nil
}

func (s *unavailableStatusSignatureService) StatusSignature(context.Context, string) (*StatusSignature, error) {
	s.statusSignatureCalls++
	return nil, errStatusSignatureUnavailable
}

func (s *unavailableStatusSignatureService) SignatureProof(context.Context, int64, string) (*SignatureProof, error) {
	s.signatureProofCalls++
	return nil, errStatusSignatureUnavailable
}

func statusSignatureTestContext(t *testing.T, adminTransactions *int, savepointActions *[]tenant.SavepointAction) context.Context {
	t.Helper()
	runtime, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error {
			*adminTransactions++
			return fn(ctx, struct{}{})
		},
		func(_ context.Context, action tenant.SavepointAction) error {
			*savepointActions = append(*savepointActions, action)
			return nil
		},
		func(error) bool { return false },
	)
	require.NoError(t, err)
	return tenant.WithUnitOfWork(context.Background(), runtime)
}

func TestStatusSignatureFailureDoesNotHideJSONStatus(t *testing.T) {
	t.Parallel()
	service := &unavailableStatusSignatureService{}
	rs := &Resource{RequestService: service}
	adminTransactions := 0
	var savepointActions []tenant.SavepointAction

	request := httptest.NewRequest(http.MethodGet, "/requests/token", nil)
	route := chi.NewRouteContext()
	route.URLParams.Add("statusToken", "token")
	request = request.WithContext(context.WithValue(statusSignatureTestContext(t, &adminTransactions, &savepointActions), chi.RouteCtxKey, route))
	response := httptest.NewRecorder()

	rs.getStatus(response, request)

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, 1, service.statusSignatureCalls)
	assert.Zero(t, service.signatureProofCalls, "the JSON status must not load the PDF proof")
	assert.Equal(t, 1, adminTransactions, "the optional signature read stays inside the status transaction")
	assert.Equal(t, []tenant.SavepointAction{
		tenant.CreateSavepoint, tenant.ReleaseSavepoint,
		tenant.CreateSavepoint, tenant.RollbackSavepoint, tenant.ReleaseSavepoint,
	}, savepointActions)
}

func TestStatusSignatureProofFailureStillRejectsPDF(t *testing.T) {
	t.Parallel()
	service := &unavailableStatusSignatureService{}
	rs := &Resource{RequestService: service}
	adminTransactions := 0
	var savepointActions []tenant.SavepointAction

	request := httptest.NewRequest(http.MethodGet, "/requests/token?format=pdf", nil)
	route := chi.NewRouteContext()
	route.URLParams.Add("statusToken", "token")
	request = request.WithContext(context.WithValue(statusSignatureTestContext(t, &adminTransactions, &savepointActions), chi.RouteCtxKey, route))
	response := httptest.NewRecorder()

	rs.getStatus(response, request)

	require.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
	assert.Zero(t, service.statusSignatureCalls, "the PDF loads its complete proof directly")
	assert.Equal(t, 1, service.signatureProofCalls)
	assert.Equal(t, 1, adminTransactions)
	assert.Equal(t, []tenant.SavepointAction{tenant.CreateSavepoint, tenant.ReleaseSavepoint}, savepointActions)
}
