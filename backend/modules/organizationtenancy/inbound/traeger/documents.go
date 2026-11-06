package traeger

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/sharedkernel/textnorm"
)

// documentField is the multipart field of the document upload, titleField
// the field of its title.
const (
	documentField = "file"
	titleField    = "title"
)

// maxDocumentBody leaves room for the multipart boundaries, the title and
// the headers around a file of the largest admitted size.
const maxDocumentBody = organizationtenancy.TraegerDocumentMaxBytes + 64<<10

// DocumentsResource serves the documents the Träger shares with parents and
// schools (#4692) at /traeger/documents. Every route sits behind the Träger
// session; the Träger comes from the session, never from the request, and
// the Owner opens the transactions itself. Reading needs the Träger right
// "Übersicht lesen", uploading, releasing and revoking "Anmeldung
// verwalten", the right of the Verwaltung that works with parents and
// schools.
type DocumentsResource struct {
	documents     organizationtenancy.TraegerDocuments
	requireRead   common.Middleware
	requireManage common.Middleware
}

// NewDocumentsResource builds the resource. The root binds requireRead and
// requireManage to the rights Identity & Access bundles into the Träger
// roles (ADR 0084 section 4).
func NewDocumentsResource(documents organizationtenancy.TraegerDocuments, requireRead, requireManage common.Middleware) *DocumentsResource {
	if documents == nil || requireRead == nil || requireManage == nil {
		panic("organization tenancy traeger: the documents and the right checks are required")
	}
	return &DocumentsResource{documents: documents, requireRead: requireRead, requireManage: requireManage}
}

// TraegerRouter returns the routes mounted at /traeger/documents.
func (rs *DocumentsResource) TraegerRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, _ common.Middleware) {
		read := r.With(rs.requireRead)
		read.Get("/", rs.list)
		read.Get("/{id}/file", rs.file)
		manage := r.With(rs.requireManage)
		manage.Post("/", rs.upload)
		manage.Get("/audiences", rs.audiences)
		manage.Post("/{id}/releases", rs.release)
		manage.Delete("/{id}/releases/{releaseId}", rs.revoke)
	})
	return r
}

func (rs *DocumentsResource) list(w http.ResponseWriter, r *http.Request) {
	organizationID, _, ok := session(w, r)
	if !ok {
		return
	}
	documents, err := rs.documents.ListDocuments(r.Context(), organizationID)
	if err != nil {
		renderDocumentError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, documents, "Träger documents retrieved successfully")
}

// upload reads the multipart file through the shared upload check (size,
// extension and content) before the Owner checks it again and stores it.
func (rs *DocumentsResource) upload(w http.ResponseWriter, r *http.Request) {
	organizationID, accountID, ok := session(w, r)
	if !ok {
		return
	}
	uploaded, err := common.ParseOfficeFileWithLimits(w, r, documentField, organizationtenancy.TraegerDocumentMaxBytes, maxDocumentBody)
	if err != nil {
		common.RenderError(w, r, common.ErrorUpload(err, organizationtenancy.TraegerDocumentMaxBytes))
		return
	}
	defer common.CloseFile(uploaded.File)
	defer func() { _ = r.MultipartForm.RemoveAll() }()
	data, err := io.ReadAll(io.LimitReader(uploaded.File, organizationtenancy.TraegerDocumentMaxBytes+1))
	if err != nil {
		common.RenderError(w, r, common.ErrorUpload(common.ErrUploadMissing, organizationtenancy.TraegerDocumentMaxBytes))
		return
	}
	document, err := rs.documents.Upload(r.Context(), organizationID, accountID, organizationtenancy.TraegerDocumentUpload{
		Title: r.FormValue(titleField), FileName: uploaded.Filename, Data: data,
	})
	if err != nil {
		renderDocumentError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, document, "Träger document uploaded successfully")
}

// file serves the stored file as a download. no-store keeps a revoked or
// replaced document out of caches the application does not control.
func (rs *DocumentsResource) file(w http.ResponseWriter, r *http.Request) {
	organizationID, accountID, ok := session(w, r)
	if !ok {
		return
	}
	documentID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	file, err := rs.documents.File(r.Context(), organizationID, accountID, documentID)
	if err != nil {
		renderDocumentError(w, r, err)
		return
	}
	WriteDocumentFile(w, file)
}

// WriteDocumentFile writes a shared document as a download with the headers
// every reader of it gets.
func WriteDocumentFile(w http.ResponseWriter, file organizationtenancy.TraegerDocumentFile) {
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.Header().Set("Content-Disposition", textnorm.ContentDisposition("attachment", file.FileName))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

func (rs *DocumentsResource) audiences(w http.ResponseWriter, r *http.Request) {
	organizationID, _, ok := session(w, r)
	if !ok {
		return
	}
	schools, err := rs.documents.Audiences(r.Context(), organizationID)
	if err != nil {
		renderDocumentError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, schools, "Träger document audiences retrieved successfully")
}

// ReleaseRequest is the body of POST /traeger/documents/{id}/releases.
type ReleaseRequest struct {
	organizationtenancy.TraegerDocumentReleaseInput
}

// Bind accepts the body; the Owner validates it.
func (req *ReleaseRequest) Bind(*http.Request) error { return nil }

func (rs *DocumentsResource) release(w http.ResponseWriter, r *http.Request) {
	organizationID, accountID, ok := session(w, r)
	if !ok {
		return
	}
	documentID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	req := &ReleaseRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	release, err := rs.documents.Release(r.Context(), organizationID, accountID, documentID, req.TraegerDocumentReleaseInput)
	if err != nil {
		renderDocumentError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, release, "Träger document released successfully")
}

func (rs *DocumentsResource) revoke(w http.ResponseWriter, r *http.Request) {
	organizationID, accountID, ok := session(w, r)
	if !ok {
		return
	}
	documentID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	releaseID, err := common.ParseIDParam(r, "releaseId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.documents.Revoke(r.Context(), organizationID, accountID, documentID, releaseID); err != nil {
		renderDocumentError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Träger document release revoked successfully")
}

// renderDocumentError answers a refused document operation; every other
// failure answers 500 and rolls the transaction back.
func renderDocumentError(w http.ResponseWriter, r *http.Request, err error) {
	var fields validation.Errors
	switch {
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidOnField(err, common.CodeTraegerDocumentInvalid, titleField))
	case errors.Is(err, organizationtenancy.ErrTraegerDocumentFileMissing):
		common.RenderError(w, r, common.ErrorUpload(common.ErrUploadMissing, organizationtenancy.TraegerDocumentMaxBytes))
	case errors.Is(err, organizationtenancy.ErrTraegerDocumentTooLarge):
		common.RenderError(w, r, common.ErrorUpload(common.ErrUploadTooLarge, organizationtenancy.TraegerDocumentMaxBytes))
	case errors.Is(err, organizationtenancy.ErrTraegerDocumentFileType):
		common.RenderError(w, r, common.ErrorInvalidOnField(err, common.CodeFilesFileTypeNotAllowed, documentField))
	case errors.Is(err, organizationtenancy.ErrTraegerDocumentNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeTraegerDocumentNotFound))
	case errors.Is(err, organizationtenancy.ErrTraegerDocumentReleaseNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeTraegerDocumentReleaseNotFound))
	case errors.Is(err, organizationtenancy.ErrTraegerDocumentAudienceInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeTraegerDocumentAudienceInvalid))
	case errors.Is(err, organizationtenancy.ErrTraegerDocumentAlreadyReleased):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeTraegerDocumentAlreadyReleased))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
