package api

import (
	"fmt"
	"net/url"
	"strings"

	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/uptrace/bun"
)

// standingDemoSchoolSlug is the school every demo access enters until a
// prospect gets a demo school of its own. `go run . demo` provisions it.
const standingDemoSchoolSlug = "messe-demo"

// mountDemoAccess adds the public demo routes (#3462); they exist under
// APP_ENV=demo only, and only there the entry origin must resolve.
func mountDemoAccess(api *API, db *bun.DB, appEnv, frontendURL, tenantDomain string) error {
	entryBase, err := demoEntryBase(frontendURL, tenantDomain)
	if err != nil {
		// Outside the demo environment the origin is never used.
		entryBase = ""
	}
	compose := func() (authAPI.DemoAccesses, error) {
		accesses, err := services.NewDemoAccess(db, api.Services.AccountAuthentication(), api.Services.Schools)
		if err != nil {
			return nil, err
		}
		return accesses, nil
	}
	return authAPI.MountDemoRoutes(api.Router, appEnv, compose, standingDemoSchoolSlug, entryBase)
}

// demoEntryBase is the demo school's origin: the scheme of FRONTEND_URL and
// the school's subdomain under TENANT_DOMAIN.
func demoEntryBase(frontendURL, tenantDomain string) (string, error) {
	parsed, err := url.Parse(frontendURL)
	tenantDomain = strings.TrimSpace(tenantDomain)
	if err != nil || parsed.Scheme == "" || tenantDomain == "" {
		return "", fmt.Errorf("the demo routes require FRONTEND_URL and TENANT_DOMAIN")
	}
	return parsed.Scheme + "://" + standingDemoSchoolSlug + "." + tenantDomain, nil
}
