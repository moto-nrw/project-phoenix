package enrollmenthttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

type uploadOpenStub struct {
	upload *capability.RequestUpload
}

func (s uploadOpenStub) PrepareUpload(context.Context, capability.UploadInput) (*capability.PreparedUpload, error) {
	return nil, errors.New("not implemented")
}

func (s uploadOpenStub) DiscardUpload(context.Context, int64, int64) error {
	return errors.New("not implemented")
}

func (s uploadOpenStub) OpenRequestUpload(context.Context, capability.UploadAccess) (*capability.RequestUpload, error) {
	return s.upload, nil
}

func TestExtensionMatchesContent(t *testing.T) {
	t.Parallel()

	assert.True(t, extensionMatchesContent("Bescheid.PDF", "application/pdf"))
	assert.True(t, extensionMatchesContent("foto.jpeg", "image/jpeg"))
	assert.True(t, extensionMatchesContent("IMG_0001.jpg", "image/jpeg"))
	assert.True(t, extensionMatchesContent("scan.png", "image/png"))
	assert.False(t, extensionMatchesContent("bescheid.png", "application/pdf"), "a renamed PDF is refused")
	assert.False(t, extensionMatchesContent("bescheid", "application/pdf"), "a file without extension is refused")
	assert.False(t, extensionMatchesContent("brief.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"))
}

func TestOpenAdminRequestUpload_StorageFactoryFailureIsServerError(t *testing.T) {
	t.Parallel()

	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "12")
	routeCtx.URLParams.Add("uploadId", "34")
	req := httptest.NewRequest(http.MethodGet, "/admin/requests/12/uploads/34", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	w := httptest.NewRecorder()
	resource := &Resource{
		Uploads: uploadOpenStub{upload: &capability.RequestUpload{
			ID: 34, TenantID: 56, StoredName: "proof.pdf", Filename: "Nachweis.pdf", ContentType: "application/pdf",
		}},
		UploadFiles: func(string) (UploadFileStore, error) { return nil, errors.New("object store unavailable") },
	}

	resource.openAdminRequestUpload(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"general.server"`)
}
