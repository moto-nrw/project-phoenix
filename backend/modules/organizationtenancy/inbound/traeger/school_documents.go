package traeger

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// SchoolRuntime opens the tenant transaction of the session's school.
type SchoolRuntime interface {
	WithinSchool(ctx context.Context, schoolID int64, fn func(context.Context) error) error
}

// SchoolDocumentsResource serves a school the documents its Träger shared
// with it (#4692, spec #4611 user story 99): the documents the Träger
// released for the session's school or for the parents at it, at /api/traeger-documents. The
// OGS-Leitung reads them: the routes need the right to manage the school's
// files, the role that also puts a document into the school's folders.
// The school comes from the session; the database answers only for the
// releases that cover it, and a revoked release ends the access at once.
type SchoolDocumentsResource struct {
	documents     organizationtenancy.SchoolTraegerDocuments
	runtime       SchoolRuntime
	requireLeader common.Middleware
}

// NewSchoolDocumentsResource builds the resource; the root binds requireLeader to
// the school permission files:manage.
func NewSchoolDocumentsResource(documents organizationtenancy.SchoolTraegerDocuments, runtime SchoolRuntime, requireLeader common.Middleware) *SchoolDocumentsResource {
	if documents == nil || runtime == nil || requireLeader == nil {
		panic("organization tenancy school: the documents, the runtime and the right check are required")
	}
	return &SchoolDocumentsResource{documents: documents, runtime: runtime, requireLeader: requireLeader}
}

// Router returns the routes mounted at /api/traeger-documents.
func (rs *SchoolDocumentsResource) Router(protectedTenantRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTenantRoutes(r, func(r chi.Router, _ common.Middleware) {
		leader := r.With(rs.requireLeader)
		leader.Get("/", rs.listSchool)
		leader.Get("/{id}/file", rs.schoolFile)
	})
	return r
}

// schoolSession returns the school and the account of the session, or
// renders 401.
func schoolSession(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.TenantID() <= 0 || principal.AccountID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return 0, 0, false
	}
	return principal.TenantID(), principal.AccountID(), true
}

func (rs *SchoolDocumentsResource) listSchool(w http.ResponseWriter, r *http.Request) {
	schoolID, _, ok := schoolSession(w, r)
	if !ok {
		return
	}
	var documents []organizationtenancy.SharedDocument
	err := rs.runtime.WithinSchool(r.Context(), schoolID, func(txCtx context.Context) error {
		var listErr error
		documents, listErr = rs.documents.SchoolDocuments(txCtx, schoolID)
		return listErr
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, documents, "Träger documents retrieved successfully")
}

// file serves a released document. The read is recorded in the school's
// transaction, which commits before the file is written.
func (rs *SchoolDocumentsResource) schoolFile(w http.ResponseWriter, r *http.Request) {
	schoolID, accountID, ok := schoolSession(w, r)
	if !ok {
		return
	}
	documentID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	var file organizationtenancy.TraegerDocumentFile
	err = rs.runtime.WithinSchool(r.Context(), schoolID, func(txCtx context.Context) error {
		var openErr error
		file, openErr = rs.documents.OpenSchoolDocument(txCtx, schoolID, accountID, documentID)
		return openErr
	})
	switch {
	case errors.Is(err, organizationtenancy.ErrTraegerDocumentNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeTraegerSharedDocumentNotFound))
	case err != nil:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	default:
		WriteDocumentFile(w, file)
	}
}
