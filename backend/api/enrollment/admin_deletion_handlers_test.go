package enrollment

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/stretchr/testify/assert"
)

type deletionServiceStub struct{}

func (deletionServiceStub) PreviewRequest(context.Context, int64) (*enrollmentModels.DeletionImpact, error) {
	return &enrollmentModels.DeletionImpact{}, nil
}

func (deletionServiceStub) PreviewChild(context.Context, int64, int64) (*enrollmentModels.DeletionImpact, error) {
	return &enrollmentModels.DeletionImpact{}, nil
}

func (deletionServiceStub) DeleteRequest(context.Context, int64, int64, string) (*enrollmentModels.DeletionImpact, error) {
	return &enrollmentModels.DeletionImpact{}, nil
}

func (deletionServiceStub) DeleteChild(context.Context, int64, int64, int64, string) (*enrollmentModels.DeletionImpact, error) {
	return &enrollmentModels.DeletionImpact{}, nil
}

func TestAdminEnrollmentDeletionRoutesRequireConfigManage(t *testing.T) {
	rs := &Resource{DeletionService: deletionServiceStub{}}
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.With(authorize.RequiresPermission("config:manage")).Get("/enrollment/admin/requests/{id}/delete-impact", rs.getAdminRequestDeleteImpact)
	router.With(authorize.RequiresPermission("config:manage")).Delete("/enrollment/admin/requests/{id}", rs.deleteAdminRequest)
	router.With(authorize.RequiresPermission("config:manage")).Get("/enrollment/admin/requests/{id}/children/{childId}/delete-impact", rs.getAdminChildDeleteImpact)
	router.With(authorize.RequiresPermission("config:manage")).Delete("/enrollment/admin/requests/{id}/children/{childId}", rs.deleteAdminChild)

	for _, test := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/enrollment/admin/requests/42/delete-impact"},
		{http.MethodDelete, "/enrollment/admin/requests/42"},
		{http.MethodGet, "/enrollment/admin/requests/42/children/7/delete-impact"},
		{http.MethodDelete, "/enrollment/admin/requests/42/children/7"},
	} {
		response := executeAdminJSONWithPermissions(t, router, test.method, test.path, []string{"config:read"})
		assert.Equal(t, http.StatusForbidden, response.Code, "%s %s", test.method, test.path)
	}
}
