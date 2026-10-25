// Package traeger serves the master data of a Träger in the Träger-Portal
// (#4653, ADR 0112): its logo, its contact person for documents and its bank
// accounts.
package traeger

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// logoField is the multipart field of the logo upload.
const logoField = "logo"

// maxLogoBody leaves room for the multipart boundaries and headers around a
// logo of the largest admitted size.
const maxLogoBody = organizationtenancy.TraegerLogoMaxBytes + 64<<10

// logoExtensions are the file names a logo upload may carry; the content is
// checked as well.
var logoExtensions = map[string]bool{".png": true, ".jpg": true, ".jpeg": true}

var (
	errLogoMissing  = errors.New("no logo uploaded")
	errLogoTooLarge = errors.New("the logo exceeds 1 MB")
)

// MasterDataResource serves the master data of the session's Träger. Every
// route sits behind the Träger session and runs in the Träger transaction of
// that Träger (ADR 0105); the Träger comes from the session, never from the
// request. Reading needs the Träger right "Übersicht lesen", changing it
// "Einstellungen".
type MasterDataResource struct {
	masterData    organizationtenancy.TraegerMasterData
	requireRead   common.Middleware
	requireManage common.Middleware
}

// NewMasterDataResource builds the resource. The root binds requireRead and
// requireManage to the rights Identity & Access bundles into the Träger roles
// (ADR 0084 section 4).
func NewMasterDataResource(masterData organizationtenancy.TraegerMasterData, requireRead, requireManage common.Middleware) *MasterDataResource {
	if masterData == nil || requireRead == nil || requireManage == nil {
		panic("organization tenancy traeger: the master data and the right checks are required")
	}
	return &MasterDataResource{masterData: masterData, requireRead: requireRead, requireManage: requireManage}
}

// TraegerRouter returns the routes mounted at /traeger/master-data.
func (rs *MasterDataResource) TraegerRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		read := r.With(rs.requireRead, withTx)
		read.Get("/", rs.profile)
		read.Get("/logo", rs.logo)
		// The upload is read and checked for size before the Träger
		// transaction opens, so a slow client holds no connection.
		r.With(rs.requireManage, readLogoUpload, withTx).Put("/logo", rs.replaceLogo)
		manage := r.With(rs.requireManage, withTx)
		manage.Delete("/logo", rs.removeLogo)
		manage.Put("/contact", rs.updateContact)
		manage.Post("/bank-accounts", rs.createBankAccount)
		manage.Put("/bank-accounts/{id}", rs.updateBankAccount)
		manage.Delete("/bank-accounts/{id}", rs.deleteBankAccount)
	})
	return r
}

// ContactRequest is the body of PUT /traeger/master-data/contact.
type ContactRequest struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

// Bind accepts the body; the Owner validates it.
func (req *ContactRequest) Bind(*http.Request) error { return nil }

// BankAccountRequest is the body of POST /traeger/master-data/bank-accounts
// and PUT /traeger/master-data/bank-accounts/{id}.
type BankAccountRequest struct {
	Label         string `json:"label"`
	AccountHolder string `json:"account_holder"`
	IBAN          string `json:"iban"`
	BIC           string `json:"bic"`
	BankName      string `json:"bank_name"`
	IsDefault     bool   `json:"is_default"`
}

// Bind accepts the body; the Owner validates it.
func (req *BankAccountRequest) Bind(*http.Request) error { return nil }

func (req *BankAccountRequest) input() organizationtenancy.TraegerBankAccountInput {
	return organizationtenancy.TraegerBankAccountInput(*req)
}

// session returns the Träger and the account of the session, or renders 401.
func session(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.OrganizationID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return 0, 0, false
	}
	return principal.OrganizationID(), principal.AccountID(), true
}

func (rs *MasterDataResource) profile(w http.ResponseWriter, r *http.Request) {
	organizationID, accountID, ok := session(w, r)
	if !ok {
		return
	}
	profile, err := rs.masterData.Profile(r.Context(), organizationID, accountID)
	if err != nil {
		renderMasterDataError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, profile, "Träger master data retrieved successfully")
}

// logo serves the stored image. Only PNG and JPEG are ever stored, and the
// type comes from the stored content check, never from the request.
func (rs *MasterDataResource) logo(w http.ResponseWriter, r *http.Request) {
	organizationID, _, ok := session(w, r)
	if !ok {
		return
	}
	logo, err := rs.masterData.LogoImage(r.Context(), organizationID)
	if err != nil {
		renderMasterDataError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", logo.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(logo.Data)))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "inline")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(logo.Data)
}

type logoUploadKey struct{}

// readLogoUpload reads the multipart logo into memory within the size limit
// and refuses a missing file or a file name outside PNG and JPEG. The Owner
// checks the content itself.
func readLogoUpload(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := logoUpload(w, r)
		if err != nil {
			renderLogoUploadError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), logoUploadKey{}, data)))
	})
}

