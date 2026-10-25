package traegerportal

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The Träger-Büro account routes (ADR 0084 section 4, #4650). The member
// list and its changes sit in the Träger route group behind the right
// "Konten verwalten"; the invitation link is public and opens the
// acceptance. Identity & Access re-checks the acting person in the database
// on every change.

var errAccountsUnavailable = errors.New("the Träger account management is not composed")

// accountRouter returns the routes mounted at /traeger/accounts.
func (rs *Resource) accountRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router))) chi.Router {
	r := chi.NewRouter()
	protectedTraegerRoutes(r, func(r chi.Router) {
		r.Use(common.RequiresPermission(identityaccess.TraegerRightAccountsManage))
		r.Get("/", rs.listAccounts)
		r.Post("/invitations", rs.inviteAccount)
		r.Delete("/invitations/{invitationId}", rs.revokeInvitation)
		r.Put("/{accountId}/roles", rs.changeRoles)
		r.Delete("/{accountId}", rs.removeMember)
	})
	return r
}

// invitationRouter returns the public routes mounted at /traeger/invitations.
func (rs *Resource) invitationRouter(authRateLimiter func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	if authRateLimiter != nil {
		r.Use(authRateLimiter)
	}
	r.Get("/{token}", rs.previewInvitation)
	r.Post("/{token}/accept", rs.acceptInvitation)
	return r
}

// MemberResponse is a person of the Träger-Büro.
type MemberResponse struct {
	AccountID   int64      `json:"account_id"`
	Email       string     `json:"email"`
	Roles       []string   `json:"roles"`
	ActivatedAt *time.Time `json:"activated_at"`
}

