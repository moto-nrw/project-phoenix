package api

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/auth"
)

// standingDemoSchoolSlug is the school every demo access enters until a
// prospect gets a demo school of its own. `go run . demo` provisions it.
const standingDemoSchoolSlug = "messe-demo"

// mountDemoAccess adds the public demo routes (#3462); they exist under
// APP_ENV=demo only, and only there the entry origin must resolve.
func mountDemoAccess(router chi.Router, accesses authAPI.DemoAccesses, appEnv, frontendURL, tenantDomain string) error {
	entryBase, err := demoEntryBase(frontendURL, tenantDomain)
	if err != nil {
		// Outside the demo environment the origin is never used.
		entryBase = ""
	}
	compose := func() (authAPI.DemoAccesses, error) {
		if accesses == nil {
			return nil, fmt.Errorf("demo access is not composed")
		}
		return accesses, nil
	}
	return authAPI.MountDemoRoutes(router, appEnv, compose, standingDemoSchoolSlug, entryBase)
}

// demoEntryBase is the demo school's origin: the scheme of FRONTEND_URL and
// the school's subdomain under TENANT_DOMAIN.
func demoEntryBase(frontendURL, tenantDomain string) (string, error) {
	parsed, err := url.Parse(frontendURL)
	tenantDomain = strings.TrimSpace(tenantDomain)
	if err != nil || parsed.Scheme == "" || tenantDomain == "" {
		return "", fmt.Errorf("the demo routes require FRONTEND_URL and TENANT_DOMAIN")
	}
	host := standingDemoSchoolSlug + "." + tenantDomain
	if port := parsed.Port(); port != "" {
		host = net.JoinHostPort(host, port)
	}
	return parsed.Scheme + "://" + host, nil
}
