package test

import "github.com/moto-nrw/project-phoenix/modules/securityruntime"

// NewTestPassword creates a throwaway credential satisfying the password policy.
// Reuse it within a scenario (or a package fixture) so setup and login agree;
// never use it for assertions about a particular weak or malformed password.
func NewTestPassword() string {
	random, err := securityruntime.RandomLowerAlphanumeric(24)
	if err != nil {
		panic("generate test password: " + err.Error())
	}
	return "Aa1!" + random
}
