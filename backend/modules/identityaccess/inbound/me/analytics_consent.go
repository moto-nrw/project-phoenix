package me

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

var (
	errConsentNotOwnSession = errors.New("only the account's own portal session decides on usage analytics")
	errConsentMissing       = errors.New("granted must be true or false")
)

// AnalyticsConsentResource serves the signed-in account's own answer to the
// usage analytics in the browser (§ 25 TDDDG, #4901). Every portal session
// reaches it (OGS, parents, school), so the root mounts it once outside the
// portal routers. Operator and Träger sessions have no portal analytics, and a
// staff-view preview carries someone else's identity: none may read or decide.
type AnalyticsConsentResource struct {
	consent identityaccess.AccountAnalyticsConsent
}

// NewAnalyticsConsentResource builds the consent routes over the account's
// consent capability.
func NewAnalyticsConsentResource(consent identityaccess.AccountAnalyticsConsent) *AnalyticsConsentResource {
	if consent == nil {
		panic("analytics consent resource: the consent capability is required")
	}
	return &AnalyticsConsentResource{consent: consent}
}

// Router mounts GET / and PUT / behind the given session chain, which must
// authenticate the access token and block writes of a preview.
func (res *AnalyticsConsentResource) Router(configureSession func(chi.Router)) chi.Router {
	router := chi.NewRouter()
	configureSession(router)
	router.Get("/", res.get)
	router.Put("/", res.decide)
	return router
}

// analyticsConsentResponse is the wire shape: granted is null while the
// account has not decided.
type analyticsConsentResponse struct {
	Granted *bool `json:"granted"`
}

type analyticsConsentRequest struct {
	Granted *bool `json:"granted"`
}

func (req *analyticsConsentRequest) Bind(*http.Request) error {
	if req.Granted == nil {
		return errConsentMissing
	}
	return nil
}

// ownAccount returns the account of a real portal session.
func ownAccount(r *http.Request) (int64, bool) {
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID <= 0 || claims.IsPlatformScope() || claims.IsOrgScope() || claims.IsReadOnlyPreview() {
		return 0, false
	}
	return int64(claims.ID), true
}

func (res *AnalyticsConsentResource) get(w http.ResponseWriter, r *http.Request) {
	accountID, ok := ownAccount(r)
	if !ok {
		common.RenderError(w, r, common.ErrorForbidden(errConsentNotOwnSession))
		return
	}
	consent, err := res.consent.FindAnalyticsConsent(r.Context(), accountID)
	res.respondConsent(w, r, consent.Granted, err, "Analytics consent retrieved successfully")
}

func (res *AnalyticsConsentResource) decide(w http.ResponseWriter, r *http.Request) {
	accountID, ok := ownAccount(r)
	if !ok {
		common.RenderError(w, r, common.ErrorForbidden(errConsentNotOwnSession))
		return
	}
	req := &analyticsConsentRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	consent, err := res.consent.DecideAnalyticsConsent(r.Context(), accountID, *req.Granted)
	res.respondConsent(w, r, consent.Granted, err, "Analytics consent saved successfully")
}

func (res *AnalyticsConsentResource) respondConsent(w http.ResponseWriter, r *http.Request, granted *bool, err error, message string) {
	switch {
	case errors.Is(err, identityaccess.ErrAccountNotFound):
		common.RenderError(w, r, common.ErrorUnauthorized(err))
	case err != nil:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	default:
		w.Header().Set("Cache-Control", "no-store")
		common.Respond(w, r, http.StatusOK, analyticsConsentResponse{Granted: granted}, message)
	}
}
