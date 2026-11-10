package carecontracthttp

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The signature of a contract (#4679): the paper contract of the
// Träger-Büro and the public one-time link of a contract mail.

var signatureErrorRules = []struct {
	err    error
	render func(error) render.Renderer
}{
	{carecontract.ErrSigningLinkInvalid, func(err error) render.Renderer {
		return common.ErrorGoneWithCode(err, common.CodeCareContractSigningLinkInvalid)
	}},
	{carecontract.ErrContractNotSignable, func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeCareContractContractNotSignable)
	}},
	{carecontract.ErrSignatureInvalid, func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeCareContractSignatureInvalid)
	}},
	{carecontract.ErrWithdrawalNoticeRequired, func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeCareContractWithdrawalNoticeRequired)
	}},
	{carecontract.ErrPaperDocumentInvalid, func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeCareContractPaperDocumentInvalid)
	}},
}

// renderSignatureError answers a signature error with its code and every
// other error like the contracts.
func renderSignatureError(w http.ResponseWriter, r *http.Request, err error) {
	for _, rule := range signatureErrorRules {
		if errors.Is(err, rule.err) {
			common.RenderError(w, r, rule.render(err))
			return
		}
	}
	renderError(w, r, err)
}

// paperFormOverhead leaves room for the form fields beside the PDF.
const paperFormOverhead = 64 << 10

// recordPaper stores a contract the parents signed on paper: the PDF in
// "file", the signer in "signer_name" and the day in "signed_on".
func (rs *Resource) recordPaper(w http.ResponseWriter, r *http.Request) {
	reviewer, id, ok := contractKey(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, carecontract.PaperMaxBytes+paperFormOverhead)
	if err := r.ParseMultipartForm(carecontract.PaperMaxBytes + paperFormOverhead); err != nil {
		renderSignatureError(w, r, fmt.Errorf("%w: %w", carecontract.ErrPaperDocumentInvalid, err))
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()
	file, header, err := r.FormFile("file")
	if err != nil {
		renderSignatureError(w, r, fmt.Errorf("%w: %w", carecontract.ErrPaperDocumentInvalid, err))
		return
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, carecontract.PaperMaxBytes+1))
	if err != nil {
		renderSignatureError(w, r, fmt.Errorf("%w: %w", carecontract.ErrPaperDocumentInvalid, err))
		return
	}
	input := carecontract.PaperInput{SignerName: r.FormValue("signer_name"), Filename: header.Filename, PDF: data}
	if raw := strings.TrimSpace(r.FormValue("signed_on")); raw != "" {
		if input.SignedOn, err = calendar.ParseDate(raw); err != nil {
			renderError(w, r, &carecontract.ValidationError{Fields: map[string]string{"signed_on": "invalid"}})
			return
		}
	}
	if err := rs.paper.RecordPaperSignature(r.Context(), reviewer, id, input); err != nil {
		tenant.MarkRollback(r.Context())
		renderSignatureError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (rs *Resource) paperDocument(w http.ResponseWriter, r *http.Request) {
	reviewer, id, ok := contractKey(w, r)
	if !ok {
		return
	}
	file, err := rs.paper.PaperDocument(r.Context(), reviewer, id)
	if err != nil {
		renderSignatureError(w, r, err)
		return
	}
	WriteFile(w, file, "attachment")
}

// SigningResource serves the one-time link of a contract mail at
// /public/care-contract-signing, without an account. The link is the
// capability; every route opens the Träger transaction the link names.
type SigningResource struct {
	signing carecontract.Signing
}

func NewSigningResource(signing carecontract.Signing) *SigningResource {
	if signing == nil {
		panic("care contract http: the signing is required")
	}
	return &SigningResource{signing: signing}
}

// PublicRouter returns the routes of the link. Reading leaves the link
// open; only the POST signs and closes it, so a mail scanner that follows
// the link uses nothing up.
func (rs *SigningResource) PublicRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Get("/{link}", rs.view)
	r.Get("/{link}/document", rs.document)
	r.Post("/{link}/sign", rs.sign)
	r.Get("/{link}/family-children", rs.familyChildren)
	r.Put("/{link}/family-children", rs.stateFamilyChildren)
	return r
}

func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

func (rs *SigningResource) view(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	view, err := rs.signing.LinkSigning(r.Context(), chi.URLParam(r, "link"))
	if err != nil {
		renderSignatureError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, view, "Vertrag geladen")
}

func (rs *SigningResource) document(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	file, err := rs.signing.LinkSigningDocument(r.Context(), chi.URLParam(r, "link"))
	if err != nil {
		renderSignatureError(w, r, err)
		return
	}
	WriteFile(w, file, "inline")
}

// SignRequest is a signature with the confirmation of the withdrawal
// notice.
type SignRequest struct {
	carecontract.SignatureInput
}

func (req *SignRequest) Bind(*http.Request) error { return nil }

func (rs *SigningResource) sign(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	r.Body = http.MaxBytesReader(w, r.Body, 400<<10)
	req := &SignRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	view, err := rs.signing.SignByLink(r.Context(), chi.URLParam(r, "link"), req.SignatureInput)
	if err != nil {
		renderSignatureError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, view, "Vertrag unterschrieben")
}

// FamilyChildrenRequest is the number of children the parents state on the
// link (#6335).
type FamilyChildrenRequest struct {
	Children int `json:"children"`
}

func (req *FamilyChildrenRequest) Bind(*http.Request) error { return nil }

// familyChildren reads the number of the family of the link's contract;
// like the contract it leaves the link open.
func (rs *SigningResource) familyChildren(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	children, err := rs.signing.LinkFamilyChildren(r.Context(), chi.URLParam(r, "link"))
	if err != nil {
		renderSignatureError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, children, "Kinderzahl geladen")
}

// stateFamilyChildren stores the number the parents state on the link.
func (rs *SigningResource) stateFamilyChildren(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)
	req := &FamilyChildrenRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	children, err := rs.signing.StateLinkFamilyChildren(r.Context(), chi.URLParam(r, "link"), req.Children)
	if err != nil {
		renderSignatureError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, children, "Kinderzahl gespeichert")
}
