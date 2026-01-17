package seed

import (
	"fmt"
	"os"
	"strings"
)

// DefaultPasswordEnv is the env var used for seeded account passwords.
const DefaultPasswordEnv = "SEED_DEFAULT_PASSWORD"

// DefaultSeedPassword returns the configured default password for seeded accounts.
func DefaultSeedPassword() (string, error) {
	password := strings.TrimSpace(os.Getenv(DefaultPasswordEnv))
	if password == "" {
		return "", fmt.Errorf("%s environment variable is required for seeding accounts", DefaultPasswordEnv)
	}
	return password, nil
}
