package carecontracthttp

import (
	"errors"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NoticeResource serves /traeger/contract-notices (#4682): the Träger-Büro
// reads what a contract allows today and records a withdrawal or a
// termination the parents sent by mail or letter. Every route requires the
// right "Verträge verwalten"; the owner opens the Träger transaction itself,
// because the read also writes the school's access log.
type NoticeResource struct {
	notices         carecontract.Notices
	requireContract common.Middleware
}

// NewNoticeResource binds the routes to the owner and the right check.
func NewNoticeResource(notices carecontract.Notices, requireContract common.Middleware) *NoticeResource {
	if notices == nil || requireContract == nil {
		panic("care contract http: the notices and the right check are required")
	}
	return &NoticeResource{notices: notices, requireContract: requireContract}
}

// Router returns the routes mounted at /traeger/contract-notices.
func (rs *NoticeResource) Router(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, _ common.Middleware) {
		open := r.With(rs.requireContract)
		open.Get("/{id}", rs.options)
		open.Post("/{id}", rs.declare)
	})
	return r
}

func (rs *NoticeResource) options(w http.ResponseWriter, r *http.Request) {
	reviewer, id, ok := contractKey(w, r)
	if !ok {
		return
	}
	var receivedOn calendar.Date
	if raw := r.URL.Query().Get("received_on"); raw != "" {
		var err error
		if receivedOn, err = calendar.ParseDate(raw); err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
	}
	options, err := rs.notices.NoticeOptions(r.Context(), reviewer, id, receivedOn)
	if err != nil {
		RenderNoticeError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, options, "Contract notice options retrieved successfully")
}

// NoticeRequest is the body of a declaration.
type NoticeRequest struct {
	carecontract.NoticeInput
}

// Bind accepts the body; the owner checks the declaration.
func (req *NoticeRequest) Bind(*http.Request) error { return nil }

func (rs *NoticeResource) declare(w http.ResponseWriter, r *http.Request) {
	reviewer, id, ok := contractKey(w, r)
	if !ok {
		return
	}
	req := &NoticeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	options, err := rs.notices.DeclareNotice(r.Context(), reviewer, id, req.NoticeInput)
	if err != nil {
		RenderNoticeError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, options, "Contract notice recorded successfully")
}

var noticeErrorRules = []struct {
	err  error
	code string
}{
	{carecontract.ErrContractNotEndable, common.CodeCareContractContractNotEndable},
	{carecontract.ErrWithdrawalClosed, common.CodeCareContractWithdrawalClosed},
	{carecontract.ErrSpecialTerminationClosed, common.CodeCareContractSpecialTerminationClosed},
	{carecontract.ErrNoticeTemplateMissing, common.CodeCareContractNoticeTemplateMissing},
	{carecontract.ErrRecipientMissing, common.CodeCareContractRecipientMissing},
}

// RenderNoticeError answers an error of a declaration with its registered
// code; the parents portal uses it as well.
func RenderNoticeError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *carecontract.ValidationError
	if errors.As(err, &invalid) {
		names := make([]string, 0, len(invalid.Fields))
		for name := range invalid.Fields {
			names = append(names, name)
		}
		sort.Strings(names)
		fields := make([]common.FieldError, len(names))
		for i, name := range names {
			fields[i] = common.FieldError{Field: name, Reason: invalid.Fields[name]}
		}
		response := common.ErrorValidation("the declaration is invalid", fields).(*common.ErrResponse)
		response.Code = common.CodeCareContractContractInputInvalid
		common.RenderError(w, r, response)
		return
	}
	for _, rule := range noticeErrorRules {
		if errors.Is(err, rule.err) {
			common.RenderError(w, r, common.ErrorConflictWithCode(err, rule.code))
			return
		}
	}
	if errors.Is(err, carecontract.ErrInboxDocumentNotFound) {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeCareContractContractNotFound))
		return
	}
	renderError(w, r, err)
}
