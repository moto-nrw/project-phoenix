package e2e

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	contract "github.com/moto-nrw/project-phoenix/e2e/contract"
)

const (
	canonicalBackendURL   = "http://localhost:8081"
	canonicalTenantDomain = "localtest.me"
	canonicalFrontendPort = 3030
)

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
		OperatorHostname: fmt.Sprintf("operator.%s:%d", canonicalTenantDomain, canonicalFrontendPort),
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
