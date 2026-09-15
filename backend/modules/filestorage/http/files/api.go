// Package files serves the school file storage (#2596) under /api/files and
// the attachments of Elternmitteilungen (#2890) under
// /api/announcement-attachments and /parent-news-attachments through the
// public File Storage capability (#2707). Multipart parsing and magic-byte
// validation stay here; authority, metadata, bytes and cleanup intents are the
// owner's.
package files

import (
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/modules/filestorage"
	"github.com/uptrace/bun"
)

const (
	// maxFile is the advertised upload cap; the multipart body cap adds
	// headroom for boundaries and headers.
	maxFile = 25 * 1024 * 1024
	maxBody = maxFile + 4096
)

// Resource is the /api/files resource and the attachment routers.
type Resource struct {
	files  filestorage.Capability
	db     *bun.DB
	logger *slog.Logger
}

// NewResource wires the file storage routes over the public capability.
func NewResource(files filestorage.Capability, db *bun.DB, logger *slog.Logger) *Resource {
	if files == nil {
		panic("files HTTP: file storage capability is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Resource{files: files, db: db, logger: logger}
}

// Router mounts the file storage routes.
//
// Reading needs no permission beyond a tenant session: the folder visibility
// decides in the owner. Folder management is gated on files:manage at the
// route. Upload and delete are gated in the owner (manager, or staff upload
// enabled), because the answer depends on a setting and on who uploaded.
//
// upload + download skip withTx so a slow body or file stream doesn't pin a
// bun pool connection; the owner opens its own short transactions.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {
		manage := common.RequiresPermission(permissions.FilesManage)

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

// actorFromRequest builds the capability actor from the JWT.
func actorFromRequest(r *http.Request) (filestorage.Actor, error) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		return filestorage.Actor{}, errors.New("invalid token: no account id")
	}
	return filestorage.Actor{
		AccountID:   int64(claims.ID),
		Name:        strings.TrimSpace(strings.TrimSpace(claims.FirstName) + " " + strings.TrimSpace(claims.LastName)),
		Permissions: jwt.PermissionsFromCtx(r.Context()),
	}, nil
}

// renderError maps capability errors onto HTTP responses.
func renderError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, filestorage.ErrForbidden):
		common.RenderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, filestorage.ErrInvalid), errors.Is(err, filestorage.ErrTenantRequired):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, filestorage.ErrFolderNameTaken):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "folder_name_taken"))
	case errors.Is(err, filestorage.ErrQuotaExceeded):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "quota_exceeded"))
	case errors.Is(err, filestorage.ErrNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, filestorage.ErrObjectNotFound):
		common.RenderError(w, r, common.ErrorNotFound(errors.New("file not found")))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// idString renders a database id the way this API carries every id: a decimal
// string. Ids are bigints and JavaScript has no integer type that holds one;
// JSON.parse rounds anything past 2^53 silently, and a rounded id is a valid
// id for a different row.
func idString(id int64) string {
	return strconv.FormatInt(id, 10)
}

// idStrings renders a share list, empty rather than null so the UI can iterate
// without a guard.
func idStrings(ids []int64) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, idString(id))
	}
	return out
}

// int64IDs unwraps a decoded share list. nil stays nil: "no list sent" and
// "empty list sent" are different requests to the owner.
func int64IDs(ids []common.JSONID) []int64 {
	if ids == nil {
		return nil
	}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.Int64())
	}
	return out
}

// serveContent streams one stored object. ?inline=1 opens PDFs and images in
// the browser instead of downloading; office files ignore it and download
// either way.
//
// The inline response carries a sandboxing CSP: an inline document is
// user-uploaded content rendered on the application's own origin, and
// `sandbox` denies it scripts, forms and same-origin access, so a PDF with
// embedded JavaScript cannot act as the signed-in user.
func (rs *Resource) serveContent(w http.ResponseWriter, r *http.Request, content filestorage.Content) {
	defer func() {
		if err := content.Close(); err != nil {
			rs.logger.Error("document close error", "error", err)
		}
	}()
	inline := r.URL.Query().Get("inline") == "1" && filestorage.InlineViewable(content.ContentType)
	disposition := "attachment"
	if inline {
		disposition = "inline"
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src 'self'; style-src 'unsafe-inline'")
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": content.Filename}))
	w.Header().Set("Content-Type", content.ContentType)
	// no-store: a revoked permission or a deleted document must not be
	// served from a cache the application no longer controls.
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, content.Filename, content.ModTime, io.NewSectionReader(content.Object, 0, content.Object.Size()))
}

// parseUpload reads the multipart file, validates it by magic bytes and
// returns the capability upload plus the closer for the temporary file.
func parseUpload(w http.ResponseWriter, r *http.Request) (filestorage.Upload, func(), error) {
	uploaded, err := common.ParseOfficeFileWithLimits(w, r, "file", maxFile, maxBody)
	if err != nil {
		return filestorage.Upload{}, func() {}, err
	}
	return filestorage.Upload{
		Filename:    uploaded.Filename,
		ContentType: uploaded.ContentType,
		Extension:   common.DocumentFileExtension(uploaded.ContentType),
		Content:     uploaded.File,
	}, func() { common.CloseFile(uploaded.File) }, nil
}
