package config

import (
	"context"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Middleware = func(http.Handler) http.Handler

type Access uint8

const (
	AccessRead Access = iota
	AccessWrite
	AccessManage
	AccessReadOrWrite
)

type UploadedFile struct {
	File        io.ReadSeekCloser
	ContentType string
}

type Actor struct {
	TenantID    int64
	AccountID   int64
	Permissions []string
}

// Runtime supplies delivery and request-context mechanics to the isolated
// settings HTTP adapter. Implementations live in the composition root.
type Runtime interface {
	ProtectedTenantGroup(chi.Router, func(chi.Router, Middleware))
	Require(Access) Middleware
	TenantOperation() Middleware
	Actor(context.Context) Actor
	CanEdit(context.Context) bool

	Respond(http.ResponseWriter, *http.Request, int, any, string)
	RespondNoContent(http.ResponseWriter, *http.Request)
	RenderError(http.ResponseWriter, *http.Request, int, error)

	ParseImage(http.ResponseWriter, *http.Request, string, int64) (*UploadedFile, error)
	ParsePDF(http.ResponseWriter, *http.Request, string, int64, int64) (*UploadedFile, error)
	SaveImage(io.Reader, string, string, string) (string, error)
	SavePDF(io.Reader, string, string) (string, error)
	RemoveFile(string)
	ResolveStoredPath(string, string, string) (string, error)
	LegalDocumentReferenced(context.Context, string) (bool, error)
}

type RuntimeDependencies struct {
	Protected              func(chi.Router, func(chi.Router, Middleware))
	Permission             func(Access) Middleware
	TenantGuard            Middleware
	RequestActor           func(context.Context) (int64, int64, []string)
	Editable               func(context.Context) bool
	Success                func(http.ResponseWriter, *http.Request, int, any, string)
	NoContent              func(http.ResponseWriter, *http.Request)
	Failure                func(http.ResponseWriter, *http.Request, int, error)
	ImageUpload            func(http.ResponseWriter, *http.Request, string, int64) (*UploadedFile, error)
	PDFUpload              func(http.ResponseWriter, *http.Request, string, int64, int64) (*UploadedFile, error)
	ImageSave              func(io.Reader, string, string, string) (string, error)
	PDFSave                func(io.Reader, string, string) (string, error)
	FileRemove             func(string)
	StoredPath             func(string, string, string) (string, error)
	LegalDocumentReference func(context.Context, string) (bool, error)
}

type runtime struct{ deps RuntimeDependencies }

func NewRuntime(deps RuntimeDependencies) Runtime { return &runtime{deps: deps} }

func NewActor(tenantID, accountID int64, permissions []string) Actor {
	return Actor{TenantID: tenantID, AccountID: accountID, Permissions: permissions}
}

func (rt *runtime) ProtectedTenantGroup(r chi.Router, fn func(chi.Router, Middleware)) {
	rt.deps.Protected(r, fn)
}
func (rt *runtime) Require(access Access) Middleware { return rt.deps.Permission(access) }
func (rt *runtime) TenantOperation() Middleware      { return rt.deps.TenantGuard }
func (rt *runtime) Actor(ctx context.Context) Actor {
	tenantID, accountID, permissions := rt.deps.RequestActor(ctx)
	return NewActor(tenantID, accountID, permissions)
}
func (rt *runtime) CanEdit(ctx context.Context) bool { return rt.deps.Editable(ctx) }
func (rt *runtime) Respond(w http.ResponseWriter, r *http.Request, status int, data any, message string) {
	rt.deps.Success(w, r, status, data, message)
}
func (rt *runtime) RespondNoContent(w http.ResponseWriter, r *http.Request) {
	rt.deps.NoContent(w, r)
}
func (rt *runtime) RenderError(w http.ResponseWriter, r *http.Request, status int, err error) {
	rt.deps.Failure(w, r, status, err)
}
func (rt *runtime) ParseImage(w http.ResponseWriter, r *http.Request, field string, maxBody int64) (*UploadedFile, error) {
	return rt.deps.ImageUpload(w, r, field, maxBody)
}
func (rt *runtime) ParsePDF(w http.ResponseWriter, r *http.Request, field string, maxFile, maxBody int64) (*UploadedFile, error) {
	return rt.deps.PDFUpload(w, r, field, maxFile, maxBody)
}
func (rt *runtime) SaveImage(file io.Reader, dir, prefix, contentType string) (string, error) {
	return rt.deps.ImageSave(file, dir, prefix, contentType)
}
func (rt *runtime) SavePDF(file io.Reader, dir, prefix string) (string, error) {
	return rt.deps.PDFSave(file, dir, prefix)
}
func (rt *runtime) RemoveFile(path string) { rt.deps.FileRemove(path) }
func (rt *runtime) ResolveStoredPath(publicDir, urlPath, prefix string) (string, error) {
	return rt.deps.StoredPath(publicDir, urlPath, prefix)
}
func (rt *runtime) LegalDocumentReferenced(ctx context.Context, storedURL string) (bool, error) {
	return rt.deps.LegalDocumentReference(ctx, storedURL)
}
