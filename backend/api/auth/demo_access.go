package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// DemoAccesses is the Identity & Access capability behind the public demo
// routes (#3462).
type DemoAccesses interface {
	RequestDemoAccess(ctx context.Context, request identityaccess.DemoAccessRequest) (identityaccess.IssuedDemoAccess, error)
	DemoAccessStatus(ctx context.Context, token string) (status, schoolSlug string, err error)
	RedeemDemoAccess(ctx context.Context, token, ipAddress, userAgent string) (string, string, error)
}

// ComposedDemoAccess keeps a capability that was not composed a nil
// interface, so the demo mount fails startup instead of serving it.
func ComposedDemoAccess(capability *identityaccess.DemoAccess) DemoAccesses {
	if capability == nil {
		return nil
	}
	return capability
}

// DemoResource serves the public demo routes. The root mounts it in the demo
// environment only. The token travels in request bodies and the
// Authorization header, never in a URL, so no access log records it.
type DemoResource struct {
	accesses DemoAccesses
	origins  DemoOrigins
}

// DemoOrigins are the frontend origins of the public demo (#3463). A demo
// school, and with it its subdomain, exists only after its seed. So the
// prospect first waits at Waiting, an origin that always exists, and moves to
// School(slug) once the status is ready.
type DemoOrigins struct {
	Waiting string
	School  func(schoolSlug string) string
}

func NewDemoResource(accesses DemoAccesses, origins DemoOrigins) (*DemoResource, error) {
	if accesses == nil || origins.Waiting == "" || origins.School == nil {
		return nil, errors.New("demo routes require the capability and the entry origins")
	}
	origins.Waiting = strings.TrimRight(origins.Waiting, "/")
	return &DemoResource{accesses: accesses, origins: origins}, nil
}

// MountDemoRoutes adds the public demo routes under /demo when appEnv is
// "demo". Everywhere else it does nothing and never calls compose: the
// routes, and the capability behind them, do not exist there.
func MountDemoRoutes(router chi.Router, appEnv string, compose func() (DemoAccesses, error), origins DemoOrigins) error {
	if !strings.EqualFold(strings.TrimSpace(appEnv), "demo") {
		return nil
	}
	accesses, err := compose()
	if err != nil {
		return err
	}
	resource, err := NewDemoResource(accesses, origins)
	if err != nil {
		return err
	}
	router.Mount("/demo", resource.Router())
	return nil
}

func (rs *DemoResource) Router() chi.Router {
	r := chi.NewRouter()
	r.Post("/access-requests", rs.requestAccess)
	r.Get("/access/status", rs.accessStatus)
	r.Post("/access/sessions", rs.createSession)
	return r
}

type demoAccessRequestBody struct {
	Email        string `json:"email"`
	SchoolName   string `json:"school_name"`
	PersonName   string `json:"person_name"`
	ContactOptIn bool   `json:"contact_opt_in"`
	Source       string `json:"src"`
}

func (rs *DemoResource) requestAccess(w http.ResponseWriter, r *http.Request) {
	var body demoAccessRequestBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14)).Decode(&body); err != nil {
		rs.renderError(w, r, identityaccess.ErrDemoAccessInvalid)
		return
	}
	issued, err := rs.accesses.RequestDemoAccess(r.Context(), identityaccess.DemoAccessRequest{
		Email: body.Email, PersonName: body.PersonName, SchoolName: body.SchoolName,
		Source: body.Source, ContactOptIn: body.ContactOptIn,
	})
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	render.Status(r, http.StatusAccepted)
	if issued.Token == "" {
		// The address still has an active demo access: no second school and
		// no entry. The answer does not tell a stranger more than that.
		render.JSON(w, r, map[string]string{})
		return
	}
	// The fragment keeps the token out of every server and proxy log.
	render.JSON(w, r, map[string]string{"entry_url": rs.origins.Waiting + "/demo#token=" + issued.Token})
}

func (rs *DemoResource) accessStatus(w http.ResponseWriter, r *http.Request) {
	token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	status, schoolSlug, err := rs.accesses.DemoAccessStatus(r.Context(), strings.TrimSpace(token))
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	response := map[string]string{"status": status}
	if status == identityaccess.DemoSchoolReady {
		// Only now the school's subdomain resolves.
		response["school_url"] = strings.TrimRight(rs.origins.School(schoolSlug), "/")
	}
	render.JSON(w, r, response)
}

func (rs *DemoResource) createSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&body); err != nil {
		rs.renderError(w, r, identityaccess.ErrDemoAccessUnknown)
		return
	}
	accessToken, refreshToken, err := rs.accesses.RedeemDemoAccess(r.Context(), body.Token, getClientIP(r), r.UserAgent())
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	render.JSON(w, r, TokenResponse{AccessToken: accessToken, RefreshToken: refreshToken})
}

// demoError answers with the status and the stable code the entry page
// branches on.
func demoError(status int, code string) func(error) render.Renderer {
	return func(err error) render.Renderer {
		return &common.ErrResponse{Err: err, HTTPStatusCode: status, Status: "error", ErrorText: err.Error(), Code: code}
	}
}

var demoAccessErrorRules = []common.ErrorRule{
	{Target: identityaccess.ErrDemoAccessInvalid, Render: demoError(http.StatusUnprocessableEntity, "demo_access_invalid")},
	{Target: identityaccess.ErrDemoAccessUnknown, Render: demoError(http.StatusNotFound, "demo_access_unknown")},
	{Target: identityaccess.ErrDemoAccessExpired, Render: demoError(http.StatusGone, "demo_access_expired")},
	{Target: identityaccess.ErrDemoSchoolPreparing, Render: demoError(http.StatusConflict, "demo_school_preparing")},
}

func (rs *DemoResource) renderError(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, common.RenderWithRules(err, demoAccessErrorRules, common.ErrorInternalServer))
}
