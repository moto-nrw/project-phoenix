// Package filestore serves the school file storage (#2596) under /api/files:
// folders with a visibility rule, the files inside them, and the share list
// picker. Authority lives in services/filestore; the bytes go through the
// shared document coordinator under files/{tenant_id}/{uuid}.
package filestore

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	apiDocuments "github.com/moto-nrw/project-phoenix/api/common/documents"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	filestoreSvc "github.com/moto-nrw/project-phoenix/services/filestore"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	// maxFile is the advertised upload cap; the multipart body cap adds
	// headroom for boundaries and headers.
	maxFile = 25 * 1024 * 1024
	maxBody = maxFile + 4096
	// storageKind is the storage key prefix for the school file storage.
	storageKind = "files"
)

// Resource is the /api/files resource.
type Resource struct {
	Service filestoreSvc.Service
	DB      *bun.DB
	Logger  *slog.Logger
}

// NewResource wires the file storage resource.
func NewResource(service filestoreSvc.Service, db *bun.DB, logger *slog.Logger) *Resource {
	return &Resource{Service: service, DB: db, Logger: logger}
}

func (rs *Resource) getLogger() *slog.Logger {
	if rs.Logger == nil {
		return slog.Default()
	}
	return rs.Logger
}

// Router mounts the file storage routes.
//
// Reading needs no permission beyond a tenant session: the folder visibility
// decides in the service. Folder management is gated on files:manage at the
// route. Upload and delete are gated in the service (manager, or staff upload
// enabled), because the answer depends on a setting and on who uploaded.
//
// upload + download skip withTx so a slow body or file stream doesn't pin a
// bun pool connection; those handlers open their own short transactions.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	common.ProtectedTenantGroup(r, rs.DB, func(r chi.Router, withTx common.Middleware) {
		manage := authorize.RequiresPermission(permissions.FilesManage)

		r.With(withTx).Get("/folders", rs.listFolders)
		r.With(manage, withTx).Post("/folders", rs.createFolder)
		r.With(manage, withTx).Get("/audience", rs.listAudience)
		r.With(manage, withTx).Put("/folders/{folderId}", rs.updateFolder)
		r.With(manage, withTx).Delete("/folders/{folderId}", rs.deleteFolder)

		r.With(withTx).Get("/folders/{folderId}/files", rs.listFiles)
		r.Post("/folders/{folderId}/files", rs.uploadFile)
		r.Get("/folders/{folderId}/files/{fileId}/download", rs.downloadFile)
		r.With(withTx).Delete("/folders/{folderId}/files/{fileId}", rs.deleteFile)
	})
	return r
}

// coordinator builds the storage coordinator. It is constructed per request
// rather than stored on the Resource so a bare test Resource stays usable.
func (rs *Resource) coordinator() (*apiDocuments.Coordinator, error) {
	backend, err := common.UploadsBackend()
	if err != nil {
		return nil, err
	}
	return &apiDocuments.Coordinator{
		Kind:             storageKind,
		Backend:          backend,
		Store:            rs.Service,
		NewTenantContext: func(tenantID int64) context.Context { return tenant.WithTenantID(context.Background(), tenantID) },
		Logger:           rs.getLogger(),
	}, nil
}

// actorFromRequest builds the service actor from the JWT.
func actorFromRequest(r *http.Request) (filestoreSvc.Actor, error) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		return filestoreSvc.Actor{}, errors.New("invalid token: no account id")
	}
	return filestoreSvc.Actor{
		AccountID:   int64(claims.ID),
		Name:        strings.TrimSpace(strings.TrimSpace(claims.FirstName) + " " + strings.TrimSpace(claims.LastName)),
		Permissions: jwt.PermissionsFromCtx(r.Context()),
	}, nil
}

// renderError maps service errors onto HTTP responses.
func renderError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, filestoreSvc.ErrForbidden):
		common.RenderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, filestoreSvc.ErrInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, filestoreSvc.ErrFolderNameTaken):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "folder_name_taken"))
	case errors.Is(err, filestoreSvc.ErrQuotaExceeded):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "quota_exceeded"))
	case modelBase.IsNoRows(err):
		common.RenderError(w, r, common.ErrorNotFound(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
