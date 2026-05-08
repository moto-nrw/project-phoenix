package e2e

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	contract "github.com/moto-nrw/project-phoenix/e2e/contract"
	"github.com/moto-nrw/project-phoenix/e2e/scenarios"
)

const (
	canonicalBackendURL   = "http://localhost:8081"
	canonicalTenantDomain = "localtest.me"
	canonicalFrontendPort = 3030
)

func canonicalOperatorHost() string {
	return fmt.Sprintf("operator.%s", canonicalTenantDomain)
}

// HostsForScenario returns the hostnames that must resolve locally for the
// canonical E2E scenario. This keeps even the optional /etc/hosts helper tied
// to the Go-owned scenario registry instead of a second bash-only host list.
func HostsForScenario(name string) ([]string, error) {
	definition, ok := scenarios.Lookup(name)
	if !ok {
		return nil, fmt.Errorf("unknown e2e scenario %q", name)
	}

	hosts := []string{
		canonicalTenantDomain,
		fmt.Sprintf("%s.%s", definition.Seed.TenantSlug, canonicalTenantDomain),
		canonicalOperatorHost(),
	}

	if definition.Auth.RequiresSecondaryTenant && definition.Seed.SecondTenant.Slug != "" {
		hosts = append(
			hosts,
			fmt.Sprintf("%s.%s", definition.Seed.SecondTenant.Slug, canonicalTenantDomain),
		)
	}

	return uniqueStrings(hosts), nil
}

func canonicalRuntime() (contract.Runtime, error) {
	nextAuthSecret, err := requireEnv("NEXTAUTH_SECRET")
	if err != nil {
		return contract.Runtime{}, err
	}

	authTrustHostRaw, err := requireEnv("AUTH_TRUST_HOST")
	if err != nil {
		return contract.Runtime{}, err
	}

	authTrustHost, err := strconv.ParseBool(authTrustHostRaw)
	if err != nil {
		return contract.Runtime{}, fmt.Errorf("AUTH_TRUST_HOST must be a boolean: %w", err)
	}

	return contract.Runtime{
		BackendURL:       canonicalBackendURL,
		TenantDomain:     canonicalTenantDomain,
		FrontendPort:     canonicalFrontendPort,
		OperatorHostname: fmt.Sprintf("%s:%d", canonicalOperatorHost(), canonicalFrontendPort),
		NextAuthSecret:   nextAuthSecret,
		AuthTrustHost:    authTrustHost,
	}, nil
}

func requireEnv(key string) (string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return "", fmt.Errorf("%s is not set", key)
	}
	return value, nil
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
