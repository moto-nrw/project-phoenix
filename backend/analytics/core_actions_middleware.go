package analytics

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/jwtauth/v5"
)

// Analytics surfaces and roles, the same values the frontend sends
// (frontend/src/lib/analytics-policy.ts).
const (
	SurfaceOGS     = "ogs"
	SurfaceParents = "parents"
	SurfaceSchool  = "school"
	SurfacePublic  = "public"

	RoleAdmin     = "admin"
	RoleStaff     = "staff"
	RoleLehrkraft = "lehrkraft"
	RoleGuardian  = "guardian"
)

// Actor is what analytics may know about who performed a core action: the
// portal, the role, and the school. Never an account or person.
type Actor struct {
	Surface  string
	Role     string
	SchoolID int64 // 0 when the session has no school (parents portal)
}

// CoreActionConfig wires the middleware to the session model. The resolvers
// live with the composition root, which owns token verification.
type CoreActionConfig struct {
	Tracker Tracker
	// RequestActor reads the actor from the verified session of the request.
	RequestActor func(*http.Request) (Actor, bool)
	// SessionActor reads the actor from an access token the response mints
	// (login). It must verify the token and reject MFA interim tokens.
	SessionActor func(accessToken string) (Actor, bool)
}

// sessionBodyLimit caps how much of a session-minting response is kept to
// find its access token; token responses are far smaller.
const sessionBodyLimit = 64 << 10

// CoreActionMiddleware captures the core action of a request (coreActions)
// once the response is 2xx. It runs router-wide: the route pattern is known
// only after routing, so the lookup happens when next has returned. A 4xx or
// 5xx, a route that is not captured, or a request without an actor sends
// nothing. The event carries school_id, surface, role, the deployment, and
// the browser session; never a path parameter or a body value.
func CoreActionMiddleware(cfg CoreActionConfig) func(http.Handler) http.Handler {
	methods := coreActionMethods()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(WithSessionID(r.Context(), r.Header.Get(SessionIDHeader)))
			if cfg.Tracker == nil || !methods[r.Method] {
				next.ServeHTTP(w, r)
				return
			}

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			body := &cappedBuffer{limit: sessionBodyLimit}
			ww.Tee(body)
			next.ServeHTTP(ww, r)

			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			if status < http.StatusOK || status >= http.StatusMultipleChoices {
				return
			}
			action, ok := lookupCoreAction(r)
			if !ok || action.Event == "" {
				return
			}
			actor, ok := resolveActor(cfg, r, action, body)
			if !ok {
				return
			}
			cfg.Tracker.CaptureContext(r.Context(), distinctID(actor), action.Event, action.properties(actor))
		})
	}
}

// VerifiedSessionClaims returns the raw claims of the session token the root
// verifier accepted for r (signature and expiry checked), or false when the
// request carries none. The caller maps them onto its session model.
func VerifiedSessionClaims(r *http.Request) (map[string]any, bool) {
	token, claims, err := jwtauth.FromContext(r.Context())
	if err != nil || token == nil {
		return nil, false
	}
	return claims, true
}

func lookupCoreAction(r *http.Request) (CoreAction, bool) {
	routeCtx := chi.RouteContext(r.Context())
	if routeCtx == nil {
		return CoreAction{}, false
	}
	return coreActionFor(r.Method, routeCtx.RoutePattern())
}

// resolveActor names who acted. A session-minting route takes the minted
// session; a 2xx without one is an interim step (second factor required) and
// counts only where the route names its surface itself. Every other route
// takes the session of the request, or the route's own surface when it is
// public.
func resolveActor(cfg CoreActionConfig, r *http.Request, action CoreAction, body *cappedBuffer) (Actor, bool) {
	if action.Session {
		if actor, ok := mintedSessionActor(cfg, body); ok {
			return actor, true
		}
	} else if cfg.RequestActor != nil {
		if actor, ok := cfg.RequestActor(r); ok {
			return actor, true
		}
	}
	if action.Surface != "" {
		return Actor{Surface: action.Surface}, true
	}
	return Actor{}, false
}

func mintedSessionActor(cfg CoreActionConfig, body *cappedBuffer) (Actor, bool) {
	if cfg.SessionActor == nil || body.overflow {
		return Actor{}, false
	}
	token := accessTokenFromBody(body.Bytes())
	if token == "" {
		return Actor{}, false
	}
	return cfg.SessionActor(token)
}

func (action CoreAction) properties(actor Actor) map[string]any {
	props := map[string]any{"surface": actor.Surface}
	if actor.Role != "" {
		props["role"] = actor.Role
	}
	if actor.SchoolID > 0 {
		props["school_id"] = strconv.FormatInt(actor.SchoolID, 10)
	}
	if action.ExportType != "" {
		props["export_type"] = action.ExportType
	}
	return props
}

// distinctID keys the event without a person: the school, or the surface
// where the session has no school. Person profiles stay off.
func distinctID(actor Actor) string {
	if actor.SchoolID > 0 {
		return "school:" + strconv.FormatInt(actor.SchoolID, 10)
	}
	return "surface:" + actor.Surface
}

// accessTokenFromBody finds the access token of a token response, flat or
// in the {"data": ...} envelope.
func accessTokenFromBody(body []byte) string {
	var response struct {
		AccessToken string `json:"access_token"`
		Data        struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(body), &response); err != nil {
		return ""
	}
	if response.AccessToken != "" {
		return response.AccessToken
	}
	return response.Data.AccessToken
}

// cappedBuffer keeps the first limit bytes of a response and notes when it
// was cut; a cut body is never parsed.
type cappedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.overflow {
		return len(p), nil
	}
	if b.Len()+len(p) > b.limit {
		b.overflow = true
		b.Reset()
		return len(p), nil
	}
	return b.Buffer.Write(p)
}

// normalizePattern maps a runtime route pattern onto the route table's form:
// chi reports a mounted collection root with or without its trailing slash
// depending on the request path.
func normalizePattern(pattern string) []string {
	if trimmed, ok := strings.CutSuffix(pattern, "/"); ok {
		return []string{pattern, trimmed}
	}
	return []string{pattern, pattern + "/"}
}