func logoUpload(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLogoBody)
	if err := r.ParseMultipartForm(maxLogoBody); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, errLogoTooLarge
		}
		return nil, errLogoMissing
	}
	file, header, err := r.FormFile(logoField)
	if err != nil {
		return nil, errLogoMissing
	}
	defer common.CloseFile(file)
	defer func() { _ = r.MultipartForm.RemoveAll() }()
	if header.Size > organizationtenancy.TraegerLogoMaxBytes {
		return nil, errLogoTooLarge
	}
	if !logoExtensions[strings.ToLower(filepath.Ext(header.Filename))] {
		return nil, organizationtenancy.ErrTraegerLogoInvalid
	}
	data, err := io.ReadAll(io.LimitReader(file, organizationtenancy.TraegerLogoMaxBytes+1))
	if err != nil {
		return nil, errLogoMissing
	}
	if len(data) > organizationtenancy.TraegerLogoMaxBytes {
		return nil, errLogoTooLarge
	}
	return data, nil
}

func renderLogoUploadError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errLogoTooLarge) {
		common.RenderError(w, r, common.ErrorInvalidOnField(err, common.CodeTraegerLogoTooLarge, logoField))
		return
	}
	common.RenderError(w, r, common.ErrorInvalidOnField(err, common.CodeTraegerLogoInvalid, logoField))
}

func (rs *MasterDataResource) replaceLogo(w http.ResponseWriter, r *http.Request) {
	organizationID, accountID, ok := session(w, r)
	if !ok {
		return
	}
	data, _ := r.Context().Value(logoUploadKey{}).([]byte)
	logo, err := rs.masterData.ReplaceLogo(r.Context(), organizationID, accountID, data)
	if err != nil {
		renderMasterDataError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, logo, "Träger logo saved successfully")
}

func (rs *MasterDataResource) removeLogo(w http.ResponseWriter, r *http.Request) {
	organizationID, accountID, ok := session(w, r)
	if !ok {
		return
	}
	if err := rs.masterData.RemoveLogo(r.Context(), organizationID, accountID); err != nil {
		renderMasterDataError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Träger logo removed successfully")
}

func (rs *MasterDataResource) updateContact(w http.ResponseWriter, r *http.Request) {
	organizationID, accountID, ok := session(w, r)
	if !ok {
		return
	}
	req := &ContactRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	contact, err := rs.masterData.UpdateContact(r.Context(), organizationID, accountID, organizationtenancy.TraegerContact(*req))
	if err != nil {
		renderMasterDataError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, contact, "Träger contact saved successfully")
}

func (rs *MasterDataResource) createBankAccount(w http.ResponseWriter, r *http.Request) {
	organizationID, accountID, ok := session(w, r)
	if !ok {
		return
	}
	req := &BankAccountRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	account, err := rs.masterData.CreateBankAccount(r.Context(), organizationID, accountID, req.input())
	if err != nil {
		renderMasterDataError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, account, "Träger bank account created successfully")
}

func (rs *MasterDataResource) updateBankAccount(w http.ResponseWriter, r *http.Request) {
	organizationID, accountID, ok := session(w, r)
	if !ok {
		return
	}
	bankAccountID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	req := &BankAccountRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	account, err := rs.masterData.UpdateBankAccount(r.Context(), organizationID, accountID, bankAccountID, req.input())
	if err != nil {
		renderMasterDataError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, account, "Träger bank account updated successfully")
}

func (rs *MasterDataResource) deleteBankAccount(w http.ResponseWriter, r *http.Request) {
	organizationID, accountID, ok := session(w, r)
	if !ok {
		return
	}
	bankAccountID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.masterData.DeleteBankAccount(r.Context(), organizationID, accountID, bankAccountID); err != nil {
		renderMasterDataError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Träger bank account deleted successfully")
}

// renderMasterDataError answers a refused master data operation. The Owner
// refuses before it writes, and every other failure answers 500, which rolls
// the Träger transaction back.
func renderMasterDataError(w http.ResponseWriter, r *http.Request, err error) {
	var fields validation.Errors
	switch {
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeTraegerMasterDataInvalid))
	case errors.Is(err, organizationtenancy.ErrTraegerLogoInvalid):
		common.RenderError(w, r, common.ErrorInvalidOnField(err, common.CodeTraegerLogoInvalid, logoField))
	case errors.Is(err, organizationtenancy.ErrTraegerLogoNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeTraegerLogoNotFound))
	case errors.Is(err, organizationtenancy.ErrTraegerBankAccountNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeTraegerBankAccountNotFound))
	case errors.Is(err, organizationtenancy.ErrTraegerBankAccountLabelTaken):
		common.RenderError(w, r, common.ErrorInvalidOnField(err, common.CodeTraegerBankAccountLabelTaken, "label"))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
