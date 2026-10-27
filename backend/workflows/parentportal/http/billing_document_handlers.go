package parent

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// Document inbox (#4671, ADR 0126): the documents the Träger of the
// account's schools sent to the account's address. The account comes from
// the claims; the service checks the relationships and the address.

// BillingDocumentResponse is one document in the inbox.
type BillingDocumentResponse struct {
	ID             string        `json:"id"`
	OrganizationID string        `json:"organization_id"`
	TraegerName    string        `json:"traeger_name"`
	Kind           string        `json:"kind"`
	Number         string        `json:"number"`
	PeriodName     string        `json:"period_name"`
	ServiceFrom    timezone.Date `json:"service_from"`
	ServiceUntil   timezone.Date `json:"service_until"`
	IssuedOn       timezone.Date `json:"issued_on"`
	TotalCents     int64         `json:"total_cents"`
	ChildNames     []string      `json:"child_names"`
	SentAt         time.Time     `json:"sent_at"`
}

// listBillingDocuments answers the documents in the inbox of the account.
func (rs *Resource) listBillingDocuments(w http.ResponseWriter, r *http.Request) {
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(sessiontoken.ErrTokenUnauthorized))
		return
	}
	documents, err := rs.ParentService.ListBillingDocuments(r.Context(), int64(claims.ID))
	if err != nil {
		renderBillingDocumentError(w, r, err)
		return
	}
	out := make([]BillingDocumentResponse, len(documents))
	for i, document := range documents {
		out[i] = BillingDocumentResponse{
			ID: strconv.FormatInt(document.ID, 10), OrganizationID: strconv.FormatInt(document.OrganizationID, 10),
			TraegerName: document.TraegerName, Kind: document.Kind, Number: document.Number, PeriodName: document.PeriodName,
			ServiceFrom: document.ServiceFrom, ServiceUntil: document.ServiceUntil, IssuedOn: document.IssuedOn,
			TotalCents: document.TotalCents, ChildNames: document.ChildNames, SentAt: document.SentAt,
		}
	}
	common.Respond(w, r, http.StatusOK, out, "Documents retrieved")
}

// billingDocumentPDF answers the PDF of one document in the inbox.
func (rs *Resource) billingDocumentPDF(w http.ResponseWriter, r *http.Request) {
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
	file, err := rs.ParentService.BillingDocumentLetter(r.Context(), int64(claims.ID), organizationID, documentID)
	if err != nil {
		renderBillingDocumentError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

// renderBillingDocumentError answers an account without a relationship
// that grants the inbox with 403 and a document outside it with 404.
func renderBillingDocumentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, parentService.ErrGuardianPermissionDenied):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareGuardianPermissionDenied))
	case errors.Is(err, parentService.ErrBillingDocumentNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