// InvitationResponse is an open invitation.
type InvitationResponse struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Roles     []string  `json:"roles"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// AccountsResponse is the member list of the Träger-Büro.
type AccountsResponse struct {
	Members     []MemberResponse     `json:"members"`
	Invitations []InvitationResponse `json:"invitations"`
}

// InviteRequest is the body of POST /traeger/accounts/invitations.
type InviteRequest struct {
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

// Bind normalizes and validates the request.
func (req *InviteRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
		validation.Field(&req.Roles, validation.Required),
	)
}

// RolesRequest is the body of PUT /traeger/accounts/{accountId}/roles.
type RolesRequest struct {
	Roles []string `json:"roles"`
}

// Bind validates the request.
func (req *RolesRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req, validation.Field(&req.Roles, validation.Required))
}

// InvitationPreviewResponse is the public view of a redeemable invitation.
type InvitationPreviewResponse struct {
	OrganizationName         string    `json:"organization_name"`
	Email                    string    `json:"email"`
	Roles                    []string  `json:"roles"`
	RequiresExistingPassword bool      `json:"requires_existing_password"`
	ExpiresAt                time.Time `json:"expires_at"`
}

// AcceptRequest is the body of POST /traeger/invitations/{token}/accept.
type AcceptRequest struct {
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

// Bind validates the request.
func (req *AcceptRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req, validation.Field(&req.Password, validation.Required))
}

// AcceptResponse names the address the invitee signs in with.
type AcceptResponse struct {
	Email string `json:"email"`
}

func (rs *Resource) listAccounts(w http.ResponseWriter, r *http.Request) {
	actor, ok := rs.actor(w, r)
	if !ok {
		return
	}
	buero, err := rs.accounts.ListTraegerBuero(r.Context(), actor)
	if err != nil {
		common.RenderError(w, r, renderAccountError(err))
		return
	}
	response := AccountsResponse{Members: make([]MemberResponse, 0, len(buero.Members)), Invitations: make([]InvitationResponse, 0, len(buero.Invitations))}
	for _, member := range buero.Members {
		response.Members = append(response.Members, memberResponse(member))
	}
	for _, invitation := range buero.Invitations {
		response.Invitations = append(response.Invitations, invitationResponse(invitation))
	}
	common.Respond(w, r, http.StatusOK, response, "Träger-Büro retrieved successfully")
}

func (rs *Resource) inviteAccount(w http.ResponseWriter, r *http.Request) {
	actor, ok := rs.actor(w, r)
	if !ok {
		return
	}
	req := &InviteRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	invitation, err := rs.accounts.InviteTraegerAccount(r.Context(), actor, req.Email, req.Roles)
	if err != nil {
		common.RenderError(w, r, renderAccountError(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, invitationResponse(invitation), "Invitation sent")
}

func (rs *Resource) revokeInvitation(w http.ResponseWriter, r *http.Request) {
	rs.withTarget(w, r, "invitationId", func(ctx context.Context, actor identityaccess.TraegerActor, id int64) error {
		return rs.accounts.RevokeTraegerInvitation(ctx, actor, id)
	})
}

func (rs *Resource) removeMember(w http.ResponseWriter, r *http.Request) {
	rs.withTarget(w, r, "accountId", func(ctx context.Context, actor identityaccess.TraegerActor, id int64) error {
		return rs.accounts.RemoveTraegerMember(ctx, actor, id)
	})
}

func (rs *Resource) changeRoles(w http.ResponseWriter, r *http.Request) {
	actor, ok := rs.actor(w, r)
	if !ok {
		return
	}
	accountID, err := common.ParseIDParam(r, "accountId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	req := &RolesRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	member, err := rs.accounts.ChangeTraegerRoles(r.Context(), actor, accountID, req.Roles)
	if err != nil {
		common.RenderError(w, r, renderAccountError(err))
		return
	}
	common.Respond(w, r, http.StatusOK, memberResponse(member), "Roles changed")
}

func (rs *Resource) previewInvitation(w http.ResponseWriter, r *http.Request) {
	if rs.accounts == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errAccountsUnavailable))
		return
	}
	preview, err := rs.accounts.PreviewTraegerInvitation(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		common.RenderError(w, r, renderAccountError(err))
		return
	}
	common.Respond(w, r, http.StatusOK, InvitationPreviewResponse{
		OrganizationName: preview.OrganizationName, Email: preview.Email, Roles: preview.Roles,
		RequiresExistingPassword: preview.RequiresExistingPassword, ExpiresAt: preview.ExpiresAt,
	}, "Invitation is valid")
}

func (rs *Resource) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	if rs.accounts == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errAccountsUnavailable))
		return
	}
	req := &AcceptRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	address, err := rs.accounts.AcceptTraegerInvitation(r.Context(), chi.URLParam(r, "token"), identityaccess.TraegerInvitationAcceptance{
		Password: req.Password, ConfirmPassword: req.ConfirmPassword,
	})
	if err != nil {
		common.RenderError(w, r, renderAccountError(err))
		return
	}
	common.Respond(w, r, http.StatusOK, AcceptResponse{Email: address}, "Invitation accepted")
}

// actor reads the acting person from the Träger session.
func (rs *Resource) actor(w http.ResponseWriter, r *http.Request) (identityaccess.TraegerActor, bool) {
	if rs.accounts == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errAccountsUnavailable))
		return identityaccess.TraegerActor{}, false
	}
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.OrganizationID() <= 0 || principal.AccountID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return identityaccess.TraegerActor{}, false
	}
	return identityaccess.TraegerActor{OrganizationID: principal.OrganizationID(), AccountID: principal.AccountID()}, true
}

// withTarget runs a change addressed by an ID in the path and answers 204.
func (rs *Resource) withTarget(w http.ResponseWriter, r *http.Request, param string, change func(context.Context, identityaccess.TraegerActor, int64) error) {
	actor, ok := rs.actor(w, r)
	if !ok {
		return
	}
	id, err := common.ParseIDParam(r, param)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := change(r.Context(), actor, id); err != nil {
		common.RenderError(w, r, renderAccountError(err))
		return
	}
	common.RespondNoContent(w, r)
}

var accountErrorRules = []common.ErrorRule{
	{Target: identityaccess.ErrInvalidTraegerRole, Render: common.ErrorInvalidRequest},
	{Target: identityaccess.ErrTraegerInvalidEmail, Render: func(err error) render.Renderer {
		return common.ErrorInvalidOnField(err, common.CodeGeneralInput, "email")
	}},
	{Target: identityaccess.ErrTraegerAlreadyMember, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeTraegerAlreadyMember)
	}},
	{Target: identityaccess.ErrTraegerLastAdministrator, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeTraegerLastAdministrator)
	}},
	{Target: identityaccess.ErrTraegerMemberNotFound, Render: func(err error) render.Renderer {
		return common.ErrorNotFoundWithCode(err, common.CodeTraegerMemberNotFound)
	}},
	{Target: identityaccess.ErrNoTraegerMembership, Render: func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(err, common.CodeTraegerNoMembership)
	}},
	{Target: identityaccess.ErrTraegerAccountsForbidden, Render: common.ErrorForbidden},
	{Target: identityaccess.ErrInvitationNotFound, Render: func(err error) render.Renderer {
		return common.ErrorNotFoundWithCode(err, common.CodeIdentityInvitationNotFound)
	}},
	{Target: identityaccess.ErrInvitationExpired, Render: func(err error) render.Renderer {
		return common.ErrorGoneWithCode(err, common.CodeIdentityInvitationExpired)
	}},
	{Target: identityaccess.ErrInvitationUsed, Render: func(err error) render.Renderer {
		return common.ErrorGoneWithCode(err, common.CodeIdentityInvitationExpired)
	}},
	{Target: identityaccess.ErrInvalidCredentials, Render: func(err error) render.Renderer {
		return common.ErrorUnauthorizedWithCode(err, common.CodeIdentityInvalidCredentials)
	}},
	{Target: identityaccess.ErrAccountInactive, Render: func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(err, common.CodeIdentityAccountInactive)
	}},
	{Target: identityaccess.ErrPasswordTooWeak, Render: func(err error) render.Renderer {
		return common.ErrorInvalidOnField(err, common.CodeIdentityPasswordTooWeak, "password")
	}},
	{Target: identityaccess.ErrInvitationPasswordMismatch, Render: func(err error) render.Renderer {
		return common.ErrorInvalidOnField(err, common.CodeIdentityPasswordMismatch, "confirm_password")
	}},
}

func renderAccountError(err error) render.Renderer {
	return common.RenderWithRules(err, accountErrorRules, common.ErrorInternalServer)
}

func memberResponse(member identityaccess.TraegerMember) MemberResponse {
	return MemberResponse{AccountID: member.AccountID, Email: member.Email, Roles: member.Roles, ActivatedAt: member.ActivatedAt}
}

func invitationResponse(invitation identityaccess.TraegerInvitation) InvitationResponse {
	return InvitationResponse{ID: invitation.ID, Email: invitation.Email, Roles: invitation.Roles, ExpiresAt: invitation.ExpiresAt, CreatedAt: invitation.CreatedAt}
}
