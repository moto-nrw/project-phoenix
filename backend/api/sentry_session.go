package api

import (
	"net/http"
	"strconv"

	"github.com/getsentry/sentry-go"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/tokenprimitives"
)

// SentrySessionContext tags every Sentry event of a request with the session
// it acts for (#3643), a panic event included. It runs after the root
// verifier, which leaves a token only when its signature and expiry hold;
// only a session token counts, MFA interim tokens do not. A request without a session
// carries no user and no session tags.
//
// The data boundary of #3590 holds: user.id is the internal account ID and
// nothing else about the person; names, e-mail and IP stay out.
func SentrySessionContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// sentryhttp bound a hub cloned for this request. The global hub is
		// shared by all requests, so without one nothing is tagged.
		if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
			if claims, ok := verifiedSession(r); ok {
				tagSentrySession(hub.Scope(), claims)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func verifiedSession(r *http.Request) (sessiontoken.AppClaims, bool) {
	claims, ok := sessiontoken.VerifiedAccessClaims(r.Context())
	if !ok || claims.ID == 0 {
		return sessiontoken.AppClaims{}, false
	}
	return claims, true
}

// tagSentrySession maps the token scope to the portal and its coarse role
// with the values of the frontend (frontend/src/lib/sentry-context.ts), so
// both projects filter alike. Role names are school data, so the OGS portal
// reports only carrier, admin or staff; the read-only staff preview (#2893)
// is an admin at work. Only OGS and school tokens are bound to one school:
// parent tokens carry none (a guardian can have children in several) and
// operator tokens are platform-wide. There school_id is absent, never a
// placeholder.
func tagSentrySession(scope *sentry.Scope, claims sessiontoken.AppClaims) {
	portal, role, schoolID := "tenant", tenantRole(claims), claims.TenantID
	if claimed, ok := tokenprimitives.PortalOfClaim(claims.Scope); ok {
		if tag, tagged := sentryPortalTags[claimed.Name]; tagged {
			portal, role = tag.portal, tag.role
			if !tag.bindsSchool {
				schoolID = 0
			}
		}
	}

	scope.SetUser(sentry.User{ID: strconv.Itoa(claims.ID)})
	scope.SetTag("portal", portal)
	scope.SetTag("role", role)
	if schoolID > 0 {
		scope.SetTag("school_id", strconv.FormatInt(schoolID, 10))
	}
}

// sentryPortalTags are the portals with tags of their own; every other
// session reports as the OGS portal.
var sentryPortalTags = map[string]struct {
	portal, role string
	bindsSchool  bool
}{
	tokenprimitives.PortalPlatform: {portal: "operator", role: "operator"},
	tokenprimitives.PortalParent:   {portal: "parent", role: "guardian"},
	tokenprimitives.PortalCaterer:  {portal: "caterer", role: "caterer"},
	tokenprimitives.PortalSchool:   {portal: "school", role: "lehrkraft", bindsSchool: true},
}

func tenantRole(claims sessiontoken.AppClaims) string {
	switch {
	case claims.IsOrgScope():
		return "carrier"
	case claims.IsAdmin || claims.IsReadOnlyPreview():
		return "admin"
	default:
		return "staff"
	}
}
