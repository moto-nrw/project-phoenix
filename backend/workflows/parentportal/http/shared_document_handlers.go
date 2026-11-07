package parent

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// Documents a Träger released for parents (#4692): a house rule, a letter,
// a list. They stand in the document inbox beside the letters and contracts.
// The account comes from the claims; the service checks the relationships.

// SharedDocumentResponse is one released document in the inbox.
type SharedDocumentResponse struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	TraegerName    string    `json:"traeger_name"`
	Title          string    `json:"title"`
	FileName       string    `json:"file_name"`
	ContentType    string    `json:"content_type"`
	SizeBytes      int       `json:"size_bytes"`
	ReleasedAt     time.Time `json:"released_at"`
}

// listSharedDocuments answers the documents released for the account.
func (rs *Resource) listSharedDocuments(w http.ResponseWriter, r *http.Request) {
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(sessiontoken.ErrTokenUnauthorized))
		return
	}
	documents, err := rs.ParentService.ListSharedDocuments(r.Context(), int64(claims.ID))
	if err != nil {
		renderSharedDocumentError(w, r, err)
		return
	}
	out := make([]SharedDocumentResponse, len(documents))
	for i, document := range documents {
		out[i] = SharedDocumentResponse{
			ID: strconv.FormatInt(document.ID, 10), OrganizationID: strconv.FormatInt(document.OrganizationID, 10),
			TraegerName: document.TraegerName, Title: document.Title, FileName: document.FileName,
			ContentType: document.ContentType, SizeBytes: document.SizeBytes, ReleasedAt: document.ReleasedAt,
		}
	}
	common.Respond(w, r, http.StatusOK, out, "Shared documents retrieved")
}

// sharedDocumentFile answers the file of one released document and records
// the opening.
func (rs *Resource) sharedDocumentFile(w http.ResponseWriter, r *http.Request) {
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(sessiontoken.ErrTokenUnauthorized))
		return
	}
	organizationID, err := common.ParseIDParam(r, "organizationId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	documentID, err := common.ParseIDParam(r, "documentId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	file, err := rs.ParentService.SharedDocumentFile(r.Context(), int64(claims.ID), organizationID, documentID)
	if err != nil {
		renderSharedDocumentError(w, r, err)
		return
	}
	writeDocumentFile(w, file)
}

// renderSharedDocumentError answers an account whose relationships grant
// the portal at no school with 403 and a document no release covers with
// 404.
func renderSharedDocumentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, parentService.ErrGuardianPermissionDenied):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareGuardianPermissionDenied))
	case errors.Is(err, parentService.ErrSharedDocumentNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeTraegerSharedDocumentNotFound))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
