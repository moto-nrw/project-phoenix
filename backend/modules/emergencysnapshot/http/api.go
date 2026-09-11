// Package http is the HTTP adapter of the emergency snapshot projection
// (#2704): it mounts the Notfallliste export under the tenant middleware
// chain and calls exactly one public capability.
package http

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/emergencysnapshot"
)

// Resource serves the emergency snapshot routes.
type Resource struct {
	Snapshot emergencysnapshot.Query
	db       *bun.DB
}

// NewResource binds the routes to the projection. db is the concrete
// database the shared tenant middleware takes.
func NewResource(snapshot emergencysnapshot.Query, db *bun.DB) *Resource {
	return &Resource{Snapshot: snapshot, db: db}
}

// Router mounts the routes.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Post("/snapshot/export", rs.exportSnapshot)
	})

	return r
}

func (rs *Resource) exportSnapshot(w http.ResponseWriter, r *http.Request) {
	if rs.Snapshot == nil {
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("emergency snapshot projection is not configured")))
		return
	}

	file, err := rs.Snapshot.Export(r.Context())
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
