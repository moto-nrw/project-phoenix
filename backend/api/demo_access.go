package api

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/auth"
)

// mountDemoAccess adds the public demo routes (#3462); they exist under
// APP_ENV=demo only, and only there the entry origins must resolve.
func mountDemoAccess(router chi.Router, accesses authAPI.DemoAccesses, appEnv, frontendURL, tenantDomain string) error {
	origins, err := demoOrigins(frontendURL, tenantDomain)
	if err != nil {
		// Outside the demo environment the origins are never used.
		origins = authAPI.DemoOrigins{}
	}
	compose := func() (authAPI.DemoAccesses, error) {
		if accesses == nil {
			return nil, fmt.Errorf("demo access is not composed")
		}
		return accesses, nil
	}
	return authAPI.MountDemoRoutes(router, appEnv, compose, origins)
}

// demoOrigins yields where a prospect waits and where the demo school lives
// (#3463). The school's origin is the scheme of FRONTEND_URL and the school's
// subdomain under TENANT_DOMAIN; it resolves only once the school is seeded.
// Until then the prospect waits at FRONTEND_URL itself.
func demoOrigins(frontendURL, tenantDomain string) (authAPI.DemoOrigins, error) {
	parsed, err := url.Parse(frontendURL)
	tenantDomain = strings.TrimSpace(tenantDomain)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || tenantDomain == "" {
		return authAPI.DemoOrigins{}, fmt.Errorf("the demo routes require FRONTEND_URL and TENANT_DOMAIN")
	}
	return authAPI.DemoOrigins{
		Waiting: parsed.Scheme + "://" + parsed.Host,
		School: func(schoolSlug string) string {
			host := schoolSlug + "." + tenantDomain
			if port := parsed.Port(); port != "" {
				host = net.JoinHostPort(host, port)
			}
			return parsed.Scheme + "://" + host
		},
	}, nil
}
