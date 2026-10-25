package traegerportal

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// The Absprung into a school of the Träger (ADR 0084 section 5, #4654). The
// route sits in the Träger route group behind the right "In OGS wechseln";
// Identity & Access re-checks membership, right and the Träger of the school
// in the database and files the Absprung in the school's audit. The answer is
// a short-lived, single-use hand-off token the school's host redeems through
// /auth/refresh into its own OGS session.

var errSchoolSessionsUnavailable = errors.New("the Träger school sessions are not composed")

// schoolSessionRouter returns the route mounted at /traeger/school-sessions.
func (rs *Resource) schoolSessionRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	protectedTraegerRoutes(r, func(r chi.Router, _ common.Middleware) {
		r.With(common.RequiresPermission(identityaccess.TraegerRightSchoolSwitch)).Post("/", rs.startSchoolSession)
	})
	return r
}

// SchoolSessionRequest is the body of POST /traeger/school-sessions.
type SchoolSessionRequest struct {
	SchoolID int64 `json:"school_id"`
}

// Bind validates the request.
func (req *SchoolSessionRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req, validation.Field(&req.SchoolID, validation.Required, validation.Min(int64(1))))
}

// SchoolSessionResponse carries the hand-off token for the school's host.
type SchoolSessionResponse struct {
	HandoffToken string `json:"handoff_token"`
}

func (rs *Resource) startSchoolSession(w http.ResponseWriter, r *http.Request) {
	if rs.schoolSessions == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errSchoolSessionsUnavailable))
		return
	}
	req := &SchoolSessionRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID <= 0 || claims.OrgID <= 0 || !claims.IsOrgScope() {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	token, err := rs.schoolSessions.StartTraegerSchoolSession(r.Context(), identityaccess.TraegerSchoolSessionRequest{
		AccountID: int64(claims.ID), OrganizationID: claims.OrgID, TraegerFamilyID: claims.FamilyID, SchoolID: req.SchoolID,
		IPAddress: common.GetClientIPString(r), UserAgent: r.Header.Get(headerUserAgent),
	})
	if err != nil {
		common.RenderError(w, r, renderSchoolSessionError(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, SchoolSessionResponse{HandoffToken: token}, "School session started")
}

var schoolSessionErrorRules = []common.ErrorRule{
	{Target: identityaccess.ErrTraegerSchoolNotFound, Render: func(err error) render.Renderer {
		return common.ErrorNotFoundWithCode(err, common.CodeTraegerSchoolNotFound)
	}},
	{Target: identityaccess.ErrTraegerSchoolSwitchForbidden, Render: common.ErrorForbidden},
	{Target: identityaccess.ErrNoTraegerMembership, Render: func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(err, common.CodeTraegerNoMembership)
	}},
	// A Träger session without second-factor proof stays valid in the Träger
	// portal; it only cannot open a school. 403 keeps the portal from reading
	// the refusal as an ended session.
	{Target: identityaccess.ErrInvalidToken, Render: func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(errors.New("the Absprung requires a live Träger session with a second factor; sign in again"), common.CodeTraegerSecondFactorRequired)
	}},
	{Target: identityaccess.ErrAccountInactive, Render: func(err error) render.Renderer {
		return common.ErrorUnauthorizedWithCode(err, common.CodeIdentitySessionAccountInactive)
	}},
	{Target: identityaccess.ErrAccountNotFound, Render: common.ErrorUnauthorized},
	{Target: identityaccess.ErrMFAStatusUnavailable, Render: common.ErrorServiceUnavailable},
}

func renderSchoolSessionError(err error) render.Renderer {
	return common.RenderWithRules(err, schoolSessionErrorRules, common.ErrorInternalServer)
}
