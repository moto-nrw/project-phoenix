package students

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

type fakeOfferingChangeRequestService struct {
	input enrollmentService.DecideOfferingChangeInput
}

func (f *fakeOfferingChangeRequestService) Catalog(context.Context, int64) (*enrollmentService.OfferingChangeCatalog, error) {
	return nil, nil
}

func (f *fakeOfferingChangeRequestService) CatalogAt(context.Context, int64, timezone.Date) (*enrollmentService.OfferingChangeCatalog, error) {
	return nil, nil
}

func (f *fakeOfferingChangeRequestService) GetForStudent(context.Context, int64) (*enrollmentService.OfferingChangeView, error) {
	return nil, nil
}

func (f *fakeOfferingChangeRequestService) Create(context.Context, enrollmentService.CreateOfferingChangeInput) (*enrollmentModels.OfferingChangeRequest, error) {
	return nil, nil
}

func (f *fakeOfferingChangeRequestService) Withdraw(context.Context, int64, int64, int64) error {
	return nil
}

func (f *fakeOfferingChangeRequestService) ListPending(context.Context) ([]*enrollmentService.OfferingChangeView, error) {
	return nil, nil
}

func (f *fakeOfferingChangeRequestService) PendingCount(context.Context) (int, error) {
	return 0, nil
}

func (f *fakeOfferingChangeRequestService) Decide(_ context.Context, input enrollmentService.DecideOfferingChangeInput) error {
	f.input = input
	return nil
}

func (f *fakeOfferingChangeRequestService) EarliestEffectiveFrom(context.Context) (timezone.Date, error) {
	return timezone.Date{}, nil
}

func TestDecideOfferingChangeRequest_UsesReviewerRolesForAudit(t *testing.T) {
	svc := &fakeOfferingChangeRequestService{}
	rs := &Resource{ResourceConfig: ResourceConfig{OfferingChangeService: svc}}
	req := httptest.NewRequest(http.MethodPost, "/offering-change-requests/100/decide", strings.NewReader(`{"approve":true}`))
	claims := jwt.AppClaims{ID: 55, Roles: []string{"group_supervisor", "staff"}}
	req = req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, claims))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("requestId", "100")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	rs.decideOfferingChangeRequest(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(55), svc.input.ReviewedBy)
	assert.Equal(t, "group_supervisor,staff", svc.input.ActorRole)
}
