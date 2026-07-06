package emergency

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	emergencyService "github.com/moto-nrw/project-phoenix/services/emergency"
	"github.com/uptrace/bun"
)

type Resource struct {
	EmergencyService *emergencyService.Service
	db               *bun.DB
}

func NewResource(service *emergencyService.Service, db *bun.DB) *Resource {
	return &Resource{
		EmergencyService: service,
		db:               db,
	}
}

func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {

		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Post("/snapshot/export", rs.exportSnapshot)
	})

	return r
}

func (rs *Resource) exportSnapshot(w http.ResponseWriter, r *http.Request) {
	if rs.EmergencyService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("emergency service is not configured")))
		return
	}

	file, err := rs.EmergencyService.RenderSnapshot(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}
